package controllers

//
//import (
//	danav1 "github.com/dana-team/hns/api/v1"
//	"github.com/dana-team/hns/internals/utils"
//	"context"
//	. "github.com/onsi/ginkgo"
//	. "github.com/onsi/gomega"
//	corev1 "k8s.io/api/core/v1"
//	"sigs.k8s.io/controller-runtime/pkg/client"
//)
//
//var _ = Describe("UpdateQuota controller tests", func() {
//	ctx := context.Background()
//	childSns := utils.ComposeSns(getRandomName("childsns"), testRootNs.Name, composeDefaultSnsQuota(int64(childResources)), map[string]string{})
//	grandChildSns := utils.ComposeSns(getRandomName("grandchildsns"), childSns.Name, composeDefaultSnsQuota(int64(grandsonResources)), map[string]string{})
//	updatequotaadd := utils.ComposeUpdateQuota(getRandomName("updatequota"), danav1.Add, testRootNs.Name, grandChildSns.Name, composeDefaultSnsQuota(int64(grandsonResources)))
//	updatequotareclaim := utils.ComposeUpdateQuota(getRandomName("updatequota"), danav1.Reclaim, testRootNs.Name, grandChildSns.Name, composeDefaultSnsQuota(int64(grandsonResources/2)))
//	updatequotaaddinvalid := utils.ComposeUpdateQuota(getRandomName("updatequota"), danav1.Add, testRootNs.Name, grandChildSns.Name, composeDefaultSnsQuota(int64(rootResources)))
//	updatequotareclaiminvalid := utils.ComposeUpdateQuota(getRandomName("updatequota"), danav1.Reclaim, testRootNs.Name, grandChildSns.Name, composeDefaultSnsQuota(int64(rootResources)))
//	updatequotarewrongoperand := utils.ComposeUpdateQuota(getRandomName("updatequota"), "test", testRootNs.Name, grandChildSns.Name, composeDefaultSnsQuota(int64(grandsonResources)))
//
//	//var checksSubNamespace danav1.Subnamespace
//
//	Context("When updatequota valid is created", func() {
//
//		It("Creating a sub namespace in root namespace", func() {
//			Expect(k8sClient.Create(ctx, childSns)).Should(Succeed())
//			isCreated(ctx, k8sClient, childSns)
//			Expect(k8sClient.Create(ctx, grandChildSns)).Should(Succeed())
//			isCreated(ctx, k8sClient, grandChildSns)
//		})
//
//		It("check childSns and grandChildSns updated by update quota - Add operation", func() {
//			Eventually(func() bool {
//				if err := k8sClient.Create(ctx, updatequotaadd); err != nil {
//					return false
//				}
//				return true
//			}, timeout, interval).Should(BeTrue())
//			isCreated(ctx, k8sClient, updatequotaadd)
//			Eventually(func() bool {
//				if err := k8sClient.Get(ctx, client.ObjectKey{Name: childSns.Name, Namespace: childSns.Namespace}, checksSubNamespace); err != nil {
//					return false
//				}
//				cpu := childSns.Spec.ResourceQuotaSpec.Hard["cpu"]
//				cpu.Add(updatequotaadd.Spec.ResourceQuotaSpec.Hard["cpu"])
//				memory := childSns.Spec.ResourceQuotaSpec.Hard["memory"]
//				memory.Add(updatequotaadd.Spec.ResourceQuotaSpec.Hard["memory"])
//				var test = corev1.ResourceList{
//					"cpu":    cpu,
//					"memory": memory,
//				}
//				if isResourceListsEqual(&checksSubNamespace.Spec.ResourceQuotaSpec.Hard, &test) {
//					return true
//				} else {
//					return false
//				}
//			}, timeout, interval).Should(BeTrue())
//			Eventually(func() bool {
//				cpu := grandChildSns.Spec.ResourceQuotaSpec.Hard["cpu"]
//				cpu.Add(updatequotaadd.Spec.ResourceQuotaSpec.Hard["cpu"])
//				memory := grandChildSns.Spec.ResourceQuotaSpec.Hard["memory"]
//				memory.Add(updatequotaadd.Spec.ResourceQuotaSpec.Hard["memory"])
//				var test = corev1.ResourceList{
//					"cpu":    cpu,
//					"memory": memory,
//				}
//				if err := k8sClient.Get(ctx, client.ObjectKey{Name: grandChildSns.Name, Namespace: grandChildSns.Namespace}, checksSubNamespace); err != nil {
//					return false
//				}
//				if isResourceListsEqual(&checksSubNamespace.Spec.ResourceQuotaSpec.Hard, &test) {
//					return true
//				}
//				return false
//			}, timeout, interval).Should(BeTrue())
//		})
//
//		It("Should set updatequotaadd phase to Done", func() {
//			Eventually(func() bool {
//				if err := k8sClient.Get(ctx, client.ObjectKey{Name: updatequotaadd.Name}, updatequotaadd); err != nil {
//					return false
//				}
//				if updatequotaadd.Status.Phase == danav1.Done {
//					return true
//				}
//				return false
//			}, timeout, interval).Should(BeTrue())
//		})
//
//		It("check childSns and grandChildSns updated by update quota - Reclaim operation", func() {
//			Eventually(func() bool {
//				if err := k8sClient.Create(ctx, updatequotareclaim); err != nil {
//					return false
//				}
//				return true
//			}, timeout, interval).Should(BeTrue())
//			isCreated(ctx, k8sClient, updatequotareclaim)
//			Eventually(func() bool {
//				if err := k8sClient.Get(ctx, client.ObjectKey{Name: childSns.Name, Namespace: childSns.Namespace}, checksSubNamespace); err != nil {
//					return false
//				}
//				cpu := childSns.Spec.ResourceQuotaSpec.Hard["cpu"]
//				cpu.Add(updatequotaadd.Spec.ResourceQuotaSpec.Hard["cpu"])
//				memory := childSns.Spec.ResourceQuotaSpec.Hard["memory"]
//				memory.Add(updatequotaadd.Spec.ResourceQuotaSpec.Hard["memory"])
//				var test = corev1.ResourceList{
//					"cpu":    cpu,
//					"memory": memory,
//				}
//				if isResourceListsEqual(&checksSubNamespace.Spec.ResourceQuotaSpec.Hard, &test) {
//					return true
//				} else {
//					return false
//				}
//			}, timeout, interval).Should(BeTrue())
//			Eventually(func() bool {
//				cpu := grandChildSns.Spec.ResourceQuotaSpec.Hard["cpu"]
//				cpu.Add(updatequotaadd.Spec.ResourceQuotaSpec.Hard["cpu"])
//				cpu.Sub(updatequotareclaim.Spec.ResourceQuotaSpec.Hard["cpu"])
//				memory := grandChildSns.Spec.ResourceQuotaSpec.Hard["memory"]
//				memory.Add(updatequotaadd.Spec.ResourceQuotaSpec.Hard["memory"])
//				memory.Sub(updatequotareclaim.Spec.ResourceQuotaSpec.Hard["memory"])
//				var test = corev1.ResourceList{
//					"cpu":    cpu,
//					"memory": memory,
//				}
//				if err := k8sClient.Get(ctx, client.ObjectKey{Name: grandChildSns.Name, Namespace: grandChildSns.Namespace}, checksSubNamespace); err != nil {
//					return false
//				}
//				if isResourceListsEqual(&checksSubNamespace.Spec.ResourceQuotaSpec.Hard, &test) {
//					return true
//				}
//				return false
//			}, timeout, interval).Should(BeTrue())
//		})
//
//		It("Should set updatequotareclaim phase to Done", func() {
//			Eventually(func() bool {
//				if err := k8sClient.Get(ctx, client.ObjectKey{Name: updatequotaadd.Name}, updatequotareclaim); err != nil {
//					return false
//				}
//				if updatequotareclaim.Status.Phase == danav1.Done {
//					return true
//				}
//				return false
//			}, timeout, interval).Should(BeTrue())
//		})
//
//	})
//
//	Context("When updatequota invalid is created", func() {
//		It("check childSns and grandChildSns updated by update quota - Add operation", func() {
//			Expect(k8sClient.Create(ctx, updatequotaaddinvalid)).ShouldNot(Succeed())
//			Expect(k8sClient.Create(ctx, updatequotareclaiminvalid)).ShouldNot(Succeed())
//		})
//	})
//
//	Context("When updatequota with wrong operand is created", func() {
//		It("Should not create the updatequota", func() {
//			Expect(k8sClient.Create(ctx, updatequotarewrongoperand)).ShouldNot(Succeed())
//		})
//	})
//})
