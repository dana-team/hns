package webhooks

import (
	"context"
	"github.com/dana-team/hns/internals/utils"
	admissionv1 "k8s.io/api/admission/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/types"
	"net/http"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

type RoleBindingAnnotator struct {
	Client  client.Client
	Decoder *admission.Decoder
}

// +kubebuilder:webhook:path=/validate-v1-rolebinding,mutating=false,sideEffects=NoneOnDryRun,failurePolicy=fail,groups="rbac.authorization.k8s.io",resources=rolebindings,verbs=delete,versions=v1,name=rolebinding.dana.io,admissionReviewVersions=v1;v1beta1

// Handle implements the validation webhook
func (a *RoleBindingAnnotator) Handle(ctx context.Context, req admission.Request) admission.Response {
	logger := log.FromContext(ctx).WithValues("webhook", "roleBinding Webhook", "Name", req.Name)
	logger.Info("webhook request received")

	rbObject, err := utils.NewObjectContext(ctx, a.Client, types.NamespacedName{}, &rbacv1.RoleBinding{})
	if err != nil {
		logger.Error(err, "failed to create object context")
		return admission.Errored(http.StatusBadRequest, err)
	}

	if req.Operation == admissionv1.Delete {
		if err := a.Decoder.DecodeRaw(req.OldObject, rbObject.Object); err != nil {
			logger.Error(err, "failed to decode object", "request object", req.Object)
			return admission.Errored(http.StatusBadRequest, err)
		}

		if response := a.handle(rbObject); !response.Allowed {
			return response
		}
	}

	return admission.Allowed("")
}
