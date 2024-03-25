package subnamespace

import (
	"fmt"

	danav1 "github.com/dana-team/hns/api/v1"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// handleDelete implements the logic for the deletion of a subnamespace
func (v *SubnamespaceValidator) handleDelete(req admission.Request) admission.Response {

	response := v.validateServiceAccount(req.UserInfo.Username)
	return response
}

// validateServiceAccount validates that the account requesting the deletion of a subnamespace
// is the service account of the sns operator. Otherwise it will deny the request
func (v *SubnamespaceValidator) validateServiceAccount(userName string) admission.Response {
	if userName != fmt.Sprintf("system:serviceaccount:%s:%s", danav1.SNSNamespace, danav1.SNSServiceAccount) {
		return admission.Denied(fmt.Sprintf("%s is not allowed to delete subnamespaces", userName))

	}
	return admission.Allowed("")
}
