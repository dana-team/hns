package webhooks

import (
	"fmt"
	defaults "github.com/dana-team/hns/internals/controllers/subnamespace/defaults"
	"github.com/dana-team/hns/internals/utils"
	"net/http"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
	"strings"
)

// validateResourceQuotaParams validates that in a regular Subnamespace all quota params: storage, cpu, memory, gpu exists and are positive.
// In a ResourcePool it validates that the upper resource pool cannot be created with an empty quota
func (a *SubnamespaceAnnotator) validateResourceQuotaParams(snsObject *utils.ObjectContext, isSNSResourcePool bool) admission.Response {
	snsName := snsObject.Object.GetName()

	snsQuota := utils.GetSnsQuotaSpec(snsObject.Object).Hard
	resourceQuotaParams := defaults.ZeroedQuota.Hard

	var missingResources []string
	var negativeResources []string

	if isSNSResourcePool {
		isSNSUpperResourcePool, err := utils.IsSNSUpperResourcePool(snsObject)
		if err != nil {
			return admission.Errored(http.StatusBadRequest, err)
		}

		if isSNSUpperResourcePool && len(snsQuota) == 0 {
			message := fmt.Sprintf("it's forbidden to create a subnamespace '%s' to be under a ResourcePool without setting resources "+
				"when it's the first subnamespace in its ResourcePool.", snsObject.Object.GetName())
			return admission.Denied(message)
		}
	}

	if !isSNSResourcePool {
		for resourceName := range resourceQuotaParams {
			requestedRes := snsQuota[resourceName]
			if requestedRes.Format == "" {
				missingResources = append(missingResources, resourceName.String())
			}
			if requestedRes.Sign() == -1 {
				negativeResources = append(negativeResources, resourceName.String())
			}
		}

		if missingResources != nil && negativeResources != nil {
			denyMessage := fmt.Sprintf("it's forbidden to set a subnamespace '%s' without providing or providing negative amount of", snsName)
			message := denyMessage + " " + strings.Join(missingResources, ", ") + ", " + strings.Join(negativeResources, ", ")
			return admission.Denied(message)
		}

		if missingResources != nil {
			denyMessage := fmt.Sprintf("it's forbidden to set a subnamespace '%s' without providing requested amount of", snsName)
			message := denyMessage + " " + strings.Join(missingResources, ", ")
			return admission.Denied(message)
		}

		if negativeResources != nil {
			denyMessage := fmt.Sprintf("it's forbidden to set a subnamespace '%s' with negative amount of", snsName)
			message := denyMessage + " " + strings.Join(negativeResources, ", ")
			return admission.Denied(message)
		}
	}
	return admission.Allowed("")
}
