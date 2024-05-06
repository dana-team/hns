package updatequota

import (
	"context"
	"encoding/json"
	"net/http"

	danav1 "github.com/dana-team/hns/api/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

type UpdateQuotaMutator struct {
	Client  client.Client
	Decoder admission.Decoder
}

// +kubebuilder:webhook:path=/mutate-v1-updatequota,mutating=true,sideEffects=NoneOnDryRun,failurePolicy=fail,groups="dana.hns.io",resources=updatequota,verbs=create,versions=v1,name=updatequota.dana.io,admissionReviewVersions=v1;v1beta1

// Handle implements the mutation webhook.
func (m *UpdateQuotaMutator) Handle(ctx context.Context, req admission.Request) admission.Response {
	logger := log.FromContext(ctx).WithValues("webhook", "UpdateQuota mutation Webhook")
	logger.Info("webhook request received")
	updateQuota := danav1.Updatequota{}
	if err := m.Decoder.DecodeRaw(req.Object, &updateQuota); err != nil {
		logger.Error(err, "failed to decode object", "request object", req.Object)
		return admission.Errored(http.StatusBadRequest, err)
	}
	marshalUpdateQuota, err := m.UpdateRequester(updateQuota, req.UserInfo.Username)
	if err != nil {
		logger.Error(err, "failed to marshal object", "object", updateQuota)
		return admission.Errored(http.StatusInternalServerError, err)
	}

	return admission.PatchResponseFromRaw(req.Object.Raw, marshalUpdateQuota)
}

// UpdateRequester adds a requester annotation to the object.
func (m *UpdateQuotaMutator) UpdateRequester(updateQuotaObject danav1.Updatequota, requester string) ([]byte, error) {
	if updateQuotaObject.Annotations == nil {
		updateQuotaObject.Annotations = make(map[string]string)
	}
	updateQuotaObject.Annotations["requester"] = requester
	marshalUpdateQuota, err := json.Marshal(updateQuotaObject)
	if err != nil {
		return nil, err
	}
	return marshalUpdateQuota, nil
}
