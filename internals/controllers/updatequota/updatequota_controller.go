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
	"sigs.k8s.io/controller-runtime/pkg/log"
	"strings"
	"time"

	danav1 "github.com/dana-team/hns/api/v1"
	"github.com/dana-team/hns/internals/utils"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// UpdateQuotaReconciler reconciles a UpdateQuota object
type UpdateQuotaReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=dana.hns.io,resources=updatequota,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=dana.hns.io,resources=updatequota/status,verbs=get;update;patch

func (r *UpdateQuotaReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&danav1.Updatequota{}).
		Complete(r)
}

func (r *UpdateQuotaReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx).WithName("controllers").WithName("UpdateQuota").WithValues("upq", req.NamespacedName)
	logger.Info("starting to reconcile")

	upqObject, err := utils.NewObjectContext(ctx, r.Client, client.ObjectKey{Name: req.Name, Namespace: req.Namespace}, &danav1.Updatequota{})
	if err != nil {
		return ctrl.Result{}, fmt.Errorf("failed to get object '%s': "+err.Error(), upqObject.Object.GetName())
	}

	if !upqObject.IsPresent() {
		logger.Info("resource not found. Ignoring since object must be deleted")
		return ctrl.Result{}, nil
	}

	phase := upqObject.Object.(*danav1.Updatequota).Status.Phase
	if utils.ShouldReconcile(phase) {
		return ctrl.Result{}, r.reconcile(upqObject)
	} else {
		logger.Info("no need to reconcile, object phase is: ", "phase", phase)
	}

	return ctrl.Result{}, nil
}

func (r *UpdateQuotaReconciler) reconcile(upqObject *utils.ObjectContext) error {
	ctx := upqObject.Ctx

	sourceNSName := upqObject.Object.(*danav1.Updatequota).Spec.SourceNamespace
	sourceNS, err := utils.NewObjectContext(ctx, r.Client, client.ObjectKey{Name: sourceNSName}, &corev1.Namespace{})
	if err != nil {
		return fmt.Errorf("failed to get object '%s': "+err.Error(), sourceNSName)
	}

	destNSName := upqObject.Object.(*danav1.Updatequota).Spec.DestNamespace
	destNS, err := utils.NewObjectContext(ctx, r.Client, client.ObjectKey{Name: destNSName}, &corev1.Namespace{})
	if err != nil {
		return fmt.Errorf("failed to get object '%s': "+err.Error(), destNSName)
	}

	// get the Ancestor namespace of the source and destination namespaces. The Ancestor namespace is the
	// first namespace the two namespaces have in common in their hierarchy. There are several cases and
	// each case is treated differently based on the Ancestor namespace
	sourceNSSliced := utils.GetNSDisplayNameSlice(sourceNS)
	destNSSliced := utils.GetNSDisplayNameSlice(destNS)

	ancestorNSName, _, err := utils.GetAncestor(sourceNSSliced, destNSSliced)
	if err != nil {
		return fmt.Errorf("failed to find ancestor namespace of '%s' and '%s': "+err.Error(), sourceNSSliced, destNSSliced)
	}

	if isNSAncestor(sourceNSName, ancestorNSName) {
		er := moveResourcesDown(ancestorNSName, destNS, upqObject)
		if er != nil {
			err := updateUPQErrorStatus(upqObject, er)
			if err != nil {
				return fmt.Errorf("failed updating the status of object '%s': "+err.Error(), upqObject.GetName())
			}
			return fmt.Errorf("failed move resources down: " + er.Error())
		}
	} else if isNSAncestor(destNSName, ancestorNSName) {
		er := moveResourcesUp(ancestorNSName, sourceNS, upqObject)
		if er != nil {
			err := updateUPQErrorStatus(upqObject, er)
			if err != nil {
				return fmt.Errorf("failed updating the status of object '%s': "+err.Error(), upqObject.GetName())
			}
			return fmt.Errorf("failed move resources up: " + er.Error())
		}
	} else {
		if er := moveBetweenBranches(ancestorNSName, upqObject, sourceNS, destNS); err != nil {
			err := updateUPQErrorStatus(upqObject, er)
			if err != nil {
				return fmt.Errorf("failed updating the status of object '%s': "+err.Error(), upqObject.GetName())
			}
			return fmt.Errorf("failed to move resources between branches: " + er.Error())
		}
	}

	err = r.updateUPQSuccessStatus(upqObject)
	if err != nil {
		return fmt.Errorf("failed updating the status of object '%s': "+err.Error(), upqObject.GetName())
	}

	return nil
}

// isNSAncestor returns true if the namespace and ancestor are the same
func isNSAncestor(namespace, ancestor string) bool {
	return namespace == ancestor
}

// moveBetweenBranches moves resources when ancestor namespace is not the source
// namespace and not the destination namespace. In this case resources need to be moved to the
// ancestor from the source up a branch and then from the ancestor to the destination down the branch
func moveBetweenBranches(ancestorNSName string, upqObject, sourceNS, destNS *utils.ObjectContext) error {
	er := moveResourcesUp(ancestorNSName, sourceNS, upqObject)
	if er != nil {
		err := updateUPQErrorStatus(upqObject, er)
		if err != nil {
			return fmt.Errorf("failed updating the status of object '%s': "+err.Error(), upqObject.GetName())
		}
		return fmt.Errorf("failed move resources up: " + er.Error())
	}

	er = moveResourcesDown(ancestorNSName, destNS, upqObject)
	if er != nil {
		err := updateUPQErrorStatus(upqObject, er)
		if err != nil {
			return fmt.Errorf("failed updating the status of object '%s': "+err.Error(), upqObject.GetName())
		}
		return fmt.Errorf("failed move resources down: " + er.Error())
	}

	return nil
}

// updateUPQErrorStatus updates the status of the UPQ object in case of an error
func updateUPQErrorStatus(upqObject *utils.ObjectContext, er error) error {
	err := upqObject.UpdateObject(func(object client.Object, l logr.Logger) (client.Object, logr.Logger) {
		object.(*danav1.Updatequota).Status.Phase = danav1.Error
		object.(*danav1.Updatequota).Status.Reason = er.Error()
		return object, l
	})

	return err
}

// updateUPQSuccessStatus updates the status of the UPQ object in case of a success
func (r *UpdateQuotaReconciler) updateUPQSuccessStatus(upqObject *utils.ObjectContext) error {
	err := upqObject.UpdateObject(func(object client.Object, l logr.Logger) (client.Object, logr.Logger) {
		object.(*danav1.Updatequota).Status.Phase = danav1.Complete
		object.(*danav1.Updatequota).Status.Reason = ""
		return object, l
	})

	return err
}

// moveResourcesDown moves the ResourceQuota specified in the `upqObject` to all subnamespaces
// that descend from the `ancestorNS` namespace to the `ns` namespace
func moveResourcesDown(ancestorNS string, ns, upqObject *utils.ObjectContext) error {
	logger := upqObject.Log

	snsListDown, err := getSnsListDown(ancestorNS, ns)
	if err != nil {
		return err
	}

	resourcesToAdd := upqObject.Object.(*danav1.Updatequota).Spec.ResourceQuotaSpec
	for i := 0; i < len(snsListDown); i++ {
		err := addSnsQuota(snsListDown[i], resourcesToAdd)
		if err != nil {
			return fmt.Errorf("updating the quota down the hierarchy failed at namespace '%s': %s", snsListDown[i].GetName(), err.Error())
		}
		logger.Info("successfully added resources to subnamespace", "subnamespace", snsListDown[i].GetName(), "resources", resourcesToAdd.Hard)
	}

	return nil
}

// moveResourcesUp moves the ResourceQuota specified in the `upqObject` to all subnamespaces
// that ascend from the `ns` namespace to the `ancestorNS` namespace
func moveResourcesUp(ancestorNS string, ns, upqObject *utils.ObjectContext) error {
	logger := upqObject.Log

	snsListUp, err := getSnsListUp(ns, ancestorNS)
	if err != nil {
		return err
	}

	resourcesToSub := upqObject.Object.(*danav1.Updatequota).Spec.ResourceQuotaSpec
	for i := 0; i < len(snsListUp); i++ {
		err := subSnsQuota(snsListUp[i], resourcesToSub)
		if err != nil {
			return fmt.Errorf("updating the quota up the hierarchy failed at namespace '%s': %s", snsListUp[i].GetName(), err.Error())
		}
		logger.Info("successfully subtracted resources from subnamespace", "subnamespace", snsListUp[i].GetName(), "resources", resourcesToSub)
	}
	return nil
}

// getSnsListDown creates a slice of all subnamespaces in the hierarchy from `ancestorNS` to `ns`
func getSnsListDown(ancestorNS string, ns *utils.ObjectContext) ([]*utils.ObjectContext, error) {
	displayName := ns.Object.GetAnnotations()[danav1.DisplayName]
	namespaces := strings.Split(displayName, "/")

	index, err := utils.IndexOf(ancestorNS, namespaces)
	if err != nil {
		return nil, err
	}
	snsArr := namespaces[index:]

	var snsList []*utils.ObjectContext
	for i := 1; i < len(snsArr); i++ {
		sns, err := utils.NewObjectContext(ns.Ctx, ns.Client, client.ObjectKey{Name: snsArr[i], Namespace: snsArr[i-1]}, &danav1.Subnamespace{})
		if err != nil {
			return nil, err
		}
		snsList = append(snsList, sns)
	}

	return snsList, nil
}

// getSnsListUp creates a slice of all subnamespaces in the hierarchy from `ns` to `ancestorNS`
func getSnsListUp(ns *utils.ObjectContext, ancestorNS string) ([]*utils.ObjectContext, error) {
	displayName := ns.Object.GetAnnotations()[danav1.DisplayName]
	namespaces := strings.Split(displayName, "/")

	index, err := utils.IndexOf(ancestorNS, namespaces)
	if err != nil {
		return nil, err
	}
	snsArr := namespaces[index:]

	var snsList []*utils.ObjectContext
	for i := len(snsArr) - 1; i >= 1; i-- {
		sns, err := utils.NewObjectContext(ns.Ctx, ns.Client, client.ObjectKey{Name: snsArr[i], Namespace: snsArr[i-1]}, &danav1.Subnamespace{})
		if err != nil {
			return nil, err
		}
		snsList = append(snsList, sns)
	}

	return snsList, nil
}

// addSnsQuota updates the resource quota for a subnamespace by adding the quota specified
// in quotaSpec to the existing quota. It retrieves the existing quota from the subnamespace object and
// loops through each resource, adding the requested quota if it is specified
func addSnsQuota(sns *utils.ObjectContext, quotaSpec corev1.ResourceQuotaSpec) error {
	err := sns.EnsureUpdateObject(func(object client.Object, l logr.Logger) (client.Object, logr.Logger, error) {
		snsQuota := object.(*danav1.Subnamespace).Spec.ResourceQuotaSpec
		for resourceName := range snsQuota.Hard {
			var (
				before, _  = snsQuota.Hard[resourceName]
				request, _ = quotaSpec.Hard[resourceName]
			)
			before.Set(before.Value() + request.Value())
			object.(*danav1.Subnamespace).Spec.ResourceQuotaSpec.Hard[resourceName] = before
		}
		return object, l, nil
	}, false)

	if err != nil {
		return err
	}

	// since the update of the subnamespace spec done above triggers reconciliation for the subnamespace,
	// it is needed to make sure that its reconciliation completes successfully before continuing;
	// a race condition can be created if this is not ensured, potentially causing the UpdateQuota to fail
	err = ensureSnsEqualQuota(sns)
	if err != nil {
		return err
	}

	return nil
}

// addSnsQuota updates the resource quota for a subnamespace by subtracting the quota specified
// in quotaSpec from the existing quota. It retrieves the existing quota from the subnamespace object and
// loops through each resource, subtracting the requested quota if it is specified
func subSnsQuota(sns *utils.ObjectContext, quotaSpec corev1.ResourceQuotaSpec) error {
	err := sns.EnsureUpdateObject(func(object client.Object, l logr.Logger) (client.Object, logr.Logger, error) {
		snsQuota := object.(*danav1.Subnamespace).Spec.ResourceQuotaSpec
		for res := range snsQuota.Hard {
			var (
				vBefore, _  = snsQuota.Hard[res]
				vRequest, _ = quotaSpec.Hard[res]
			)
			vBefore.Set(vBefore.Value() - vRequest.Value())
			object.(*danav1.Subnamespace).Spec.ResourceQuotaSpec.Hard[res] = vBefore
		}
		return object, l, nil
	}, false)

	if err != nil {
		return err
	}

	// since the update of the subnamespace spec done above triggers reconciliation for the subnamespace,
	// it is needed to make sure that its reconciliation completes successfully before continuing;
	// a race condition can be created if this is not ensured, potentially causing the UpdateQuota to fail
	err = ensureSnsEqualQuota(sns)
	if err != nil {
		return err
	}

	return nil
}

// ensureSnsEqualQuota compares the sns quota spec and the resource quota spec in a loop until they are equal,
// this way we can know that the subnamespace has been properly updated before doing the updatequota operation
func ensureSnsEqualQuota(sns *utils.ObjectContext) error {
	ok := false
	retries := 0

	snsQuotaSpec := sns.Object.(*danav1.Subnamespace).Spec.ResourceQuotaSpec

	// To avoid an infinite loop in case of an actual failure, the loop runs at most a MAX_RETRIES number times
	for (!ok) && (retries < danav1.MaxRetries) {
		ok = true
		quotaObject, err := utils.GetSNSQuotaObject(sns)
		if err != nil {
			return err
		}
		resourceQuotaSpec := utils.GetQuotaObjectSpec(quotaObject.Object)
		for res := range resourceQuotaSpec.Hard {
			if snsQuotaSpec.Hard[res] != resourceQuotaSpec.Hard[res] {
				ok = false
			}
		}
		// wait between iterations because we don't want to overload the API with many requests
		time.Sleep(danav1.SleepTimeout * time.Millisecond)
		retries++
	}
	return nil
}
