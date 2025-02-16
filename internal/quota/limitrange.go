package quota

import (
	"context"
	"fmt"

	"github.com/dana-team/hns/internal/common"

	"k8s.io/apimachinery/pkg/api/resource"

	"github.com/dana-team/hns/internal/objectcontext"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	cpu                   = "cpu"
	memory                = "memory"
	storage               = "storage"
	persistentVolumeClaim = "PersistentVolumeClaim"
	container             = "Container"
)

// getLimits returns the limits that are set in the limitRange.yaml field in HNSconfig.
func getLimits(ctx context.Context, k8sClient client.Client) ([]corev1.LimitRangeItem, error) {
	HNSConfigData, err := common.GetHNSConfigData(ctx, k8sClient)
	if err != nil {
		return nil, err
	}

	limitRangeData := HNSConfigData.Spec.LimitRange

	minContainer := corev1.ResourceList{
		cpu:    resource.MustParse(limitRangeData.Minimum[cpu]),
		memory: resource.MustParse(limitRangeData.Minimum[memory]),
	}

	defaultRequest := corev1.ResourceList{
		cpu:    resource.MustParse(limitRangeData.DefaultRequest[cpu]),
		memory: resource.MustParse(limitRangeData.DefaultRequest[memory]),
	}

	defaultLimit := corev1.ResourceList{
		cpu:    resource.MustParse(limitRangeData.DefaultLimit[cpu]),
		memory: resource.MustParse(limitRangeData.DefaultLimit[memory]),
	}

	maxRequest := corev1.ResourceList{
		cpu: resource.MustParse(limitRangeData.Maximum[cpu]),
	}

	ContainerLimits := corev1.LimitRangeItem{
		Type:           container,
		Min:            minContainer,
		Max:            maxRequest,
		Default:        defaultLimit,
		DefaultRequest: defaultRequest,
	}

	minPVC := corev1.ResourceList{
		storage: resource.MustParse(limitRangeData.MinimumPVC[storage]),
	}

	PVCLimits := corev1.LimitRangeItem{
		Type: persistentVolumeClaim,
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
