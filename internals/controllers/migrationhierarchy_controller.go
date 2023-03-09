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

	danav1 "github.com/dana-team/hns/api/v1"
	"github.com/dana-team/hns/internals/namespaceDB"
	"github.com/dana-team/hns/internals/utils"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
)

// MigrationHierarchyReconciler reconciles a MigrationHierarchy object
type MigrationHierarchyReconciler struct {
	client.Client
	Log         logr.Logger
	Scheme      *runtime.Scheme
	NamespaceDB *namespaceDB.NamespaceDB
	SnsEvents   chan event.GenericEvent
}

// +kubebuilder:rbac:groups=dana.hns.io,resources=migrationhierarchies,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=dana.hns.io,resources=migrationhierarchies/status,verbs=get;update;patch

func (r *MigrationHierarchyReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = context.Background()
	log := r.Log.WithValues("migrationhierarchy", req.NamespacedName)
	log.Info("starting to reconcile")

	migrationObject, err := utils.NewObjectContext(ctx, log, r.Client, client.ObjectKey{Name: req.NamespacedName.Name}, &danav1.MigrationHierarchy{})
	if err != nil {
		return ctrl.Result{}, err
	}

	if !migrationObject.IsPresent() {
		log.Info("migration object deleted")
		return ctrl.Result{}, nil
	}

	if migrationObject.Object.(*danav1.MigrationHierarchy).Status.Phase != danav1.Complete {

		currentNamespace := migrationObject.Object.(*danav1.MigrationHierarchy).Spec.CurrentNamespace
		toNamespace := migrationObject.Object.(*danav1.MigrationHierarchy).Spec.ToNamespace

		ns, err := utils.NewObjectContext(ctx, log, r.Client, client.ObjectKey{Namespace: "", Name: currentNamespace}, &corev1.Namespace{})
		if err != nil {
			return ctrl.Result{}, err
		}

		// at the end of the migration operation we need to sync the original parent of the subnamespace that is being
		// migrated in order for its status to show the correct list of child subnamespaces. 
		// Therefore, we here store in a variable the original parent of the subnamespace that is being migrated
		sourceParentNs, err := utils.NewObjectContext(ctx, log, r.Client, client.ObjectKey{Namespace: "", Name: utils.GetNamespaceParent(ns.Object)}, &corev1.Namespace{})
		if err != nil {
			return ctrl.Result{}, err
		}
		sourceSnsParentName := utils.GetNamespaceParent(ns.Object)
		sourceSnsParentNamespace := utils.GetNamespaceParent(sourceParentNs.Object)

		oldSns, err := utils.NewObjectContext(ctx, log, r.Client, client.ObjectKey{Name: currentNamespace, Namespace: utils.GetNamespaceParent(ns.Object)}, &danav1.Subnamespace{})
		if err != nil {
			return ctrl.Result{}, err
		}
		toNs, err := utils.NewObjectContext(ctx, log, r.Client, client.ObjectKey{Namespace: "", Name: toNamespace}, &corev1.Namespace{})
		if err != nil {
			return ctrl.Result{}, err
		}
		destparentSns, err := utils.NewObjectContext(ctx, log, r.Client, client.ObjectKey{Name: toNamespace, Namespace: utils.GetNamespaceParent(toNs.Object)}, &danav1.Subnamespace{})
		if err != nil {
			return ctrl.Result{}, err
		}

		if err := namespaceDB.MigrateNsHierarchy(r.NamespaceDB, r.Client, ns.GetName(), toNs.GetName()); err != nil {
			return ctrl.Result{}, err
		}

		destparentSnsLabels := destparentSns.Object.(*danav1.Subnamespace).GetLabels()
		labels := make(map[string]string)

		x := oldSns.Object.(*danav1.Subnamespace).GetLabels()
		_ = x
		// change old sns type to his parent type
		if destparentSnsLabels[danav1.ResourcePool] == "true" {
			labels[danav1.ResourcePool] = "true"
		}
		if destparentSnsLabels[danav1.ResourcePool] == "false" {
			labels[danav1.ResourcePool] = "false"
		}

		// create new sns and delete old sns
		composedNewSns := utils.ComposeSns(currentNamespace, toNamespace, oldSns.Object.(*danav1.Subnamespace).Spec.ResourceQuotaSpec.Hard, labels)
		composedNewSns.Status.Phase = danav1.Migrated
		newSns, err := utils.NewObjectContext(ctx, log, r.Client, types.NamespacedName{Name: oldSns.Object.GetName()}, composedNewSns)
		if err != nil {
			return ctrl.Result{}, err
		}
		if err := newSns.EnsureCreateObject(); err != nil {
			return ctrl.Result{}, err
		}
		if err := oldSns.EnsureDeleteObject(); err != nil {
			return ctrl.Result{}, err
		}
		log.Info("new subnamespace created")

		//update namespace label and annotations
		ns.UpdateNsByparent(toNs, ns)

		//update every namespace childrens label and annotations
		utils.UpdateAllNsChildsOfNs(ns)

		//toNs update role label to None
		toNs.UpdateObject(func(object client.Object, log logr.Logger) (client.Object, logr.Logger) {
			object.(*corev1.Namespace).GetLabels()[danav1.Role] = danav1.NoRole
			log = log.WithValues("update toNs role label", toNs.Object.GetName())
			return object, log
		})

		// change status to done
		migrationObject.UpdateObject(func(object client.Object, log logr.Logger) (client.Object, logr.Logger) {
			object.(*danav1.MigrationHierarchy).Status.Phase = danav1.Complete
			log = log.WithValues("migration phase", danav1.Complete)
			return object, log
		})

		// resync for the parent source sns
		r.addSnsToSnsEvent(sourceSnsParentName, sourceSnsParentNamespace)
	}

	return ctrl.Result{}, nil
}

func (r *MigrationHierarchyReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&danav1.MigrationHierarchy{}).
		Complete(r)
}

// addSnsToSnsEvent takes two paramaters: snsName and snsNamespace
// then adds the sns to the sns event channel to trigger new resync for the sns
func (r *MigrationHierarchyReconciler) addSnsToSnsEvent(snsName string, snsNamespace string) {
	r.SnsEvents <- event.GenericEvent{Object: &danav1.Subnamespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:      snsName,
			Namespace: snsNamespace,
		},
	}}
}
