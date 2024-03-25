package quota

import (
	"strconv"

	danav1 "github.com/dana-team/hns/api/v1"
	"github.com/dana-team/hns/internal/objectcontext"
	"github.com/dana-team/hns/internal/subnamespace/resourcepool"
	"github.com/dana-team/hns/internal/subnamespace/snsutils"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

// ResourceQuota returns a ResourceQuota object.
func ResourceQuota(sns *objectcontext.ObjectContext) (*objectcontext.ObjectContext, error) {
	quotaObject, err := objectcontext.New(sns.Ctx, sns.Client, client.ObjectKey{Namespace: sns.Name(), Name: sns.Name()}, &corev1.ResourceQuota{})
	if err != nil {
		return quotaObject, err
	}

	return quotaObject, nil
}

// composeRQ returns a ResourceQuota object based on the given parameters.
func composeRQ(name, namespace string, resources corev1.ResourceList) *corev1.ResourceQuota {
	return &corev1.ResourceQuota{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: corev1.ResourceQuotaSpec{
			Hard: resources,
		},
	}
}

// DoesSNSRQExists returns true if a ResourceQuota exists.
func DoesSNSRQExists(sns *objectcontext.ObjectContext) (bool, *objectcontext.ObjectContext, error) {
	snsRQ, err := objectcontext.New(sns.Ctx, sns.Client, types.NamespacedName{Name: sns.Name(), Namespace: sns.Name()}, &corev1.ResourceQuota{})
	if err != nil {
		return false, nil, err
	}

	if snsRQ.IsPresent() {
		if !IsZeroed(snsRQ.Object) && !IsDefault(snsRQ.Object) {
			return true, snsRQ, nil
		}
	}
	return false, nil, nil
}

// IsRQ returns true if the depth of the subnamespace is less or equal
// the pre-set rqDepth AND if the subnamespace is not a ResourcePool.
func IsRQ(sns *objectcontext.ObjectContext, offset int) (bool, error) {
	rootRQDepth, err := snsutils.GetRqDepthFromSNS(sns)
	if err != nil {
		return false, err
	}

	nsDepth, err := subnamespaceDepth(sns)
	if err != nil {
		return false, err
	}

	rootRQDepthInt, _ := strconv.Atoi(rootRQDepth)
	nsDepthInt, _ := strconv.Atoi(nsDepth)

	depthFlag := (nsDepthInt + offset) <= rootRQDepthInt
	if offset == danav1.ParentOffset {
		return depthFlag, nil
	}

	resourcePoolFlag, err := resourcepool.IsSNSResourcePool(sns.Object)
	if err != nil {
		return resourcePoolFlag, err
	}

	return depthFlag && !resourcePoolFlag, nil
}

// CreateDefaultSNSResourceQuota creates a ResourceQuota object with some default values
// that we would like to limit and are not set by the user. This is only created in subnamespaces that
// have a ClusterResourceQuota.
func CreateDefaultSNSResourceQuota(snsObject *objectcontext.ObjectContext) error {
	snsName := snsObject.Name()
	composedDefaultRQ := composeRQ(snsName, snsName, DefaultQuotaHard)

	snsDefaultRQ, err := objectcontext.New(snsObject.Ctx, snsObject.Client, types.NamespacedName{Name: snsName, Namespace: snsName}, composedDefaultRQ)
	if err != nil {
		return err
	}

	err = snsDefaultRQ.EnsureCreate()

	return err
}
