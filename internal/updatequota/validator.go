package updatequota

import (
	"context"
	"fmt"
	danav1 "github.com/dana-team/hns/api/v1"
	"github.com/dana-team/hns/internal/objectcontext"
	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/types"
	"net/http"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

type UpdateQuotaValidator struct {
	Client  client.Client
	Decoder *admission.Decoder
}

// +kubebuilder:webhook:path=/validate-v1-updatequota,mutating=false,sideEffects=NoneOnDryRun,failurePolicy=fail,groups="dana.hns.io",resources=updatequota,verbs=create;update,versions=v1,name=updatequota.dana.io,admissionReviewVersions=v1;v1beta1

// Handle implements the validation webhook.
func (v *UpdateQuotaValidator) Handle(ctx context.Context, req admission.Request) admission.Response {
	logger := log.FromContext(ctx).WithValues("webhook", "UpdateQuota Webhook", "Name", req.Name)
	logger.Info("webhook request received")

	upqObject, err := objectcontext.New(ctx, v.Client, types.NamespacedName{}, &danav1.Updatequota{})
	if err != nil {
		logger.Error(err, "failed to create object context")
		return admission.Errored(http.StatusBadRequest, err)

	}

	if err := v.Decoder.DecodeRaw(req.Object, upqObject.Object); err != nil {
		logger.Error(err, "failed to decode object", "request object", req.Object)
		return admission.Errored(http.StatusBadRequest, err)
	}

	if req.Operation == admissionv1.Create {
		username := req.UserInfo.Username
		if response := v.handleCreate(upqObject, username); !response.Allowed {
			return response
		}
	}

	// deny update of an UpdateQuota object after it's already been created
	// (i.e. the Phase in the Status is not empty)
	if req.Operation == admissionv1.Update {
		oldUPQ := &danav1.Updatequota{}
		if err := v.Decoder.DecodeRaw(req.OldObject, oldUPQ); err != nil {
			logger.Error(err, "failed to decode object", "request object", req.OldObject)
			return admission.Errored(http.StatusBadRequest, err)
		}
		if !reflect.ValueOf(oldUPQ.Status).IsZero() {
			message := fmt.Sprintf("it is forbidden to update an object of type %q", oldUPQ.TypeMeta.Kind)
			return admission.Denied(message)
		}
	}

	return admission.Allowed("all validations passed")
}
