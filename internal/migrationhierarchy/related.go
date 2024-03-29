package migrationhierarchy

import (
	"context"
	"fmt"
	"strconv"

	danav1 "github.com/dana-team/hns/api/v1"
	"github.com/dana-team/hns/internal/namespace/nsutils"
	"github.com/dana-team/hns/internal/objectcontext"
	"github.com/dana-team/hns/internal/subnamespace/resourcepool"
	"github.com/go-logr/logr"
	quotav1 "github.com/openshift/api/quota/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// updateRelatedObjects handles the update of objects related to the
// migrated subnamesapce such as its namespace and its children.
func (r *MigrationHierarchyReconciler) updateRelatedObjects(mhObject, toNS, ns *objectcontext.ObjectContext) error {
	ctx := mhObject.Ctx

	if er := r.UpdateNSBasedOnParent(ctx, toNS, ns); er != nil {
		err := r.updateMHStatus(mhObject, danav1.Error, er.Error())
		if err != nil {
			return fmt.Errorf("failed updating the status of object %q: "+err.Error(), mhObject.Name())
		}
		return fmt.Errorf("failed updating the labels and annotations of namespace %q according to its parent %q: "+er.Error(), ns.Name(), toNS.Name())
	}

	if er := r.UpdateAllNSChildrenOfNs(ctx, ns); er != nil {
		err := r.updateMHStatus(mhObject, danav1.Error, er.Error())
		if err != nil {
			return fmt.Errorf("failed updating the status of object %q: "+err.Error(), mhObject.Name())
		}
		return fmt.Errorf("failed updating labels and annotations of child namespaces of sunamespace %q: "+er.Error(), ns.Name())
	}

	if er := r.updateRole(toNS, danav1.NoRole); er != nil {
		err := r.updateMHStatus(mhObject, danav1.Error, er.Error())
		if err != nil {
			return fmt.Errorf("failed updating the status of object %q: "+err.Error(), mhObject.Name())
		}
		return fmt.Errorf("failed updating role of subnamespace %q: "+er.Error(), toNS.Name())
	}

	return nil
}

// UpdateAllNSChildrenOfNs updates all the children namespaces of a parent namespace recursively.
func (r *MigrationHierarchyReconciler) UpdateAllNSChildrenOfNs(ctx context.Context, parentNS *objectcontext.ObjectContext) error {
	snsChildren, err := objectcontext.NewList(ctx, parentNS.Client, &danav1.SubnamespaceList{}, client.InNamespace(parentNS.Name()))
	if err != nil {
		return err
	}

	for _, sns := range snsChildren.Objects.(*danav1.SubnamespaceList).Items {
		ns, _ := objectcontext.New(ctx, parentNS.Client, types.NamespacedName{Name: sns.GetName()}, &corev1.Namespace{})
		if err := r.UpdateNSBasedOnParent(ctx, parentNS, ns); err != nil {
			return err
		}

		if err = r.UpdateAllNSChildrenOfNs(ctx, ns); err != nil {
			return err
		}
	}

	return nil
}

// UpdateNSBasedOnParent updates the labels and annotations of a namespace
// based on its parent labels and annotations.
func (r *MigrationHierarchyReconciler) UpdateNSBasedOnParent(ctx context.Context, parentNS, childNS *objectcontext.ObjectContext) error {
	nsName := childNS.Name()
	labels := nsutils.LabelsBasedOnParent(parentNS, nsName)
	annotations := nsutils.AnnotationsBasedOnParent(parentNS, nsName)

	if err := childNS.AppendAnnotations(annotations); err != nil {
		return err
	}

	if err := childNS.AppendLabels(labels); err != nil {
		return err
	}

	// update the ClusterResourceQuota AnnotationSelector if needed
	isChildNSResourcePool, err := resourcepool.IsNSResourcePool(childNS)
	if err != nil {
		return err
	}

	isChildNSUpperResourcePool, err := resourcepool.IsNSUpperResourcePool(childNS)
	if err != nil {
		return err
	}

	if !isChildNSResourcePool || isChildNSUpperResourcePool {
		if err := r.updateCRQSelector(childNS, parentNS, nsName); err != nil {
			return err
		}
	}

	// verify that the update succeeded before continuing since
	// the updates need to be serial
	if err := ensureSnsEqualAnnotations(ctx, parentNS, childNS, annotations); err != nil {
		return err
	}
	if err := ensureSnsEqualLabels(ctx, parentNS, childNS, labels); err != nil {
		return err
	}

	return nil
}

// updateCRQSelector updates the ClusterResourceQuota selector of a namespace.
func (r *MigrationHierarchyReconciler) updateCRQSelector(childNS, parentNS *objectcontext.ObjectContext, nsName string) error {
	ctx := childNS.Ctx

	crq := quotav1.ClusterResourceQuota{}
	crqAnnotation := make(map[string]string)
	childNamespaceDepth := strconv.Itoa(nsutils.Depth(parentNS.Object) + 1)
	crqAnnotation[danav1.CrqSelector+"-"+childNamespaceDepth] = nsName

	// use retry on conflict to update the CRQ
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		if err := r.Client.Get(ctx, types.NamespacedName{Name: childNS.Name()}, &crq); err != nil {
			if errors.IsNotFound(err) {
				return nil
			}
			return err
		}

		crq.Spec.Selector.AnnotationSelector = crqAnnotation
		if err := r.Client.Update(ctx, &crq); err != nil {
			return err
		}

		return nil
	})

	return err
}

// updateRole updates the role of a subnamespace.
func (r *MigrationHierarchyReconciler) updateRole(sns *objectcontext.ObjectContext, role string) error {
	err := sns.UpdateObject(func(object client.Object, l logr.Logger) (client.Object, logr.Logger) {
		object.(*corev1.Namespace).GetLabels()[danav1.Role] = role
		return object, l
	})

	return err
}
