package utils

import (
	"context"
	"fmt"
	danav1 "github.com/dana-team/hns/api/v1"
	authv1 "k8s.io/api/authorization/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"net/http"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

// ValidateNamespaceExist validates that a namespace exists
func ValidateNamespaceExist(ns *ObjectContext) admission.Response {
	if !(ns.IsPresent()) {
		message := fmt.Sprintf("namespace '%s' does not exist", ns.Object.GetName())
		return admission.Denied(message)
	}

	return admission.Allowed("")
}

// ValidateToNamespaceName validates that a namespace is not trying to be migrated
// to be under the same namespace it's already in
func ValidateToNamespaceName(ns *ObjectContext, toNSName string) admission.Response {
	currentParent := GetNamespaceParent(ns.Object)

	if toNSName == currentParent {
		message := fmt.Sprintf("'%s' is already under '%s'", ns.Object.GetName(), toNSName)
		return admission.Denied(message)
	}

	return admission.Allowed("")
}

// ValidateSecondaryRoot denies if trying to perform UpdateQuota involving namesapces from different secondary root namespaces
// a secondary root is the first subnamespace after the root namespace in the hierarchy of a subnamespace
func ValidateSecondaryRoot(ctx context.Context, c client.Client, aNSArray, bNSArray []string) admission.Response {
	logger := log.FromContext(ctx)

	aNSSecondaryRootName := aNSArray[1]
	bNSSecondaryRootName := bNSArray[1]

	if aNSSecondaryRootName == "" || bNSSecondaryRootName == "" {
		message := fmt.Sprintf("it is forbidden to do operations on subnamespaces without a set display-name")
		return admission.Denied(message)
	}

	aNSSecondaryRoot, err := NewObjectContext(ctx, c, client.ObjectKey{Name: aNSSecondaryRootName}, &corev1.Namespace{})
	if err != nil {
		logger.Error(err, "failed to create object", "sourceNSSecondaryRoot", aNSSecondaryRootName)
		return admission.Errored(http.StatusBadRequest, err)
	}

	bNSSecondaryRoot, err := NewObjectContext(ctx, c, client.ObjectKey{Name: bNSSecondaryRootName}, &corev1.Namespace{})
	if err != nil {
		logger.Error(err, "failed to create object", "destNSSecondaryRoot", bNSSecondaryRootName)
		return admission.Errored(http.StatusBadRequest, err)
	}

	if IsSecondaryRootNamespace(aNSSecondaryRoot.Object) || IsSecondaryRootNamespace(bNSSecondaryRoot.Object) {
		if aNSSecondaryRootName != bNSSecondaryRootName {
			message := fmt.Sprintf("it is forbidden to perform operations between subnamespaces from hierarchy '%s' and "+
				"subnamespaces from hierarchy '%s'", aNSSecondaryRootName, bNSSecondaryRootName)
			return admission.Denied(message)
		}
	}

	return admission.Allowed("")
}

// ValidatePermissions checks if a reqistered user has the needed permissions on the namespaces and denies otherwise
// there are 3 scenarios in which things are allowed: if the user has the needed permissions on the Ancestor
// of the two namespaces; if the user has the needed permissions on both namespaces; if the user has the needed
// permissions on the namespace from which resources are moved and both namespaces are in the same branch
// (only checked when the branch flag is true)
func ValidatePermissions(ctx context.Context, aNS []string, aNSName, bNSName, ancestorNSName, reqUser string, branch bool) admission.Response {
	logger := log.FromContext(ctx)

	hasSourcePermissions, err := PermissionsExist(ctx, reqUser, aNSName)
	if err != nil {
		logger.Error(err, "failed to verify source permissions")
		return admission.Errored(http.StatusBadRequest, err)
	}

	hasDestPermissions, err := PermissionsExist(ctx, reqUser, bNSName)
	if err != nil {
		logger.Error(err, "failed to verify destination permissions")
		return admission.Errored(http.StatusBadRequest, err)
	}

	hasAncestorPermissions, err := PermissionsExist(ctx, reqUser, ancestorNSName)
	if err != nil {
		logger.Error(err, "failed to verify ancestor permissions")
		return admission.Errored(http.StatusBadRequest, err)
	}

	inBranch := ContainsString(aNS, bNSName)

	if branch {
		if !hasAncestorPermissions && !(hasSourcePermissions && hasDestPermissions) && !(hasSourcePermissions && inBranch) {
			message := fmt.Sprintf("you must have permissions on: '%s' and '%s', or permissions on '%s', to perform "+
				"this operation. Having permissions only on '%s', is enough just when resources are moved in the same branch of the hierarchy",
				aNSName, bNSName, ancestorNSName, aNSName)
			return admission.Denied(message)
		}
	} else {
		if !hasAncestorPermissions && !(hasSourcePermissions && hasDestPermissions) {
			message := fmt.Sprintf("you must have permissions on: '%s' and '%s', or permissions on '%s', to perform "+
				"this operation", aNSName, bNSName, ancestorNSName)
			return admission.Denied(message)
		}
	}

	return admission.Allowed("")
}

// PermissionsExist checks if a user has permission to create a pod in a given namespace.
// It impersonates the reqUser and uses SelfSubjectAccessReview API to check if the action is allowed or denied.
// It returns a boolean value indicating whether the user has permission to create the pod or not
func PermissionsExist(ctx context.Context, reqUser, namespace string) (bool, error) {
	if reqUser == fmt.Sprintf("system:serviceaccount:%s:%s", danav1.SNSNamespace, danav1.SNSServiceAccount) {
		return true, nil
	}

	config, err := rest.InClusterConfig()
	if err != nil {
		return false, fmt.Errorf("unable to get in cluster config: %v", err.Error())
	}

	// create a new Kubernetes client using the configuration
	clientSet, err := kubernetes.NewForConfig(config)
	if err != nil {
		return false, fmt.Errorf("unable to create clientSet: %v", err.Error())
	}

	// create a new SelfSubjectAccessReview API object for checking permissions
	action := authv1.ResourceAttributes{
		Namespace: namespace,
		Verb:      "create",
		Resource:  "pods",
	}

	check := authv1.SubjectAccessReview{
		Spec: authv1.SubjectAccessReviewSpec{
			ResourceAttributes: &action,
			User:               reqUser,
		},
	}

	// check the permissions for the user
	resp, err := clientSet.AuthorizationV1().SubjectAccessReviews().Create(ctx, &check, metav1.CreateOptions{})
	if err != nil {
		panic(err.Error())
	}

	// check the response status to determine whether the user has permission to create the pod or not
	if resp.Status.Allowed {
		return true, nil
	}

	return false, nil
}
