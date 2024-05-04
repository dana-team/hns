package namespace

import (
	"context"
	"net/http"

	"github.com/dana-team/hns/internal/objectcontext"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

type NamespaceValidator struct {
	Client  client.Client
	Decoder *admission.Decoder
}

// +kubebuilder:webhook:path=/validate-v1-namespace,mutating=false,sideEffects=NoneOnDryRun,failurePolicy=fail,groups="core",resources=namespaces,verbs=delete,versions=v1,name=namespace.dana.io,admissionReviewVersions=v1;v1beta1

// Handle implements the validation webhook.
func (v *NamespaceValidator) Handle(ctx context.Context, req admission.Request) admission.Response {
	logger := log.FromContext(ctx).WithValues("webhook", "Namespace Webhook", "Name", req.Name)
	logger.Info("webhook request received")

	nsObject, err := objectcontext.New(ctx, v.Client, types.NamespacedName{}, &corev1.Namespace{})
	if err != nil {
		logger.Error(err, "failed to create object context")
		return admission.Errored(http.StatusBadRequest, err)
	}

	if err := v.Decoder.DecodeRaw(req.OldObject, nsObject.Object); err != nil {
		logger.Error(err, "failed to decode object", "request object", req.Object)
		return admission.Errored(http.StatusBadRequest, err)
	}

	if req.Operation == admissionv1.Delete {
		if response := v.handleDelete(nsObject); !response.Allowed {
			return response
		}
	}

	return admission.Allowed("all validations passed")
}
