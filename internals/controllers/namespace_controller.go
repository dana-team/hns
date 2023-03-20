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
	"reflect"

	danav1 "github.com/dana-team/hns/api/v1"
	"github.com/dana-team/hns/internals/utils"
	"github.com/go-logr/logr"
	quotav1 "github.com/openshift/api/quota/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// NamespaceReconciler reconciles a Namespace object
type NamespaceReconciler struct {
	client.Client
	Log                logr.Logger
	Scheme             *runtime.Scheme
	ResourcePoolEvents chan event.GenericEvent
	SnsEvents          chan event.GenericEvent
}

type NamespacePredicate struct {
	predicate.Funcs
}

// +kubebuilder:rbac:groups=core,resources=namespaces,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=core,resources=namespaces/status,verbs=get;update;patch

func (r *NamespaceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("namespace", req.NamespacedName)
	log.Info("starting to reconcile")

	// Creating subspace's namespace objectContext- described as ownerNamespace
	namespace, err := utils.NewObjectContext(ctx, log, r.Client, req.NamespacedName, &corev1.Namespace{})
	if err != nil {
		return ctrl.Result{}, err
	}

	if !namespace.IsPresent() {
		log.Info("namespace deleted")
		return ctrl.Result{}, nil
	}

	//we do not want to reconcile on root namespace since there is no need to init, sync or cleanup after them
	//also we must not add finalizer to root namespace since we woll not be able to be deleted

	//ns is being deleted
	if utils.DeletionTimeStampExists(namespace.Object) {
		log.Info("starting to clean up")
		return ctrl.Result{}, r.CleanUp(namespace)
	}

	if utils.IsRootNamespace(namespace.Object) {
		log.Info("ns is root, skip")
		return ctrl.Result{}, nil
	}

	if utils.NamespaceFinalizerExists(namespace.Object) {
		log.Info("starting to sync")
		return ctrl.Result{}, r.Sync(namespace)
	}

	log.Info("starting to init")
	return ctrl.Result{}, r.Init(namespace)
}

func (r *NamespaceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&corev1.Namespace{}).
		Watches(&source.Channel{Source: r.ResourcePoolEvents}, &handler.EnqueueRequestForObject{}).
		//filter all namespace without the HNS label
		WithEventFilter(NamespacePredicate{predicate.NewPredicateFuncs(func(object client.Object) bool {
			//always allow reconciliation of subnamespaces
			if reflect.TypeOf(object) == reflect.TypeOf(&danav1.Subnamespace{}) {
				return true
			}
			objLabels := object.GetLabels()
			if _, ok := objLabels[danav1.Hns]; ok {
				return true
			}
			return false
		})}).
		//also reconcile when subnamespace is changed
		Owns(&danav1.Subnamespace{}).
		Complete(r)
}

///////////////////////////////////////////////////////////////////////////////////////////////////

// CleanUp is being called when a namespace is being deleted,it deletes the subnamespace object related to the namespace inside its parent namespace,
// also removing the finalizer from the namespace, so it can be deleted
func (r *NamespaceReconciler) CleanUp(namespace *utils.ObjectContext) error {

	// delete the quotaObj of the namespace
	if err := deleteNamespaceQuotaObj(namespace); err != nil {
		return err
	}

	//delete the sns object from parent ns
	if !utils.IsRootNamespace(namespace.Object) {
		if err := deleteSubNamespace(namespace); err != nil {
			return err
		}
	}

	var parent = utils.GetNamespaceParent(namespace.Object)
	grandparentNs, err := utils.NewObjectContext(namespace.Ctx, namespace.Log, r.Client, types.NamespacedName{Name: parent}, &corev1.Namespace{})
	if err != nil {
		return err
	}
	var grandparent = utils.GetNamespaceParent(grandparentNs.Object)

	r.SnsEvents <- event.GenericEvent{Object: &danav1.Subnamespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:      parent,
			Namespace: grandparent,
		},
	}}
	if err := ensureDeleteNamespaceHnsView(namespace); err != nil {
		return err
	}
	return removeNamespaceFinalizer(namespace)
}

// Sync is being called every time there is an update in the namepsace's children and make sure its role is up-to-date
func (r *NamespaceReconciler) Sync(namespace *utils.ObjectContext) error {
	if err := ensureSnsResourcePool(namespace); err != nil {
		return err
	}

	if err := ensureCreateNamespaceHnsView(namespace); err != nil {
		return err
	}
	//if ns has no sns then he is a leaf
	if utils.IsChildlessNamespace(namespace) {
		return updateRole(namespace, danav1.Leaf)
	}
	return updateRole(namespace, danav1.NoRole)
}

// Init is being called at the first time namespace is reconciled and adds a finalizer to it
func (r *NamespaceReconciler) Init(namespace *utils.ObjectContext) error {
	if err := addNamespaceFinalizer(namespace); err != nil {
		return err
	}
	if err := ensureCreateNamespaceHnsView(namespace); err != nil {
		return err
	}
	return createParentRoleBindings(namespace)
}

func ensureDeleteNamespaceHnsView(namespace *utils.ObjectContext) error {
	nsHnsViewRole, err := utils.NewObjectContext(namespace.Ctx, namespace.Log, namespace.Client, types.NamespacedName{Name: utils.GetNsHnsViewRoleName(namespace.Object.GetName())}, utils.ComposeNsHnsViewClusterRole(namespace.Object.GetName()))
	if err != nil {
		return err
	}

	if err := nsHnsViewRole.EnsureDeleteObject(); err != nil {
		return err
	}

	nsHnsViewCrb, err := utils.NewObjectContext(namespace.Ctx, namespace.Log, namespace.Client, types.NamespacedName{Name: utils.GetNsHnsViewRoleName(namespace.Object.GetName())}, utils.ComposeNsHnsViewClusterRoleBinding(namespace.Object.GetName()))
	if err != nil {
		return err
	}
	return nsHnsViewCrb.EnsureDeleteObject()
}

func ensureCreateNamespaceHnsView(namespace *utils.ObjectContext) error {
	nsHnsViewRole, err := utils.NewObjectContext(namespace.Ctx, namespace.Log, namespace.Client, types.NamespacedName{Name: utils.GetNsHnsViewRoleName(namespace.Object.GetName())}, utils.ComposeNsHnsViewClusterRole(namespace.Object.GetName()))
	if err != nil {
		return err
	}

	if err := nsHnsViewRole.EnsureCreateObject(); err != nil {
		return err
	}

	nsHnsViewRoleBinding, err := utils.NewObjectContext(namespace.Ctx, namespace.Log, namespace.Client, types.NamespacedName{Name: utils.GetNsHnsViewRoleName(namespace.Object.GetName())}, utils.ComposeNsHnsViewClusterRoleBinding(namespace.Object.GetName()))
	if err != nil {
		return err
	}
	return nsHnsViewRoleBinding.EnsureCreateObject()
}

func deleteNamespaceQuotaObj(ns *utils.ObjectContext) error {
	rqFlag, err := utils.IsRq(ns, danav1.SelfOffset)
	if err != nil {
		ns.Log.Error(err, "unable to determine if sns is Rq")
	}

	if rqFlag {
		quotaObj, err := utils.NewObjectContext(ns.Ctx, ns.Log, ns.Client, types.NamespacedName{Namespace: ns.Object.GetName(), Name: ns.Object.GetName()}, &corev1.ResourceQuota{})
		if err != nil {
			return err
		}
		return quotaObj.EnsureDeleteObject()
	}

	quotaObj, err := utils.NewObjectContext(ns.Ctx, ns.Log, ns.Client, types.NamespacedName{Name: ns.Object.GetName()}, &quotav1.ClusterResourceQuota{})
	if err != nil {
		return err
	}
	return quotaObj.EnsureDeleteObject()
}

func deleteSubNamespace(namespace *utils.ObjectContext) error {
	sns, err := utils.NewObjectContext(namespace.Ctx, namespace.Log, namespace.Client, types.NamespacedName{Name: utils.GetNamespaceSnsPointer(namespace.Object), Namespace: utils.GetNamespaceParent(namespace.Object)}, &danav1.Subnamespace{})
	if err != nil {
		return err
	}
	return sns.EnsureDeleteObject()
}

func removeNamespaceFinalizer(namespace *utils.ObjectContext) error {
	//remove finalizer so the ns will be able to delete
	return namespace.UpdateObject(func(object client.Object, log logr.Logger) (client.Object, logr.Logger) {
		log = log.WithValues("removed nsFinalizer", danav1.NsFinalizer)
		controllerutil.RemoveFinalizer(object, danav1.NsFinalizer)
		return object, log
	})
}

func addNamespaceFinalizer(namespace *utils.ObjectContext) error {
	return namespace.UpdateObject(func(object client.Object, log logr.Logger) (client.Object, logr.Logger) {
		log = log.WithValues("added nsFinalizer", danav1.NsFinalizer)
		controllerutil.AddFinalizer(object, danav1.NsFinalizer)
		return object, log
	})
}

func updateRole(namespace *utils.ObjectContext, role string) error {
	return namespace.UpdateObject(func(object client.Object, log logr.Logger) (client.Object, logr.Logger) {
		log = log.WithValues("role & annotation", role)
		object.(*corev1.Namespace).Labels[danav1.Role] = role
		object.(*corev1.Namespace).Annotations[danav1.Role] = role
		return object, log
	})
}

func createParentRoleBindings(namespace *utils.ObjectContext) error {
	roleBindingList, err := utils.NewObjectContextList(namespace.Ctx, namespace.Log, namespace.Client, &rbacv1.RoleBindingList{}, client.InNamespace(utils.GetNamespaceParent(namespace.Object)), client.MatchingFields{"rb.propagate": "true"})
	if err != nil {
		return err
	}

	for _, roleBinding := range roleBindingList.Objects.(*rbacv1.RoleBindingList).Items {
		roleBindingToCreate, err := utils.NewObjectContext(namespace.Ctx, namespace.Log, namespace.Client, types.NamespacedName{}, utils.ComposeRoleBinding(roleBinding.Name, namespace.Object.GetName(), roleBinding.Subjects, roleBinding.RoleRef))
		if err != nil {
			return err
		}
		if err := roleBindingToCreate.CreateObject(); err != nil {
			return err
		}
	}
	return nil
}

func ensureSnsResourcePool(namespace *utils.ObjectContext) error {
	snsList, err := utils.NewObjectContextList(namespace.Ctx, namespace.Log, namespace.Client, &danav1.SubnamespaceList{}, client.InNamespace(namespace.Object.GetName()))
	if err != nil {
		return err
	}
	namespaceResourcePooled := utils.GetNamespaceResourcePooled(namespace)
	for _, sns := range snsList.Objects.(*danav1.SubnamespaceList).Items {
		snsToUpdate, err := utils.NewObjectContext(namespace.Ctx, namespace.Log, namespace.Client, types.NamespacedName{Namespace: namespace.Object.GetName(), Name: sns.GetName()}, &danav1.Subnamespace{})
		if err != nil {
			return err
		}

		isUpperRp, err := utils.IsUpperResourcePool(snsToUpdate)
		//if the sns is not the same type as his parent and is not upper resourcepool - the type should be as his parent
		if utils.GetSnsResourcePooled(snsToUpdate.Object) != namespaceResourcePooled && !isUpperRp {
			if err := snsToUpdate.UpdateObject(func(object client.Object, log logr.Logger) (client.Object, logr.Logger) {
				log = log.WithValues(danav1.ResourcePool, namespaceResourcePooled)
				object.SetLabels(map[string]string{danav1.ResourcePool: namespaceResourcePooled})
				return object, log
			}); err != nil {
				return err
			}
		}
	}
	return nil
}
