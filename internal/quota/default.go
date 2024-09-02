package quota

import (
	"context"
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	resourcesKey string = "resources"
)

var (
	Configmaps        = resource.NewQuantity(100, resource.DecimalSI)
	Builds            = resource.NewQuantity(100, resource.DecimalSI)
	Cronjobs          = resource.NewQuantity(100, resource.DecimalSI)
	Daemonsets        = resource.NewQuantity(100, resource.DecimalSI)
	Deployments       = resource.NewQuantity(100, resource.DecimalSI)
	Replicasets       = resource.NewQuantity(100, resource.DecimalSI)
	Routes            = resource.NewQuantity(100, resource.DecimalSI)
	Secrets           = resource.NewQuantity(100, resource.DecimalSI)
	deploymentconfigs = resource.NewQuantity(100, resource.DecimalSI)
	buildconfigs      = resource.NewQuantity(100, resource.DecimalSI)
	serviceaccounts   = resource.NewQuantity(100, resource.DecimalSI)
	statefulsets      = resource.NewQuantity(100, resource.DecimalSI)
	templates         = resource.NewQuantity(100, resource.DecimalSI)
	imagestreams      = resource.NewQuantity(100, resource.DecimalSI)
	ZeroDecimal       = resource.NewQuantity(0, resource.DecimalSI)

	quotaConfig = "sns-quota-resources"

	DefaultQuota = corev1.ResourceQuotaSpec{Hard: DefaultQuotaHard}

	DefaultQuotaHard = corev1.ResourceList{"configmaps": *Configmaps, "count/builds.build.openshift.io": *Builds, "count/cronjobs.batch": *Cronjobs, "count/daemonsets.apps": *Daemonsets,
		"count/deployments.apps": *Deployments, "count/jobs.batch": *Cronjobs, "count/replicasets.apps": *Replicasets, "count/routes.route.openshift.io": *Routes,
		"secrets": *Secrets, "count/deploymentconfigs.apps.openshift.io": *deploymentconfigs, "count/buildconfigs.build.openshift.io": *buildconfigs, "count/serviceaccounts": *serviceaccounts,
		"count/statefulsets.apps": *statefulsets, "count/templates.template.openshift.io": *templates, "openshift.io/imagestreams": *imagestreams}
)

// GetObservedResources returns default values for all observed resources inside a ResourceQuotaSpec object.
// The observed resources are read from a configMap.
func GetObservedResources(ctx context.Context, k8sClient client.Client) (corev1.ResourceQuotaSpec, error) {
	resourcesConfig := &corev1.ConfigMap{}
	if err := k8sClient.Get(ctx, types.NamespacedName{Name: quotaConfig, Namespace: namespaceName}, resourcesConfig); err != nil {
		return corev1.ResourceQuotaSpec{}, fmt.Errorf("failed to get ConfigMap %q: %v", resourcesConfig, err)
	}

	resources := corev1.ResourceList{}
	resourceNames := strings.Split(resourcesConfig.Data[resourcesKey], ",")
	for _, name := range resourceNames {
		resources[corev1.ResourceName(name)] = *ZeroDecimal
	}

	return corev1.ResourceQuotaSpec{Hard: resources}, nil
}
