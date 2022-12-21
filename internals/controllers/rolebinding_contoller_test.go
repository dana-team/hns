package controllers

import (
	"context"
	"github.com/dana-team/hns/internals/utils"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	rbacv1 "k8s.io/api/rbac/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = PDescribe("RoleBinding controller", func() {
	ctx := context.Background()
	rootRb := utils.ComposeRoleBinding(getRandomName("rb"), testRootNs.Name, []rbacv1.Subject{{Kind: "User", Name: getRandomName("subject")}}, rbacv1.RoleRef{Kind: "ClusterRole", Name: "admin"})
	childSns := utils.ComposeSns(getRandomName("childsns"), testRootNs.Name, composeDefaultSnsQuota(int64(childResources)), map[string]string{})
	childNs := utils.ComposeNamespace(childSns.Name, map[string]string{danaTestLabel: "true"}, map[string]string{})
	childRb := utils.ComposeRoleBinding(rootRb.Name, childSns.Name, rootRb.Subjects, rootRb.RoleRef)

	Context("When root rb is created", func() {
		It("Creating a roleBinding in root namespace", func() {
			Expect(k8sClient.Create(ctx, rootRb)).Should(Succeed())
			isCreated(ctx, k8sClient, rootRb)
		})

		It("Should add finalizer to rootRb", func() {
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, client.ObjectKey{Namespace: testRootNs.Name, Name: rootRb.Name}, checksRoleBinding); err != nil {
					return false
				}
				if !utils.IsRoleBindingFinalizerExists(checksRoleBinding) {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())
		})

		//It("Should create clusterRole", func() {
		//	isCreated(ctx, k8sClient, utils.ComposeClusterRole(rootRb))
		//})
		//
		//It("Should create clusterRoleBinding", func() {
		//	isCreated(ctx, k8sClient, utils.ComposeClusterRoleBinding(rootRb, utils.GetRoleBindingClusterRoleName(rootRb)))
		//})

		It("Creating sub namespace", func() {
			Expect(k8sClient.Create(ctx, childSns)).Should(Succeed())
			isCreated(ctx, k8sClient, childNs)
		})

		It("Should copy rootRb with the finalizer to childNs", func() {
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, client.ObjectKey{Namespace: childRb.Namespace, Name: childRb.Name}, checksRoleBinding); err != nil {
					return false
				}
				if !utils.IsRoleBindingFinalizerExists(checksRoleBinding) {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())
		})

		//It("Should create childNs clusterRole", func() {
		//	isCreated(ctx, k8sClient, utils.ComposeClusterRole(childRb))
		//})
		//
		//It("Should create childNs clusterRoleBinding", func() {
		//	isCreated(ctx, k8sClient, utils.ComposeClusterRoleBinding(childRb, utils.GetRoleBindingClusterRoleName(childRb)))
		//})
	})

	Context("When rb is deleted from child namespace", func() {
		It("Should not delete the rb", func() {
			By("Deleting the rb from the child ns")
			Expect(k8sClient.Delete(ctx, childRb)).ShouldNot(Succeed())
		})
	})

	Context("When roleBinding is deleted from root ns", func() {
		It("Should delete the roleBinding from root namespace", func() {
			Expect(k8sClient.Delete(ctx, rootRb)).Should(Succeed())
			isDeleted(ctx, k8sClient, childRb)
		})

		//It("Should delete root ns clusterRole", func() {
		//	isDeleted(ctx, k8sClient, utils.ComposeClusterRole(rootRb))
		//})
		//
		//It("Should delete root ns clusterRoleBinding", func() {
		//	isDeleted(ctx, k8sClient, utils.ComposeClusterRoleBinding(rootRb, utils.GetRoleBindingClusterRoleName(rootRb)))
		//})

		It("Should delete child ns roleBinding", func() {
			isDeleted(ctx, k8sClient, childRb)
		})

		//It("Should delete child ns clusterRole", func() {
		//	isDeleted(ctx, k8sClient, utils.ComposeClusterRole(childRb))
		//})
		//
		//It("Should delete child ns clusterRoleBinding", func() {
		//	isDeleted(ctx, k8sClient, utils.ComposeClusterRoleBinding(childRb, utils.GetRoleBindingClusterRoleName(childRb)))
		//})
	})

	Context("When delete sns from root ns", func() {
		It("Should delete the sns", func() {
			Expect(k8sClient.Delete(ctx, childNs)).Should(Succeed())
			isDeleted(ctx, k8sClient, childSns)
		})
	})
})
