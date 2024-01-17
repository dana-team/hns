package migrationhierarchy

import (
	"context"
	"fmt"
	"github.com/dana-team/hns/internal/namespacedb"
	"github.com/dana-team/hns/internal/objectcontext"
	"net/http"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/log"

	danav1 "github.com/dana-team/hns/api/v1"
	admissionv1 "k8s.io/api/admission/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

type MigrationHierarchyValidator struct {
	Client      client.Client
	Decoder     *admission.Decoder
	NamespaceDB *namespacedb.NamespaceDB
	MaxSNS      int
}

// +kubebuilder:webhook:path=/validate-v1-migrationhierarchy,mutating=false,sideEffects=NoneOnDryRun,failurePolicy=fail,groups="dana.hns.io",resources=migrationhierarchies,verbs=create;update,versions=v1,name=migrationhierarchy.dana.io,admissionReviewVersions=v1;v1beta1

// Handle implements the validation webhook.
func (v *MigrationHierarchyValidator) Handle(ctx context.Context, req admission.Request) admission.Response {
	logger := log.FromContext(ctx).WithValues("webhook", "MigrationHierarchy Webhook", "Name", req.Name)
	logger.Info("webhook request received")

	mhObject, err := objectcontext.New(ctx, v.Client, types.NamespacedName{}, &danav1.MigrationHierarchy{})
	if err != nil {
		logger.Error(err, "failed to create object context")
		return admission.Errored(http.StatusBadRequest, err)
	}

	if err := v.Decoder.DecodeRaw(req.Object, mhObject.Object); err != nil {
		logger.Error(err, "failed to decode object", "request object", req.Object)
		return admission.Errored(http.StatusBadRequest, err)
	}

	if req.Operation == admissionv1.Create {
		reqUser := req.UserInfo.Username
		if response := v.handleCreate(mhObject, reqUser); !response.Allowed {
			return response
		}
	}

	// deny update of an MigrationHierarchy object after it's already been created
	// (i.e. the Phase in the Status is not empty)
	if req.Operation == admissionv1.Update {
		oldMH := &danav1.MigrationHierarchy{}
		if err := v.Decoder.DecodeRaw(req.OldObject, oldMH); err != nil {
			logger.Error(err, "could not decode object")
			return admission.Errored(http.StatusBadRequest, err)
		}
		if !reflect.DeepEqual(mhObject.Object.(*danav1.MigrationHierarchy).Spec, oldMH.Spec) {
			message := fmt.Sprintf("it is forbidden to update an object of type %q", oldMH.TypeMeta.Kind)
			return admission.Denied(message)
		}
	}

	return admission.Allowed("all validations passed")
}
