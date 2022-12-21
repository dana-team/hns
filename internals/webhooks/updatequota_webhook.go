package webhooks

import (
	danav1 "github.com/dana-team/hns/api/v1"
	"github.com/dana-team/hns/internals/utils"
	"context"
	"fmt"
	"github.com/go-logr/logr"
	authv1 "k8s.io/api/authorization/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	// "k8s.io/client-go/tools/clientcmd"
	"net/http"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
	"strings"
)

type UpdateQuotaAnnotator struct {
	Client  client.Client
	Decoder *admission.Decoder
	Log     logr.Logger
}

// +kubebuilder:webhook:path=/validate-v1-updatequota,mutating=false,sideEffects=NoneOnDryRun,failurePolicy=fail,groups="dana.hns.io",resources=updatequota,verbs=create;update,versions=v1,name=updatequota.dana.io,admissionReviewVersions=v1;v1beta1

func (a *UpdateQuotaAnnotator) Handle(ctx context.Context, req admission.Request) admission.Response {
	log := a.Log.WithValues("webhook", "updateQuota Webhook", "Name", req.Name)
	log.Info("webhook request received")

	//Decode sns object
	updatingObject, err := utils.NewObjectContext(ctx, log, a.Client, types.NamespacedName{}, &danav1.Updatequota{})
	if err != nil {
		log.Error(err, "unable to create sns objectContext")
	}
	if err := a.Decoder.DecodeRaw(req.Object, updatingObject.Object); err != nil {
		log.Error(err, "could not decode sns object")
		return admission.Errored(http.StatusBadRequest, err)
	}

	//validate the user can do operations on nsparent and ns child
	nsFrom, err := utils.NewObjectContext(ctx, log, a.Client, client.ObjectKey{Name: updatingObject.Object.(*danav1.Updatequota).Spec.SourceNamespace}, &corev1.Namespace{})
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}
	nsTo, err := utils.NewObjectContext(ctx, log, a.Client, client.ObjectKey{Name: updatingObject.Object.(*danav1.Updatequota).Spec.DestNamespace}, &corev1.Namespace{})
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	if !(nsFrom.IsPresent()) {
		return admission.Denied("Namespace " + updatingObject.Object.(*danav1.Updatequota).Spec.SourceNamespace + " does not exist")
	}
	if !(nsTo.IsPresent()) {
		return admission.Denied("Namespace " + updatingObject.Object.(*danav1.Updatequota).Spec.DestNamespace + " does not exist")
	}

	nsRootName, err := a.getAncestor(nsFrom, nsTo)
	if err != nil {
		return admission.Denied(err.Error())
	}

	nsFromName := nsFrom.GetName()
	nsToName := nsTo.GetName()
	hasFromPermissions := a.validatePermissions(req, nsFromName)
	hasToPermissions := a.validatePermissions(req, nsToName)
	hasRootPermissions := a.validatePermissions(req, nsRootName)

	if !hasRootPermissions && !(hasFromPermissions && hasToPermissions) {
		return admission.Denied("you must have permissions on: " + nsFromName + " and " + nsToName +
			" or permissions on:" + nsRootName + " to perform this operation")
	}

	var snsFrom *utils.ObjectContext
	var snsTo *utils.ObjectContext
	var snsFromQuotaObj *utils.ObjectContext
	var snsToQuotaObj *utils.ObjectContext

	// handle root namespace differently since it doesn't have a subnamespace
	if utils.IsRootNamespace(nsFrom.Object) {
		snsFrom = nsFrom
		snsFromQuotaObj, err = utils.GetRootNSQuotaObj(snsFrom)
		if err != nil {
			return admission.Errored(http.StatusInternalServerError, err)
		}
	} else {
		snsFrom, err = utils.GetNamespaceSns(nsFrom)
		if err != nil {
			return admission.Errored(http.StatusBadRequest, err)
		}
		snsFromQuotaObj, err = utils.GetSNSQuotaObj(snsFrom)
		if err != nil {
			return admission.Errored(http.StatusInternalServerError, err)
		}
	}

	if utils.IsRootNamespace(nsTo.Object) {
		snsTo = nsTo
		snsToQuotaObj, err = utils.GetRootNSQuotaObj(snsTo)
		if err != nil {
			return admission.Errored(http.StatusInternalServerError, err)
		}
	} else {
		snsTo, err = utils.GetNamespaceSns(nsTo)
		if err != nil {
			return admission.Errored(http.StatusBadRequest, err)
		}
		snsToQuotaObj, err = utils.GetSNSQuotaObj(snsTo)
		if err != nil {
			return admission.Errored(http.StatusInternalServerError, err)
		}
	}

	if !(snsFromQuotaObj.IsPresent()) {
		return admission.Denied("Quota Object " + updatingObject.Object.(*danav1.Updatequota).Spec.SourceNamespace + " does not exist")
	}
	if !(snsToQuotaObj.IsPresent()) {
		return admission.Denied("Quota Object " + updatingObject.Object.(*danav1.Updatequota).Spec.DestNamespace + " does not exist")
	}

	return admission.Allowed("")
}

func (a *UpdateQuotaAnnotator) validatePermissions(req admission.Request, namespace string) bool {

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
		Verb:      "create",
		Resource:  "pods",
	}

	selfCheck := authv1.SelfSubjectAccessReview{
		Spec: authv1.SelfSubjectAccessReviewSpec{
			ResourceAttributes: &action,
		},
	}
	resp, err := clientset.AuthorizationV1().SelfSubjectAccessReviews().Create(context.Background(), &selfCheck, metav1.CreateOptions{})
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

func (a *UpdateQuotaAnnotator) getAncestor(sourcens *utils.ObjectContext, destns *utils.ObjectContext) (string, error) {
	sourceDisplayName := sourcens.Object.GetAnnotations()["openshift.io/display-name"]
	destDisplayName := destns.Object.GetAnnotations()["openshift.io/display-name"]

	sourceArr := strings.Split(sourceDisplayName, "/")
	destArr := strings.Split(destDisplayName, "/")

	for i := len(sourceArr) - 1; i >= 0; i-- {
		for j := len(destArr) - 1; j >= 0; j-- {
			if sourceArr[i] == destArr[j] {
				return sourceArr[i], nil
			}
		}
	}
	return "", fmt.Errorf("root namespace does not exist")
}
