package migrationhierarchy

import (
	"context"
	"encoding/json"
	"net/http"

	danav1 "github.com/dana-team/hns/api/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

type MigrationHierarchyMutator struct {
	Client  client.Client
	Decoder *admission.Decoder
}

// +kubebuilder:webhook:path=/mutate-v1-migrationhierarchy,mutating=true,sideEffects=NoneOnDryRun,failurePolicy=fail,groups="dana.hns.io",resources=migrationhierarchies,verbs=create,versions=v1,name=migrationhierarchy.dana.io,admissionReviewVersions=v1;v1beta1

// Handle implements the mutation webhook.
func (m *MigrationHierarchyMutator) Handle(ctx context.Context, req admission.Request) admission.Response {
	logger := log.FromContext(ctx).WithValues("webhook", "MigrationHierarchy mutation Webhook")
	logger.Info("webhook request received")

	migrationHierarchy := danav1.MigrationHierarchy{}
	if err := m.Decoder.DecodeRaw(req.Object, &migrationHierarchy); err != nil {
		logger.Error(err, "failed to decode object", "request object", req.Object)
		return admission.Errored(http.StatusBadRequest, err)
	}
	marshalMigrationHierarchy, err := m.UpdateRequester(migrationHierarchy, req.UserInfo.Username)
	if err != nil {
		logger.Error(err, "failed to marshal object", "object", migrationHierarchy)
		return admission.Errored(http.StatusInternalServerError, err)
	}

	return admission.PatchResponseFromRaw(req.Object.Raw, marshalMigrationHierarchy)
}

// UpdateRequester adds a requester annotation to the object.
func (m *MigrationHierarchyMutator) UpdateRequester(migrationHierarchyObject danav1.MigrationHierarchy, requester string) ([]byte, error) {
	migrationHierarchyObject.Annotations["requester"] = requester
	marshalUpdateQuota, err := json.Marshal(migrationHierarchyObject)
	if err != nil {
		return nil, err
	}
	return marshalUpdateQuota, nil
}
