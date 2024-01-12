package webhooks

import (
	"fmt"
	"github.com/dana-team/hns/internals/utils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"net/http"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// handleUpdate implements the non-boilerplate logic of the validator, allowing it to be more easily unit
// tested (i.e. without constructing a full admission.Request)
func (a *SubnamespaceAnnotator) handleUpdate(snsObject, snsOldObject *utils.ObjectContext) admission.Response {
	if response := a.validateRPLabelDeletion(snsObject, snsOldObject); !response.Allowed {
		return response
	}

	snsParentName := snsObject.Object.GetNamespace()
	snsParentNS, err := utils.NewObjectContext(snsObject.Ctx, snsObject.Client, types.NamespacedName{Name: snsParentName}, &corev1.Namespace{})
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	isSNSResourcePool, err := utils.IsSNSResourcePool(snsObject.Object)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	isOldSNSResourcePool, err := utils.IsSNSResourcePool(snsOldObject.Object)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	isParentNSResourcePool, err := utils.IsNamespaceResourcePool(snsParentNS)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	if response := a.validateRPToSubnamespace(snsObject, isSNSResourcePool, isOldSNSResourcePool, isParentNSResourcePool); !response.Allowed {
		return response
	}

	if !isOldSNSResourcePool {
		if response := a.validateResourceQuotaParams(snsObject, isSNSResourcePool); !response.Allowed {
			return response
		}
	}

	if response := a.validateSNSQuotaObject(snsObject); !response.Allowed {
		return admission.Allowed("")
	}

	if response := a.validateEnoughResourcesForChildren(snsObject); !response.Allowed {
		return response
	}

	// validate the request if the subnamespace is not a ResourcePool OR the subnamespace is the upper ResourcePool
	// this is because only in that case the subnamespace would have a RQ or CRQ attached to it.
	// Otherwise, the subnamespace is part of a ResourcePool (and does not have a RQ/CRQ attached to it) and this check is unneeded
	isSNSUpperResourcePool, err := utils.IsSNSUpperResourcePool(snsObject)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	if !isSNSResourcePool || isSNSUpperResourcePool {
		if response := a.validateUpdateSnsRequest(snsObject, snsOldObject); !response.Allowed {
			return response
		}
	}

	return admission.Allowed("")
}

// validateRPLabelDeletion validates that the ResourcePool label has not been deleted
func (a *SubnamespaceAnnotator) validateRPLabelDeletion(snsObject, snsOldObject *utils.ObjectContext) admission.Response {
	snsResourcePoolLabel := utils.GetSNSResourcePoolLabel(snsObject.Object)
	oldSNSResourcePoolLabel := utils.GetSNSResourcePoolLabel(snsOldObject.Object)

	if snsResourcePoolLabel == "" && oldSNSResourcePoolLabel != "" {
		message := fmt.Sprintf("it's forbidden to delete the ResourcePool label from '%s", snsObject.Object.GetName())
		return admission.Denied(message)
	}

	return admission.Allowed("")
}

// validateRPToSubnamespace validates that a ResourcePool is changed to a regular subnamespace
// only when its parent is not a ResourcePool, i.e. the subnamespace is the upper-rp
func (a *SubnamespaceAnnotator) validateRPToSubnamespace(snsObject *utils.ObjectContext, isSNSResourcePool, isOldSNSResourcePool, isParentNSResourcePool bool) admission.Response {
	snsName := snsObject.Object.GetName()
	snsParentName := snsObject.Object.GetNamespace()

	if isParentNSResourcePool {
		if !isSNSResourcePool && isOldSNSResourcePool {
			message := fmt.Sprintf("it's forbidden to change a ResourcePool label not at the top of hierarchy. '%s' is "+
				"part of a ResourcePool, and its parent '%s' is also part of a ResourcePool", snsName, snsParentName)
			return admission.Denied(message)
		}
	}

	return admission.Allowed("")
}

// validateSNSQuotaObject validates that a subnamespace has a corresponding quota object
func (a *SubnamespaceAnnotator) validateSNSQuotaObject(snsObject *utils.ObjectContext) admission.Response {
	logger := log.FromContext(snsObject.Ctx)

	quotaObject, err := utils.GetSNSQuotaObject(snsObject)
	if err != nil {
		logger.Error(err, "failed to get object", "quotaObject", snsObject.Object.GetName())
		return admission.Errored(http.StatusInternalServerError, err)
	}

	if !(quotaObject.IsPresent()) {
		message := fmt.Sprintf("quota object '%s' does not exist", snsObject.Object.GetName())
		return admission.Denied(message)
	}

	return admission.Allowed("")
}

// validateEnoughResourcesForChildren validates that the requested resources for the subnamespace
// are not less than what is already allocated to the children of the subnamespace
func (a *SubnamespaceAnnotator) validateEnoughResourcesForChildren(snsObject *utils.ObjectContext) admission.Response {
	snsName := snsObject.Object.GetName()
	quotaRequest := utils.GetSnsQuotaSpec(snsObject.Object).Hard
	childrenQuotaResources := utils.GetQuotaObjectsListResources(utils.GetSnsChildrenQuotaObjects(snsObject))

	if len(childrenQuotaResources) > 0 {
		for resourceName, vMy := range quotaRequest {
			var vChildren, _ = childrenQuotaResources[resourceName]
			vMy.Sub(vChildren)
			if vMy.Value() < 0 {
				message := fmt.Sprintf("it's forbidden to update '%s' to have resource of type '%s' that are "+
					"fewer than the resources of type '%s' that are already allocated to the subnamespace children",
					snsName, resourceName.String(), resourceName.String())
				return admission.Denied(message)
			}
		}
	}
	return admission.Allowed("")
}

// validateUpdateSnsRequest validates that the new requested resources of a subnamespace are
// not more than what its parent has to allocate, and not less than what the subnamespace already uses
func (a *SubnamespaceAnnotator) validateUpdateSnsRequest(snsObject, snsOldObject *utils.ObjectContext) admission.Response {
	logger := log.FromContext(snsObject.Ctx)
	snsName := snsObject.Object.GetName()
	snsParentName := snsObject.Object.GetNamespace()
	snsQuotaObject, err := utils.GetSNSQuotaObjectFromAnnotation(snsObject)
	if err != nil {
		logger.Error(err, "unable to get sns quota object")
		return admission.Denied(err.Error())
	}

	parentQuotaObject, err := utils.GetSNSParentQuotaObject(snsObject)
	if err != nil {
		logger.Error(err, "unable to get parent quota object")
		return admission.Denied(err.Error())
	}

	quotaRequest := utils.GetSnsQuotaSpec(snsObject.Object).Hard
	quotaParent := utils.GetQuotaObjectSpec(parentQuotaObject.Object).Hard
	quotaOld := utils.GetSnsQuotaSpec(snsOldObject.Object).Hard
	quotaUsed := utils.GetQuotaUsed(snsQuotaObject.Object)
	siblingsResources := utils.GetQuotaObjectsListResources(utils.GetSnsSiblingQuotaObjects(snsObject))

	for resourceName := range quotaRequest {
		var (
			siblings, _ = siblingsResources[resourceName]
			parent, _   = quotaParent[resourceName]
			request, _  = quotaRequest[resourceName]
			old, _      = quotaOld[resourceName]
			used, _     = quotaUsed[resourceName]
		)

		parent.Sub(siblings)
		parent.Sub(request)
		parent.Add(old)
		if parent.Value() < 0 {
			message := fmt.Sprintf("it's forbidden to update subnamespace '%s' because there are not enough resources of type '%s' "+
				"in parent subnamespace '%s' to complete the request", snsName, resourceName.String(), snsParentName)
			return admission.Denied(message)
		}
		request.Sub(used)
		if request.Value() < 0 {
			message := fmt.Sprintf("it's forbidden to update subnamespace '%s' because active workloads "+
				"in the hierarchy of '%s' request more resources of type '%s' than the new desired quantity",
				snsName, snsName, resourceName.String())
			return admission.Denied(message)
		}
	}
	return admission.Allowed("")
}
