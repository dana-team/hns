package webhooks

import (
	"context"
	"fmt"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/log"

	danav1 "github.com/dana-team/hns/api/v1"
	"github.com/dana-team/hns/internals/utils"
	"k8s.io/apimachinery/pkg/types"
	"net/http"

	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	admissionv1 "k8s.io/api/admission/v1"
)

type UpdateQuotaAnnotator struct {
	Client  client.Client
	Decoder *admission.Decoder
}

// +kubebuilder:webhook:path=/validate-v1-updatequota,mutating=false,sideEffects=NoneOnDryRun,failurePolicy=fail,groups="dana.hns.io",resources=updatequota,verbs=create;update,versions=v1,name=updatequota.dana.io,admissionReviewVersions=v1;v1beta1

// Handle implements the validation webhook
func (a *UpdateQuotaAnnotator) Handle(ctx context.Context, req admission.Request) admission.Response {
	logger := log.FromContext(ctx).WithValues("webhook", "UpdateQuota Webhook", "Name", req.Name)
	logger.Info("webhook request received")

	upqObject, err := utils.NewObjectContext(ctx, a.Client, types.NamespacedName{}, &danav1.Updatequota{})
	if err != nil {
		logger.Error(err, "failed to create object context")
		return admission.Errored(http.StatusBadRequest, err)

	}

	if err := a.Decoder.DecodeRaw(req.Object, upqObject.Object); err != nil {
		logger.Error(err, "failed to decode object", "request object", req.Object)
		return admission.Errored(http.StatusBadRequest, err)
	}

	if req.Operation == admissionv1.Create {
		username := req.UserInfo.Username
		if response := a.handleCreate(upqObject, username); !response.Allowed {
			return response
		}
	}

	// deny update of an UpdateQuota object after it's already been created
	// (i.e. the Phase in the Status is not empty)
	if req.Operation == admissionv1.Update {
		oldUPQ := &danav1.Updatequota{}
		if err := a.Decoder.DecodeRaw(req.OldObject, oldUPQ); err != nil {
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
