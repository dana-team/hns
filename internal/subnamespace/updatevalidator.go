package subnamespace

import (
	"fmt"
	"net/http"

	"github.com/dana-team/hns/internal/objectcontext"
	"github.com/dana-team/hns/internal/quota"
	"github.com/dana-team/hns/internal/subnamespace/resourcepool"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// handleUpdate implements the non-boilerplate logic of the validator, allowing it to be more easily unit
// tested (i.e. without constructing a full admission.Request).
func (v *SubnamespaceValidator) handleUpdate(snsObject, snsOldObject *objectcontext.ObjectContext) admission.Response {
	if response := v.validateRPLabelDeletion(snsObject, snsOldObject); !response.Allowed {
		return response
	}

	snsParentName := snsObject.Object.GetNamespace()
	snsParentNS, err := objectcontext.New(snsObject.Ctx, snsObject.Client, types.NamespacedName{Name: snsParentName}, &corev1.Namespace{})
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	isSNSResourcePool, err := resourcepool.IsSNSResourcePool(snsObject.Object)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	isOldSNSResourcePool, err := resourcepool.IsSNSResourcePool(snsOldObject.Object)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	isParentNSResourcePool, err := resourcepool.IsNSResourcePool(snsParentNS)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	if response := v.validateRPToSubnamespace(snsObject, isSNSResourcePool, isOldSNSResourcePool, isParentNSResourcePool); !response.Allowed {
		return response
	}

	if !isOldSNSResourcePool {
		if response := ValidateResourceQuotaParams(snsObject, isSNSResourcePool); !response.Allowed {
			return response
		}
	}

	// validate the request if the subnamespace is a regular subnamespace OR is the upper ResourcePool,
	// this is because only in that case there would be a RQ or CRQ attached to the SNS.
	// Otherwise, the subnamespace is part of a ResourcePool (and does not have a RQ/CRQ attached to it) and this check is unneeded
	isSNSUpperResourcePool, err := resourcepool.IsSNSUpper(snsObject)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	if !isSNSResourcePool || isSNSUpperResourcePool {
		snsQuotaObject, err := quota.SubnamespaceObjectFromAnnotation(snsObject)
		if err != nil {
			return admission.Errored(http.StatusBadRequest, err)
		}

		if snsQuotaObject.IsPresent() {
			if response := v.validateUpdateSnsRequest(snsObject, snsOldObject, snsQuotaObject); !response.Allowed {
				return response
			}
		}

		if response := v.validateEnoughResourcesForChildren(snsObject); !response.Allowed {
			return response
		}
	}

	return admission.Allowed("")
}

// validateRPLabelDeletion validates that the ResourcePool label has not been deleted.
func (v *SubnamespaceValidator) validateRPLabelDeletion(snsObject, snsOldObject *objectcontext.ObjectContext) admission.Response {
	snsResourcePoolLabel := resourcepool.SNSLabel(snsObject.Object)
	oldSNSResourcePoolLabel := resourcepool.SNSLabel(snsOldObject.Object)

	if snsResourcePoolLabel == "" && oldSNSResourcePoolLabel != "" {
		message := fmt.Sprintf("it's forbidden to delete the ResourcePool label from %q", snsObject.Name())
		return admission.Denied(message)
	}

	return admission.Allowed("")
}

// validateRPToSubnamespace validates that a ResourcePool is changed to a regular subnamespace
// only when its parent is not a ResourcePool, i.e. the subnamespace is the upper-rp.
func (v *SubnamespaceValidator) validateRPToSubnamespace(snsObject *objectcontext.ObjectContext, isSNSResourcePool, isOldSNSResourcePool, isParentNSResourcePool bool) admission.Response {
	snsName := snsObject.Name()
	snsParentName := snsObject.Object.GetNamespace()

	if isParentNSResourcePool {
		if !isSNSResourcePool && isOldSNSResourcePool {
			message := fmt.Sprintf("it's forbidden to change a ResourcePool label not at the top of hierarchy. %q is "+
				"part of a ResourcePool, and its parent %q is also part of a ResourcePool", snsName, snsParentName)
			return admission.Denied(message)
		}
	}

	return admission.Allowed("")
}

// validateEnoughResourcesForChildren validates that the requested resources for the subnamespace
// are not less than what is already allocated to the children of the subnamespace.
func (v *SubnamespaceValidator) validateEnoughResourcesForChildren(snsObject *objectcontext.ObjectContext) admission.Response {
	snsName := snsObject.Name()
	quotaRequest := quota.SubnamespaceSpec(snsObject.Object).Hard
	childrenQuotaResources := quota.GetQuotaObjectsListResources(quota.SubnamespaceChildrenObjects(snsObject))

	if len(childrenQuotaResources) > 0 {
		for resourceName, vMy := range quotaRequest {
			var vChildren, _ = childrenQuotaResources[resourceName]
			vMy.Sub(vChildren)
			if vMy.Value() < 0 {
				message := fmt.Sprintf("it's forbidden to update %q to have resource of type %q that are "+
					"fewer than the resources of type %q that are already allocated to the subnamespace children",
					snsName, resourceName.String(), resourceName.String())
				return admission.Denied(message)
			}
		}
	}
	return admission.Allowed("")
}

// validateUpdateSnsRequest validates that the new requested resources of a subnamespace are
// not more than what its parent has to allocate, and not less than what the subnamespace already uses.
func (v *SubnamespaceValidator) validateUpdateSnsRequest(snsObject, snsOldObject, snsQuotaObject *objectcontext.ObjectContext) admission.Response {
	logger := log.FromContext(snsObject.Ctx)
	snsName := snsObject.Name()
	snsParentName := snsObject.Object.GetNamespace()

	parentQuotaObject, err := quota.SubnamespaceParentObject(snsObject)
	if err != nil {
		logger.Error(err, "unable to get parent quota object")
		return admission.Denied(err.Error())
	}

	quotaRequest := quota.SubnamespaceSpec(snsObject.Object).Hard
	quotaParent := quota.GetQuotaObjectSpec(parentQuotaObject.Object).Hard
	quotaOld := quota.SubnamespaceSpec(snsOldObject.Object).Hard
	quotaUsed := quota.GetQuotaUsed(snsQuotaObject.Object)
	siblingsResources := quota.GetQuotaObjectsListResources(quota.SubnamespaceSiblingObjects(snsObject))

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
			message := fmt.Sprintf("it's forbidden to update subnamespace %q because there are not enough resources of type %q "+
				"in parent subnamespace %q to complete the request", snsName, resourceName.String(), snsParentName)
			return admission.Denied(message)
		}
		request.Sub(used)
		if request.Value() < 0 {
			message := fmt.Sprintf("it's forbidden to update subnamespace %q because active workloads "+
				"in the hierarchy of %q request more resources of type %q than the new desired quantity",
				snsName, snsName, resourceName.String())
			return admission.Denied(message)
		}
	}
	return admission.Allowed("")
}
