package controllers

import (
	danav1 "github.com/dana-team/hns/api/v1"
	"github.com/dana-team/hns/internals/utils"
	"context"
	"fmt"
	"github.com/go-logr/logr"
	. "github.com/onsi/gomega"
	quotav1 "github.com/openshift/api/quota/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/types"
	"math/rand"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"strings"
	"time"
)

const (
	timeout           = time.Second * 15
	interval          = time.Second * 1
	danaTestLabel     = "dana-test"
	rootResources     = 200
	childResources    = 50
	grandsonResources = 10
)

var (
	// DO NOT EDIT OR CHANGE THE COMPOSED OBJECTS!!
	randomName  = getRandomName("rootns")
	testRootNs  = utils.ComposeNamespace(randomName, map[string]string{danav1.Hns: "true", danaTestLabel: "true", "dana.hns.io/aggragator-" + randomName: "true"}, map[string]string{danav1.Role: danav1.Root, danav1.Depth: "0"})
	testRootCrq = utils.ComposeCrq(randomName, corev1.ResourceQuotaSpec{Hard: composeDefaultSnsQuota(int64(rootResources))}, map[string]string{fmt.Sprintf("%s-%s", danav1.CrqSelector, "0"): testRootNs.Name})

	checksNamespace    = &corev1.Namespace{}
	checksRoleBinding  = &rbacv1.RoleBinding{}
	checksSubNamespace = &danav1.Subnamespace{}
	checksCrq          = &quotav1.ClusterResourceQuota{}
)

func composeDefaultSnsQuota(quantity int64) map[corev1.ResourceName]resource.Quantity {
	var (
		defaultQuantity       = quantity
		defaultQuantityFormat = resource.DecimalSI
		defaultResources      = []corev1.ResourceName{"cpu", "memory"}
	)

	quota := make(map[corev1.ResourceName]resource.Quantity)
	for _, r := range defaultResources {
		quota[r] = *resource.NewQuantity(defaultQuantity, defaultQuantityFormat)
	}
	return quota
}

func getRandomName(prefix string) string {
	rand.Seed(time.Now().UnixNano())
	suffix := make([]byte, 5)
	rand.Read(suffix)
	return fmt.Sprintf("%s%x%s", prefix, suffix, "-dana-test")
}

func isCreated(ctx context.Context, k8sClient client.Client, obj client.Object) {
	EventuallyWithOffset(1, func() bool {
		if err := k8sClient.Get(ctx, client.ObjectKey{
			Namespace: obj.GetNamespace(),
			Name:      obj.GetName(),
		}, obj); err != nil {
			return false
		}
		return true
	}, timeout, interval).Should(BeTrue())
}

func isDeleted(ctx context.Context, k8sClient client.Client, obj client.Object) {
	EventuallyWithOffset(1, func() bool {
		if err := k8sClient.Get(ctx, client.ObjectKey{
			Namespace: obj.GetNamespace(),
			Name:      obj.GetName(),
		}, obj); err != nil {
			if apierrors.IsNotFound(err) {
				return true
			}
		}
		return false
	}, timeout, interval).Should(BeTrue())
}

func cleanUp(ctx context.Context, k8sClient client.Client) error {
	testLog := ctrl.Log

	if err := deleteLeafTestNamespaces(ctx, k8sClient, testLog); err != nil {
		return err
	}

	if err := deleteTestCrqs(ctx, k8sClient, testLog); err != nil {
		return err
	}

	if err := deleteTestRootNs(ctx, k8sClient, testLog); err != nil {
		return err
	}

	return nil
}

func deleteTestRootNs(ctx context.Context, k8sClient client.Client, testLog logr.Logger) error {
	danaTestRootNamespaces, err := utils.NewObjectContextList(ctx, testLog.WithName("deleteRootTestNamespaces"), k8sClient, &corev1.NamespaceList{}, client.MatchingLabels{danaTestLabel: "true"})
	if err != nil {
		return err
	}
	for len(danaTestRootNamespaces.Objects.(*corev1.NamespaceList).Items) > 0 {
		for _, namespace := range danaTestRootNamespaces.Objects.(*corev1.NamespaceList).Items {
			namespaceToDelete, err := utils.NewObjectContext(danaTestRootNamespaces.Ctx, danaTestRootNamespaces.Log, danaTestRootNamespaces.Client, types.NamespacedName{Name: namespace.Name}, &corev1.Namespace{})
			if err != nil {
				return err
			}

			if err = deleteNsRoleBindingsFinalizer(namespaceToDelete); err != nil {
				return err
			}

			err = namespaceToDelete.UpdateObject(func(object client.Object, log logr.Logger) (client.Object, logr.Logger) {
				controllerutil.RemoveFinalizer(object, danav1.NsFinalizer)
				return object, log
			})
			if err != nil {
				return err
			}

			if err := namespaceToDelete.EnsureDeleteObject(); err != nil {
				return err
			}
		}

		danaTestRootNamespaces, err = utils.NewObjectContextList(ctx, testLog.WithName("deleteRootTestNamespaces"), k8sClient, &corev1.NamespaceList{}, client.MatchingLabels{danaTestLabel: "true"}, client.MatchingLabels{danav1.Role: danav1.Root})
		if err != nil {
			return err
		}
	}

	return nil
}

func deleteTestCrqs(ctx context.Context, k8sClient client.Client, testLog logr.Logger) error {
	danaTestCrqs, err := utils.NewObjectContextList(ctx, testLog.WithName("deleteTestCrqs"), k8sClient, &quotav1.ClusterResourceQuotaList{})
	if err != nil {
		return err
	}

	for _, crq := range danaTestCrqs.Objects.(*quotav1.ClusterResourceQuotaList).Items {
		if strings.Contains(crq.Name, "dana-test") {
			crqToDelete, err := utils.NewObjectContext(danaTestCrqs.Ctx, danaTestCrqs.Log, danaTestCrqs.Client, types.NamespacedName{Name: crq.Name}, &quotav1.ClusterResourceQuota{})
			if err != nil {
				return err
			}

			if err := crqToDelete.EnsureDeleteObject(); err != nil {
				return err
			}
		}
	}
	return nil
}

func deleteLeafTestNamespaces(ctx context.Context, k8sClient client.Client, testLog logr.Logger) error {
	danaTestLeafNamespaces, err := utils.NewObjectContextList(ctx, testLog.WithName("deleteLeafTestNamespaces"), k8sClient, &corev1.NamespaceList{}, client.MatchingLabels{danaTestLabel: "true"}, client.MatchingLabels{danav1.Role: danav1.Leaf})
	if err != nil {
		return err
	}
	for len(danaTestLeafNamespaces.Objects.(*corev1.NamespaceList).Items) > 0 {
		for _, namespace := range danaTestLeafNamespaces.Objects.(*corev1.NamespaceList).Items {
			namespaceToDelete, err := utils.NewObjectContext(danaTestLeafNamespaces.Ctx, danaTestLeafNamespaces.Log, danaTestLeafNamespaces.Client, types.NamespacedName{Name: namespace.Name}, &corev1.Namespace{})
			if err != nil {
				return err
			}

			if err = deleteNsRoleBindingsFinalizer(namespaceToDelete); err != nil {
				return err
			}

			err = namespaceToDelete.UpdateObject(func(object client.Object, log logr.Logger) (client.Object, logr.Logger) {
				controllerutil.RemoveFinalizer(object, danav1.NsFinalizer)
				return object, log
			})
			if err != nil {
				return err
			}

			if err := namespaceToDelete.EnsureDeleteObject(); err != nil {
				return err
			}
		}

		danaTestLeafNamespaces, err = utils.NewObjectContextList(ctx, testLog.WithName("deleteLeafTestNamespaces"), k8sClient, &corev1.NamespaceList{}, client.MatchingLabels{danaTestLabel: "true"}, client.MatchingLabels{danav1.Role: danav1.Leaf})
		if err != nil {
			return err
		}
	}

	return nil
}

func deleteNsRoleBindingsFinalizer(namespace *utils.ObjectContext) error {
	roleBindingsToDelete, err := utils.NewObjectContextList(namespace.Ctx, namespace.Log, namespace.Client, &rbacv1.RoleBindingList{}, client.InNamespace(namespace.Object.GetName()))
	if err != nil {
		return err
	}

	for _, roleBinding := range roleBindingsToDelete.Objects.(*rbacv1.RoleBindingList).Items {
		roleBindingToDelete, err := utils.NewObjectContext(roleBindingsToDelete.Ctx, roleBindingsToDelete.Log, roleBindingsToDelete.Client, types.NamespacedName{Name: roleBinding.Name, Namespace: namespace.Object.GetName()}, &rbacv1.RoleBinding{})
		if err != nil {
			return err
		}

		err = roleBindingToDelete.UpdateObject(func(object client.Object, log logr.Logger) (client.Object, logr.Logger) {
			controllerutil.RemoveFinalizer(object, danav1.RbFinalizer)
			return object, log
		})
		if err != nil {
			return err
		}
	}
	return nil
}

func isResourceListsEqual(rl1 *corev1.ResourceList, rl2 *corev1.ResourceList) bool {
	for i, _ := range rl1.DeepCopy() {
		for j, _ := range rl2.DeepCopy() {
			if !rl1.DeepCopy()[i].Equal(rl2.DeepCopy()[j]) {
				return false
			}
		}
	}
	return true
}
