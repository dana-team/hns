package webhooks

import (
	"context"
	"github.com/dana-team/hns/internals/utils"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"net/http"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

type NamespaceAnnotator struct {
	Client  client.Client
	Decoder *admission.Decoder
}

// +kubebuilder:webhook:path=/validate-v1-namespace,mutating=false,sideEffects=NoneOnDryRun,failurePolicy=fail,groups="core",resources=namespaces,verbs=delete,versions=v1,name=namespace.dana.io,admissionReviewVersions=v1;v1beta1

// Handle implements the validation webhook
func (a *NamespaceAnnotator) Handle(ctx context.Context, req admission.Request) admission.Response {
	logger := log.FromContext(ctx).WithValues("webhook", "Namespace Webhook", "Name", req.Name)
	logger.Info("webhook request received")

	nsObject, err := utils.NewObjectContext(ctx, a.Client, types.NamespacedName{}, &corev1.Namespace{})
	if err != nil {
		logger.Error(err, "failed to create object context")
		return admission.Errored(http.StatusBadRequest, err)
	}

	if err := a.Decoder.DecodeRaw(req.OldObject, nsObject.Object); err != nil {
		logger.Error(err, "failed to decode object", "request object", req.Object)
		return admission.Errored(http.StatusBadRequest, err)
	}

	if req.Operation == admissionv1.Delete {
		if response := a.handleDelete(nsObject); !response.Allowed {
			return response
		}
	}

	return admission.Allowed("all validations passed")
}
