package namespace

import (
	"fmt"
	"strconv"

	danav1 "github.com/dana-team/hns/api/v1"
	"github.com/dana-team/hns/internal/namespace/nsutils"
	"github.com/dana-team/hns/internal/objectcontext"
	"github.com/dana-team/hns/internal/rolebinding/rbutils"
	"github.com/dana-team/hns/internal/subnamespace/resourcepool"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// sync is being called every time there is an update in the namespace and makes sure its role is up-to-date.
func (r *NamespaceReconciler) sync(nsObject *objectcontext.ObjectContext) error {
	ctx := nsObject.Ctx
	logger := log.FromContext(ctx)
	logger.Info("syncing namespace")

	nsName := nsObject.Name()

	if err := rbutils.CreateHNSView(nsObject); err != nil {
		return fmt.Errorf("failed to create role and roleBinding objects associated with namespace %q: "+err.Error(), nsName)
	}
	logger.Info("successfully created role and roleBinding objects associated with namespace", "namespace", nsName)

	if nsutils.IsChildless(nsObject) {
		if err := updateNSRole(nsObject, danav1.Leaf); err != nil {
			return fmt.Errorf("failed to update role of namespace %q: "+err.Error(), nsName)
		}
	} else if err := updateNSRole(nsObject, danav1.NoRole); err != nil {
		return fmt.Errorf("failed to update role of namespace %q: "+err.Error(), nsName)
	}
	logger.Info("successfully updated role of namespace", "namespace", nsName)

	if err := ensureHierarchyLabels(nsObject); err != nil {
		return fmt.Errorf("failed to set hierarchy labels of namespace %q: "+err.Error(), nsName)
	}
	logger.Info("successfully set hierarchy labels of namespace", "namespace", nsName)

	if err := ensureChildrenSNSResourcePoolLabel(nsObject); err != nil {
		return fmt.Errorf("failed to set ResourcePool labels of children subnamespaces of namespace %q: "+err.Error(), nsName)
	}
	logger.Info("successfully set ResourcePool labels of children subnamespaces of namespace", "namespace", nsName)

	return nil
}

// updateNSRole updates the role of a namespace.
func updateNSRole(namespace *objectcontext.ObjectContext, role string) error {
	return namespace.UpdateObject(func(object client.Object, log logr.Logger) (client.Object, logr.Logger) {
		log = log.WithValues("updated role annotation", role)
		object.(*corev1.Namespace).Labels[danav1.Role] = role
		object.(*corev1.Namespace).Annotations[danav1.Role] = role
		return object, log
	})
}

// ensureHierarchyLabels makes sure that the hierarchy labels of a namespace are set correctly
func ensureHierarchyLabels(nsObject *objectcontext.ObjectContext) error {
	snsParentName := nsObject.Object.GetLabels()[danav1.Parent]

	parentNS, err := objectcontext.New(nsObject.Ctx, nsObject.Client, types.NamespacedName{Name: snsParentName}, &corev1.Namespace{})
	if err != nil {
		return err
	}

	labels := nsutils.LabelsBasedOnParent(parentNS, nsObject.Name())
	if err := nsObject.AppendLabels(labels); err != nil {
		return err
	}

	return nil
}

// ensureChildrenSNSResourcePoolLabel makes sure that the ResourcePool label on the children
// subnamespaces of a namespace are set correctly, according to the namespace.
func ensureChildrenSNSResourcePoolLabel(nsObject *objectcontext.ObjectContext) error {
	nsName := nsObject.Name()

	snsList, err := objectcontext.NewList(nsObject.Ctx, nsObject.Client, &danav1.SubnamespaceList{}, client.InNamespace(nsName))
	if err != nil {
		return err
	}

	isNSResourcePool, err := resourcepool.IsNSResourcePool(nsObject)
	if err != nil {
		return err
	}

	for _, sns := range snsList.Objects.(*danav1.SubnamespaceList).Items {
		if err := updateChildResourcePoolLabel(nsObject, sns, isNSResourcePool); err != nil {
			return err
		}
	}

	return nil
}

// updateChildResourcePoolLabel checks  if the child subnamespace is not an upper-rp and
// is not the same type as its parent (for instance one is a ResourcePool and the other is not),
// and then sets the type of the child subnamespace to be the same as its parent.
func updateChildResourcePoolLabel(nsObject *objectcontext.ObjectContext, sns danav1.Subnamespace, isNSResourcePool bool) error {
	nsName := nsObject.Name()
	snsName := sns.GetName()

	snsObj, err := objectcontext.New(nsObject.Ctx, nsObject.Client, types.NamespacedName{Namespace: nsName, Name: snsName}, &danav1.Subnamespace{})
	if err != nil {
		return err
	}

	isSNSUpperResourcePool, err := resourcepool.IsSNSUpper(snsObj)
	if err != nil {
		return err
	}

	isOldSNSResourcePool, err := resourcepool.IsSNSResourcePool(snsObj.Object)
	if err != nil {
		return err
	}

	if isOldSNSResourcePool != isNSResourcePool && !isSNSUpperResourcePool {
		if err := snsObj.UpdateObject(func(object client.Object, log logr.Logger) (client.Object, logr.Logger) {
			log = log.WithValues(danav1.ResourcePool, isNSResourcePool)
			object.SetLabels(map[string]string{danav1.ResourcePool: strconv.FormatBool(isNSResourcePool)})
			return object, log
		}); err != nil {
			return err
		}
	}

	return nil
}
