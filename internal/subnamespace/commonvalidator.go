package subnamespace

import (
	"github.com/dana-team/hns/internal/objectcontext"
	"github.com/dana-team/hns/internal/quota"
	"github.com/dana-team/hns/internal/subnamespace/resourcepool"
	corev1 "k8s.io/api/core/v1"
	"net/http"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
	"strings"
)

// ValidateResourceQuotaParams validates that in a regular Subnamespace all quota params: storage, cpu, memory, gpu exists and are positive.
// In a ResourcePool it validates that the upper resource pool cannot be created with an empty quota.
func ValidateResourceQuotaParams(snsObject *objectcontext.ObjectContext, isSNSResourcePool bool) admission.Response {
	snsQuota := quota.SubnamespaceSpec(snsObject.Object).Hard
	resourceQuotaParams := quota.ZeroedQuota.Hard

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

	return admission.Allowed("")
}

// validateUpperResourcePool validates that an upper ResourcePool contains a ResourceList.
func validateUpperResourcePool(snsObject *objectcontext.ObjectContext, snsQuota corev1.ResourceList) admission.Response {
	isSNSUpperResourcePool, err := resourcepool.IsSNSUpper(snsObject)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	if isSNSUpperResourcePool && len(snsQuota) == 0 {
		message := "it's forbidden to create an upper ResourcePool without setting resources"
		return admission.Denied(message)
	}

	return admission.Allowed("")
}

// validateNegativeResources checks if the ResourceList of a subnamespace has missing resources.
func validateMissingResources(resourceQuotaParams, snsQuota corev1.ResourceList) admission.Response {
	var missingResources []string

	for resourceName := range resourceQuotaParams {
		quantity := snsQuota[resourceName]
		if quantity.Format == "" {
			missingResources = append(missingResources, resourceName.String())
		}
	}

	if len(missingResources) > 0 {
		denyMessage := "it's forbidden to set a subnamespace without providing requested amount of"
		message := denyMessage + " " + strings.Join(missingResources, ", ")
		return admission.Denied(message)
	}

	return admission.Allowed("")
}

// validateNegativeResources checks if the ResourceList of a subnamespace contains negative resources.
func validateNegativeResources(resourceQuotaParams, snsQuota corev1.ResourceList) admission.Response {
	var negativeResources []string

	for resourceName := range resourceQuotaParams {
		quantity := snsQuota[resourceName]
		if quantity.Sign() == -1 {
			negativeResources = append(negativeResources, resourceName.String())
		}
	}

	if len(negativeResources) > 0 {
		denyMessage := "it's forbidden to set a subnamespace with negative amount of"
		message := denyMessage + " " + strings.Join(negativeResources, ", ")
		return admission.Denied(message)
	}

	return admission.Allowed("")
}
