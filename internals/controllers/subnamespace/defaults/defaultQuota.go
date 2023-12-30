package controllers

import (
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

const (
	BasicStorage corev1.ResourceName = "basic.storageclass.storage.k8s.io/requests.storage"
	CPU          corev1.ResourceName = "cpu"
	Memory       corev1.ResourceName = "memory"
	Pods         corev1.ResourceName = "pods"
	GPU          corev1.ResourceName = "requests.nvidia.com/gpu"
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

	DefaultQuotaHard = corev1.ResourceList{"configmaps": *Configmaps, "count/builds.build.openshift.io": *Builds, "count/cronjobs.batch": *Cronjobs, "count/daemonsets.apps": *Daemonsets,
		"count/deployments.apps": *Deployments, "count/jobs.batch": *Cronjobs, "count/replicasets.apps": *Replicasets, "count/routes.route.openshift.io": *Routes,
		"secrets": *Secrets, "count/deploymentconfigs.apps.openshift.io": *deploymentconfigs, "count/buildconfigs.build.openshift.io": *buildconfigs, "count/serviceaccounts": *serviceaccounts,
		"count/statefulsets.apps": *statefulsets, "count/templates.template.openshift.io": *templates, "openshift.io/imagestreams": *imagestreams}

	ZeroedQuota = corev1.ResourceQuotaSpec{
		Hard: corev1.ResourceList{BasicStorage: *ZeroDecimal, CPU: *ZeroDecimal, Memory: *ZeroDecimal, Pods: *ZeroDecimal, GPU: *ZeroDecimal},
	}
)
