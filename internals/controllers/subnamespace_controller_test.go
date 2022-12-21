package controllers

import (
	danav1 "github.com/dana-team/hns/api/v1"
	"github.com/dana-team/hns/internals/utils"
	"context"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	v1 "github.com/openshift/api/quota/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = PDescribe("Subnamespace controller tests", func() {
	ctx := context.Background()
	childSns := utils.ComposeSns(getRandomName("childsns"), testRootNs.Name, composeDefaultSnsQuota(int64(childResources)), map[string]string{})
	childNs := utils.ComposeNamespace(childSns.Name, map[string]string{danaTestLabel: "true"}, map[string]string{})
	grandsonSns := utils.ComposeSns(getRandomName("grandsonsns"), childNs.Name, composeDefaultSnsQuota(int64(grandsonResources)), map[string]string{})
	grandsonNs := utils.ComposeNamespace(grandsonSns.Name, map[string]string{danaTestLabel: "true"}, map[string]string{})
	grandsonNonResourcePool := utils.ComposeSns(grandsonSns.Name, childNs.Name, composeDefaultSnsQuota(int64(grandsonResources)), map[string]string{danav1.ResourcePool: "false"})
	childResourcePool := utils.ComposeSns(childSns.Name, testRootNs.Name, composeDefaultSnsQuota(int64(childResources)), map[string]string{danav1.ResourcePool: "true"})
	invalidResourcesSns := utils.ComposeSns(getRandomName("sns"), testRootNs.Name, composeDefaultSnsQuota(int64(rootResources+10)), map[string]string{})

	Context("When sub namespace created", func() {
		It("Creating a sns in root namespace", func() {
			Expect(k8sClient.Create(ctx, childSns)).Should(Succeed())
			isCreated(ctx, k8sClient, childNs)
		})
		It("Should create childNs crq", func() {
			isCreated(ctx, k8sClient, utils.ComposeCrq(childNs.Name, corev1.ResourceQuotaSpec{}, map[string]string{}))
		})
		It("Should create childNs resource quota", func() {
			isCreated(ctx, k8sClient, utils.ComposeResourceQuota(childNs.Name, childNs.Name, corev1.ResourceList{}))
		})
		It("Should create childNs limit range", func() {
			isCreated(ctx, k8sClient, utils.ComposeLimitRange(childNs.Name, childNs.Name, corev1.LimitRangeItem{}))
		})
		It("Should set sns phase to created & sns namespace ref to childNs & sns owner", func() {
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, client.ObjectKey{Name: childSns.Name, Namespace: childSns.Namespace}, checksSubNamespace); err != nil {
					return false
				}
				if owner := metav1.GetControllerOf(checksSubNamespace); owner.Name != testRootNs.Name {
					return false
				}
				if utils.GetSnsPhase(checksSubNamespace) != danav1.Created {
					return false
				}

				if utils.GetSnsNamespaceRef(checksSubNamespace) != childNs.Name {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())
		})
	})

	Context("When sns with same name is created", func() {
		It("Should not create the sns", func() {
			Expect(k8sClient.Create(ctx, childSns)).ShouldNot(Succeed())
		})
	})

	Context("When create sns that requests more resources from parent crq", func() {
		It("Creating a sns in root namespace", func() {
			Expect(k8sClient.Create(ctx, invalidResourcesSns)).ShouldNot(Succeed())
		})
	})

	Context("When creating grandson sns", func() {
		It("Should create grandson namespace", func() {
			Expect(k8sClient.Create(ctx, grandsonSns)).Should(Succeed())
			isCreated(ctx, k8sClient, grandsonNs)
		})

		It("Should set sns free resources to his status", func() {
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, client.ObjectKey{Name: childSns.Name, Namespace: childSns.Namespace}, checksSubNamespace); err != nil {
					return false
				}

				cpu := childSns.Spec.ResourceQuotaSpec.Hard["cpu"]
				cpu.Sub(grandsonSns.Spec.ResourceQuotaSpec.Hard["cpu"])
				memory := childSns.Spec.ResourceQuotaSpec.Hard["memory"]
				memory.Sub(grandsonSns.Spec.ResourceQuotaSpec.Hard["memory"])
				var freeResources = corev1.ResourceList{
					"cpu":    cpu,
					"memory": memory,
				}
				if isResourceListsEqual(&checksSubNamespace.Status.Total.Free, &freeResources) {
					return true
				}
				return false
			}, timeout, interval).Should(BeTrue())
		})

	})

	Context("When changing child sns resourcepool false->true", func() {
		It("Should delete all descendants crqs & update false->true on grandson sns", func() {
			Expect(k8sClient.Get(ctx, client.ObjectKey{Name: childSns.Name, Namespace: childSns.Namespace}, checksSubNamespace)).Should(Succeed())
			checksSubNamespace.Labels[danav1.ResourcePool] = "true"
			Expect(k8sClient.Update(ctx, checksSubNamespace)).Should(Succeed())
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, client.ObjectKey{Name: grandsonSns.Name, Namespace: grandsonSns.Namespace}, checksSubNamespace); err != nil {
					return false
				}
				if checksSubNamespace.Labels[danav1.ResourcePool] != "true" {
					return false
				}
				if err := k8sClient.Get(ctx, client.ObjectKey{Name: grandsonSns.Name}, &v1.ClusterResourceQuota{}); err != nil {
					if !apierrors.IsNotFound(err) {
						return false
					}
				}
				return true
			}, timeout, interval).Should(BeTrue())
		})
	})

	Context("When changing grandson resourcepool true->false", func() {
		It("Should fail because parent is true", func() {
			Expect(k8sClient.Get(ctx, client.ObjectKey{Name: grandsonSns.Name, Namespace: grandsonSns.Namespace}, checksSubNamespace)).Should(Succeed())
			checksSubNamespace.Labels[danav1.ResourcePool] = "false"
			Expect(k8sClient.Update(ctx, checksSubNamespace)).ShouldNot(Succeed())
		})
	})

	Context("When changing child sns resourcepool true->false", func() {
		It("Should create all descendants crqs & update true->false on grandson sns", func() {
			Expect(k8sClient.Get(ctx, client.ObjectKey{Name: childSns.Name, Namespace: childSns.Namespace}, checksSubNamespace)).Should(Succeed())
			checksSubNamespace.Labels[danav1.ResourcePool] = "false"
			Expect(k8sClient.Update(ctx, checksSubNamespace)).Should(Succeed())
			Eventually(func() bool {
				if err := k8sClient.Get(ctx, client.ObjectKey{Name: grandsonSns.Name, Namespace: grandsonSns.Namespace}, checksSubNamespace); err != nil {
					return false
				}
				if checksSubNamespace.Labels[danav1.ResourcePool] != "false" {
					return false
				}
				if err := k8sClient.Get(ctx, client.ObjectKey{Name: grandsonSns.Name}, &v1.ClusterResourceQuota{}); err != nil {
					return false
				}
				return true
			}, timeout, interval).Should(BeTrue())
		})
	})

	Context("When changing child resourcepool false->true while grandson is true", func() {
		It("Should change grandson to true", func() {
			Expect(k8sClient.Get(ctx, client.ObjectKey{Name: grandsonSns.Name, Namespace: grandsonSns.Namespace}, checksSubNamespace)).Should(Succeed())
			checksSubNamespace.Labels[danav1.ResourcePool] = "true"
			Expect(k8sClient.Update(ctx, checksSubNamespace)).Should(Succeed())
		})
		It("Should fail because descendant (grandson) is true", func() {
			Expect(k8sClient.Get(ctx, client.ObjectKey{Name: childSns.Name, Namespace: childSns.Namespace}, checksSubNamespace)).Should(Succeed())
			checksSubNamespace.Labels[danav1.ResourcePool] = "true"
			Expect(k8sClient.Update(ctx, checksSubNamespace)).ShouldNot(Succeed())
		})
	})

	Context("When Deleting grandson sns", func() {
		It("Should delete grandson namespace", func() {
			Expect(k8sClient.Delete(ctx, grandsonNs)).Should(Succeed())
			isDeleted(ctx, k8sClient, grandsonNs)
		})
	})

	Context("When update sns resources to more than parent resources", func() {
		It("Should not update the sns", func() {
			Expect(k8sClient.Get(ctx, client.ObjectKey{Name: childSns.Name, Namespace: childSns.Namespace}, checksSubNamespace)).Should(Succeed())
			newQuantity := checksSubNamespace.Spec.ResourceQuotaSpec.Hard.Cpu()
			newQuantity.Add(*resource.NewQuantity(rootResources, resource.DecimalSI))
			checksSubNamespace.Spec.ResourceQuotaSpec.Hard["cpu"] = *newQuantity
			Expect(k8sClient.Update(ctx, checksSubNamespace)).ShouldNot(Succeed())
		})
	})

	Context("When delete sns from root ns", func() {
		It("Should delete the sns", func() {
			Expect(k8sClient.Delete(ctx, childNs)).Should(Succeed())
			isDeleted(ctx, k8sClient, childNs)
		})
	})

	Context("When creating child resource pool & creating non-resource pool grandson", func() {
		It("Should create child root resource pool", func() {
			Expect(k8sClient.Create(ctx, childResourcePool)).Should(Succeed())
			isCreated(ctx, k8sClient, childNs)
		})
		It("Should fail creating non-resource pool grandson", func() {
			Expect(k8sClient.Create(ctx, grandsonNonResourcePool)).ShouldNot(Succeed())
		})
	})
})
