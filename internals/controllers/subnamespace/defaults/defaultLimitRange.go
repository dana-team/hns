package controllers

import (
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
)

var (
	MinPodCpu               = resource.MustParse("25m")
	MinPodMem               = resource.MustParse("200M")
	min                     = v1.ResourceList{"cpu": MinPodCpu, "memory": MinPodMem}
	DefaultRequestPodCpu    = resource.MustParse("100m")
	DefaultRequestPodMem    = resource.MustParse("200M")
	defaultRequest          = v1.ResourceList{"cpu": DefaultRequestPodCpu, "memory": DefaultRequestPodMem}
	MaxLimitRequestRatioCpu = resource.NewQuantity(44, resource.DecimalSI)
	MaxLimitRequestRatioMem = resource.NewQuantity(29, resource.DecimalSI)
	maxLimitRequestRatio    = v1.ResourceList{"cpu": *MaxLimitRequestRatioCpu, "memory": *MaxLimitRequestRatioMem}
	Limits                  = v1.LimitRangeItem{
		Type:                 "Container",
		Min:                  min,
		Default:              defaultRequest, //default limit
		DefaultRequest:       defaultRequest,
		MaxLimitRequestRatio: maxLimitRequestRatio,
	}
)
