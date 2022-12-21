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
	//"strings"
	//"strconv"
	danav1 "github.com/dana-team/hns/api/v1"
	"github.com/dana-team/hns/internals/utils"
	"context"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/predicate"
)

// RoleBindingReconciler reconciles a RoleBinding object
type RoleBindingReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=rbac.dana.sns.io,resources=rolebindings,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=rbac.dana.sns.io,resources=rolebindings/status,verbs=get;update;patch

func (r *RoleBindingReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("rolebinding", req.NamespacedName)
	log.Info("starting to reconcile")

	roleBinding, err := utils.NewObjectContext(ctx, log, r.Client, req.NamespacedName, &rbacv1.RoleBinding{})
	if err != nil {
		return ctrl.Result{}, err
	}

	if !roleBinding.IsPresent() {
		log.Info("roleBinding deleted")
		return ctrl.Result{}, nil
	}

	if !utils.IsValidRoleBinding(roleBinding.Object) {
		return ctrl.Result{}, nil
	}

	snsList, err := utils.NewObjectContextList(ctx, log, r.Client, &danav1.SubnamespaceList{}, client.InNamespace(req.Namespace))
	if err != nil {
		return ctrl.Result{}, err
	}

	if utils.DeletionTimeStampExists(roleBinding.Object) {
		log.Info("starting to cleanup")
		return ctrl.Result{}, r.CleanUp(roleBinding, snsList)
	}
	log.Info("starting to init")
	return ctrl.Result{}, r.Init(roleBinding, snsList)
}

func (r *RoleBindingReconciler) SetupWithManager(mgr ctrl.Manager) error {
	indexFunc := func(rawObj client.Object) []string {
		if utils.IsValidRoleBinding(rawObj) {
			return []string{"true"}
		}
		return nil
	}
	if err := mgr.GetFieldIndexer().IndexField(context.Background(), &rbacv1.RoleBinding{}, "rb.propagate", indexFunc); err != nil {
		return err
	}
	return ctrl.NewControllerManagedBy(mgr).
		WithEventFilter(NamespacePredicate{predicate.NewPredicateFuncs(func(object client.Object) bool {
			var rbNs corev1.Namespace
			if err := r.Get(context.Background(), types.NamespacedName{Name: object.GetNamespace()}, &rbNs); err != nil {
				return false
			}
			objLabels := rbNs.GetLabels()
			if _, ok := objLabels[danav1.Hns]; ok {
				return true
			}
			return false
		})}).
		For(&rbacv1.RoleBinding{}).
		Complete(r)
}

///////////////////////////////////////////////////////////////////////////////////////////////

func (r *RoleBindingReconciler) Init(roleBinding *utils.ObjectContext, sns *utils.ObjectContextList) error {
	if !utils.IsRoleBindingFinalizerExists(roleBinding.Object) {
		if err := createRoleBindingFinalizer(roleBinding); err != nil {
			return err
		}
	}
	if err := createRoleBindingsInSnsList(roleBinding, sns); err != nil {
		return err
	}

	if !utils.IsServiceAccount(roleBinding.Object) {
		return ensureAddRbToNsHnsView(roleBinding)
		//if err := deleteClusterRole(roleBinding); err != nil {
		//	return err
		//}
		//if err := deleteClusterRoleBinding(roleBinding); err != nil {
		//	return err
		//}
		//if err := deleteSnsViewClusterRole(roleBinding); err != nil {
		//	return err
		//}
		//if err := deleteSnsViewClusterRoleBinding(roleBinding); err != nil {
		//	return err
		//}
		//----------------------------------------------------------------------------
		//if err := createClusterRole(roleBinding); err != nil {
		//	return err
		//}
		//if err := createClusterRoleBinding(roleBinding); err != nil {
		//	return err
		//}
		//if err := createSnsViewClusterRole(roleBinding); err != nil {
		//	return err
		//}
		//return createSnsViewClusterRoleBinding(roleBinding)
	}
	return nil
}

func (r *RoleBindingReconciler) CleanUp(roleBinding *utils.ObjectContext, sns *utils.ObjectContextList) error {
	if !utils.IsServiceAccount(roleBinding.Object) {
		if err := deleteClusterRole(roleBinding); err != nil {
			return err
		}
		if err := deleteClusterRoleBinding(roleBinding); err != nil {
			return err
		}
		if err := deleteSnsViewClusterRole(roleBinding); err != nil {
			return err
		}
		if err := deleteSnsViewClusterRoleBinding(roleBinding); err != nil {
			return err
		}
	}
	if err := removeSubjectFromSubjects(roleBinding); err != nil {
		return err
	}
	if err := deleteRoleBindingsInSnsList(roleBinding, sns); err != nil {
		return err
	}
	return deleteRoleBindingFinalizer(roleBinding)
}

func ensureAddRbToNsHnsView(roleBinding *utils.ObjectContext) error {
	nsHnsViewCrb, err := getNsHnsViewCrb(roleBinding)
	if err != nil {
		return err
	}

	subjects := nsHnsViewCrb.Object.(*rbacv1.ClusterRoleBinding).Subjects

	for _, subject := range roleBinding.Object.(*rbacv1.RoleBinding).Subjects {
		if !isSubjectInSubjects(subjects, subject) {
			subjects = append(subjects, subject)
		}
	}
	return updateNsHnsViewCrbSubjects(roleBinding, subjects)
}
func isSubjectInSubjects(subjects []rbacv1.Subject, subjectToFind rbacv1.Subject) bool {
	for _, subject := range subjects {
		if isSubjectsEqual(subject, subjectToFind) {
			return true
		}
	}
	return false
}

func updateNsHnsViewCrbSubjects(roleBinding *utils.ObjectContext, subjects []rbacv1.Subject) error {
	nsHnsViewCrb, err := getNsHnsViewCrb(roleBinding)
	if err != nil {
		return err
	}

	return nsHnsViewCrb.UpdateObject(func(object client.Object, log logr.Logger) (client.Object, logr.Logger) {
		log = log.WithValues("updating subjects", roleBinding.Object.(*rbacv1.RoleBinding).Subjects)
		nsHnsViewCrb.Object.(*rbacv1.ClusterRoleBinding).Subjects = subjects
		return object, log
	})
}

func getNsHnsViewCrb(roleBinding *utils.ObjectContext) (*utils.ObjectContext, error) {
	namespace := roleBinding.Object.GetNamespace()
	return utils.NewObjectContext(roleBinding.Ctx, roleBinding.Log, roleBinding.Client, types.NamespacedName{Name: utils.GetNsHnsViewRoleName(namespace)}, &rbacv1.ClusterRoleBinding{})
}

func removeSubjectFromSubjects(roleBinding *utils.ObjectContext) error {
	var newSubjects []rbacv1.Subject
	nsHnsViewCrb, err := getNsHnsViewCrb(roleBinding)
	if err != nil {
		return err
	}

	roleBindingSubjects := roleBinding.Object.(*rbacv1.RoleBinding).Subjects
	for _, subjectToFind := range nsHnsViewCrb.Object.(*rbacv1.ClusterRoleBinding).Subjects {
		if !isSubjectInSubjects(roleBindingSubjects, subjectToFind) {
			newSubjects = append(newSubjects, subjectToFind)
		}
	}
	return updateNsHnsViewCrbSubjects(roleBinding, newSubjects)
}

func isSubjectsEqual(subject1 rbacv1.Subject, subject2 rbacv1.Subject) bool {
	if subject1.Name == subject2.Name && subject1.Kind == subject2.Kind && subject1.APIGroup == subject1.APIGroup {
		return true
	}
	return false
}

func deleteRoleBindingsInSnsList(roleBinding *utils.ObjectContext, snsList *utils.ObjectContextList) error {
	for _, sns := range snsList.Objects.(*danav1.SubnamespaceList).Items {
		roleBindingToDelete, err := utils.NewObjectContext(roleBinding.Ctx, roleBinding.Log, roleBinding.Client, types.NamespacedName{},
			utils.ComposeRoleBinding(roleBinding.Object.GetName(), sns.Spec.NamespaceRef.Name, utils.GetRoleBindingSubjects(roleBinding.Object), utils.GetRoleBindingRoleRef(roleBinding.Object)))
		if err != nil {
			return err
		}

		if err := roleBindingToDelete.DeleteObject(); err != nil {
			return err
		}
	}

	return roleBinding.UpdateObject(func(object client.Object, log logr.Logger) (client.Object, logr.Logger) {
		log = log.WithValues("removed rbFinalizer", danav1.RbFinalizer)
		controllerutil.RemoveFinalizer(object, danav1.RbFinalizer)
		return object, log
	})
}

func deleteClusterRole(roleBinding *utils.ObjectContext) error {

	ClusterRoleToDelete, err := utils.NewObjectContext(roleBinding.Ctx, roleBinding.Log, roleBinding.Client, types.NamespacedName{Name: utils.GetRoleBindingClusterRoleName(roleBinding.Object)}, &rbacv1.ClusterRole{})
	if err != nil {
		return err
	}

	return ClusterRoleToDelete.EnsureDeleteObject()
}

func deleteClusterRoleBinding(roleBinding *utils.ObjectContext) error {
	ClusterRoleBindingToDelete, err := utils.NewObjectContext(roleBinding.Ctx, roleBinding.Log, roleBinding.Client, types.NamespacedName{Name: utils.GetRoleBindingClusterRoleName(roleBinding.Object)}, &rbacv1.ClusterRoleBinding{})
	if err != nil {
		return err
	}

	return ClusterRoleBindingToDelete.EnsureDeleteObject()
}

func deleteSnsViewClusterRole(roleBinding *utils.ObjectContext) error {

	ClusterRoleToDelete, err := utils.NewObjectContext(roleBinding.Ctx, roleBinding.Log, roleBinding.Client, types.NamespacedName{Name: utils.GetRoleBindingSnsViewClusterRoleName(roleBinding.Object)}, &rbacv1.ClusterRole{})
	if err != nil {
		return err
	}

	return ClusterRoleToDelete.EnsureDeleteObject()
}

func deleteSnsViewClusterRoleBinding(roleBinding *utils.ObjectContext) error {
	ClusterRoleBindingToDelete, err := utils.NewObjectContext(roleBinding.Ctx, roleBinding.Log, roleBinding.Client, types.NamespacedName{Name: utils.GetRoleBindingSnsViewClusterRoleName(roleBinding.Object)}, &rbacv1.ClusterRoleBinding{})
	if err != nil {
		return err
	}

	return ClusterRoleBindingToDelete.EnsureDeleteObject()
}

func deleteRoleBindingFinalizer(roleBinding *utils.ObjectContext) error {
	return roleBinding.UpdateObject(func(object client.Object, log logr.Logger) (client.Object, logr.Logger) {
		log = log.WithValues("removed rbFinalizer", danav1.RbFinalizer)
		controllerutil.RemoveFinalizer(object, danav1.RbFinalizer)
		return object, log
	})
}

func createRoleBindingFinalizer(roleBinding *utils.ObjectContext) error {
	return roleBinding.UpdateObject(func(object client.Object, log logr.Logger) (client.Object, logr.Logger) {
		log = log.WithValues("added rbFinalizer", danav1.RbFinalizer)
		controllerutil.AddFinalizer(object, danav1.RbFinalizer)
		return object, log
	})
}

//func createClusterRole(roleBinding *utils.ObjectContext) error {
//	ClusterRoleToCreate, err := utils.NewObjectContext(roleBinding.Ctx, roleBinding.Log, roleBinding.Client, types.NamespacedName{}, utils.ComposeClusterRole(roleBinding.Object))
//	if err != nil {
//		return err
//	}
//
//	return ClusterRoleToCreate.EnsureCreateObject()
//}
//
//func createSnsViewClusterRole(roleBinding *utils.ObjectContext) error {
//	ClusterRoleToCreate, err := utils.NewObjectContext(roleBinding.Ctx, roleBinding.Log, roleBinding.Client, types.NamespacedName{}, utils.ComposeSnsViewClusterRole(roleBinding.Object))
//	if err != nil {
//		return err
//	}
//
//	return ClusterRoleToCreate.EnsureCreateObject()
//}
//
//func createSnsViewClusterRoleBinding(roleBinding *utils.ObjectContext) error {
//	ClusterRoleBindingToCreate, err := utils.NewObjectContext(roleBinding.Ctx, roleBinding.Log, roleBinding.Client, types.NamespacedName{}, utils.ComposeClusterRoleBinding(roleBinding.Object, utils.GetRoleBindingSnsViewClusterRoleName(roleBinding.Object)))
//	if err != nil {
//		return err
//	}
//
//	return ClusterRoleBindingToCreate.EnsureCreateObject()
//}
//
//func createClusterRoleBinding(roleBinding *utils.ObjectContext) error {
//	ClusterRoleBindingToCreate, err := utils.NewObjectContext(roleBinding.Ctx, roleBinding.Log, roleBinding.Client, types.NamespacedName{}, utils.ComposeClusterRoleBinding(roleBinding.Object, utils.GetRoleBindingClusterRoleName(roleBinding.Object)))
//	if err != nil {
//		return err
//	}
//
//	return ClusterRoleBindingToCreate.EnsureCreateObject()
//}

func createRoleBindingsInSnsList(roleBinding *utils.ObjectContext, snsList *utils.ObjectContextList) error {

	//create the roleBinding
	for _, sns := range snsList.Objects.(*danav1.SubnamespaceList).Items {
		roleBindingToCreate, err := utils.NewObjectContext(roleBinding.Ctx, roleBinding.Log, roleBinding.Client, types.NamespacedName{Name: roleBinding.Object.GetName(), Namespace: sns.Name},
			utils.ComposeRoleBinding(roleBinding.Object.GetName(), sns.Name, utils.GetRoleBindingSubjects(roleBinding.Object), utils.GetRoleBindingRoleRef(roleBinding.Object)))
		if err != nil {
			return err
		}

		if err := roleBindingToCreate.EnsureCreateObject(); err != nil {
			return err
		}
	}
	return nil
}
