package webhooks

import (
	"context"
	"fmt"
	"github.com/dana-team/hns/internals/utils"
	"github.com/go-logr/logr"
	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/types"
	"net/http"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
	"time"
)

type RoleBindingAnnotator struct {
	Client  client.Client
	Decoder *admission.Decoder
	Log     logr.Logger
}

// +kubebuilder:webhook:path=/validate-v1-rolebinding,mutating=false,sideEffects=NoneOnDryRun,failurePolicy=fail,groups="rbac.authorization.k8s.io",resources=rolebindings,verbs=delete;create,versions=v1,name=rolebinding.dana.io,admissionReviewVersions=v1;v1beta1

func (a *RoleBindingAnnotator) Handle(ctx context.Context, req admission.Request) admission.Response {
	log := a.Log.WithValues("webhook", "roleBinding Webhook", "Name", req.Name)
	log.Info("webhook request received")

	if req.Operation == admissionv1.Delete {
		roleBinding, err := utils.NewObjectContext(ctx, log, a.Client, types.NamespacedName{}, &rbacv1.RoleBinding{})
		if err != nil {
			log.Error(err, "unable to create roleBinding objectContext")
		}

		if err := a.Decoder.DecodeRaw(req.OldObject, roleBinding.Object); err != nil {
			log.Error(err, "could not decode object")
			return admission.Errored(http.StatusBadRequest, err)
		}
		clusterName, err := utils.GetClusterName(roleBinding.Ctx, roleBinding.Log, roleBinding.Client)
		if err != nil {
			log.Error(err, "unable to get cluster name")
		}

		namespace, err := utils.NewObjectContext(ctx, log, a.Client, types.NamespacedName{Name: roleBinding.Object.GetNamespace()}, &corev1.Namespace{})
		if err != nil {
			log.Error(err, "unable to get roleBinding namespace")
			return admission.Denied(err.Error())
		}

		if utils.DeletionTimeStampExists(namespace.Object) {
			return admission.Allowed(allowMessageValidateNamespace)
		}

		parentRoleBinding, err := utils.NewObjectContext(ctx, log, a.Client, types.NamespacedName{Namespace: utils.GetNamespaceParent(namespace.Object), Name: roleBinding.Object.GetName()}, &rbacv1.RoleBinding{})
		if err != nil {
			log.Error(err, "unable to get parent roleBinding")
			return admission.Denied(err.Error())
		}

		if !parentRoleBinding.IsPresent() {
			if !utils.UsernameToFilter(req.UserInfo.Username) {
				webhookLog := utils.NewWebhookLog(time.Now(), roleBinding.Object.GetObjectKind().GroupVersionKind().Kind,
					roleBinding.Object.GetNamespace(), utils.Delete, req.UserInfo.Username,
					fmt.Sprintf("roleBinding %s deleted from %s namespace", roleBinding.Object.GetName(),
						roleBinding.Object.GetNamespace()), corev1.ResourceList{}, clusterName)
				err := webhookLog.UploadLogToElastic()
				if err != nil {
					log.Error(err, "unable to upload log to elastic")
				}
			}
			return admission.Allowed(allowMessageValidateNamespace)
		}

		if utils.DeletionTimeStampExists(parentRoleBinding.Object) {
			if !utils.UsernameToFilter(req.UserInfo.Username) {
				webhookLog := utils.NewWebhookLog(time.Now(), roleBinding.Object.GetObjectKind().GroupVersionKind().Kind,
					roleBinding.Object.GetNamespace(), utils.Delete, req.UserInfo.Username,
					fmt.Sprintf("roleBinding %s deleted from %s namespace", roleBinding.Object.GetName(),
						roleBinding.Object.GetNamespace()), corev1.ResourceList{}, clusterName)
				err := webhookLog.UploadLogToElastic()
				if err != nil {
					log.Error(err, "unable to upload log to elastic")
				}
			}
			return admission.Allowed(allowMessageValidateNamespace)
		}
		return admission.Denied(denyMessageRoleBinding)
	} else {
		roleBinding, err := utils.NewObjectContext(ctx, log, a.Client, types.NamespacedName{}, &rbacv1.RoleBinding{})
		if err != nil {
			log.Error(err, "unable to create roleBinding objectContext")
		}

		if err := a.Decoder.DecodeRaw(req.Object, roleBinding.Object); err != nil {
			log.Error(err, "could not decode object")
			return admission.Errored(http.StatusBadRequest, err)
		}

		clusterName, err := utils.GetClusterName(roleBinding.Ctx, roleBinding.Log, roleBinding.Client)
		if err != nil {
			log.Error(err, "unable to get cluster name")
		}

		if !utils.UsernameToFilter(req.UserInfo.Username) {
			webhookLog := utils.NewWebhookLog(time.Now(), roleBinding.Object.GetObjectKind().GroupVersionKind().Kind,
				roleBinding.Object.GetNamespace(), utils.Create, req.UserInfo.Username,
				fmt.Sprintf("roleBinding %s created in %s namespace", roleBinding.Object.GetName(), roleBinding.Object.GetNamespace()),
				corev1.ResourceList{}, clusterName)
			err = webhookLog.UploadLogToElastic()
			if err != nil {
				log.Error(err, "unable to upload log to elastic")
			}
		}
		return admission.Allowed(allowMessageValidateRoleBinding)
	}
}
