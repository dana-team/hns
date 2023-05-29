package webhooks

import (
	"context"
	"fmt"
	danav1 "github.com/dana-team/hns/api/v1"
	"github.com/dana-team/hns/internals/utils"
	quotav1 "github.com/openshift/api/quota/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"net/http"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

func (a *MigrationHierarchyAnnotator) handleCreate(mhObject *utils.ObjectContext, reqUser string) admission.Response {
	ctx := mhObject.Ctx
	logger := log.FromContext(ctx)

	currentNSName := mhObject.Object.(*danav1.MigrationHierarchy).Spec.CurrentNamespace
	currentNS, err := utils.NewObjectContext(ctx, a.Client, client.ObjectKey{Namespace: "", Name: currentNSName}, &corev1.Namespace{})
	if err != nil {
		logger.Error(err, "failed to create object", "currentNS", currentNSName)
		return admission.Errored(http.StatusBadRequest, err)
	}

	toNSName := mhObject.Object.(*danav1.MigrationHierarchy).Spec.ToNamespace
	toNS, err := utils.NewObjectContext(ctx, a.Client, client.ObjectKey{Namespace: "", Name: toNSName}, &corev1.Namespace{})
	if err != nil {
		logger.Error(err, "failed to create object", "toNS", toNSName)
		return admission.Errored(http.StatusBadRequest, err)
	}

	if response := utils.ValidateNamespaceExist(currentNS); !response.Allowed {
		return response
	}

	if response := utils.ValidateNamespaceExist(toNS); !response.Allowed {
		return response
	}

	currentNSSliced := utils.GetNSDisplayNameSlice(currentNS)
	toNSSliced := utils.GetNSDisplayNameSlice(toNS)
	ancestorNSName, _, err := utils.GetAncestor(currentNSSliced, toNSSliced)
	if err != nil {
		logger.Error(err, "failed to get ancestor", "source namespace", currentNSSliced, "destination namespace", toNSSliced)
		return admission.Errored(http.StatusBadRequest, err)
	}

	if response := utils.ValidateSecondaryRoot(ctx, a.Client, currentNSSliced, toNSSliced); !response.Allowed {
		return response
	}

	if response := utils.ValidatePermissions(ctx, currentNSSliced, currentNSName, toNSName, ancestorNSName, reqUser, false); !response.Allowed {
		return response
	}

	isCurrentNSResoucePool, err := utils.IsNamespaceResourcePool(currentNS)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	isToNSResourcePool, err := utils.IsNamespaceResourcePool(toNS)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	if response := a.validateSNSToRPMigration(isCurrentNSResoucePool, isToNSResourcePool); !response.Allowed {
		return response
	}

	if !isCurrentNSResoucePool == !isToNSResourcePool {
		currentNSKey := a.NamespaceDB.GetKey(currentNSName)
		toNSKey := a.NamespaceDB.GetKey(toNSName)

		// validate that if the subnamespace is not a resourcepool, then it is only allowed to migrate subnamespaces that
		// either have a CRQ or their direct parent have a CRQ
		if response := a.validateKeyExists(currentNSName, currentNSKey); !response.Allowed {
			return response
		}

		if response := a.validateKeyExists(toNSName, toNSKey); !response.Allowed {
			return response
		}

		// validate that the subnamespace that should be migrated is not the key itself in the DB since that would
		// mean that its parent does not have a CRQ
		if response := a.validateKeyHierarchy(currentNSName, currentNSKey); !response.Allowed {
			return response
		}

		// validate that the new requested parent doesn't already have too many subnamespaces in its branch
		// the maximum number a subnamespace can have in its branch is called by the danav1.MaxSNS env var
		if response := a.validateKeyCountInDB(ctx, toNSKey, currentNSName); !response.Allowed {
			return response
		}
	}

	currSNS, err := utils.GetSNSFromNamespace(currentNS)
	if err != nil {
		logger.Error(err, "failed to get subnamespace object", "subnamespace", currentNS.GetName())
		return admission.Errored(http.StatusBadRequest, err)
	}
	toSNS, err := utils.GetSNSFromNamespace(toNS)
	if err != nil {
		logger.Error(err, "failed to get subnamespace object", "subnamespace", toNS.GetName())
		return admission.Errored(http.StatusBadRequest, err)
	}

	if isToNSResourcePool {
		if response := a.validateMigrateRPRequest(currSNS, toSNS); !response.Allowed {
			return response
		}
	} else {
		if response := a.validateMigrateRPRequest(currSNS, toSNS); !response.Allowed {
			return response
		}
	}

	return admission.Allowed("")
}

// validateSNSToRPMigration validates that a Subnamespace is not asked to be migrated to be under a ResourcePool
func (a *MigrationHierarchyAnnotator) validateSNSToRPMigration(isCurrentNSResourcePool, isToNSResourcePool bool) admission.Response {
	if !isCurrentNSResourcePool && isToNSResourcePool {
		message := "it's forbidden to migrate from a Subnamespace to a ResourcePool. You can convert the subnamespace to a ResourcePool and try again"
		return admission.Denied(message)
	}

	return admission.Allowed("")
}

// validateKeyExists validates that a namespace has a key in the namespaceDB
func (a *MigrationHierarchyAnnotator) validateKeyExists(namespace, key string) admission.Response {
	if key == "" {
		message := fmt.Sprintf("it's forbidden to migrate from or to this level of the hierarchy. The issue is with '%s'", namespace)
		return admission.Denied(message)
	}

	return admission.Allowed("")
}

// validateKeyHierarchy validates that if a namespace is the key itself in the namespaceDB
func (a *MigrationHierarchyAnnotator) validateKeyHierarchy(namespace, key string) admission.Response {
	if namespace == key {
		message := "it's forbidden to migrate from or to this level of the hierarchy"
		return admission.Denied(message)
	}

	return admission.Allowed("")
}

// validateKeyCountInDB validates that migrating a subnamespace and all its children
// to the new parent subnamespace will not cause the new parent to exceed the maximum
// limit of namespaces in its hierarchy
func (a *MigrationHierarchyAnnotator) validateKeyCountInDB(ctx context.Context, toNSKey, currentNSName string) admission.Response {
	logger := log.FromContext(ctx)
	childrenNum, err := getNSChildrenNum(ctx, a.Client, currentNSName)
	if err != nil {
		logger.Error(err, "failed to compute number of children", "currentNS", currentNSName)
		return admission.Denied(err.Error())
	}

	if (a.NamespaceDB.GetKeyCount(toNSKey) + childrenNum) >= danav1.MaxSNS {
		message := fmt.Sprintf("it's forbidden to create more than '%v' namespaces under hierarchy '%s'", danav1.MaxSNS, toNSKey)
		return admission.Denied(message)
	}

	return admission.Allowed("")
}

// validateMigrateSNSRequest validates that there are enough resources to complete a
// migration in case the subnamespace is not a ResourcePool
func (a *MigrationHierarchyAnnotator) validateMigrateSNSRequest(currSNS *utils.ObjectContext, toSNS *utils.ObjectContext) admission.Response {
	quotaParent, err := utils.GetSNSQuota(toSNS)
	if err != nil {
		message := fmt.Sprintf("failed to get quota of subnamespace '%s': "+err.Error(), toSNS.GetName())
		return admission.Denied(message)
	}

	quotaObjectRequest := utils.GetSnsQuotaSpec(currSNS.Object).Hard
	childQuotaResources := utils.GetQuotaObjectsListResources(utils.GetSnsChildrenQuotaObjects(toSNS))

	for resourceName := range quotaParent {
		var (
			children, _ = childQuotaResources[resourceName]
			parent, _   = quotaParent[resourceName]
			request, _  = quotaObjectRequest[resourceName]
		)

		parent.Sub(children)
		parent.Sub(request)
		if parent.Value() < 0 {
			message := fmt.Sprintf("it's forbidden to migrate because there are not enough resources of type '%s' "+
				"in the requested new parent '%s'", resourceName.String(), toSNS.GetName())
			return admission.Denied(message)
		}
	}
	return admission.Allowed("")
}

// validateMigrateRPRequest validates that there are enough resources to complete a
// migration in case the subnamespace is a ResourcePool
func (a *MigrationHierarchyAnnotator) validateMigrateRPRequest(currSNS *utils.ObjectContext, toSNS *utils.ObjectContext) admission.Response {
	quotaNewParent, err := utils.GetSNSQuota(toSNS)
	if err != nil {
		message := fmt.Sprintf("failed to get quota of subnamespace '%s': "+err.Error(), toSNS.GetName())
		return admission.Denied(message)
	}

	snsRequest, err := utils.GetSNSQuotaUsed(currSNS)
	if err != nil {
		message := fmt.Sprintf("failed to get used quota of subnamespace '%s': "+err.Error(), currSNS.GetName())
		return admission.Denied(message)
	}
	usedNewParentResources, err := utils.GetSNSQuotaUsed(toSNS)
	if err != nil {
		message := fmt.Sprintf("failed to get used quota of subnamespace '%s': "+err.Error(), toSNS.GetName())
		return admission.Denied(message)
	}

	for resourceName := range quotaNewParent {
		var (
			used, _    = usedNewParentResources[resourceName]
			parent, _  = quotaNewParent[resourceName]
			request, _ = snsRequest[resourceName]
		)

		parent.Sub(used)
		parent.Sub(request)
		if parent.Value() < 0 {
			message := fmt.Sprintf("it's forbidden to migrate because there are not enough free resources of type '%s' "+
				"in the ResourcePool the requeested new parent '%s' is a part of", resourceName.String(), toSNS.GetName())
			return admission.Denied(message)
		}
	}
	return admission.Allowed("")
}

// getNSChildrenNum returns the number of children of a subnamespace by looking at its CRQ
func getNSChildrenNum(ctx context.Context, c client.Client, nsname string) (int, error) {
	crq := quotav1.ClusterResourceQuota{}
	if err := c.Get(ctx, types.NamespacedName{Name: nsname}, &crq); err != nil {
		return 0, err
	}

	return len(crq.Status.Namespaces), nil
}
