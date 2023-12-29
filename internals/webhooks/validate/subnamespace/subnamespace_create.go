package webhooks

import (
	"fmt"
	"net/http"
	"regexp"

	danav1 "github.com/dana-team/hns/api/v1"
	"github.com/dana-team/hns/internals/utils"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// handleCreate implements the non-boilerplate logic of the validator, allowing it to be more easily unit
// tested (i.e. without constructing a full admission.Request)
func (a *SubnamespaceAnnotator) handleCreate(snsObject *utils.ObjectContext) admission.Response {
	if response := a.validateSubnamespaceName(snsObject); !response.Allowed {
		return response
	}

	if response := a.validateUniqueSNSName(snsObject); !response.Allowed {
		return response
	}

	// validate that the new parent doesn't already have too many subnamespaces in its branch
	// the maximum number a subnamespace can have in its branch is called by the danav1.MaxSNS var
	if response := a.validateKeyCountInDB(snsObject); !response.Allowed {
		return response
	}

	isSNSResourcePool, err := utils.IsSNSResourcePool(snsObject.Object)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	if response := a.validateSNSUnderRP(snsObject, isSNSResourcePool); !response.Allowed {
		return response
	}

	if rsp := a.validateResourceQuotaParams(snsObject, isSNSResourcePool); !rsp.Allowed {
		return rsp
	}

	if rsp := a.validateEnoughResourcesInParentSNS(snsObject); !rsp.Allowed {
		return rsp
	}

	return admission.Allowed("")
}

// validateSubnamespaceName validate name for subnamespace according to RFC 1123, to match namespace name validation.
func (a *SubnamespaceAnnotator) validateSubnamespaceName(snsObject *utils.ObjectContext) admission.Response {
	snsName := snsObject.Object.GetName()
	if len(snsName) > 63 {
		message := fmt.Sprintf("Invalid value: %s: the subnamespace name should be at most 63 characters", snsName)
		return admission.Denied(message)
	}
	if match, _ := regexp.MatchString("^[a-z0-9]([-a-z0-9]*[a-z0-9])?$", snsName); !match {
		message := fmt.Sprintf("Invalid value: %s: a lowercase RFC 1123 label must consist of lower case alphanumeric characters or '-', and must start and end with an alphanumeric character (e.g. 'my-name', or '123-abc',", snsName)
		return admission.Denied(message)
	}

	return admission.Allowed("")
}

// validateUniqueSNSName validates that a namespace with the given subnamespace name doesn't already exist
func (a *SubnamespaceAnnotator) validateUniqueSNSName(snsObject *utils.ObjectContext) admission.Response {
	logger := log.FromContext(snsObject.Ctx)

	snsName := snsObject.Object.GetName()
	exists, err := utils.DoesSNSNamespaceExist(snsObject)
	if err != nil {
		logger.Error(err, "failed to check if namespace exists", "subnamespace", snsName)
		return admission.Denied(err.Error())
	}

	if exists {
		message := fmt.Sprintf("it's forbidden to create a subnamespace with a name that already exists. A subnamespace "+
			"name must be unique across the cluster, and a namespace of name %q already exists; change "+
			"the subnamespace name and try again", snsName)
		return admission.Denied(message)
	}

	return admission.Allowed("")
}

// validateKeyCountInDB validates that creating a new subnamespace under a given parent
// will not cause the new parent to exceed the maximum limit of namespaces in its hierarchy
func (a *SubnamespaceAnnotator) validateKeyCountInDB(snsObject *utils.ObjectContext) admission.Response {
	parentSNSName := snsObject.Object.GetNamespace()
	key := a.NamespaceDB.GetKey(parentSNSName)

	if key != "" {
		if a.NamespaceDB.GetKeyCount(key) >= danav1.MaxSNS {
			message := fmt.Sprintf("it's forbidden to create more than '%v' namespaces under hierarchy %q", danav1.MaxSNS, key)
			return admission.Denied(message)
		}
	}

	return admission.Allowed("")
}

// validateSNSUnderRP validates if a subnamespace tries to be created under a ResourcePool
func (a *SubnamespaceAnnotator) validateSNSUnderRP(snsObject *utils.ObjectContext, isSNSResourcePool bool) admission.Response {
	snsParentNSName := snsObject.Object.GetNamespace()
	snsParentNS, err := utils.NewObjectContext(snsObject.Ctx, snsObject.Client, types.NamespacedName{Name: snsParentNSName}, &corev1.Namespace{})
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	isParentNSResourcePool, err := utils.IsNamespaceResourcePool(snsParentNS)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	if !isSNSResourcePool && isParentNSResourcePool {
		message := fmt.Sprintf("it's forbidden to create a regular subnamespace under a ResourcePool. Only a ResourcePool SNS can be "+
			"created under a ResourcePool. %q is part of a ResourcePool", snsParentNSName)
		return admission.Denied(message)
	}

	return admission.Allowed("")
}

// validateEnoughResourcesInParentSNS validates that there are enough resources available in a parent subnamespace
// to create a new subnamespace with certain resources under it
func (a *SubnamespaceAnnotator) validateEnoughResourcesInParentSNS(snsObject *utils.ObjectContext) admission.Response {
	logger := log.FromContext(snsObject.Ctx)

	snsName := snsObject.Object.GetName()
	snsParentName := snsObject.Object.GetNamespace()

	parentQuotaObject, err := utils.GetSNSParentQuotaObject(snsObject)
	if err != nil {
		logger.Error(err, "unable to get parent quota object")
		return admission.Denied(err.Error())
	}

	quotaParent := utils.GetQuotaObjectSpec(parentQuotaObject.Object).Hard
	quotaSNS := utils.GetSnsQuotaSpec(snsObject.Object).Hard
	siblingsResources := utils.GetQuotaObjectsListResources(utils.GetSnsSiblingQuotaObjects(snsObject))

	for resourceName := range quotaParent {
		var (
			siblings, _ = siblingsResources[resourceName]
			parent, _   = quotaParent[resourceName]
			request, _  = quotaSNS[resourceName]
		)

		parent.Sub(siblings)
		parent.Sub(request)
		if parent.Value() < 0 {
			message := fmt.Sprintf("it's forbidden to create subnamespace %q under %q when there are are not "+
				"enough resources of type %q in %q", snsName, snsParentName, resourceName.String(), snsParentName)
			return admission.Denied(message)
		}
	}
	return admission.Allowed("")
}
