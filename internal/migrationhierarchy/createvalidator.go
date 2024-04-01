package migrationhierarchy

import (
	"context"
	"fmt"
	"net/http"

	danav1 "github.com/dana-team/hns/api/v1"
	"github.com/dana-team/hns/internal/common"
	"github.com/dana-team/hns/internal/namespace/nsutils"
	"github.com/dana-team/hns/internal/objectcontext"
	"github.com/dana-team/hns/internal/subnamespace/resourcepool"
	"github.com/dana-team/hns/internal/subnamespace/snsutils"
	quotav1 "github.com/openshift/api/quota/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

func (v *MigrationHierarchyValidator) handleCreate(mhObject *objectcontext.ObjectContext, reqUser string) admission.Response {
	ctx := mhObject.Ctx
	logger := log.FromContext(ctx)

	currentNSName := mhObject.Object.(*danav1.MigrationHierarchy).Spec.CurrentNamespace
	currentNS, err := objectcontext.New(ctx, v.Client, client.ObjectKey{Name: currentNSName}, &corev1.Namespace{})
	if err != nil {
		logger.Error(err, "failed to create object", "currentNS", currentNSName)
		return admission.Errored(http.StatusBadRequest, err)
	}

	toNSName := mhObject.Object.(*danav1.MigrationHierarchy).Spec.ToNamespace
	toNS, err := objectcontext.New(ctx, v.Client, client.ObjectKey{Name: toNSName}, &corev1.Namespace{})
	if err != nil {
		logger.Error(err, "failed to create object", "toNS", toNSName)
		return admission.Errored(http.StatusBadRequest, err)
	}

	if response := v.validateCurrentNSAndToNSEqual(currentNSName, toNSName); !response.Allowed {
		return response
	}

	if response := common.ValidateNamespaceExist(currentNS); !response.Allowed {
		return response
	}

	if response := common.ValidateNamespaceExist(toNS); !response.Allowed {
		return response
	}

	if response := common.ValidateToNamespaceName(currentNS, toNSName); !response.Allowed {
		return response
	}

	currentNSSliced := nsutils.DisplayNameSlice(currentNS)
	toNSSliced := nsutils.DisplayNameSlice(toNS)
	ancestorNSName, isAncestorRoot, err := snsutils.GetAncestor(currentNSSliced, toNSSliced)
	if err != nil {
		logger.Error(err, "failed to get ancestor", "source namespace", currentNSSliced, "destination namespace", toNSSliced)
		return admission.Errored(http.StatusBadRequest, err)
	}

	if response := v.validateMigrationLoop(toNSSliced, currentNSName); !response.Allowed {
		return response
	}

	// validate the source and destination namespaces are under the same secondary root only
	// if you are not trying to migrate to or from the root namespace of the cluster
	if (isAncestorRoot) && (!nsutils.IsRoot(currentNS.Object) && !nsutils.IsRoot(toNS.Object)) {
		if response := common.ValidateSecondaryRoot(ctx, v.Client, currentNSSliced, toNSSliced); !response.Allowed {
			return response
		}
	}

	if response := common.ValidatePermissions(ctx, currentNSSliced, currentNSName, toNSName, ancestorNSName, reqUser, false, v.Client); !response.Allowed {
		return response
	}

	isCurrentNSResourcePool, err := resourcepool.IsNSResourcePool(currentNS)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	isToNSResourcePool, err := resourcepool.IsNSResourcePool(toNS)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	if response := v.validateSNSToRPMigration(isCurrentNSResourcePool, isToNSResourcePool); !response.Allowed {
		return response
	}

	if isCurrentNSResourcePool {
		isNSUpperResourcePool, err := resourcepool.IsNSUpperResourcePool(currentNS)
		if err != nil {
			return admission.Errored(http.StatusBadRequest, err)
		}

		if response := v.validateNonUpperRPToSNSMigration(isNSUpperResourcePool, isToNSResourcePool); !response.Allowed {
			return response
		}
	}

	if !isCurrentNSResourcePool && !isToNSResourcePool {
		currentNSKey := v.NamespaceDB.Key(currentNSName)
		toNSKey := v.NamespaceDB.Key(toNSName)

		if currentNSKey != "" && toNSKey != "" {
			// validate that the new requested parent doesn't already have too many subnamespaces in its branch
			// the maximum number a subnamespace can have in its branch is called by the MaxSNS cli flag
			if response := v.validateKeyCountInDB(ctx, toNSKey, currentNSName); !response.Allowed {
				return response
			}
		}
	}

	return admission.Allowed("")
}

// validateCurrentNSAndToNSEqual validates that a Subnamespace is not asked to be migrated to be under itself.
func (v *MigrationHierarchyValidator) validateCurrentNSAndToNSEqual(currentNSName, toNSName string) admission.Response {
	if currentNSName == toNSName {
		message := "it's forbidden to migrate a Subnamespace to be under itself"
		return admission.Denied(message)
	}

	return admission.Allowed("")
}

// validateMigrationLoop validates that a Subnamespace is not asked to be migrated to be under its own
// descendant since that can create a loop.
func (v *MigrationHierarchyValidator) validateMigrationLoop(toNSSliced []string, currentNSName string) admission.Response {
	if common.ContainsString(toNSSliced, currentNSName) {
		message := "it's forbidden to migrate a Subnamespace to be under its own descendant since it would create a loop"
		return admission.Denied(message)
	}

	return admission.Allowed("")
}

// validateSNSToRPMigration validates that a Subnamespace is not asked to be migrated to be under a ResourcePool.
func (v *MigrationHierarchyValidator) validateSNSToRPMigration(isCurrentNSResourcePool, isToNSResourcePool bool) admission.Response {
	if !isCurrentNSResourcePool && isToNSResourcePool {
		message := "it's forbidden to migrate from a Subnamespace to a ResourcePool. You can convert the subnamespace to a ResourcePool and try again"
		return admission.Denied(message)
	}

	return admission.Allowed("")
}

// validateNonUpperRPToSNSMigration validates that a non-upper ResourcePool is not asked to be migrated to be under a subnamespace.
func (v *MigrationHierarchyValidator) validateNonUpperRPToSNSMigration(isCurrentUpperResourcePool, isToNSResourcePool bool) admission.Response {
	if !isCurrentUpperResourcePool && !isToNSResourcePool {

		message := "it's forbidden to migrate a non-upper ResourcePool to be under a Subnamespace"
		return admission.Denied(message)
	}

	return admission.Allowed("")
}

// validateKeyCountInDB validates that migrating a subnamespace and all its children
// to the new parent subnamespace will not cause the new parent to exceed the maximum
// limit of namespaces in its hierarchy.
func (v *MigrationHierarchyValidator) validateKeyCountInDB(ctx context.Context, toNSKey, currentNSName string) admission.Response {
	logger := log.FromContext(ctx)
	childrenNum, err := getNSChildrenNum(ctx, v.Client, currentNSName)
	if err != nil {
		logger.Error(err, "failed to compute number of children", "currentNS", currentNSName)
		return admission.Denied(err.Error())
	}

	if (v.NamespaceDB.KeyCount(toNSKey) + childrenNum) >= v.MaxSNS {
		message := fmt.Sprintf("it's forbidden to create more than %v namespaces under hierarchy %q", v.MaxSNS, toNSKey)
		return admission.Denied(message)
	}

	return admission.Allowed("")
}

// getNSChildrenNum returns the number of children of a subnamespace by looking at its CRQ.
func getNSChildrenNum(ctx context.Context, c client.Client, nsname string) (int, error) {
	crq := quotav1.ClusterResourceQuota{}
	if err := c.Get(ctx, types.NamespacedName{Name: nsname}, &crq); err != nil {
		return 0, err
	}

	return len(crq.Status.Namespaces), nil
}
