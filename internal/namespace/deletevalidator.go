package namespace

import (
	"fmt"
	danav1 "github.com/dana-team/hns/api/v1"
	"github.com/dana-team/hns/internal/namespace/nsutils"
	"github.com/dana-team/hns/internal/objectcontext"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// handle implements the non-boilerplate logic of the validator, allowing it to be more easily unit
// tested (i.e. without constructing a full admission.Request).
func (v *NamespaceValidator) handleDelete(nsObject *objectcontext.ObjectContext) admission.Response {
	if nsObject.Object.(*corev1.Namespace).Labels[danav1.Hns] == "" {
		return admission.Allowed("namespaces not managed by HNS are not validated")
	}

	if response := v.validateNamespaceRole(nsObject); !response.Allowed {
		return response
	}

	return admission.Allowed("")
}

// validateNamespaceRole validates the role of a namespace.
func (v *NamespaceValidator) validateNamespaceRole(nsObject *objectcontext.ObjectContext) admission.Response {
	nsName := nsObject.Name()
	nsRole := nsObject.Object.GetAnnotations()[danav1.Role]

	allowedMessage := fmt.Sprintf("deleting root namespace %q is allowed because it has not children", nsName)
	deniedMessage := fmt.Sprintf("it's forbidden to delete namespace %q because it currently has "+
		"children subnamespaces. Please delete them and try again", nsName)

	if nsRole == danav1.Leaf {
		return admission.Allowed(allowedMessage)
	} else if nsRole == danav1.NoRole {
		return admission.Denied(deniedMessage)
	} else if nsRole == danav1.Root {
		if nsutils.IsChildless(nsObject) {
			return admission.Allowed(allowedMessage)
		}
		return admission.Denied(deniedMessage)
	}

	message := fmt.Sprintf("expected error, namespace does not have %q annotation", danav1.Role)
	return admission.Denied(message)
}
