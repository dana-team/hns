/*GetCrqUsed
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package controllers

import (
	"context"
	danav1 "github.com/dana-team/hns/api/v1"
	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	qoutav1 "github.com/openshift/api/quota/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"path/filepath"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"testing"
	// +kubebuilder:scaffold:imports
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var cfg *rest.Config
var k8sClient client.Client
var testEnv *envtest.Environment

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecs(t, "Controller Suite")
}

var _ = BeforeSuite(func(done Done) {
	ctx := context.Background()
	existingCluster := true
	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths:  []string{filepath.Join("..", "config", "crd", "bases")},
		UseExistingCluster: &existingCluster,
	}
	var err error
	cfg, err = testEnv.Start()
	Expect(err).ToNot(HaveOccurred())
	Expect(cfg).ToNot(BeNil())

	err = corev1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	err = danav1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	err = rbacv1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	err = qoutav1.AddToScheme(scheme.Scheme)
	Expect(err).NotTo(HaveOccurred())

	// +kubebuilder:scaffold:scheme

	k8sManager, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme: scheme.Scheme,
	})
	Expect(err).ToNot(HaveOccurred())

	//err = (&NamespaceReconciler{
	//	Client: k8sManager.GetClient(),
	//	Log:    ctrl.Log.WithName("controllers").WithName("Namespace"),
	//	Scheme: scheme.Scheme,
	//}).SetupWithManager(k8sManager)
	//Expect(err).ToNot(HaveOccurred())
	//
	//err = (&SubnamespaceReconciler{
	//	Client: k8sManager.GetClient(),
	//	Log:    ctrl.Log.WithName("controllers").WithName("SubNamespace"),
	//	Scheme: scheme.Scheme,
	//}).SetupWithManager(k8sManager)
	//Expect(err).ToNot(HaveOccurred())
	//
	//err = (&RoleBindingReconciler{
	//	Client: k8sManager.GetClient(),
	//	Log:    ctrl.Log.WithName("controllers").WithName("RoleBinding"),
	//	Scheme: scheme.Scheme,
	//}).SetupWithManager(k8sManager)
	//Expect(err).ToNot(HaveOccurred())

	go func() {
		defer GinkgoRecover()
		err = k8sManager.Start(ctrl.SetupSignalHandler())
		Expect(err).ToNot(HaveOccurred())
	}()

	k8sClient = k8sManager.GetClient()
	Expect(k8sClient).ToNot(BeNil())

	Expect(k8sClient.Create(ctx, testRootNs)).Should(Succeed())
	Expect(k8sClient.Create(ctx, testRootCrq)).Should(Succeed())

	close(done)
}, 120)

var _ = AfterSuite(func() {
	ctx := context.Background()
	By("tearing down the test environment")

	err := cleanUp(ctx, k8sClient)
	Expect(err).ToNot(HaveOccurred())

	err = testEnv.Stop()
	Expect(err).ToNot(HaveOccurred())
})
