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
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=dana.hns.io,resources=updatequota,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=dana.hns.io,resources=updatequota/status,verbs=get;update;patch

func (r *UpdateQuotaReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = context.Background()
	log := r.Log.WithValues("updatequota", req.NamespacedName)
	log.Info("starting to reconcile")

	// your logic here
	updatingObject, err := utils.NewObjectContext(ctx, log, r.Client, client.ObjectKey{Name: req.Name, Namespace: req.Namespace}, &danav1.Updatequota{})
	if err != nil {
		return ctrl.Result{}, err
	}

	if updatingObject.Object.(*danav1.Updatequota).Status.Phase != danav1.Complete && updatingObject.Object.(*danav1.Updatequota).Status.Phase != danav1.Error {

		sourcens, err := utils.NewObjectContext(ctx, log, r.Client, client.ObjectKey{Name: updatingObject.Object.(*danav1.Updatequota).Spec.SourceNamespace}, &corev1.Namespace{})
		if err != nil {
			return ctrl.Result{}, err
		}
		destns, err := utils.NewObjectContext(ctx, log, r.Client, client.ObjectKey{Name: updatingObject.Object.(*danav1.Updatequota).Spec.DestNamespace}, &corev1.Namespace{})
		if err != nil {
			return ctrl.Result{}, err
		}

		rootns, err := r.getAncestor(sourcens, destns)

		if sourcens.Object.GetName() == rootns { //update from root to leaf

			snslistdown, err := r.getSnsListDown(destns, rootns)
			if err != nil {
				return ctrl.Result{}, err
			}
			for i := 0; i < len(snslistdown); i++ {
				err := addSnsQuota(snslistdown[i], updatingObject.Object.(*danav1.Updatequota).Spec.ResourceQuotaSpec)
				if err != nil {
					updatingObject.UpdateObject(func(object client.Object, log logr.Logger) (client.Object, logr.Logger) {
						object.(*danav1.Updatequota).Status.Phase = danav1.Error
						object.(*danav1.Updatequota).Status.Reason = "Updating the quota down the hierarchy failed at namespace " + snslistdown[i].GetName() + "\n" + err.Error()
						log = log.WithValues("phase", danav1.Error)
						return object, log
					})
					return ctrl.Result{}, err
				}
			}
		} else if destns.Object.GetName() == rootns { // update from leaf to root

			snslistup, err := utils.GetSnsListUp(sourcens, rootns, r.Client, r.Log)
			if err != nil {
				return ctrl.Result{}, err
			}
			for i := 0; i < len(snslistup); i++ {
				err := subSnsQuota(snslistup[i], updatingObject.Object.(*danav1.Updatequota).Spec.ResourceQuotaSpec)
				if err != nil {
					updatingObject.UpdateObject(func(object client.Object, log logr.Logger) (client.Object, logr.Logger) {
						object.(*danav1.Updatequota).Status.Phase = danav1.Error
						object.(*danav1.Updatequota).Status.Reason = "Updating the quota up the hierarchy failed at namespace " + snslistup[i].GetName() + "\n" + err.Error()
						log = log.WithValues("phase", danav1.Error)
						return object, log
					})
					return ctrl.Result{}, err
				}
			}
		} else { //update from leaf to root

			snslistup, err := utils.GetSnsListUp(sourcens, rootns, r.Client, r.Log)
			if err != nil {
				return ctrl.Result{}, err
			}
			for i := 0; i < len(snslistup); i++ {
				err := subSnsQuota(snslistup[i], updatingObject.Object.(*danav1.Updatequota).Spec.ResourceQuotaSpec)
				if err != nil {
					updatingObject.UpdateObject(func(object client.Object, log logr.Logger) (client.Object, logr.Logger) {
						object.(*danav1.Updatequota).Status.Phase = danav1.Error
						object.(*danav1.Updatequota).Status.Reason = "Updating the quota up the hierarchy failed at namespace " + snslistup[i].GetName() + "\n" + err.Error()
						log = log.WithValues("phase", danav1.Error)
						return object, log
					})
					return ctrl.Result{}, err
				}
			}
			// update root to leaf
			snslistdown, err := r.getSnsListDown(destns, rootns)
			if err != nil {
				return ctrl.Result{}, err
			}
			for i := 0; i < len(snslistdown); i++ {
				err := addSnsQuota(snslistdown[i], updatingObject.Object.(*danav1.Updatequota).Spec.ResourceQuotaSpec)
				if err != nil {
					updatingObject.UpdateObject(func(object client.Object, log logr.Logger) (client.Object, logr.Logger) {
						object.(*danav1.Updatequota).Status.Phase = danav1.Error
						object.(*danav1.Updatequota).Status.Reason = "Updating the quota down the hierarchy failed at namespace " + snslistdown[i].GetName() + "\n" + err.Error()
						log = log.WithValues("phase", danav1.Error)
						return object, log
					})
					return ctrl.Result{}, err
				}
			}
		}

		updatingObject.UpdateObject(func(object client.Object, log logr.Logger) (client.Object, logr.Logger) {
			object.(*danav1.Updatequota).Status.Phase = danav1.Complete
			object.(*danav1.Updatequota).Status.Reason = ""
			log = log.WithValues("phase", danav1.Complete)
			return object, log
		})
	}
	return ctrl.Result{}, nil
}

func (r *UpdateQuotaReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&danav1.Updatequota{}).
		Complete(r)
}

func (r *UpdateQuotaReconciler) getAncestor(sourcens *utils.ObjectContext, destns *utils.ObjectContext) (string, error) {
	sourceDisplayName := sourcens.Object.GetAnnotations()["openshift.io/display-name"]
	destDisplayName := destns.Object.GetAnnotations()["openshift.io/display-name"]

	sourceArr := strings.Split(sourceDisplayName, "/")
	destArr := strings.Split(destDisplayName, "/")

	for i := len(sourceArr) - 1; i >= 0; i-- {
		for j := len(destArr) - 1; j >= 0; j-- {
			if sourceArr[i] == destArr[j] {
				return sourceArr[i], nil
			}
		}
	}

	return "", fmt.Errorf("did not find root ns")
}

func (r *UpdateQuotaReconciler) getSnsListDown(ns *utils.ObjectContext, rootns string) ([]*utils.ObjectContext, error) {

	var snsList []*utils.ObjectContext

	displayName := ns.Object.GetAnnotations()["openshift.io/display-name"]
	nsArr := strings.Split(displayName, "/")
	index, err := utils.IndexOf(rootns, nsArr)
	if err != nil {
		return nil, err
	}
	snsArr := nsArr[index:]

	for i := 1; i < len(snsArr); i++ {
		sns, err := utils.NewObjectContext(context.Background(), r.Log.WithValues("get sns list", ""), r.Client, client.ObjectKey{Name: snsArr[i], Namespace: snsArr[i-1]}, &danav1.Subnamespace{})
		if err != nil {
			return nil, err
		}
		snsList = append(snsList, sns)
	}

	return snsList, nil
}

func addSnsQuota(sns *utils.ObjectContext, quotaSpec corev1.ResourceQuotaSpec) error {
	err := sns.EnsureUpdateObject(func(object client.Object, log logr.Logger) (client.Object, logr.Logger, error) {
		log = log.WithValues("subspace", "add snsQuotaSpec")
		snsquota := object.(*danav1.Subnamespace).Spec.ResourceQuotaSpec
		for res := range snsquota.Hard {
			var (
				vBefore, _  = snsquota.Hard[res]
				vRequest, _ = quotaSpec.Hard[res]
			)
			vBefore.Set(vBefore.Value() + vRequest.Value())
			object.(*danav1.Subnamespace).Spec.ResourceQuotaSpec.Hard[res] = vBefore
		}
		return object, log, nil
	}, false)
	if err != nil {
		return err
	}

	err = ensureSnsEqualQuota(sns)
	if err != nil {
		return err
	}
	return nil
}

func subSnsQuota(sns *utils.ObjectContext, quotaSpec corev1.ResourceQuotaSpec) error {
	err := sns.EnsureUpdateObject(func(object client.Object, log logr.Logger) (client.Object, logr.Logger, error) {
		log = log.WithValues("subspace", "sub snsQuotaSpec")
		snsquota := object.(*danav1.Subnamespace).Spec.ResourceQuotaSpec
		for res := range snsquota.Hard {
			var (
				vBefore, _  = snsquota.Hard[res]
				vRequest, _ = quotaSpec.Hard[res]
			)
			vBefore.Set(vBefore.Value() - vRequest.Value())
			object.(*danav1.Subnamespace).Spec.ResourceQuotaSpec.Hard[res] = vBefore
		}
		return object, log, nil
	}, false)
	if err != nil {
		return err
	}

	err = ensureSnsEqualQuota(sns)
	if err != nil {
		return err
	}
	return nil
}

// ensureSnsEqualQuota compares the sns quota spec and the resource quota spec
// in a loop until they are equal. this way we can know that the subnamespace
// has been properly updated before doing the updatequota operation
func ensureSnsEqualQuota(sns *utils.ObjectContext) error {
	ok := false
	quotaObj, err := utils.GetSNSQuotaObj(sns)
	if err != nil {
		return err
	}
	snsQuotaSpec := sns.Object.(*danav1.Subnamespace).Spec.ResourceQuotaSpec
	for !ok {
		resourceQuotaSpec := utils.GetQuotaObjSpec(quotaObj.Object)
		for res := range resourceQuotaSpec.Hard {
			if snsQuotaSpec.Hard[res] != resourceQuotaSpec.Hard[res] {
				ok = false
			}
		}
		ok = true
		// we wait between iterations because we don't want to overload the API with many requests
		time.Sleep(500 * time.Millisecond)
	}
	return nil
}
