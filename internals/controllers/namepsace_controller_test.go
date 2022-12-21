package controllers

import (
	danav1 "github.com/dana-team/hns/api/v1"
	"github.com/dana-team/hns/internals/utils"
	"context"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = PDescribe("Namespace controller", func() {
	ctx := context.Background()
	rootRb := utils.ComposeRoleBinding(getRandomName("rb"), testRootNs.Name, []rbacv1.Subject{{Kind: "User", Name: getRandomName("subject")}}, rbacv1.RoleRef{Kind: "ClusterRole", Name: "admin"})
	childSns := utils.ComposeSns(getRandomName("childsns"), testRootNs.Name, composeDefaultSnsQuota(int64(childResources)), map[string]string{})
	childNs := utils.ComposeNamespace(childSns.Name, map[string]string{danaTestLabel: "true"}, map[string]string{})
	grandsonSns := utils.ComposeSns(getRandomName("grandsonsns"), childNs.Name, composeDefaultSnsQuota(int64(grandsonResources)), map[string]string{})
	grandsonNs := utils.ComposeNamespace(grandsonSns.Name, map[string]string{danaTestLabel: "true"}, map[string]string{})
	childRb := utils.ComposeRoleBinding(rootRb.Name, childSns.Name, rootRb.Subjects, rootRb.RoleRef)

	Context("When namespace is created", func() {
		It("Creating a roleBinding in root namespace", func() {
			Expect(k8sClient.Create(ctx, rootRb)).Should(Succeed())
			isCreated(ctx, k8sClient, rootRb)
		})

		It("Creating a sub namespace in root namespace", func() {
			Expect(k8sClient.Create(ctx, childSns)).Should(Succeed())
			isCreated(ctx, k8sClient, childNs)
		})

		It("Should add namespace finalizer", func() {
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, client.ObjectKey{Name: childNs.Name}, checksNamespace); err != nil {
					return false
				}
				if !utils.IsNamespaceFinalizerExists(checksNamespace) {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())
		})

		It("Should copy rootRb from parent", func() {
			isCreated(ctx, k8sClient, childRb)
		})
	})

	Context("When sub namespace created in namespace", func() {
		It("Creating a sns in child namespace", func() {
			Expect(k8sClient.Create(ctx, grandsonSns)).Should(Succeed())
			isCreated(ctx, k8sClient, grandsonNs)
		})

		It("Should update childNs role to None", func() {
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, client.ObjectKey{Name: childSns.Name}, checksNamespace); err != nil {
					return false
				}
				if utils.GetNamespaceRole(checksNamespace) != danav1.NoRole {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())
		})
	})

	Context("When grandsonNs deleted in childNs", func() {
		It("Delete grandsonNs from childNs", func() {
			Expect(k8sClient.Delete(ctx, grandsonNs)).Should(Succeed())
			isDeleted(ctx, k8sClient, grandsonNs)
		})

		It("Should update childNs role to leaf", func() {
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, client.ObjectKey{Name: childSns.Name}, checksNamespace); err != nil {
					return false
				}
				if utils.GetNamespaceRole(checksNamespace) != danav1.Leaf {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())
		})
	})

	Context("When namespace with children is deleted", func() {
		It("Should not delete the namespace", func() {
			Expect(k8sClient.Delete(ctx, testRootNs)).ShouldNot(Succeed())
		})
	})

	Context("When namespace without children is deleted", func() {
		It("Should delete the namespace", func() {
			Expect(k8sClient.Delete(ctx, childNs)).Should(Succeed())
			isDeleted(ctx, k8sClient, childNs)
		})

		It("Should delete childNs crq", func() {
			isDeleted(ctx, k8sClient, utils.ComposeCrq(childNs.Name, corev1.ResourceQuotaSpec{}, map[string]string{}))
		})

		It("Should delete childNs sns", func() {
			isDeleted(ctx, k8sClient, childSns)
		})
	})
})
