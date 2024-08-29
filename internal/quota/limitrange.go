package quota

import (
	"context"
	"fmt"

	"github.com/dana-team/hns/internal/objectcontext"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const limitConfigMapKey = "limitRangeDefaults"

// getLimits returns the limits that are set in the limitRange.yaml field in the sns-config configmap.
func getLimits(ctx context.Context, k8sClient client.Client) ([]corev1.LimitRangeItem, error) {
	limitRangeData, err := getConfigMapData(ctx, k8sClient, limitConfigMapKey)
	if err != nil {
		return nil, err
	}
	parsedLimits, err := parseLimitRangeData(limitRangeData)
	if err != nil {
		return nil, err
	}
	minPodCpu := parsedLimits.Minimum.CPU
	minPodMem := parsedLimits.Minimum.Memory
	minContainer := corev1.ResourceList{"cpu": minPodCpu, "memory": minPodMem}

	defaultRequestPodCpu := parsedLimits.DefaultRequest.CPU
	defaultRequestPodMem := parsedLimits.DefaultRequest.Memory
	defaultRequest := corev1.ResourceList{"cpu": defaultRequestPodCpu, "memory": defaultRequestPodMem}

	defaultLimitPodCpu := parsedLimits.DefaultLimit.CPU
	defaultLimitPodMem := parsedLimits.DefaultLimit.Memory
	defaultLimit := corev1.ResourceList{"cpu": defaultLimitPodCpu, "memory": defaultLimitPodMem}

	maxRequestPodCpu := parsedLimits.Maximum.CPU
	maxRequest := corev1.ResourceList{"cpu": maxRequestPodCpu}

	ContainerLimits := corev1.LimitRangeItem{
		Type:           "Container",
		Min:            minContainer,
		Max:            maxRequest,
		Default:        defaultLimit,
		DefaultRequest: defaultRequest,
	}

	minPVC := corev1.ResourceList{"storage": parsedLimits.MinimumPVC.Storage}
	PVCLimits := corev1.LimitRangeItem{
		Type: "PersistentVolumeClaim",
		Min:  minPVC,
	}

	return []corev1.LimitRangeItem{ContainerLimits, PVCLimits}, nil
}

// CreateDefaultSNSLimitRange creates a limit range object with some default values
// that we would like to limit and are not set by the user.
func CreateDefaultSNSLimitRange(snsObject *objectcontext.ObjectContext) error {
	snsName := snsObject.Name()

	limits, err := getLimits(snsObject.Ctx, snsObject.Client)
	if err != nil {
		return fmt.Errorf("error getting default limits: %w", err)
	}

	composedDefaultLimitRange := composeLimitRange(snsName, snsName, limits)

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
