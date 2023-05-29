package webhooks

import (
	"fmt"
	danav1 "github.com/dana-team/hns/api/v1"
	"github.com/dana-team/hns/internals/utils"
	corev1 "k8s.io/api/core/v1"
	"net/http"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// handleCreate implements the non-boilerplate logic of the validator, allowing it to be more easily unit
// tested (i.e. without constructing a full admission.Request)
func (a *UpdateQuotaAnnotator) handleCreate(upqObject *utils.ObjectContext, username string) admission.Response {

	ctx := upqObject.Ctx
	logger := log.FromContext(ctx)

	sourceNSName := upqObject.Object.(*danav1.Updatequota).Spec.SourceNamespace
	sourceNS, err := utils.NewObjectContext(ctx, a.Client, client.ObjectKey{Name: sourceNSName}, &corev1.Namespace{})
	if err != nil {
		logger.Error(err, "failed to create object", "sourceNS", sourceNSName)
		return admission.Errored(http.StatusBadRequest, err)
	}

	destNSName := upqObject.Object.(*danav1.Updatequota).Spec.DestNamespace
	destNS, err := utils.NewObjectContext(ctx, a.Client, client.ObjectKey{Name: destNSName}, &corev1.Namespace{})
	if err != nil {
		logger.Error(err, "failed to create object", "destNS", destNSName)
		return admission.Errored(http.StatusBadRequest, err)
	}

	if response := utils.ValidateNamespaceExist(sourceNS); !response.Allowed {
		return response
	}

	if response := utils.ValidateNamespaceExist(destNS); !response.Allowed {
		return response
	}

	sourceNSSliced := utils.GetNSDisplayNameSlice(sourceNS)
	destNSSliced := utils.GetNSDisplayNameSlice(destNS)
	ancestorNSName, isAncestorRoot, err := utils.GetAncestor(sourceNSSliced, destNSSliced)
	if err != nil {
		logger.Error(err, "failed to get ancestor", "source namespace", sourceNSName, "destination namespace", destNSName)
		return admission.Errored(http.StatusBadRequest, err)
	}

	// validate the source and destination namespaces are under the same secondary root only
	// if you are not trying to move resources from or to the root namespace of the cluster
	if (isAncestorRoot) && (!utils.IsRootNamespace(sourceNS.Object) && !utils.IsRootNamespace(destNS.Object)) {
		if response := utils.ValidateSecondaryRoot(ctx, a.Client, sourceNSSliced, destNSSliced); !response.Allowed {
			return response
		}
	}

	if response := utils.ValidatePermissions(ctx, sourceNSSliced, sourceNSName, destNSName, ancestorNSName, username, true); !response.Allowed {
		return response
	}

	if response := a.validateNSQuotaObject(sourceNS); !response.Allowed {
		return response
	}

	if response := a.validateNSQuotaObject(destNS); !response.Allowed {
		return response
	}

	return admission.Allowed("")
}

// validateNSQuotaObject makes sure that a namespace has a corresponding quota object and denies if it doesn't
func (a *UpdateQuotaAnnotator) validateNSQuotaObject(ns *utils.ObjectContext) admission.Response {
	if utils.IsRootNamespace(ns.Object) {
		return a.validateRootNSQuotaObject(ns)
	} else {
		return a.validateNonRootNSQuotaObject(ns)
	}
}

// validateNonRootNSQuotaObject validates that a quota object exists for a
// namespace that is a root namespace
func (a *UpdateQuotaAnnotator) validateRootNSQuotaObject(ns *utils.ObjectContext) admission.Response {
	sns := ns
	logger := ns.Log

	quotaObject, err := utils.GetRootNSQuotaObject(sns)
	if err != nil {
		logger.Error(err, "failed to get object", "quotaObject", sns.GetName())
		return admission.Errored(http.StatusInternalServerError, err)
	}

	if !(quotaObject.IsPresent()) {
		message := fmt.Sprintf("quota object '%s' does not exist", sns.GetName())
		return admission.Denied(message)
	}

	return admission.Allowed("")
}

// validateNonRootNSQuotaObject validates that a quota object exists for a
// namespace that is not a root namespace
func (a *UpdateQuotaAnnotator) validateNonRootNSQuotaObject(ns *utils.ObjectContext) admission.Response {
	sns, err := utils.GetSNSFromNamespace(ns)
	logger := ns.Log

	if err != nil {
		logger.Error(err, "failed to get object", "subnamespace", sns.GetName())
		return admission.Errored(http.StatusBadRequest, err)
	}

	quotaObject, err := utils.GetSNSQuotaObject(sns)
	if err != nil {
		logger.Error(err, "failed to get object", "quotaObj", sns.GetName())
		return admission.Errored(http.StatusInternalServerError, err)
	}

	if !(quotaObject.IsPresent()) {
		message := fmt.Sprintf("quota object '%s' does not exist", sns.GetName())
		return admission.Denied(message)
	}

	return admission.Allowed("")
}
