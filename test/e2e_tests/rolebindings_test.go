package e2e_tests

import (
	. "github.com/dana-team/hns/test/testutils"
	. "github.com/onsi/ginkgo/v2"
)

var _ = Describe("RoleBindings", func() {
	testPrefix := "rb-test"
	var randPrefix string
	var nsRoot string

	BeforeEach(func() {
		randPrefix = RandStr()

		CleanupTestNamespaces(randPrefix)

		nsRoot = GenerateE2EName("root", testPrefix, randPrefix)
		CreateRootNS(nsRoot, randPrefix, rqDepth)
		CreateResourceQuota(nsRoot, nsRoot, storage, "100Gi", cpu, "100", memory, "100Gi", pods, "100", gpu, "100")
	})

	AfterEach(func() {
		CleanupTestNamespaces(randPrefix)
	})

	It("Should copy rolebindings from root to child", func() {
		By("Creating a user in the root namespace and granting it admin role")
		user := GenerateE2EUserName("user")
		CreateUser(user, randPrefix)
		GrantTestingUserAdmin(user, nsRoot)

		By("Creating a child namespace")
		nsChild := GenerateE2EName("child", testPrefix, randPrefix)
		CreateSubnamespace(nsChild, nsRoot, randPrefix, false,
			storage, "50Gi", cpu, "50", memory, "50Gi", pods, "50", gpu, "50")

		By("Checking that the rolebinding has been created in the child namespace")
		FieldShouldContain("rolebinding", nsChild,
			"test-admin-"+user+"-"+nsRoot, ".metadata.name", "test-admin-"+user+"-"+nsRoot)
	})

	It("Should delete rolebinding from child if it has been deleted from parent", func() {
		By("Creating a user in the root namespace and granting it admin role")
		user := GenerateE2EUserName("user")
		CreateUser(user, randPrefix)
		GrantTestingUserAdmin(user, nsRoot)

		By("Creating a child namespace")
		nsChild := GenerateE2EName("child", testPrefix, randPrefix)
		CreateSubnamespace(nsChild, nsRoot, randPrefix, false,
			storage, "50Gi", cpu, "50", memory, "50Gi", pods, "50", gpu, "50")

		By("Checking that the rolebinding has been created in the child namespace")
		FieldShouldContain("rolebinding", nsChild,
			"test-admin-"+user+"-"+nsRoot, ".metadata.name", "test-admin-"+user+"-"+nsRoot)

		By("Deleting the rolebinding from the root namespace")
		ShouldDelete("rolebinding", nsRoot, "test-admin-"+user+"-"+nsRoot)

		By("Checking that the rolebinding has been deleted from the child namespace")
		ShouldNotExist("rolebinding", nsChild, "test-admin-"+user+"-"+nsRoot)
	})

	It("Should block the deletion of a rolebinding from a child namespace that was created by a parent namespace", func() {
		By("Creating a user in the root namespace and granting it admin role")
		user := GenerateE2EUserName("user")
		CreateUser(user, randPrefix)
		GrantTestingUserAdmin(user, nsRoot)

		By("Creating a child namespace")
		nsChild := GenerateE2EName("child", testPrefix, randPrefix)
		CreateSubnamespace(nsChild, nsRoot, randPrefix, false,
			storage, "50Gi", cpu, "50", memory, "50Gi", pods, "50", gpu, "50")

		By("Checking that the rolebinding has been created in the child namespace")
		FieldShouldContain("rolebinding", nsChild, "test-admin-"+user+"-"+nsRoot,
			".metadata.name", "test-admin-"+user+"-"+nsRoot)

		By("Trying to delete the rolebinding from the child namespace")
		ShouldNotDelete("rolebinding", nsChild, "test-admin-"+user+"-"+nsRoot)
	})

	It("Should create hns-view rolebindings in subnamespace and bind to all other rolebinding subjects", func() {
		By("Creating a subnamespace")
		nsChild := GenerateE2EName("child", testPrefix, randPrefix)
		CreateSubnamespace(nsChild, nsRoot, randPrefix, false,
			storage, "50Gi", cpu, "50", memory, "50Gi", pods, "50", gpu, "50")

		user := GenerateE2EUserName("user")
		CreateUser(user, randPrefix)
		GrantTestingUserAdmin(user, nsChild)
		FieldShouldContain("clusterrolebindings", "", nsChild+"-hns-view", ".metadata.name", nsChild+"-hns-view")
		ComplexFieldShouldContain("clusterrolebindings", "",
			nsChild+"-hns-view", "'{{range.subjects}}{{.name}}{{\"\\n\"}}{{end}}'", user)
	})

	It("Should not bind a serviceaccount to the hns-view rolebinding", func() {
		By("Creating a subnamespace")
		nsChild := GenerateE2EName("child", testPrefix, randPrefix)
		CreateSubnamespace(nsChild, nsRoot, randPrefix, false,
			storage, "50Gi", cpu, "50", memory, "50Gi", pods, "50", gpu, "50")

		serviceAccount := GenerateE2EUserName("serviceaccount")
		CreateServiceAccount(serviceAccount, nsRoot, randPrefix)
		GrantTestingServiceAccountAdmin(serviceAccount, nsRoot)
		ComplexFieldShouldNotContain("clusterrolebindings", "", nsChild+"-hns-view",
			"'{{range.subjects}}{{.name}}{{\"\\n\"}}{{end}}'", serviceAccount)

	})
})
