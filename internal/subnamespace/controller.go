package subnamespace

import (
	"context"
	"fmt"

	danav1 "github.com/dana-team/hns/api/v1"
	"github.com/dana-team/hns/internal/namespacedb"
	"github.com/dana-team/hns/internal/objectcontext"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// SubnamespaceReconciler reconciles a Subnamespace object
type SubnamespaceReconciler struct {
	client.Client
	Scheme      *runtime.Scheme
	NSEvents    chan event.GenericEvent
	SNSEvents   chan event.GenericEvent
	NamespaceDB *namespacedb.NamespaceDB
}

type snsPhaseFunc func(*objectcontext.ObjectContext, *objectcontext.ObjectContext) (ctrl.Result, error)

// +kubebuilder:rbac:groups=dana.hns.io,resources=subnamespaces,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=dana.hns.io,resources=subnamespaces/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=dana.hns.io,resources=subnamespaces/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=namespaces/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=resourcequotas,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="quota.openshift.io",resources=clusterresourcequotas,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=limitranges,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=configmaps,verbs=get;list;watch;

// SetupWithManager sets up the controller by specifying the following: controller is managing the reconciliation
// of subnamespace objects and is watching for changes to the SNSEvents channel and enqueues requests for the
// associated object.
func (r *SubnamespaceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		WatchesRawSource(source.Channel(r.SNSEvents, &handler.EnqueueRequestForObject{})).
		For(&danav1.Subnamespace{}).
		Complete(r)
}

func (r *SubnamespaceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithName("controllers").WithName("Subnamespace").WithValues("sns", req.NamespacedName)
	logger.Info("starting to reconcile")

	snsObject, err := objectcontext.New(ctx, r.Client, req.NamespacedName, &danav1.Subnamespace{})
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to get object %q: %v", req.NamespacedName, err.Error())
	}

	if !snsObject.IsPresent() {
		logger.Info("resource not found. Ignoring since object must be deleted")
		return ctrl.Result{}, nil
	}

	snsParentNSName := snsObject.Object.(*danav1.Subnamespace).GetNamespace()
	snsParentNS, err := objectcontext.New(ctx, r.Client, types.NamespacedName{Name: snsParentNSName}, &corev1.Namespace{})
	if err != nil {
		return ctrl.Result{}, err
	}

	if !snsParentNS.IsPresent() {
		return ctrl.Result{}, fmt.Errorf("failed to find %q, the parent namespace of subnamespace %q, it may have been deleted", snsParentNSName, req.Name)
	}

	phase := snsObject.Object.(*danav1.Subnamespace).Status.Phase
	phaseMap := map[danav1.Phase]snsPhaseFunc{
		danav1.None:     r.setup,
		danav1.Missing:  r.init,
		danav1.Migrated: r.init,
		danav1.Created:  r.sync,
	}

	return phaseMap[phase](snsParentNS, snsObject)
}
