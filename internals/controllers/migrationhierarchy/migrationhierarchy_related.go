package controllers

import (
	"context"
	"fmt"
	danav1 "github.com/dana-team/hns/api/v1"
	"github.com/dana-team/hns/internals/utils"
	"github.com/go-logr/logr"
	quotav1 "github.com/openshift/api/quota/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/util/retry"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"strconv"
)

// updateRelatedObjects handles the update of objects related to the
// migrated subnamesapce such as its namespace and its children
func (r *MigrationHierarchyReconciler) updateRelatedObjects(mhObject, toNS, ns *utils.ObjectContext) error {
	ctx := mhObject.Ctx

	if er := r.UpdateNSBasedOnParent(ctx, toNS, ns); er != nil {
		err := r.updateMHStatus(mhObject, danav1.Error, er.Error())
		if err != nil {
			return fmt.Errorf("failed updating the status of object '%s': "+err.Error(), mhObject.GetName())
		}
		return fmt.Errorf("failed updating the labels and annotations of namespace '%s' according to its parent '%s': "+err.Error(), ns.GetName(), toNS.GetName())
	}

	if er := r.UpdateAllNSChildrenOfNs(ctx, ns); er != nil {
		err := r.updateMHStatus(mhObject, danav1.Error, er.Error())
		if err != nil {
			return fmt.Errorf("failed updating the status of object '%s': "+err.Error(), mhObject.GetName())
		}
		return fmt.Errorf("failed updating labels and annotations of child namespaces of sunamespace '%s': "+err.Error(), ns.GetName())
	}

	if er := r.updateRole(toNS, danav1.NoRole); er != nil {
		err := r.updateMHStatus(mhObject, danav1.Error, er.Error())
		if err != nil {
			return fmt.Errorf("failed updating the status of object '%s': "+err.Error(), mhObject.GetName())
		}
		return fmt.Errorf("failed updating role of subnamespace '%s': "+err.Error(), toNS.GetName())
	}

	return nil
}

// UpdateAllNSChildrenOfNs updates all the children namespaces of a parent namespace recursively
func (r *MigrationHierarchyReconciler) UpdateAllNSChildrenOfNs(ctx context.Context, parentNS *utils.ObjectContext) error {
	snsChildren, err := utils.NewObjectContextList(ctx, parentNS.Client, &danav1.SubnamespaceList{}, client.InNamespace(parentNS.Object.GetName()))
	if err != nil {
		return err
	}

	for _, sns := range snsChildren.Objects.(*danav1.SubnamespaceList).Items {
		ns, _ := utils.NewObjectContext(ctx, parentNS.Client, types.NamespacedName{Name: sns.GetName()}, &corev1.Namespace{})
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
// based on its parent labels and annotations
func (r *MigrationHierarchyReconciler) UpdateNSBasedOnParent(ctx context.Context, parentNS, childNS *utils.ObjectContext) error {
	nsName := childNS.Object.GetName()
	labels, annotations := utils.GetNSLabelsAnnotationsBasedOnParent(parentNS, nsName)

	if err := childNS.AppendAnnotations(annotations); err != nil {
		return err
	}

	if err := childNS.AppendLabels(labels); err != nil {
		return err
	}

	// update the ClusterResourceQuota AnnotationSelector if needed
	isChildNSResourcePool, err := utils.IsNamespaceResourcePool(childNS)
	if err != nil {
		return err
	}

	isChildNSUpperResourcePool, err := utils.IsNSUpperResourcePool(childNS)
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

// updateCRQSelector updates the ClusterResourceQuota selector of a namespace
func (r *MigrationHierarchyReconciler) updateCRQSelector(childNS, parentNS *utils.ObjectContext, nsName string) error {
	ctx := childNS.Ctx

	crq := quotav1.ClusterResourceQuota{}
	if err := r.Client.Get(ctx, types.NamespacedName{Name: childNS.GetName()}, &crq); err != nil {
		if errors.IsNotFound(err) {
			return nil
		}
		return err
	}

	crqAnnotation := make(map[string]string)
	childNamespaceDepth := strconv.Itoa(utils.GetNamespaceDepth(parentNS.Object) + 1)

	crqAnnotation[danav1.CrqSelector+"-"+childNamespaceDepth] = nsName
	crq.Spec.Selector.AnnotationSelector = crqAnnotation

	// Use retry on conflict to update the CRQ
	err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		updateErr := r.Client.Update(ctx, &crq)
		if errors.IsConflict(updateErr) {
			// Conflict occurred, let's re-fetch the latest version of CRQ and retry the update
			if getErr := r.Client.Get(ctx, types.NamespacedName{Name: childNS.GetName()}, &crq); getErr != nil {
				return getErr
			}
		}
		return updateErr
	})

	return err
}

// updateRole updates the role of a subnamespace
func (r *MigrationHierarchyReconciler) updateRole(sns *utils.ObjectContext, role string) error {
	err := sns.UpdateObject(func(object client.Object, l logr.Logger) (client.Object, logr.Logger) {
		object.(*corev1.Namespace).GetLabels()[danav1.Role] = role
		return object, l
	})

	return err
}
