package webhooks

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	danav1 "github.com/dana-team/hns/api/v1"
	"github.com/dana-team/hns/internals/namespaceDB"
	"github.com/dana-team/hns/internals/utils"
	"github.com/go-logr/logr"
	qoutav1 "github.com/openshift/api/quota/v1"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

type SubNamespaceAnnotator struct {
	Client      client.Client
	Decoder     *admission.Decoder
	Log         logr.Logger
	NamespaceDB *namespaceDB.NamespaceDB
}

// +kubebuilder:webhook:path=/validate-v1-subnamespace,mutating=false,sideEffects=NoneOnDryRun,failurePolicy=fail,groups="dana.hns.io",resources=subnamespaces,verbs=create;update,versions=v1,name=subnamespace.dana.io,admissionReviewVersions=v1;v1beta1

func (a *SubNamespaceAnnotator) Handle(ctx context.Context, req admission.Request) admission.Response {
	log := a.Log.WithValues("webhook", "SubNamespace Webhook", "Name", req.Name)
	log.Info("webhook request received")

	//Decode sns object
	sns, err := utils.NewObjectContext(ctx, log, a.Client, types.NamespacedName{}, &danav1.Subnamespace{})
	if err != nil {
		log.Error(err, "unable to create sns objectContext")
	}
	if err := a.Decoder.DecodeRaw(req.Object, sns.Object); err != nil {
		log.Error(err, "could not decode sns object")
		return admission.Errored(http.StatusBadRequest, err)
	}

	parentQuotaObj, err := getSnsParentQuotaObj(sns)
	if err != nil {
		log.Error(err, "unable to get parent quota object")
		return admission.Denied(err.Error() + sns.Object.GetName())
	}

	clusterName, err := utils.GetClusterName(sns.Ctx, sns.Log, sns.Client)
	if err != nil {
		log.Error(err, "unable to get cluster name")
		return admission.Denied(denyCannotGetClusterName)
	}

	if req.Operation == admissionv1.Create {
		//Check if max subnamespaces in hierarchy has been reached
		key := a.NamespaceDB.GetKey(sns.Object.GetNamespace())
		if key != "" {
			if a.NamespaceDB.GetKeyCount(key) >= danav1.MaxSNS {
				return admission.Denied(fmt.Sprintf(denyMessageCreatingMoreThanLimit, danav1.MaxSNS) + key)
			}
		}
		//Check if sns namespace exists
		if isExists, err := isSnsChildNamespaceExists(sns); err != nil {
			log.Error(err, "unable to get sns child namespace")
			return admission.Denied(err.Error())
		} else if isExists {
			return admission.Denied(namespaceExistsMessage)
		}

		if validChange, err := createSubNamespaceWhenParentResourcePool(sns); err != nil {
			return admission.Errored(http.StatusInternalServerError, err)
		} else if !validChange {
			return admission.Denied(denyMessageCreateResourcePool)
		}

		if utils.GetSnsResourcePooled(sns.Object) == "false" {
			if isValid, err := ValidateAllResourceQuotaParamsValid(sns); !isValid {
				return admission.Denied(err.Error())
			}
		}

		//validate the sns creation
		if validRequest, result := ValidateCreateSnsRequest(sns, parentQuotaObj); !validRequest {
			return admission.Denied(denyMessageValidateQuotaObj + result)
		}

		if !utils.UsernameToFilter(req.UserInfo.Username) {
			//Write relevant log to elastic
			webhookLog := utils.NewWebhookLog(time.Now(), sns.Object.GetObjectKind().GroupVersionKind().Kind,
				sns.Object.GetNamespace(), utils.Create, req.UserInfo.Username,
				"sns created", utils.GetSnsQuotaSpec(sns.Object).Hard, clusterName)
			err := webhookLog.UploadLogToElastic()
			if err != nil {
				log.Error(err, "unable to upload log to elastic")
			}
		}
		return admission.Allowed(allowMessageValidateQuotaObj)
	}

	if req.Operation == admissionv1.Update {

		//Decode oldSns object
		oldSns, err := utils.NewObjectContext(ctx, log, a.Client, types.NamespacedName{}, &danav1.Subnamespace{})
		if err != nil {
			log.Error(err, "unable to get oldSns")
		}
		if err := a.Decoder.DecodeRaw(req.OldObject, oldSns.Object); err != nil {
			log.Error(err, "could not decode object")
			return admission.Errored(http.StatusBadRequest, err)
		}
		oldVal := oldSns.Object.GetAnnotations()[danav1.IsRq]
		if oldVal != "" {
			newVal, ok := sns.Object.GetAnnotations()[danav1.IsRq]
			if !ok || newVal != oldVal {
				return admission.Denied(fmt.Sprintf(denyMessageImmutableAnnotation, danav1.IsRq))
			}
		}
		if resourcePoolLabelDeleted(sns, oldSns) {
			return admission.Denied(denyMessageDeleteResourcePool)
		}

		if validChange, err := snsResourcePoolChangedWhenParentIsResourcePool(oldSns, sns); err != nil {
			return admission.Errored(http.StatusInternalServerError, err)
		} else if !validChange {
			return admission.Denied(denyMessageUpdateResourcePool)
		}

		if validChange, err := oneOfSnsDescendantIsResourcePool(oldSns, sns); err != nil {
			return admission.Errored(http.StatusInternalServerError, err)
		} else if !validChange {
			return admission.Denied(denyMessageUpdateResourcePoolDescendant)
		}

		if isExists, err := isSnsQuotaObjExists(sns); err != nil {
			log.Error(err, "unable to get sns quota object")
			return admission.Denied(err.Error())
		} else if !isExists {
			if resourceName := snsChangedObject(oldSns, sns); len(resourceName) > 0 {
				if !utils.UsernameToFilter(req.UserInfo.Username) {
					//Write relevant log to elastic
					webhookLog := utils.NewWebhookLog(time.Now(), sns.Object.GetObjectKind().GroupVersionKind().Kind,
						sns.Object.GetNamespace(), utils.Edit, req.UserInfo.Username,
						fmt.Sprintf("%+q amount changed", resourceName), utils.GetSnsQuotaSpec(sns.Object).Hard, clusterName)
					err := webhookLog.UploadLogToElastic()
					if err != nil {
						log.Error(err, "unable to upload log to elastic")
					}
				}
			}
			return admission.Allowed(allowMessageValidateQuotaObj)
		}

		if err := IsMinResources(sns); err != nil {
			return admission.Denied(err.Error())
		}

		myQuotaObj, err := getSnsQuotaObj(sns)
		if err != nil {
			sns.Log.Error(err, "unable to get sns quota object")
			return admission.Denied(err.Error())
		}

		if !myQuotaObj.IsPresent() {
			return admission.Allowed(allowMessageValidateQuotaObj)
		}

		if utils.GetSnsResourcePooled(sns.Object) == "false" {
			if isValid, err := ValidateAllResourceQuotaParamsValid(sns); !isValid {
				return admission.Denied(err.Error())
			}
		}

		isRp, _ := sns.Object.GetLabels()[danav1.ResourcePool]
		isUpperRp, _ := sns.Object.GetAnnotations()[danav1.IsUpperRp]
		// Only check the validity of the request if the subnamespace is not a ResourcePool OR the subnamespace is the upper ResourcePool
		// this is because only in that case the subnamespace would have a RQ or CRQ attached to it.
		// Otherwise, the subnamespace is part of a ResourcePool (and does not have a RQ/CRQ attached to it) and hence this check is unneeded.
		if isRp != "true" || isUpperRp == danav1.True {
			if err := ValidateUpdateSnsRequest(parentQuotaObj, sns, oldSns, myQuotaObj); err != nil {
				return admission.Denied(err.Error())
			}
		}

		if resourceName := snsChangedObject(oldSns, sns); len(resourceName) > 0 {
			if !utils.UsernameToFilter(req.UserInfo.Username) {
				//Write relevant log to elastic
				webhookLog := utils.NewWebhookLog(time.Now(), sns.Object.GetObjectKind().GroupVersionKind().Kind,
					sns.Object.GetNamespace(), utils.Edit, req.UserInfo.Username,
					fmt.Sprintf("%+q amount changed", resourceName), utils.GetSnsQuotaSpec(sns.Object).Hard, clusterName)
				err := webhookLog.UploadLogToElastic()
				if err != nil {
					log.Error(err, "unable to upload log to elastic")
				}
			}
		}
		return admission.Allowed(allowMessageValidateQuotaObj)
	}
	return admission.Denied(contactMessage)
}

func resourcePoolLabelDeleted(newSns *utils.ObjectContext, oldSns *utils.ObjectContext) bool {
	return utils.GetSnsResourcePooled(newSns.Object) == "" && utils.GetSnsResourcePooled(oldSns.Object) != ""
}

func createSubNamespaceWhenParentResourcePool(newSns *utils.ObjectContext) (bool, error) {
	snsParentNamespace, err := utils.NewObjectContext(newSns.Ctx, newSns.Log, newSns.Client, types.NamespacedName{Name: newSns.Object.GetNamespace()}, &corev1.Namespace{})
	if err != nil {
		return false, err
	}

	if utils.GetNamespaceResourcePooled(snsParentNamespace) == "true" && utils.GetSnsResourcePooled(newSns.Object) == "false" {
		return false, nil
	}
	return true, nil
}

func snsChangedObject(oldSns *utils.ObjectContext, newSns *utils.ObjectContext) []string {
	oldSnsResourceQuota := utils.GetSnsQuotaSpec(oldSns.Object).Hard
	newSnsResourceQuota := utils.GetSnsQuotaSpec(newSns.Object).Hard
	var changedObjects []string
	for resourceName, newQuantity := range newSnsResourceQuota {
		if oldQuantity, ok := oldSnsResourceQuota[resourceName]; ok {
			if oldQuantity != newQuantity {
				changedObjects = append(changedObjects, resourceName.String())
			}
		}
	}
	return changedObjects
}

func snsResourcePoolChangedWhenParentIsResourcePool(oldSns *utils.ObjectContext, newSns *utils.ObjectContext) (bool, error) {
	snsParentNamespace, err := utils.NewObjectContext(newSns.Ctx, newSns.Log, newSns.Client, types.NamespacedName{Name: newSns.Object.GetNamespace()}, &corev1.Namespace{})
	if err != nil {
		return false, err
	}

	if utils.GetNamespaceResourcePooled(snsParentNamespace) == "true" {
		if utils.GetSnsResourcePooled(newSns.Object) == "false" && utils.GetSnsResourcePooled(oldSns.Object) == "true" {
			return false, nil
		}
	}
	return true, nil
}

func oneOfSnsDescendantIsResourcePool(oldSns *utils.ObjectContext, newSns *utils.ObjectContext) (bool, error) {
	if utils.GetSnsResourcePooled(newSns.Object) == utils.GetSnsResourcePooled(oldSns.Object) || utils.GetSnsResourcePooled(oldSns.Object) == "true" {
		return true, nil
	}

	namespaceList, err := utils.NewObjectContextList(newSns.Ctx, newSns.Log, newSns.Client, &corev1.NamespaceList{}, client.MatchingLabels{danav1.Aggragator + newSns.Object.GetName(): "true"})
	if err != nil {
		return false, err
	}

	for _, namespace := range namespaceList.Objects.(*corev1.NamespaceList).Items {
		if namespace.Name != newSns.Object.GetName() {
			namespace, err := utils.NewObjectContext(newSns.Ctx, newSns.Log, newSns.Client, types.NamespacedName{Name: namespace.GetName()}, &corev1.Namespace{})
			if err != nil {
				return false, err
			}
			if utils.GetNamespaceResourcePooled(namespace) == "true" {
				return false, nil
			}
		}
	}
	return true, nil
}

func ValidateCreateSnsRequest(sns *utils.ObjectContext, parentQuotaObj *utils.ObjectContext) (bool, string) {
	quotaParent := utils.GetQuotaObjSpec(parentQuotaObj.Object).Hard
	quotaSNS := utils.GetSnsQuotaSpec(sns.Object).Hard
	siblingsResources := utils.GetQuotaObjsListResources(getSnsSiblingQuotaObjs(sns))

	for res := range quotaParent {
		var (
			vSiblings, _ = siblingsResources[res]
			vParent, _   = quotaParent[res]
			vRequest, _  = quotaSNS[res]
		)

		vParent.Sub(vSiblings)
		vParent.Sub(vRequest)
		if vParent.Value() < 0 {
			return false, res.String()

		}
	}
	return true, ""
}

// ValidateAllResourceQuotaParamsValid validates that all quota params: storage, cpu, memory, gpu exists and are positive
// if one of the params is not valid, the function returns false with the relevant error
func ValidateAllResourceQuotaParamsValid(sns *utils.ObjectContext) (bool, error) {
	quotaSNS := utils.GetSnsQuotaSpec(sns.Object).Hard
	resourceQuotaParams := danav1.ZeroedQuota.Hard
	var missinsResources []string
	var negativeResources []string
	for res := range resourceQuotaParams {
		requestedRes := quotaSNS[res]
		if requestedRes.Format == "" {
			missinsResources = append(missinsResources, res.String())
		}

		if requestedRes.Sign() == -1 {
			negativeResources = append(negativeResources, res.String())
		}
	}
	if missinsResources != nil && negativeResources != nil {
		return false, errors.New(denyMessageMissingAndNegativeResourceRequest + strings.Join(missinsResources, ", ") + ", " + strings.Join(negativeResources, ", "))
	}
	if missinsResources != nil {
		return false, errors.New(denyMessageMissingResourceRequest + strings.Join(missinsResources, ", "))
	}
	if negativeResources != nil {
		return false, errors.New(denyMessageNegativeResourceRequest + strings.Join(negativeResources, ", "))
	}
	return true, nil
}

func IsMinResources(sns *utils.ObjectContext) error {
	quotaRequest := utils.GetSnsQuotaSpec(sns.Object).Hard
	childrenQuotaResources := utils.GetQuotaObjsListResources(utils.GetSnsChildrenQuotaObjs(sns))

	_, err := utils.NewObjectContext(sns.Ctx, sns.Log, sns.Client, types.NamespacedName{Name: sns.Object.GetNamespace()}, &corev1.Namespace{})
	if err != nil {
		sns.Log.Error(err, "unable to get current namespace")
	}

	if len(childrenQuotaResources) > 0 {
		for res, vMy := range quotaRequest {
			var vChildren, _ = childrenQuotaResources[res]
			vMy.Sub(vChildren)
			if vMy.Value() < 0 {
				return errors.New(denyMessageMinQuotaObj + res.String())
			}
		}
	}
	return nil
}

func ValidateUpdateSnsRequest(parentQuotaObj *utils.ObjectContext, newSns *utils.ObjectContext, oldSns *utils.ObjectContext, myQuotaObj *utils.ObjectContext) error {
	quotaRequest := utils.GetSnsQuotaSpec(newSns.Object).Hard //request by subnamespace
	quotaParent := utils.GetQuotaObjSpec(parentQuotaObj.Object).Hard
	quotaOld := utils.GetSnsQuotaSpec(oldSns.Object).Hard
	quotaUsed := utils.GetQuotaUsed(myQuotaObj.Object)
	siblingsResources := utils.GetQuotaObjsListResources(getSnsSiblingQuotaObjs(newSns))

	for res := range quotaRequest {
		var (
			vSiblings, _ = siblingsResources[res]
			vParent, _   = quotaParent[res]
			vRequest, _  = quotaRequest[res]
			vOld, _      = quotaOld[res]
			vUsed, _     = quotaUsed[res]
		)

		vParent.Sub(vSiblings)
		vParent.Sub(vRequest)
		vParent.Add(vOld)
		if vParent.Value() < 0 {
			return errors.New(denyMessageValidateQuotaObj + res.String() + " in subnamespace: " + string(newSns.Object.GetNamespace()))
		}
		vRequest.Sub(vUsed)
		if vRequest.Value() < 0 {
			return errors.New(string(newSns.Object.GetName()) + " " + denyMessageValidateUsedQuotaObj)
		}
	}
	return nil
}

func getSnsSiblingQuotaObjs(sns *utils.ObjectContext) []*utils.ObjectContext {
	var siblings []*utils.ObjectContext

	// get all the sibling namespaces
	namespaceList, err := utils.NewObjectContextList(sns.Ctx, sns.Log, sns.Client, &corev1.NamespaceList{}, client.MatchingLabels{danav1.Parent: sns.Object.GetNamespace()})
	if err != nil {
		sns.Log.Error(err, "unable to get namespace list")
	}

	rqFlag, err := utils.IsRq(sns, danav1.SelfOffset)
	if err != nil {
		sns.Log.Error(err, "unable to determine if sns is Rq")
	}

	for _, namespace := range namespaceList.Objects.(*corev1.NamespaceList).Items {
		if rqFlag {
			siblingQuotaObj, err := utils.NewObjectContext(sns.Ctx, sns.Log, sns.Client, types.NamespacedName{Name: namespace.ObjectMeta.Name, Namespace: namespace.ObjectMeta.Name}, &corev1.ResourceQuota{})
			if err != nil {
				sns.Log.Error(err, "unable to get RQ object")
			}
			siblings = append(siblings, siblingQuotaObj)
		} else {
			siblingQuotaObj, err := utils.NewObjectContext(sns.Ctx, sns.Log, sns.Client, types.NamespacedName{Name: namespace.ObjectMeta.Name}, &qoutav1.ClusterResourceQuota{})
			if err != nil {
				sns.Log.Error(err, "unable to get CRQ object")
			}
			siblings = append(siblings, siblingQuotaObj)
		}
	}

	return siblings
}

func getSnsParentQuotaObj(sns *utils.ObjectContext) (*utils.ObjectContext, error) {
	rqFlag, err := utils.IsRq(sns, danav1.ParentOffset)
	if err != nil {
		return nil, err
	}

	if rqFlag {
		return utils.NewObjectContext(sns.Ctx, sns.Log, sns.Client, types.NamespacedName{Name: sns.Object.GetNamespace(), Namespace: sns.Object.GetNamespace()}, &corev1.ResourceQuota{})
	}
	return utils.NewObjectContext(sns.Ctx, sns.Log, sns.Client, types.NamespacedName{Name: sns.Object.GetNamespace()}, &qoutav1.ClusterResourceQuota{})
}

func getSnsQuotaObj(sns *utils.ObjectContext) (*utils.ObjectContext, error) {
	rqFlag, err := utils.IsRq(sns, danav1.SelfOffset)
	if err != nil {
		return nil, err
	}

	if rqFlag {
		return utils.NewObjectContext(sns.Ctx, sns.Log, sns.Client, types.NamespacedName{Name: sns.Object.GetName(), Namespace: sns.Object.GetName()}, &corev1.ResourceQuota{})
	}
	return utils.NewObjectContext(sns.Ctx, sns.Log, sns.Client, types.NamespacedName{Name: sns.Object.GetName()}, &qoutav1.ClusterResourceQuota{})
}

func isSnsChildNamespaceExists(sns *utils.ObjectContext) (bool, error) {
	//Check if sns child namespace exists
	snsNamespace, err := utils.NewObjectContext(sns.Ctx, sns.Log, sns.Client, types.NamespacedName{Name: sns.Object.GetName()}, &corev1.Namespace{})
	if err != nil {
		return false, err
	}

	if snsNamespace.IsPresent() {
		if utils.GetSnsPhase(sns.Object) != danav1.Migrated {
			return true, nil
		}
	}
	return false, nil
}

// isSnsQuotaObjExists returns true if the subnamespace has a ResourceQuota
// or ClusterResourceQuota object (based on what it should have), and false otherwise
func isSnsQuotaObjExists(sns *utils.ObjectContext) (bool, error) {
	var snsQuotaObj *utils.ObjectContext

	rqFlag, err := utils.IsRq(sns, danav1.SelfOffset)
	if err != nil {
		return false, err
	}
	if rqFlag {
		snsQuotaObj, err = utils.NewObjectContext(sns.Ctx, sns.Log, sns.Client, types.NamespacedName{Name: sns.Object.GetName(), Namespace: sns.Object.GetName()}, &corev1.ResourceQuota{})
		if err != nil {
			return false, err
		}
	} else {
		snsQuotaObj, err = utils.NewObjectContext(sns.Ctx, sns.Log, sns.Client, types.NamespacedName{Name: sns.Object.GetName()}, &qoutav1.ClusterResourceQuota{})
		if err != nil {
			return false, err
		}
	}
	if !snsQuotaObj.IsPresent() {
		return false, nil
	}
	return true, nil

}
