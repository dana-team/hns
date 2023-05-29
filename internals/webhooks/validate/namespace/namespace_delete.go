package webhooks

import (
	"fmt"
	danav1 "github.com/dana-team/hns/api/v1"
	"github.com/dana-team/hns/internals/utils"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// handle implements the non-boilerplate logic of the validator, allowing it to be more easily unit
// tested (i.e. without constructing a full admission.Request)
func (a *NamespaceAnnotator) handleDelete(nsObject *utils.ObjectContext) admission.Response {
	if nsObject.Object.(*corev1.Namespace).Labels[danav1.Hns] == "" {
		return admission.Allowed("namespaces not managed by HNS are not validated")
	}

	if response := a.validateNamespaceRole(nsObject); !response.Allowed {
		return response
	}

	return admission.Allowed("")
}

// validateNamespaceRole validates the role of a namespace
func (a *NamespaceAnnotator) validateNamespaceRole(nsObject *utils.ObjectContext) admission.Response {
	nsName := nsObject.Object.GetName()
	nsRole := nsObject.Object.GetAnnotations()[danav1.Role]

	allowedMessage := fmt.Sprintf("deleting root namespace '%s' is allowed because it has not children", nsName)
	deniedMessage := fmt.Sprintf("it's forbidden to delete namespace '%s' because it currently has "+
		"children subnamespaces. Please delete them and try again", nsName)

	if nsRole == danav1.Leaf {
		return admission.Allowed(allowedMessage)
	} else if nsRole == danav1.NoRole {
		return admission.Denied(deniedMessage)
	} else if nsRole == danav1.Root {
		if utils.IsChildlessNamespace(nsObject) {
			return admission.Allowed(allowedMessage)
		}
		return admission.Denied(deniedMessage)
	}

	message := fmt.Sprintf("expected error, namespace does not have '%s' annotation", danav1.Role)
	return admission.Denied(message)
}
