package controllers

import (
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

var (
	minPodCpu    = resource.MustParse("25m")
	minPodMem    = resource.MustParse("50Mi")
	minContainer = v1.ResourceList{"cpu": minPodCpu, "memory": minPodMem}

	defaultRequestPodCpu = resource.MustParse("50m")
	defaultRequestPodMem = resource.MustParse("100Mi")
	defaultRequest       = v1.ResourceList{"cpu": defaultRequestPodCpu, "memory": defaultRequestPodMem}

	defaultLimitPodCpu = resource.MustParse("150m")
	defaultLimitPodMem = resource.MustParse("300Mi")
	defaultLimit       = v1.ResourceList{"cpu": defaultLimitPodCpu, "memory": defaultLimitPodMem}

	ContainerLimits = v1.LimitRangeItem{
		Type:           "Container",
		Min:            minContainer,
		Default:        defaultLimit,
		DefaultRequest: defaultRequest,
	}

	minPVC    = v1.ResourceList{"storage": resource.MustParse("20Mi")}
	PVCLimits = v1.LimitRangeItem{
		Type: "PersistentVolumeClaim",
		Min:  minPVC,
	}

	Limits = []v1.LimitRangeItem{ContainerLimits, PVCLimits}
)
