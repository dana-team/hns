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
	"reflect"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// NamespaceReconciler reconciles a Namespace object
type NamespaceReconciler struct {
	client.Client
	Scheme      *runtime.Scheme
	NSEvents    chan event.GenericEvent
	SNSEvents   chan event.GenericEvent
	NamespaceDB *namespaceDB.NamespaceDB
}

// +kubebuilder:rbac:groups=core,resources=namespaces,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=namespaces/status,verbs=get;update;patch
// +kubebuilder:rbac:groups="rbac.authorization.k8s.io",resources=clusterrolebindings,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="rbac.authorization.k8s.io",resources=clusterroles,verbs=get;list;watch;create;update;patch;delete;escalate;bind

// SetupWithManager sets up the controller by specifying the following: controller is managing the reconciliation
// of Namespace objects, it is watching for changes to the NSEvents channel and enqueues requests for the
// associated object. NamespacePredicate is used as an event filter, the predicate function checks if the object
// being watched is an SNS object or has a specific label. The controller also owns SNS objects
func (r *NamespaceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Namespace{}).
		WatchesRawSource(&source.Channel{Source: r.NSEvents}, &handler.EnqueueRequestForObject{}).
		WithEventFilter(predicate.NewPredicateFuncs(func(object client.Object) bool {
			if reflect.TypeOf(object) == reflect.TypeOf(&danav1.Subnamespace{}) {
				return true
			}

			objLabels := object.GetLabels()
			if _, ok := objLabels[danav1.Hns]; ok {
				return true
			}

			return false
		})).
		// reconcile when subnamespace is changed
		Owns(&danav1.Subnamespace{}).
		Complete(r)
}

func (r *NamespaceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithName("controllers").WithName("Namespace").WithValues("ns", req.NamespacedName)
	logger.Info("starting to reconcile")

	nsObject, err := utils.NewObjectContext(ctx, r.Client, req.NamespacedName, &corev1.Namespace{})
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to get object %q: "+err.Error(), nsObject.Object.GetName())
	}

	if !nsObject.IsPresent() {
		logger.Info("resource not found. Ignoring since object must be deleted")
		return ctrl.Result{}, nil
	}

	// skip namespace reconciliation for a root namespace, since there is no need to do
	// anything with the root namespace as it's usually created manually by the cluster admin
	if utils.IsRootNamespace(nsObject.Object) {
		logger.Info("no need to reconcile the root namespace, skip")
		return ctrl.Result{}, nil
	}

	isBeingDeleted := utils.DeletionTimeStampExists(nsObject.Object)
	if isBeingDeleted {
		return ctrl.Result{}, r.cleanUp(ctx, nsObject)
	}

	finalizerExists := doesNamespaceFinalizerExist(nsObject.Object)
	if !finalizerExists {
		return ctrl.Result{}, r.init(nsObject)
	}

	return ctrl.Result{}, r.sync(nsObject)
}

// doesNamespaceFinalizerExist returns true if the HNS finalizer exists
func doesNamespaceFinalizerExist(namespace client.Object) bool {
	return controllerutil.ContainsFinalizer(namespace, danav1.NsFinalizer)
}
