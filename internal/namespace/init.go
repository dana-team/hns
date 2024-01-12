package namespace

import (
	"fmt"
	danav1 "github.com/dana-team/hns/api/v1"
	"github.com/dana-team/hns/internal/namespace/nsutils"
	"github.com/dana-team/hns/internal/objectcontext"
	"github.com/dana-team/hns/internal/rolebinding/rbutils"
	"github.com/go-logr/logr"
	rbacv1 "k8s.io/api/rbac/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// init takes care of initializing related operations that need to be done when
// a namespace is reconciled for the first time.
func (r *NamespaceReconciler) init(nsObject *objectcontext.ObjectContext) error {
	ctx := nsObject.Ctx
	logger := log.FromContext(ctx)
	logger.Info("initializing namespace")

	nsName := nsObject.Name()

	if err := addNSFinalizer(nsObject); err != nil {
		return fmt.Errorf("failed to add finalizer for namespace %q: "+err.Error(), nsName)
	}
	logger.Info("successfully added finalizer of namespace", "namespace", nsName)

	if err := rbutils.CreateHNSView(nsObject); err != nil {
		return fmt.Errorf("failed to create role and roleBinding objects associated with namespace %q: "+err.Error(), nsName)
	}
	logger.Info("successfully created role and roleBinding objects associated with namespace", "namespace", nsName)

	if err := createParentRoleBindingsInNS(nsObject); err != nil {
		return fmt.Errorf("failed to create parent roleBindings objects in namespace %q: "+err.Error(), nsName)
	}
	logger.Info("successfully created parent roleBinding objects in namespace", "namespace", nsName)

	return nil
}

// addNSFinalizer adds the HNS finalizer to a namespace.
func addNSFinalizer(nsObject *objectcontext.ObjectContext) error {
	return nsObject.UpdateObject(func(object client.Object, log logr.Logger) (client.Object, logr.Logger) {
		log = log.WithValues("added namespace finalizer", danav1.NsFinalizer)
		controllerutil.AddFinalizer(object, danav1.NsFinalizer)
		return object, log
	})
}

// createParentRoleBindingsInNS creates the role bindings that exist in the parent namespace
// of the reconciled namespace, in that namespace. This ensures that if a user has permissions on the
// parent of a namespace then the user would also have permissions on the child namespace.
func createParentRoleBindingsInNS(nsObject *objectcontext.ObjectContext) error {
	nsParentName := nsutils.Parent(nsObject.Object)

	roleBindingList, err := objectcontext.NewList(nsObject.Ctx, nsObject.Client, &rbacv1.RoleBindingList{}, client.InNamespace(nsParentName), client.MatchingFields{"rb.propagate": "true"})
	if err != nil {
		return err
	}

	// loop through each role binding in the list and create this role binding in the reconciled namespace
	for _, roleBinding := range roleBindingList.Objects.(*rbacv1.RoleBindingList).Items {
		if err := rbutils.Create(nsObject, roleBinding); err != nil {
			return err
		}
	}

	return nil
}
