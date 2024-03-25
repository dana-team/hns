package updatequota

import (
	"fmt"
	"net/http"

	danav1 "github.com/dana-team/hns/api/v1"
	"github.com/dana-team/hns/internal/common"
	"github.com/dana-team/hns/internal/namespace/nsutils"
	"github.com/dana-team/hns/internal/objectcontext"
	"github.com/dana-team/hns/internal/quota"
	"github.com/dana-team/hns/internal/subnamespace/snsutils"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// handleCreate implements the non-boilerplate logic of the validator, allowing it to be more easily unit
// tested (i.e. without constructing a full admission.Request).
func (v *UpdateQuotaValidator) handleCreate(upqObject *objectcontext.ObjectContext, username string) admission.Response {

	ctx := upqObject.Ctx
	logger := log.FromContext(ctx)

	sourceNSName := upqObject.Object.(*danav1.Updatequota).Spec.SourceNamespace
	sourceNS, err := objectcontext.New(ctx, v.Client, client.ObjectKey{Name: sourceNSName}, &corev1.Namespace{})
	if err != nil {
		logger.Error(err, "failed to create object", "sourceNS", sourceNSName)
		return admission.Errored(http.StatusBadRequest, err)
	}

	destNSName := upqObject.Object.(*danav1.Updatequota).Spec.DestNamespace
	destNS, err := objectcontext.New(ctx, v.Client, client.ObjectKey{Name: destNSName}, &corev1.Namespace{})
	if err != nil {
		logger.Error(err, "failed to create object", "destNS", destNSName)
		return admission.Errored(http.StatusBadRequest, err)
	}

	if response := common.ValidateNamespaceExist(sourceNS); !response.Allowed {
		return response
	}

	if response := common.ValidateNamespaceExist(destNS); !response.Allowed {
		return response
	}

	sourceNSSliced := nsutils.DisplayNameSlice(sourceNS)
	destNSSliced := nsutils.DisplayNameSlice(destNS)
	ancestorNSName, isAncestorRoot, err := snsutils.GetAncestor(sourceNSSliced, destNSSliced)
	if err != nil {
		logger.Error(err, "failed to get ancestor", "source namespace", sourceNSName, "destination namespace", destNSName)
		return admission.Errored(http.StatusBadRequest, err)
	}

	// validate the source and destination namespaces are under the same secondary root only
	// if you are not trying to move resources from or to the root namespace of the cluster
	if (isAncestorRoot) && (!nsutils.IsRoot(sourceNS.Object) && !nsutils.IsRoot(destNS.Object)) {
		if response := common.ValidateSecondaryRoot(ctx, v.Client, sourceNSSliced, destNSSliced); !response.Allowed {
			return response
		}
	}

	if response := common.ValidatePermissions(ctx, sourceNSSliced, sourceNSName, destNSName, ancestorNSName, username, true); !response.Allowed {
		return response
	}

	if response := v.validateNSQuotaObject(sourceNS); !response.Allowed {
		return response
	}

	if response := v.validateNSQuotaObject(destNS); !response.Allowed {
		return response
	}

	return admission.Allowed("")
}

// validateNSQuotaObject makes sure that a namespace has a corresponding quota object and denies if it doesn't.
func (v *UpdateQuotaValidator) validateNSQuotaObject(ns *objectcontext.ObjectContext) admission.Response {
	if nsutils.IsRoot(ns.Object) {
		return v.validateRootNSQuotaObject(ns)
	} else {
		return v.validateNonRootNSQuotaObject(ns)
	}
}

// validateNonRootNSQuotaObject validates that a quota object exists for a
// namespace that is a root namespace.
func (v *UpdateQuotaValidator) validateRootNSQuotaObject(ns *objectcontext.ObjectContext) admission.Response {
	logger := ns.Log

	quotaObject, err := quota.RootNSObject(ns)
	if err != nil {
		logger.Error(err, "failed to get object", "quotaObject", ns.Name())
		return admission.Errored(http.StatusInternalServerError, err)
	}

	if !(quotaObject.IsPresent()) {
		message := fmt.Sprintf("quota object %q does not exist", ns.Name())
		return admission.Denied(message)
	}

	return admission.Allowed("")
}

// validateNonRootNSQuotaObject validates that a quota object exists for a
// namespace that is not a root namespace.
func (v *UpdateQuotaValidator) validateNonRootNSQuotaObject(ns *objectcontext.ObjectContext) admission.Response {
	sns, err := nsutils.SNSFromNamespace(ns)
	logger := ns.Log

	if err != nil {
		logger.Error(err, "failed to get object", "subnamespace", sns.Name())
		return admission.Errored(http.StatusBadRequest, err)
	}

	quotaObject, err := quota.SubnamespaceObject(sns)
	if err != nil {
		logger.Error(err, "failed to get object", "quotaObj", sns.Name())
		return admission.Errored(http.StatusInternalServerError, err)
	}

	if !(quotaObject.IsPresent()) {
		message := fmt.Sprintf("quota object %q does not exist", sns.Name())
		return admission.Denied(message)
	}

	return admission.Allowed("")
}
