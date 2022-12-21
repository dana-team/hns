package webhooks

import (
	danav1 "github.com/dana-team/hns/api/v1"
	"github.com/dana-team/hns/internals/utils"
	"context"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"net/http"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

type NamespaceAnnotator struct {
	Client  client.Client
	Decoder *admission.Decoder
	Log     logr.Logger
}

// +kubebuilder:webhook:path=/validate-v1-namespace,mutating=false,sideEffects=NoneOnDryRun,failurePolicy=fail,groups="core",resources=namespaces,verbs=delete,versions=v1,name=namespace.dana.io,admissionReviewVersions=v1;v1beta1

func (a *NamespaceAnnotator) Handle(ctx context.Context, req admission.Request) admission.Response {
	log := a.Log.WithValues("webhook", "Namespace Webhook", "Name", req.Name)
	log.Info("webhook request received")

	namespace, err := utils.NewObjectContext(ctx, log, a.Client, types.NamespacedName{}, &corev1.Namespace{})
	if err != nil {
		log.Error(err, "unable to create namespace objectContext")
	}
	if err := a.Decoder.DecodeRaw(req.OldObject, namespace.Object); err != nil {
		log.Error(err, "could not decode namespace object")
		return admission.Errored(http.StatusBadRequest, err)
	}

	if !hasDanaLabel(namespace) {
		return admission.Allowed(allowMessageValidateNamespace)
	}

	switch namespace.Object.GetAnnotations()[danav1.Role] {
	case danav1.Leaf:
		return admission.Allowed(allowMessageValidateNamespace)
	case danav1.NoRole:
		return admission.Denied(denyMessageValidateNamespace)
	case danav1.Root:
		if utils.IsChildlessNamespace(namespace) {
			return admission.Allowed(allowMessageValidateNamespace)
		}
		return admission.Denied(denyMessageValidateNamespace)
	default:
		return admission.Denied(contactMessage)
	}
}

func hasDanaLabel(namespace *utils.ObjectContext) bool {
	return namespace.Object.(*corev1.Namespace).Labels[danav1.Hns] != ""
}
