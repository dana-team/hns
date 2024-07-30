package quota

import (
	"github.com/dana-team/hns/internal/objectcontext"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

var (
	minPodCpu    = resource.MustParse("25m")
	minPodMem    = resource.MustParse("50Mi")
	minContainer = corev1.ResourceList{"cpu": minPodCpu, "memory": minPodMem}

	defaultRequestPodCpu = resource.MustParse("50m")
	defaultRequestPodMem = resource.MustParse("100Mi")
	defaultRequest       = corev1.ResourceList{"cpu": defaultRequestPodCpu, "memory": defaultRequestPodMem}

	defaultLimitPodCpu = resource.MustParse("150m")
	defaultLimitPodMem = resource.MustParse("300Mi")
	defaultLimit       = corev1.ResourceList{"cpu": defaultLimitPodCpu, "memory": defaultLimitPodMem}

	maxRequestPodCpu = resource.MustParse("128")
	maxRequest       = corev1.ResourceList{"cpu": maxRequestPodCpu}

	ContainerLimits = corev1.LimitRangeItem{
		Type:           "Container",
		Min:            minContainer,
		Max:            maxRequest,
		Default:        defaultLimit,
		DefaultRequest: defaultRequest,
	}

	minPVC    = corev1.ResourceList{"storage": resource.MustParse("20Mi")}
	PVCLimits = corev1.LimitRangeItem{
		Type: "PersistentVolumeClaim",
		Min:  minPVC,
	}

	Limits = []corev1.LimitRangeItem{ContainerLimits, PVCLimits}
)

// CreateDefaultSNSLimitRange creates a limit range object with some default values
// that we would like to limit and are not set by the user.
func CreateDefaultSNSLimitRange(snsObject *objectcontext.ObjectContext) error {
	snsName := snsObject.Name()
	composedDefaultLimitRange := composeLimitRange(snsName, snsName, Limits)

	childLimitRange, err := objectcontext.New(snsObject.Ctx, snsObject.Client, types.NamespacedName{Name: snsName, Namespace: snsName}, composedDefaultLimitRange)
	if err != nil {
		return err
	}

	err = childLimitRange.EnsureCreate()

	return err
}

// composeLimitRange returns a LimitRange object based on the given parameters.
func composeLimitRange(name string, namespace string, limits []corev1.LimitRangeItem) *corev1.LimitRange {
	return &corev1.LimitRange{
		TypeMeta: metav1.TypeMeta{
			Kind: "LimitRange",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: corev1.LimitRangeSpec{
			Limits: limits,
		},
	}
}
