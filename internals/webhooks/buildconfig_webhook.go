package webhooks

import (
	"context"
	"encoding/json"
	"github.com/go-logr/logr"
	buildv1 "github.com/openshift/api/build/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"net/http"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

type BuildConfigAnnotator struct {
	Client  client.Client
	Decoder *admission.Decoder
	Log     logr.Logger
}

// +kubebuilder:webhook:path=/mutate-v1-buildconfig,mutating=true,sideEffects=NoneOnDryRun,failurePolicy=fail,groups="build.openshift.io",resources=buildconfigs,verbs=create,versions=v1,name=buildconfig.dana.io,admissionReviewVersions=v1;v1beta1

func (a *BuildConfigAnnotator) Handle(ctx context.Context, req admission.Request) admission.Response {
	log := a.Log.WithValues("webhook", "BuildConfig Webhook")
	log.Info("webhook request received")

	buildConfig := buildv1.BuildConfig{}

	if err := a.Decoder.DecodeRaw(req.Object, &buildConfig); err != nil {
		log.Error(err, "could not decode object")
		return admission.Errored(http.StatusBadRequest, err)
	}

	buildConfigCopy := buildConfig.DeepCopy()
	if len(buildConfigCopy.Spec.CommonSpec.Resources.Requests) != 0 {
		return admission.Allowed("BuildConfig already have resources")
	}

	buildConfigCopy.Spec.Resources.Requests = corev1.ResourceList{"cpu": resource.MustParse("1100m"), "memory": resource.MustParse("2G")}
	buildConfigCopy.Spec.Resources.Limits = corev1.ResourceList{"cpu": resource.MustParse("1100m"), "memory": resource.MustParse("2G")}
	marshalBuildConfig, err := json.Marshal(buildConfigCopy)
	if err != nil {
		log.Error(err, "could not Marshal object")
		return admission.Errored(http.StatusInternalServerError, err)
	}

	return admission.PatchResponseFromRaw(req.Object.Raw, marshalBuildConfig)
}
