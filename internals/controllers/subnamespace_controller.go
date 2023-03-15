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
	"errors"
	"strconv"
	"strings"

	danav1 "github.com/dana-team/hns/api/v1"
	"github.com/dana-team/hns/internals/namespaceDB"
	"github.com/dana-team/hns/internals/utils"
	"github.com/go-logr/logr"
	quotav1 "github.com/openshift/api/quota/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

// SubnamespaceReconciler reconciles a Subnamespace object
type SubnamespaceReconciler struct {
	client.Client
	Log                logr.Logger
	Scheme             *runtime.Scheme
	ResourcePoolEvents chan event.GenericEvent
	SnsEvents          chan event.GenericEvent
	NamespaceDB        *namespaceDB.NamespaceDB
}

// +kubebuilder:rbac:groups=dana.hns.io,resources=subnamespaces,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=dana.hns.io,resources=subnamespaces/status,verbs=get;update;patch

func (r *SubnamespaceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	log := r.Log.WithValues("subspace", req.NamespacedName)
	log.Info("starting to reconcile")

	// Creating reconciled subspace objectContext
	subspace, err := utils.NewObjectContext(ctx, log, r.Client, req.NamespacedName, &danav1.Subnamespace{})
	if err != nil {
		return ctrl.Result{}, err
	}

	if !subspace.IsPresent() {
		log.Info("subnamespace deleted")
		return ctrl.Result{}, nil
	}

	if err := r.ensureSubspaceInDB(subspace); err != nil {
		return ctrl.Result{}, err
	}

	// Creating subspace's namespace objectContext- described as ownerNamespace
	ownerNamespace, err := utils.NewObjectContext(ctx, log, r.Client, types.NamespacedName{Name: utils.GetSnsOwner(subspace.Object)}, &corev1.Namespace{})
	if err != nil {
		return ctrl.Result{}, err
	}

	if !ownerNamespace.IsPresent() {
		err := errors.New("owner namespace missing")
		log.Error(err, "subspace owner namespace not found")
		return ctrl.Result{}, err
	}

	switch currentPhase := utils.GetSnsPhase(subspace.Object); currentPhase {
	case danav1.None:
		log.Info("subspace setup")
		return ctrl.Result{}, r.Setup(ownerNamespace, subspace)

	case danav1.Missing:
		log.Info("subspace init")
		return ctrl.Result{}, r.Init(ownerNamespace, subspace)

	case danav1.Created:
		log.Info("subspace sync")
		return r.Sync(ownerNamespace, subspace)

	case danav1.Migrated:
		log.Info("subspace init")
		return ctrl.Result{}, r.Init(ownerNamespace, subspace)

	default:
		err := errors.New("unexpected subnamespace phase")
		log.Error(err, "unable to recognize subnamespace phase")
		return ctrl.Result{}, err
	}
}

// ensureSubspaceInDB ensures subnamespace in db if it should be
func (r *SubnamespaceReconciler) ensureSubspaceInDB(subspace *utils.ObjectContext) error {
	if r.NamespaceDB.GetKey(subspace.Object.GetName()) != "" {
		return nil
	}
	rqFlag, err := utils.IsRq(subspace, danav1.SelfOffset)
	if err != nil {
		return err
	}
	if !rqFlag {
		err := namespaceDB.AddNs(r.NamespaceDB, r.Client, subspace.Object.(*danav1.Subnamespace))
		return err
	}
	return nil
}

func (r *SubnamespaceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		Watches(&source.Channel{Source: r.SnsEvents}, &handler.EnqueueRequestForObject{}).
		For(&danav1.Subnamespace{}).
		Complete(r)
}

func (r *SubnamespaceReconciler) Sync(ownerNamespace *utils.ObjectContext, subspace *utils.ObjectContext) (ctrl.Result, error) {
	//status usage feature
	namespace, err := utils.NewObjectContext(subspace.Ctx, subspace.Log, subspace.Client, types.NamespacedName{Name: subspace.Object.GetName()}, &corev1.Namespace{})
	if err != nil {
		return ctrl.Result{}, err
	}
	namespaceparent, err := utils.NewObjectContext(subspace.Ctx, subspace.Log, subspace.Client, types.NamespacedName{Name: subspace.Object.GetNamespace()}, &corev1.Namespace{})
	if err != nil {
		return ctrl.Result{}, err
	}
	subspaceparent, err := utils.NewObjectContext(subspace.Ctx, subspace.Log, subspace.Client, types.NamespacedName{Name: namespaceparent.Object.GetName(), Namespace: utils.GetNamespaceParent(namespaceparent.Object)}, &danav1.Subnamespace{})
	if err != nil {
		return ctrl.Result{}, err
	}
	subspaceChilds, err := utils.NewObjectContextList(subspace.Ctx, subspace.Log, subspace.Client, &danav1.SubnamespaceList{}, client.InNamespace(namespace.Object.GetName()))

	if err != nil {
		return ctrl.Result{}, err
	}

	var childrenRequests []danav1.Namespaces
	var allocated = corev1.ResourceList{}
	var free = corev1.ResourceList{}

	for _, sns := range subspaceChilds.Objects.(*danav1.SubnamespaceList).Items {
		var x = danav1.Namespaces{
			Namespace:         sns.GetName(),
			ResourceQuotaSpec: sns.Spec.ResourceQuotaSpec,
		}
		childrenRequests = append(childrenRequests, x)

		snsRequest := sns.Spec.ResourceQuotaSpec.Hard
		for res := range sns.Spec.ResourceQuotaSpec.Hard {
			var (
				totalRequest, _ = allocated[res]
				vRequest, _     = snsRequest[res]
			)
			allocated[res] = *resource.NewQuantity(vRequest.Value()+totalRequest.Value(), resource.BinarySI)
		}
	}

	for res := range subspace.Object.(*danav1.Subnamespace).Spec.ResourceQuotaSpec.Hard {

		var (
			totalRequest, _ = allocated[res]
			vRequest, _     = subspace.Object.(*danav1.Subnamespace).Spec.ResourceQuotaSpec.Hard[res]
		)
		value := vRequest.Value() - totalRequest.Value()
		free[res] = *resource.NewQuantity(value, resource.BinarySI)
	}

	// trigger the child subnamespaces if the upper resourcepool was converted into a subnamespace
	// it will create the crqs and add the respective annotations
	for _, sns := range subspaceChilds.Objects.(*danav1.SubnamespaceList).Items {
		snsChild, err := utils.NewObjectContext(subspace.Ctx, subspace.Log, subspace.Client, types.NamespacedName{Name: sns.GetName(), Namespace: subspace.GetName()}, &danav1.Subnamespace{})
		if err != nil {
			return ctrl.Result{}, err
		}
		if utils.IsChildUpperRp(subspace.Object, snsChild.Object) {
			r.SnsEvents <- event.GenericEvent{Object: &danav1.Subnamespace{
				ObjectMeta: metav1.ObjectMeta{
					Name:      sns.GetName(),
					Namespace: subspace.Object.GetName(),
				},
			}}
		}
	}
	//subspaceparent.UpdateObject(func(object client.Object, log logr.Logger) (client.Object, logr.Logger) {
	//	object.(*danav1.Subnamespace).Status.Phase = danav1.Missing
	//	log = log.WithValues("phase", danav1.Missing)
	//	return object, log
	//})

	// resourcePool Feature
	subspaceCrqName := subspace.Object.GetName()
	subspaceCrq, err := utils.NewObjectContext(subspace.Ctx, subspace.Log, subspace.Client, types.NamespacedName{Name: subspaceCrqName}, &quotav1.ClusterResourceQuota{})
	if err != nil {
		return ctrl.Result{}, err
	}

	subspaceRqName := subspace.Object.GetName()
	composeSubspaceResourceQuota := utils.ComposeResourceQuota(subspaceRqName, subspaceRqName, danav1.Quotahard)
	subspaceResourceQuota, err := utils.NewObjectContext(subspace.Ctx, subspace.Log, subspace.Client, types.NamespacedName{Name: subspaceRqName, Namespace: subspaceRqName}, composeSubspaceResourceQuota)
	if err != nil {
		return ctrl.Result{}, err
	}
	if utils.GetSnsResourcePooled(subspace.Object) == "" {
		if err := setSnsResourcePool(ownerNamespace, subspace); err != nil {
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	if utils.GetNamespaceResourcePooled(ownerNamespace) == "true" && utils.GetSnsResourcePooled(subspace.Object) == "true" {
		if err := subspaceCrq.EnsureDeleteObject(); err != nil {
			return ctrl.Result{}, err
		}
		if err := subspace.UpdateObject(func(object client.Object, log logr.Logger) (client.Object, logr.Logger) {
			object.(*danav1.Subnamespace).Spec.ResourceQuotaSpec = corev1.ResourceQuotaSpec{}
			log = log.WithValues("spec.resourcequotaspec", "removed")
			return object, log
		}); err != nil {
			return ctrl.Result{}, err
		}
	}
	if _, ok := subspace.Object.GetAnnotations()[danav1.IsRq]; !ok {
		if err = subspace.AppendAnnotations(map[string]string{danav1.IsRq: danav1.False}); err != nil {
			return ctrl.Result{}, err
		}
	}
	if err := utils.AppendUpperResourcePoolAnnotation(subspace, subspaceparent); err != nil {
		return ctrl.Result{}, err
	}
	r.addSnsChildNamespaceEvent(subspace)
	if utils.GetSnsResourcePooled(subspace.Object) == "false" || utils.IsRootResourcePool(subspace) {
		// trigger the parent subnamespace in order to update the resourcequota status, if needed
		if subspaceparent.IsPresent() {
			r.SnsEvents <- event.GenericEvent{Object: &danav1.Subnamespace{
				ObjectMeta: metav1.ObjectMeta{
					Name:      subspaceparent.Object.GetName(),
					Namespace: subspaceparent.Object.GetNamespace(),
				},
			}}
		}
		rqFlag, err := utils.IsRq(subspace, danav1.SelfOffset)
		if err != nil {
			return ctrl.Result{}, err
		}
		//check by the annotation if the subnamespace have crq or rq, if it more than write in the annotation so it crq
		if !rqFlag {
			// call to function of sync crq and subnamespace
			_, err := syncCrq(ownerNamespace, subspace, subspaceCrq)
			if err != nil {
				return ctrl.Result{}, err
			}
			if err = subspace.AppendAnnotations(map[string]string{danav1.IsRq: danav1.False}); err != nil {
				return ctrl.Result{}, err
			}
		} else {
			//call to function of sync rq and subnamespace
			_, err := syncRq(subspace, subspaceResourceQuota)
			if err != nil {
				return ctrl.Result{}, err
			}
			if err = subspace.AppendAnnotations(map[string]string{danav1.IsRq: danav1.True}); err != nil {
				return ctrl.Result{}, err
			}
		}
}
  if err := subspace.AppendAnnotations(map[string]string{danav1.DisplayName: utils.GetNamespaceDisplayName(ownerNamespace.Object) + "/" + subspace.Object.GetName()}); err != nil {
		return ctrl.Result{}, err
	}
	if utils.IsUpdateNeeded(subspace.Object, childrenRequests, allocated, free) {
		if err := subspace.UpdateObject(func(object client.Object, log logr.Logger) (client.Object, logr.Logger) {
			object.(*danav1.Subnamespace).Status.Namespaces = childrenRequests
			object.(*danav1.Subnamespace).Status.Total.Allocated = allocated
			object.(*danav1.Subnamespace).Status.Total.Free = free
			return object, log
		}); err != nil {
			return ctrl.Result{}, err
		}
	return ctrl.Result{}, nil
}

func (r *SubnamespaceReconciler) Init(ownerNamespace *utils.ObjectContext, subspace *utils.ObjectContext) error {
	if err := ensureChildNamespace(ownerNamespace, subspace); err != nil {
		return err
	}

	if utils.GetSnsResourcePooled(subspace.Object) == "" {
		if err := setSnsResourcePool(ownerNamespace, subspace); err != nil {
			return err
		}
	}
	if utils.GetSnsResourcePooled(subspace.Object) == "false" || utils.IsRootResourcePool(subspace) {
		rqFlag, err := utils.IsRq(subspace, danav1.SelfOffset)
		if err != nil {
			return err
		}

		// set the annotation of the subnamespace accordingly based if the sns should have a RQ or CRQ
		if !rqFlag {
			_, err := ensureSubspaceCrq(ownerNamespace, subspace)
			// adds annotation to indicate whether the subnamespace has resource quota (&not CRQ)
			if err = subspace.AppendAnnotations(map[string]string{danav1.IsRq: danav1.False}); err != nil {
				return err
			}

		} else {
			_, err := ensureSubspaceRq(subspace)
			// adds annotation to indicate whether the subnamespace has resource quota (&not CRQ)
			if err = subspace.AppendAnnotations(map[string]string{danav1.IsRq: danav1.True}); err != nil {
				return err
			}
		}

	}

	if err := ensureChildResourceQuota(subspace); err != nil {
		return err
	}

	if err := ensureChildLimitRange(subspace); err != nil {
		return err
	}

	if err := subspace.AppendAnnotations(map[string]string{danav1.DisplayName: utils.GetNamespaceDisplayName(ownerNamespace.Object) + "/" + subspace.Object.GetName()}); err != nil {
		return err
	}

	return subspace.UpdateObject(func(object client.Object, log logr.Logger) (client.Object, logr.Logger) {
		object.(*danav1.Subnamespace).Status.Phase = danav1.Created

		log = log.WithValues("phase", danav1.Created)
		return object, log
	})
}

func (r *SubnamespaceReconciler) Setup(ownerNamespace *utils.ObjectContext, subspace *utils.ObjectContext) error {
	//Sets ownerNamespace as subspace owner
	if err := ctrl.SetControllerReference(ownerNamespace.Object, subspace.Object, r.Scheme); err != nil {
		return err
	}

	//Updates the subspace namespaceRef & phase
	return subspace.UpdateObject(func(object client.Object, log logr.Logger) (client.Object, logr.Logger) {
		log = log.WithValues("Phase", danav1.Missing, "namespaceRef", object.(*danav1.Subnamespace).Name)
		object.(*danav1.Subnamespace).Spec.NamespaceRef.Name = object.GetName()
		object.(*danav1.Subnamespace).Status.Phase = danav1.Missing
		return object, log
	})
}

func syncCrq(ownerNamespace *utils.ObjectContext, subspace *utils.ObjectContext, subspaceCrq *utils.ObjectContext) (ctrl.Result, error) {
	//if the crq exists, sync the ResourceQuota with Subns quota spec
	if utils.IsSubspaceCrqExists(subspace) {
		return ctrl.Result{}, syncCrqAndSnsResourceQuotaSpec(subspaceCrq, subspace)
	}
	//If the crq does not exist, create one
	return ensureSubspaceCrq(ownerNamespace, subspace)
}

func syncRq(subspace *utils.ObjectContext, subspaceResourceQuota *utils.ObjectContext) (ctrl.Result, error) {
	//when we want only RQ then we make sure that we don't have a CRQ in the subnamespace and if so - delete it
	if utils.IsSubspaceCrqExists(subspace) {
		subspaceCrqName := subspace.Object.GetName()
		subspaceCrq, err := utils.NewObjectContext(subspace.Ctx, subspace.Log, subspace.Client, types.NamespacedName{Name: subspaceCrqName}, &quotav1.ClusterResourceQuota{})
		if err != nil {
			return ctrl.Result{}, err
		}
		if err := subspaceCrq.EnsureDeleteObject(); err != nil {
			return ctrl.Result{}, err
		}
	}
	//if the rq exists, sync the ResourceQuota with Subns quota spec
	if utils.IsSubspaceRqExists(subspace) {
		return ctrl.Result{}, syncRqAndSnsResourceQuotaSpec(subspaceResourceQuota, subspace)
	}
	//If the rq does not exist, create one
	return ensureSubspaceRq(subspace)
}

func (r *SubnamespaceReconciler) addSnsChildNamespaceEvent(subspace *utils.ObjectContext) {
	r.ResourcePoolEvents <- event.GenericEvent{Object: &corev1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:   subspace.Object.GetName(),
			Labels: map[string]string{danav1.Hns: "true"},
		},
	}}
}

func syncCrqAndSnsResourceQuotaSpec(subspaceCrq *utils.ObjectContext, subspace *utils.ObjectContext) error {
	return subspaceCrq.UpdateObject(func(object client.Object, log logr.Logger) (client.Object, logr.Logger) {
		subspaceCrq.Object.(*quotav1.ClusterResourceQuota).Spec.Quota = utils.GetSnsQuotaSpec(subspace.Object)
		log = log.WithValues("subspaceCrq", "quotaSpec")
		return object, log
	})
}

func syncRqAndSnsResourceQuotaSpec(subspaceRq *utils.ObjectContext, subspace *utils.ObjectContext) error {
	return subspaceRq.UpdateObject(func(object client.Object, log logr.Logger) (client.Object, logr.Logger) {
		subspaceRq.Object.(*corev1.ResourceQuota).Spec = utils.GetSnsQuotaSpec(subspace.Object)
		log = log.WithValues("subspaceRq", "quotaSpec")
		return object, log
	})
}

func setSnsResourcePool(ownerNamespace *utils.ObjectContext, subspace *utils.ObjectContext) error {
	return subspace.UpdateObject(func(object client.Object, log logr.Logger) (client.Object, logr.Logger) {
		resourcePooled := utils.GetNamespaceResourcePooled(ownerNamespace)
		log = log.WithValues(danav1.ResourcePool, resourcePooled)
		object.SetLabels(map[string]string{danav1.ResourcePool: resourcePooled})
		return object, log
	})
}

func ensureChildNamespace(ownerNamespace *utils.ObjectContext, subspace *utils.ObjectContext) error {
	composedChildNamespace := composeChildNamespace(ownerNamespace, subspace)
	childNamespace, err := utils.NewObjectContext(subspace.Ctx, subspace.Log, subspace.Client, types.NamespacedName{Name: subspace.Object.GetName()}, composedChildNamespace)
	if err != nil {
		return err
	}

	if childNamespace.IsPresent() {
		if err := childNamespace.AppendAnnotations(composedChildNamespace.GetAnnotations()); err != nil {
			return err
		}

		if err := childNamespace.AppendLabels(composedChildNamespace.GetLabels()); err != nil {
			return err
		}
	}

	if err := childNamespace.EnsureCreateObject(); err != nil {
		return err
	}
	subspace.Log.Info("child namespace ensured")
	return nil
}

func createZeroedSnsCrq(ownerNamespace *utils.ObjectContext, subspace *utils.ObjectContext) error {
	subspaceCrqName := subspace.Object.GetName()
	composedSubspaceCrq := utils.ComposeCrq(subspaceCrqName, danav1.ZeroedQuota,
		map[string]string{
			danav1.CrqSelector + "-" + strconv.Itoa(utils.GetNamespaceDepth(ownerNamespace.Object)+1): subspaceCrqName,
		},
	)
	snsCrq, err := utils.NewObjectContext(subspace.Ctx, subspace.Log, subspace.Client, types.NamespacedName{Name: subspaceCrqName}, composedSubspaceCrq)
	if err != nil {
		return err
	}
	if err := snsCrq.EnsureCreateObject(); err != nil {
		return err
	}
	return nil
}

func createZeroedSnsRq(subspace *utils.ObjectContext) error {
	subspaceRqName := subspace.Object.GetName()
	composedSubspaceRq := utils.ComposeRq(subspaceRqName, danav1.ZeroedQuota)
	snsRq, err := utils.NewObjectContext(subspace.Ctx, subspace.Log, subspace.Client, types.NamespacedName{Name: subspaceRqName}, composedSubspaceRq)
	if err != nil {
		return err
	}
	if err := snsRq.EnsureCreateObject(); err != nil {
		return err
	}
	return nil
}

func getSnsCrqUsed(subspace *utils.ObjectContext) (corev1.ResourceList, error) {
	snsCrq, err := utils.NewObjectContext(subspace.Ctx, subspace.Log, subspace.Client, types.NamespacedName{Name: subspace.Object.GetName()}, &quotav1.ClusterResourceQuota{})
	if err != nil {
		return nil, err
	}
	return utils.GetQuotaUsed(snsCrq.Object), nil
}

func getSnsRqUsed(subspace *utils.ObjectContext) (corev1.ResourceList, error) {
	snsRq, err := utils.NewObjectContext(subspace.Ctx, subspace.Log, subspace.Client, types.NamespacedName{Name: subspace.Object.GetName()}, &corev1.ResourceQuota{})
	if err != nil {
		return nil, err
	}
	return utils.GetRqUsed(snsRq.Object), nil
}

func updateSnsQuotaSpec(subspace *utils.ObjectContext, spec corev1.ResourceList) error {
	return subspace.UpdateObject(func(object client.Object, log logr.Logger) (client.Object, logr.Logger) {
		log = log.WithValues("updated", "sns resourcequotaspec")
		object.(*danav1.Subnamespace).Spec.ResourceQuotaSpec.Hard = spec
		return object, log
	})
}

func updateCrqQuotaHard(subspace *utils.ObjectContext, spec corev1.ResourceList) error {
	return subspace.UpdateObject(func(object client.Object, log logr.Logger) (client.Object, logr.Logger) {
		log = log.WithValues("updated", "sns crq hard")
		object.(*quotav1.ClusterResourceQuota).Spec.Quota.Hard = spec
		return object, log
	})
}

func updateRqQuotaHard(subspace *utils.ObjectContext, spec corev1.ResourceList) error {
	return subspace.UpdateObject(func(object client.Object, log logr.Logger) (client.Object, logr.Logger) {
		log = log.WithValues("updated", "sns rq hard")
		object.(*corev1.ResourceQuota).Spec.Hard = spec
		return object, log
	})
}

func ensureSubspaceCrq(ownerNamespace *utils.ObjectContext, subspace *utils.ObjectContext) (ctrl.Result, error) {
	subspaceCrqName := subspace.Object.GetName()
	quota := utils.GetSnsQuotaSpec(subspace.Object)
	if len(quota.Hard) == 0 {
		err := createZeroedSnsCrq(ownerNamespace, subspace)
		if err != nil {
			return ctrl.Result{}, err
		}

		crqUsed, err := getSnsCrqUsed(subspace)

		if err != nil {
			return ctrl.Result{}, err
		}

		if crqUsed == nil {
			return ctrl.Result{Requeue: true}, nil
		}
		if err := updateSnsQuotaSpec(subspace, crqUsed); err != nil {
			return ctrl.Result{}, err
		}

		quota = corev1.ResourceQuotaSpec{
			Hard: crqUsed,
		}
	}
	composedSubspaceCrq := utils.ComposeCrq(subspaceCrqName, quota,
		map[string]string{
			danav1.CrqSelector + "-" + strconv.Itoa(utils.GetNamespaceDepth(ownerNamespace.Object)+1): subspaceCrqName,
		},
	)

	subspaceCrq, err := utils.NewObjectContext(subspace.Ctx, subspace.Log, subspace.Client, types.NamespacedName{Name: subspaceCrqName}, composedSubspaceCrq)
	if err != nil {
		return ctrl.Result{}, err
	}

	if utils.IsQuotaObjZeroed(subspaceCrq.Object) {
		if err := updateCrqQuotaHard(subspaceCrq, quota.Hard); err != nil {
			return ctrl.Result{}, err
		}
	}

	if err := subspaceCrq.EnsureCreateObject(); err != nil {
		return ctrl.Result{}, err
	}
	subspace.Log.Info("subspace crq ensured")
	return ctrl.Result{}, nil
}

func ensureSubspaceRq(subspace *utils.ObjectContext) (ctrl.Result, error) {
	subspaceRqName := subspace.Object.GetName()

	quota := utils.GetSnsQuotaSpec(subspace.Object)
	if len(quota.Hard) == 0 {
		err := createZeroedSnsRq(subspace)
		if err != nil {
			return ctrl.Result{}, err
		}

		rqUsed, err := getSnsRqUsed(subspace)
		if err != nil {
			return ctrl.Result{}, err
		}

		if rqUsed == nil {
			return ctrl.Result{Requeue: true}, nil
		}

		if err := updateSnsQuotaSpec(subspace, rqUsed); err != nil {
			return ctrl.Result{}, err
		}

		quota = corev1.ResourceQuotaSpec{
			Hard: rqUsed,
		}
	}
	composeSubspaceResourceQuota := utils.ComposeResourceQuota(subspaceRqName, subspaceRqName, danav1.Quotahard)
	subspaceResourceQuota, err := utils.NewObjectContext(subspace.Ctx, subspace.Log, subspace.Client, types.NamespacedName{Name: subspaceRqName, Namespace: subspaceRqName}, composeSubspaceResourceQuota)
	if err != nil {
		return ctrl.Result{}, err
	}

	if utils.IsRqZeroed(subspaceResourceQuota.Object) {
		if err := updateRqQuotaHard(subspaceResourceQuota, quota.Hard); err != nil {
			return ctrl.Result{}, err
		}
	}

	if err := subspaceResourceQuota.EnsureCreateObject(); err != nil {
		return ctrl.Result{}, err
	}
	subspace.Log.Info("subspace rq ensured")
	return ctrl.Result{}, nil
}

func ensureChildResourceQuota(subspace *utils.ObjectContext) error {
	childName := subspace.Object.GetName()
	composedChildResourceQuota := utils.ComposeResourceQuota(childName, childName, danav1.Quotahard)

	childResourceQuota, err := utils.NewObjectContext(subspace.Ctx, subspace.Log, subspace.Client, types.NamespacedName{Name: childName, Namespace: childName}, composedChildResourceQuota)
	if err != nil {
		return err
	}

	if err := childResourceQuota.EnsureCreateObject(); err != nil {
		return err
	}
	subspace.Log.Info("child resourcequota ensured")
	return nil
}

func ensureChildLimitRange(subspace *utils.ObjectContext) error {
	childName := subspace.Object.GetName()
	composedChildLimitRange := utils.ComposeLimitRange(childName, childName, danav1.Limits)
	childLimitRange, err := utils.NewObjectContext(subspace.Ctx, subspace.Log, subspace.Client, types.NamespacedName{Name: childName, Namespace: childName}, composedChildLimitRange)
	if err != nil {
		return err
	}

	if err := childLimitRange.EnsureCreateObject(); err != nil {
		return err
	}
	subspace.Log.Info("child limitrange ensured")

	return nil
}

func getParentAnnotations(copyFrom *utils.ObjectContext) map[string]string {
	parentAnnotations := map[string]string{}

	ContainsAny := func(key string, annotations []string) bool {
		for _, annotation := range annotations {
			if strings.Contains(key, annotation) {
				return true
			}
		}
		return false
	}

	for key, value := range copyFrom.Object.GetAnnotations() {
		if ContainsAny(key, danav1.DefaultAnnotations) {
			parentAnnotations[key] = value
		}
	}
	return parentAnnotations
}

func getParentAggragators(copyFrom *utils.ObjectContext) map[string]string {
	parentAggragators := map[string]string{}
	for key, value := range copyFrom.Object.GetLabels() {
		if strings.Contains(key, danav1.Aggragator) {
			parentAggragators[key] = value
		}
	}
	return parentAggragators
}

func composeChildNamespace(ownerNamespace *utils.ObjectContext, subspace *utils.ObjectContext) *corev1.Namespace {
	ownerNamespaceDepth := utils.GetNamespaceDepth(ownerNamespace.Object)
	childNamespaceDepth := strconv.Itoa(ownerNamespaceDepth + 1)
	childNamespaceName := subspace.Object.GetName()
	parentDisplayName := utils.GetNamespaceDisplayName(ownerNamespace.Object)
	ownerResourcePooled := utils.GetNamespaceResourcePooled(ownerNamespace)

	ann := getParentAnnotations(ownerNamespace)
	ann[danav1.Depth] = childNamespaceDepth
	ann[danav1.SnsPointer] = childNamespaceName
	ann[danav1.CrqSelector+"-"+childNamespaceDepth] = childNamespaceName
	ann[danav1.DisplayName] = parentDisplayName + "/" + childNamespaceName

	labels := getParentAggragators(ownerNamespace)
	labels[danav1.Aggragator+childNamespaceName] = "true"
	labels[danav1.Hns] = "true"
	labels[danav1.ResourcePool] = ownerResourcePooled
	labels[danav1.Parent] = ownerNamespace.Object.(*corev1.Namespace).Name

	return utils.ComposeNamespace(childNamespaceName, labels, ann)
}
