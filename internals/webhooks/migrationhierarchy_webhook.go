package webhooks

import (
	"context"
	"fmt"
	"net/http"
	"reflect"

	danav1 "github.com/dana-team/hns/api/v1"
	"github.com/dana-team/hns/internals/namespaceDB"
	"github.com/dana-team/hns/internals/utils"
	"github.com/go-logr/logr"
	quotav1 "github.com/openshift/api/quota/v1"
	admissionv1 "k8s.io/api/admission/v1"
	authv1 "k8s.io/api/authorization/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

type MigrationHierarchyAnnotator struct {
	Client      client.Client
	Decoder     *admission.Decoder
	Log         logr.Logger
	NamespaceDB *namespaceDB.NamespaceDB
}

// +kubebuilder:webhook:path=/validate-v1-migrationhierarchy,mutating=false,sideEffects=NoneOnDryRun,failurePolicy=fail,groups="dana.hns.io",resources=migrationhierarchies,verbs=create;update,versions=v1,name=migrationhierarchy.dana.io,admissionReviewVersions=v1;v1beta1

func (a *MigrationHierarchyAnnotator) Handle(ctx context.Context, req admission.Request) admission.Response {
	log := a.Log.WithValues("webhook", "migrationHierarchy Webhook", "Name", req.Name)
	log.Info("webhook request received")

	//Decode sns object
	migrationObject, err := utils.NewObjectContext(ctx, log, a.Client, types.NamespacedName{}, &danav1.MigrationHierarchy{})
	if err != nil {
		log.Error(err, "unable to create migration objectContext")
	}
	if err := a.Decoder.DecodeRaw(req.Object, migrationObject.Object); err != nil {
		log.Error(err, "could not decode migration object")
		return admission.Errored(http.StatusBadRequest, err)
	}

	// get the relevant namespace objects
	currentNamespace := migrationObject.Object.(*danav1.MigrationHierarchy).Spec.CurrentNamespace
	toNamespace := migrationObject.Object.(*danav1.MigrationHierarchy).Spec.ToNamespace

	currentNS, err := utils.NewObjectContext(ctx, log, a.Client, client.ObjectKey{Namespace: "", Name: currentNamespace}, &corev1.Namespace{})
	if err != nil {
		return admission.Denied(err.Error())
	}
	if !currentNS.IsPresent() {
		return admission.Denied(fmt.Sprintf(denyMessageNamespaceNotFound, currentNamespace))
	}

	toNS, err := utils.NewObjectContext(ctx, log, a.Client, client.ObjectKey{Namespace: "", Name: toNamespace}, &corev1.Namespace{})
	if err != nil {
		return admission.Denied(err.Error())
	}
	if !toNS.IsPresent() {
		return admission.Denied(fmt.Sprintf(denyMessageNamespaceNotFound, toNamespace))
	}

	// deny if trying to perform migration involving namesapces from different secondary root namespaces
	// a secondary root is the first subnamespace after the root namespace in the hierarchy of a subnamespace
	currentNSArray := utils.GetNSDisplayNameArray(currentNS)
	toNSArray := utils.GetNSDisplayNameArray(toNS)

	currentNSSecondaryRoot := currentNSArray[1]
	toNSSecondaryRoot := toNSArray[1]

	nsSecondaryRootCurrent, err := utils.NewObjectContext(ctx, log, a.Client, client.ObjectKey{Name: currentNSSecondaryRoot}, &corev1.Namespace{})
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	nsSecondaryRootTo, err := utils.NewObjectContext(ctx, log, a.Client, client.ObjectKey{Name: toNSSecondaryRoot}, &corev1.Namespace{})
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	if currentNSSecondaryRoot != "" && toNSSecondaryRoot != "" {
		if utils.IsSecondaryRootNamespace(nsSecondaryRootTo.Object) || utils.IsSecondaryRootNamespace(nsSecondaryRootCurrent.Object) {
			if currentNSSecondaryRoot != toNSSecondaryRoot {
				return admission.Denied("it is forbidden to migrate subnamespaces under hierarchy '" + currentNSSecondaryRoot +
					"' to subnamespaces under hierarchy '" + toNSSecondaryRoot + "'")
			}
		}
	}

	//validate the user can do operations on nsparent and ns child
	cani := a.validatePermissions(ctx, req, log, migrationObject.Object.(*danav1.MigrationHierarchy).Spec.CurrentNamespace)
	if !cani {
		return admission.Denied("you do not have permissions on namespace: " + migrationObject.Object.(*danav1.MigrationHierarchy).Spec.CurrentNamespace)
	}
	cani = a.validatePermissions(ctx, req, log, migrationObject.Object.(*danav1.MigrationHierarchy).Spec.ToNamespace)
	if !cani {
		return admission.Denied("you do not have permissions on namespace: " + migrationObject.Object.(*danav1.MigrationHierarchy).Spec.ToNamespace)
	}

	if req.Operation == admissionv1.Create {
		// migration from a subnamespace to a resourcepool is not allowed
		if utils.GetNamespaceResourcePooled(currentNS) == "false" && utils.GetNamespaceResourcePooled(toNS) == "true" {
			return admission.Denied(denyMessageMigrationNotAllowedSnsToRp)
		}

		// if the subnamespace is not a resourcepool, it is only allowed to migrate subnamespaces that either have a CRQ,
		// or their direct parent have a clusterresourcequota; otherwise, the webhook fails the request

		if utils.GetNamespaceResourcePooled(currentNS) == "false" && utils.GetNamespaceResourcePooled(toNS) == "false" {
			toKey := a.NamespaceDB.GetKey(toNamespace)
			currentKey := a.NamespaceDB.GetKey(currentNamespace)
			if (toKey == "") || (currentKey == "") || (currentKey == currentNamespace) {
				return admission.Denied(denyMessageMigrationNotAllowed)
			}
			childrenNum, err := numChildren(a.Client, currentNamespace)
			if err != nil {
				return admission.Denied(err.Error())
			}
			if (a.NamespaceDB.GetKeyCount(toKey) + childrenNum) >= danav1.MaxSNS {
				return admission.Denied(fmt.Sprintf(denyMessageCreatingMoreThanLimit, danav1.MaxSNS) + toKey)
			}
		}

		currSNS, err := utils.GetNamespaceSns(currentNS)
		if err != nil {
			return admission.Denied(err.Error())
		}
		toSNS, err := utils.GetNamespaceSns(toNS)
		if err != nil {
			return admission.Denied(err.Error())
		}

		if utils.GetNamespaceResourcePooled(toNS) == "false" {
			if validRequest, _ := ValidateMigrateSnsRequest(currSNS, toSNS); !validRequest {
				return admission.Denied(denyMessageMigrationNotAllowedTooFewResources)
			}
		} else if utils.GetNamespaceResourcePooled(toNS) == "true" {
			if validRequest, _ := ValidateMigrateRpRequest(currSNS, toSNS); !validRequest {
				return admission.Denied(denyMessageMigrationNotAllowedTooFewResourcesRP)
			}
		} else {
			return admission.Denied(contactMessage)
		}

		return admission.Allowed(allowMessageValidateQuotaObj)
	}

	if req.Operation == admissionv1.Update {
		oldObj := &danav1.MigrationHierarchy{}
		if err := a.Decoder.DecodeRaw(req.OldObject, oldObj); err != nil {
			log.Error(err, "could not decode object")
			return admission.Errored(http.StatusBadRequest, err)
		}
		if !reflect.DeepEqual(migrationObject.Object.(*danav1.MigrationHierarchy).Spec, oldObj.Spec) {
			return admission.Denied("It is forbidden to update an object of type " + oldObj.TypeMeta.Kind)
		}
		return admission.Allowed(allowMessageUpdatePhase)
	}

	return admission.Denied(contactMessage)
}

func numChildren(c client.Client, nsname string) (int, error) {
	crq := quotav1.ClusterResourceQuota{}
	if err := c.Get(context.Background(), types.NamespacedName{Name: nsname}, &crq); err != nil {
		return 0, err
	}
	return len(crq.Status.Namespaces), nil
}

func (a *MigrationHierarchyAnnotator) validatePermissions(ctx context.Context, req admission.Request, log logr.Logger, namespace string) bool {

	// check if user has cluster admin
	rbList, err := utils.NewObjectContextList(ctx, log, a.Client, &rbacv1.ClusterRoleBindingList{})
	if err != nil {
		admission.Denied(err.Error())
	}
	for _, roleBinding := range rbList.Objects.(*rbacv1.ClusterRoleBindingList).Items {
		for _, subject := range roleBinding.Subjects {
			if subject.Name == req.AdmissionRequest.UserInfo.Username {
				return true
			}
		}
	}

	//check regular user permissions
	//kubeconfig := fmt.Sprintf("%s/.kube/config", os.Getenv("HOME"))
	//config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	//if err != nil {
	//	panic(err.Error())
	//}
	config, err := rest.InClusterConfig()
	if err != nil {
		panic(err.Error())
	}
	config.Impersonate = rest.ImpersonationConfig{
		UserName: req.UserInfo.Username,
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		panic(err.Error())
	}
	action := authv1.ResourceAttributes{
		Namespace: namespace,
		Verb:      "get",
		Resource:  "pods",
	}

	selfCheck := authv1.SelfSubjectAccessReview{
		Spec: authv1.SelfSubjectAccessReviewSpec{
			ResourceAttributes: &action,
		},
	}
	resp, err := clientset.AuthorizationV1().SelfSubjectAccessReviews().Create(ctx, &selfCheck, metav1.CreateOptions{})
	if err != nil {
		panic(err.Error())
	}

	if resp.Status.Denied {
		return false
	}
	if resp.Status.Allowed {
		return true
	}
	return false

}

func ValidateMigrateSnsRequest(currSNS *utils.ObjectContext, toSNS *utils.ObjectContext) (bool, string) {
	// quotaParent will take the spec either from the RQ or CRQ object
	// based on the namespace depth
	quotaParent, err := utils.GetSNSQuota(toSNS)
	if err != nil {
		return false, err.Error()
	}

	quotaObjRequest := utils.GetSnsQuotaSpec(currSNS.Object).Hard
	childQuotaResources := utils.GetQuotaObjsListResources(utils.GetSnsChildrenQuotaObjs(toSNS))

	for res := range quotaParent {
		var (
			vChildren, _ = childQuotaResources[res]
			vParent, _   = quotaParent[res]
			vRequest, _  = quotaObjRequest[res]
		)

		vParent.Sub(vChildren)
		vParent.Sub(vRequest)
		if vParent.Value() < 0 {
			return false, res.String()
		}
	}
	return true, ""
}

func ValidateMigrateRpRequest(currSNS *utils.ObjectContext, toSNS *utils.ObjectContext) (bool, string) {
	// quotaParent will take the spec either from the RQ or CRQ object
	// based on the namespace depth
	quotaNewParent, err := utils.GetSNSQuota(toSNS)
	if err != nil {
		return false, err.Error()
	}

	snsRequest := utils.GetSNSQuotaUsed(currSNS)
	usedNewParentResources := utils.GetSNSQuotaUsed(toSNS)

	for res := range quotaNewParent {
		var (
			vUsed, _    = usedNewParentResources[res]
			vParent, _  = quotaNewParent[res]
			vRequest, _ = snsRequest[res]
		)

		vParent.Sub(vUsed)
		vParent.Sub(vRequest)
		if vParent.Value() < 0 {
			return false, res.String()

		}
	}
	return true, ""
}
