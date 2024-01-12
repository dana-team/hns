/*
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	"fmt"
	danav1 "github.com/dana-team/hns/api/v1"
	"github.com/dana-team/hns/internals/namespaceDB"
	"github.com/dana-team/hns/internals/utils"
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
	NamespaceDB *namespaceDB.NamespaceDB
}

type snsPhaseFunc func(*utils.ObjectContext, *utils.ObjectContext) (ctrl.Result, error)

// +kubebuilder:rbac:groups=dana.hns.io,resources=subnamespaces,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=dana.hns.io,resources=subnamespaces/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=dana.hns.io,resources=subnamespaces/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=namespaces/finalizers,verbs=update
// +kubebuilder:rbac:groups="",resources=resourcequotas,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="quota.openshift.io",resources=clusterresourcequotas,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=limitranges,verbs=get;list;watch;create;update;patch;delete

// SetupWithManager sets up the controller by specifying the following: controller is managing the reconciliation
// of subnamespace objects and is watching for changes to the SNSEvents channel and enqueues requests for the
// associated object
func (r *SubnamespaceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		WatchesRawSource(&source.Channel{Source: r.SNSEvents}, &handler.EnqueueRequestForObject{}).
		For(&danav1.Subnamespace{}).
		Complete(r)
}

func (r *SubnamespaceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithName("controllers").WithName("Subnamespace").WithValues("sns", req.NamespacedName)
	logger.Info("starting to reconcile")

	snsName := req.Name
	snsObject, err := utils.NewObjectContext(ctx, r.Client, req.NamespacedName, &danav1.Subnamespace{})
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to get object '%s': "+err.Error(), snsName)
	}

	if !snsObject.IsPresent() {
		logger.Info("resource not found. Ignoring since object must be deleted")
		return ctrl.Result{}, nil
	}

	snsParentNSName := snsObject.Object.(*danav1.Subnamespace).GetNamespace()
	snsParentNS, err := utils.NewObjectContext(ctx, r.Client, types.NamespacedName{Name: snsParentNSName}, &corev1.Namespace{})
	if err != nil {
		return ctrl.Result{}, err
	}

	if !snsParentNS.IsPresent() {
		return ctrl.Result{}, fmt.Errorf("failed to find '%s', the parent namespace of subnamespace '%s', it may have been deleted", snsParentNSName, snsName)
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
