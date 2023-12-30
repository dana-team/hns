package webhooks

import (
	"fmt"
	defaults "github.com/dana-team/hns/internals/controllers/subnamespace/defaults"
	"github.com/dana-team/hns/internals/utils"
	corev1 "k8s.io/api/core/v1"
	"net/http"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
	"strings"
)

const BinarySI = "BinarySI"

// validateResourceQuotaParams validates that in a regular Subnamespace all quota params: storage, cpu, memory, gpu exists and are positive.
// In a ResourcePool it validates that the upper resource pool cannot be created with an empty quota
func (a *SubnamespaceAnnotator) validateResourceQuotaParams(snsObject *utils.ObjectContext, isSNSResourcePool bool) admission.Response {
	snsQuota := utils.GetSnsQuotaSpec(snsObject.Object).Hard
	resourceQuotaParams := defaults.ZeroedQuota.Hard

	if isSNSResourcePool {
		if response := validateUpperResourcePool(snsObject, snsQuota); !response.Allowed {
			return response
		}
	} else {
		if response := validateMissingResources(resourceQuotaParams, snsQuota); !response.Allowed {
			return response
		}

		if response := validateNegativeResources(resourceQuotaParams, snsQuota); !response.Allowed {
			return response
		}
	}

	if response := validateResourceUnits(snsQuota); !response.Allowed {
		return response
	}

	return admission.Allowed("")
}

// validateUpperResourcePool validates that an upper ResourcePool contains a ResourceList
func validateUpperResourcePool(snsObject *utils.ObjectContext, snsQuota corev1.ResourceList) admission.Response {
	isSNSUpperResourcePool, err := utils.IsSNSUpperResourcePool(snsObject)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	if isSNSUpperResourcePool && len(snsQuota) == 0 {
		message := fmt.Sprintf("it's forbidden to create an upper ResourcePool without setting resources")
		return admission.Denied(message)
	}

	return admission.Allowed("")
}

// validateNegativeResources checks if the ResourceList of a subnamespace has missing resources
func validateMissingResources(resourceQuotaParams, snsQuota corev1.ResourceList) admission.Response {
	var missingResources []string

	for resourceName := range resourceQuotaParams {
		quantity := snsQuota[resourceName]
		if quantity.Format == "" {
			missingResources = append(missingResources, resourceName.String())
		}
	}

	if len(missingResources) > 0 {
		denyMessage := fmt.Sprintf("it's forbidden to set a subnamespace without providing requested amount of")
		message := denyMessage + " " + strings.Join(missingResources, ", ")
		return admission.Denied(message)
	}

	return admission.Allowed("")
}

// validateNegativeResources checks if the ResourceList of a subnamespace contains negative resources
func validateNegativeResources(resourceQuotaParams, snsQuota corev1.ResourceList) admission.Response {
	var negativeResources []string

	for resourceName := range resourceQuotaParams {
		quantity := snsQuota[resourceName]
		if quantity.Sign() == -1 {
			negativeResources = append(negativeResources, resourceName.String())
		}
	}

	if len(negativeResources) > 0 {
		denyMessage := fmt.Sprintf("it's forbidden to set a subnamespace with negative amount of")
		message := denyMessage + " " + strings.Join(negativeResources, ", ")
		return admission.Denied(message)
	}

	return admission.Allowed("")
}

// validateResourceUnits checks that relevant resources have correct units
func validateResourceUnits(snsQuota corev1.ResourceList) admission.Response {
	resources := []corev1.ResourceName{defaults.Memory, defaults.BasicStorage}

	for _, resourceName := range resources {
		quantity := snsQuota[resourceName]
		if quantity.Value() != 0 && quantity.Format != BinarySI {
			message := fmt.Sprintf("it's forbidden to set the %q resource in units other than %q (Ki, Mi, Gi, Ti)", resourceName, BinarySI)
			return admission.Denied(message)
		}

	}

	return admission.Allowed("")
}
