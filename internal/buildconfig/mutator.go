package buildconfig

import (
	"context"
	"encoding/json"
	buildv1 "github.com/openshift/api/build/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"net/http"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

type BuildConfigMutator struct {
	Client  client.Client
	Decoder *admission.Decoder
}

// +kubebuilder:webhook:path=/mutate-v1-buildconfig,mutating=true,sideEffects=NoneOnDryRun,failurePolicy=fail,groups="build.openshift.io",resources=buildconfigs,verbs=create,versions=v1,name=buildconfig.dana.io,admissionReviewVersions=v1;v1beta1

// Handle implements the mutation webhook.
func (m *BuildConfigMutator) Handle(ctx context.Context, req admission.Request) admission.Response {
	logger := log.FromContext(ctx).WithValues("webhook", "BuildConfig Webhook")
	logger.Info("webhook request received")

	buildConfig := buildv1.BuildConfig{}
	if err := m.Decoder.DecodeRaw(req.Object, &buildConfig); err != nil {
		logger.Error(err, "failed to decode object", "request object", req.Object)
		return admission.Errored(http.StatusBadRequest, err)
	}

	m.setDefaultValues(&buildConfig)
	marshalBuildConfig, err := json.Marshal(buildConfig)
	if err != nil {
		logger.Error(err, "failed to marshal object", "object", buildConfig)
		return admission.Errored(http.StatusInternalServerError, err)
	}

	return admission.PatchResponseFromRaw(req.Object.Raw, marshalBuildConfig)
}

// setDefaultValues sets values for a bulilConfig object.
func (m *BuildConfigMutator) setDefaultValues(buildConfig *buildv1.BuildConfig) {
	if len(buildConfig.Spec.CommonSpec.Resources.Requests) == 0 {
		buildConfig.Spec.Resources.Requests = corev1.ResourceList{"cpu": resource.MustParse("1"), "memory": resource.MustParse("2Gi")}
		buildConfig.Spec.Resources.Limits = corev1.ResourceList{"cpu": resource.MustParse("1"), "memory": resource.MustParse("2Gi")}
	}
}
