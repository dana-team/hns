package rolebinding

import (
	"context"
	"fmt"

	danav1 "github.com/dana-team/hns/api/v1"
	"github.com/dana-team/hns/internal/common"
	"github.com/dana-team/hns/internal/objectcontext"
	"github.com/dana-team/hns/internal/rolebinding/rbutils"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// RoleBindingReconciler reconciles a RoleBinding object.
type RoleBindingReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=rolebindings,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=rolebindings/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="rbac.authorization.k8s.io",resources=roles,verbs=get;list;watch;create;update;patch;delete;bind

// SetupWithManager sets up the controller by specifying the following: indexes the "rb.propagate" field for
// RoleBindings, filters events to only include RoleBindings that are part of a namespace with
// the "danav1.Hns" label, and then watches for events on RoleBindings.
func (r *RoleBindingReconciler) SetupWithManager(mgr ctrl.Manager) error {
	// define a function for creating the index used for filtering events; this index is used to only include
	// events for RoleBindings that are considered value
	indexFunc := func(rawObj client.Object) []string {
		if rbutils.IsHNSRelated(rawObj) {
			return []string{"true"}
		}
		return nil
	}

	// add the index to the Kubernetes API server's indexer
	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &rbacv1.RoleBinding{}, "rb.propagate", indexFunc); err != nil {
		return err
	}
	return ctrl.NewControllerManagedBy(mgr).
		WithEventFilter(predicate.NewPredicateFuncs(func(object client.Object) bool {
			// get the Namespace object associated with the RoleBinding
			var rbNs corev1.Namespace
			if err := r.Get(context.Background(), types.NamespacedName{Name: object.GetNamespace()}, &rbNs); err != nil {
				return false
			}

			// check if the Namespace has the "danav1.Hns" label
			objLabels := rbNs.GetLabels()
			if _, ok := objLabels[danav1.Hns]; ok {
				return true
			}
			return false
		})).
		For(&rbacv1.RoleBinding{}).
		Complete(r)
}

func (r *RoleBindingReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithName("controllers").WithName("RoleBinding").WithValues("rb", req.NamespacedName)
	logger.Info("starting to reconcile")

	rbObject, err := objectcontext.New(ctx, r.Client, req.NamespacedName, &rbacv1.RoleBinding{})
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to get object %q: %v", req.NamespacedName, err.Error())
	}

	if !rbObject.IsPresent() {
		logger.Info("resource not found. Ignoring since object must be deleted")
		return ctrl.Result{}, nil
	}

	if !rbutils.IsHNSRelated(rbObject.Object) {
		logger.Info("roleBinding object is not valid for HNS reconciliation. Ignoring")
		return ctrl.Result{}, nil
	}

	snsList, err := objectcontext.NewList(ctx, r.Client, &danav1.SubnamespaceList{}, client.InNamespace(req.Namespace))
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to get list of subnamespaces in namespace %q: %v", req.Namespace, err.Error())
	}

	isBeingDeleted := common.DeletionTimeStampExists(rbObject.Object)
	if isBeingDeleted {
		return ctrl.Result{}, r.cleanUp(rbObject, snsList)
	}

	return ctrl.Result{}, r.init(rbObject, snsList)
}
