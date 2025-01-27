package e2e_tests

import (
	. "github.com/dana-team/hns/test/testutils"
	. "github.com/onsi/ginkgo/v2"
)

var _ = Describe("UpdateQuota", func() {
	testPrefix := "upq-test"
	var randPrefix string
	var nsRoot string

	BeforeEach(func() {
		randPrefix = RandStr()

		CleanupTestNamespaces(randPrefix)
		CleanupTestUsers(randPrefix)

		nsRoot = GenerateE2EName("root", testPrefix, randPrefix)
		CreateRootNS(nsRoot, randPrefix, rqDepth)
		CreateResourceQuota(nsRoot, nsRoot, storage, "100Gi", cpu, "100", memory, "100Gi", pods, "100", gpu, "100")
	})

	AfterEach(func() {
		CleanupTestNamespaces(randPrefix)
		CleanupTestUsers(randPrefix)

	})

	It("should move resources from the root namespace to a subnamespace", func() {
		nsA := GenerateE2EName("a", testPrefix, randPrefix)
		nsB := GenerateE2EName("b", testPrefix, randPrefix)
		nsC := GenerateE2EName("c", testPrefix, randPrefix)

		CreateSubnamespace(nsA, nsRoot, randPrefix, false, storage, "50Gi", cpu, "50", memory, "50Gi", pods, "50", gpu, "50")
		CreateSubnamespace(nsB, nsA, randPrefix, false, storage, "25Gi", cpu, "25", memory, "25Gi", pods, "25", gpu, "25")
		CreateSubnamespace(nsC, nsB, randPrefix, false, storage, "10Gi", cpu, "10", memory, "10Gi", pods, "10", gpu, "10")

		// verify before update
		FieldShouldContain("subnamespace", nsB, nsC, ".status.total.free.pods", "10")

		// move resources
		CreateUpdateQuota("updatequota-from-"+nsRoot+"-to-"+nsC, nsRoot, nsC, "", "pods", "10")

		// verify after update
		FieldShouldContain("subnamespace", nsB, nsC, ".status.total.free.pods", "20")
	})

	It("should add a requester annotation to the updatequota object with the account name", func() {
		nsA := GenerateE2EName("a", testPrefix, randPrefix)
		nsB := GenerateE2EName("b", testPrefix, randPrefix)
		CreateSubnamespace(nsA, nsRoot, randPrefix, false, storage, "50Gi", cpu, "50", memory, "50Gi", pods, "50", gpu, "50")
		CreateSubnamespace(nsB, nsRoot, randPrefix, false, storage, "25Gi", cpu, "25", memory, "25Gi", pods, "25", gpu, "25")

		userA := GenerateE2EUserName("user-a")
		CreateUser(userA, randPrefix)
		GrantTestingUserClusterAdmin(userA)

		CreateUpdateQuota("updatequota-from-"+nsA+"-to-"+nsB, nsA, nsB, userA, "pods", "10")

		FieldShouldContain("updatequota", nsA, "updatequota-from-"+nsA+"-to-"+nsB, ".metadata.annotations.requester", userA)

	})

	It("should not move resources from one sns to another if requesting user doesn't"+
		" have permissions on both subnamespaces", func() {
		nsA := GenerateE2EName("a", testPrefix, randPrefix)
		nsB := GenerateE2EName("b", testPrefix, randPrefix)
		nsC := GenerateE2EName("c", testPrefix, randPrefix)

		CreateSubnamespace(nsA, nsRoot, randPrefix, false, storage, "50Gi", cpu, "50", memory, "50Gi", pods, "50", gpu, "50")
		CreateSubnamespace(nsB, nsA, randPrefix, false, storage, "25Gi", cpu, "25", memory, "25Gi", pods, "25", gpu, "25")
		CreateSubnamespace(nsC, nsA, randPrefix, false, storage, "10Gi", cpu, "10", memory, "10Gi", pods, "10", gpu, "10")

		// create user and give it admin rolebinding on source subnamespace
		userA := GenerateE2EUserName("user-a")
		CreateUser(userA, randPrefix)
		GrantTestingUserAdmin(userA, nsB)

		// verify before update
		FieldShouldContain("subnamespace", nsA, nsC, ".status.total.free.pods", "10")

		// move resources
		ShouldNotCreateUpdateQuota("updatequota-from-"+nsB+"-to-"+nsC, nsB, nsC, userA, "pods", "10")

		// verify after update
		FieldShouldContain("subnamespace", nsA, nsC, ".status.total.free.pods", "10")
	})
	It("should move resources from one sns to another if requesting user has permissions on both subnamespaces", func() {
		nsA := GenerateE2EName("a", testPrefix, randPrefix)
		nsB := GenerateE2EName("b", testPrefix, randPrefix)
		nsC := GenerateE2EName("c", testPrefix, randPrefix)

		CreateSubnamespace(nsA, nsRoot, randPrefix, false, storage, "50Gi", cpu, "50", memory, "50Gi", pods, "50", gpu, "50")
		CreateSubnamespace(nsB, nsA, randPrefix, false, storage, "25Gi", cpu, "25", memory, "25Gi", pods, "25", gpu, "25")
		CreateSubnamespace(nsC, nsA, randPrefix, false, storage, "10Gi", cpu, "10", memory, "10Gi", pods, "10", gpu, "10")

		// create user and give it admin rolebinding on both subnamespaces
		userA := GenerateE2EUserName("user-a")
		CreateUser(userA, randPrefix)
		GrantTestingUserAdmin(userA, nsB)
		GrantTestingUserAdmin(userA, nsC)

		// verify before update
		FieldShouldContain("subnamespace", nsA, nsC, ".status.total.free.pods", "10")

		// move resources
		CreateUpdateQuota("updatequota-from-"+nsB+"-to-"+nsC, nsB, nsC, userA, "pods", "10")

		// verify after update
		FieldShouldContain("subnamespace", nsA, nsC, ".status.total.free.pods", "20")
	})

	It("should move resources from one sns to another if requesting user has permissions on ancestor", func() {
		nsA := GenerateE2EName("a", testPrefix, randPrefix)
		nsB := GenerateE2EName("b", testPrefix, randPrefix)
		nsC := GenerateE2EName("c", testPrefix, randPrefix)

		CreateSubnamespace(nsA, nsRoot, randPrefix, false, storage, "50Gi", cpu, "50", memory, "50Gi", pods, "50", gpu, "50")
		CreateSubnamespace(nsB, nsA, randPrefix, false, storage, "25Gi", cpu, "25", memory, "25Gi", pods, "25", gpu, "25")
		CreateSubnamespace(nsC, nsA, randPrefix, false, storage, "10Gi", cpu, "10", memory, "10Gi", pods, "10", gpu, "10")

		// create user and give it admin rolebinding on both subnamespaces
		userA := GenerateE2EUserName("user-a")
		CreateUser(userA, randPrefix)
		GrantTestingUserAdmin(userA, nsA)

		// verify before update
		FieldShouldContain("subnamespace", nsA, nsC, ".status.total.free.pods", "10")

		// move resources
		CreateUpdateQuota("updatequota-from-"+nsB+"-to-"+nsC, nsB, nsC, userA, "pods", "10")

		// verify after update
		FieldShouldContain("subnamespace", nsA, nsC, ".status.total.free.pods", "20")
	})

	It("should give back resources up the branch even if requesting user has"+
		" permissions only on source subnamespace", func() {
		nsA := GenerateE2EName("a", testPrefix, randPrefix)
		nsB := GenerateE2EName("b", testPrefix, randPrefix)
		nsC := GenerateE2EName("c", testPrefix, randPrefix)

		CreateSubnamespace(nsA, nsRoot, randPrefix, false, storage, "50Gi", cpu, "50", memory, "50Gi", pods, "50", gpu, "50")
		CreateSubnamespace(nsB, nsA, randPrefix, false, storage, "25Gi", cpu, "25", memory, "25Gi", pods, "25", gpu, "25")
		CreateSubnamespace(nsC, nsB, randPrefix, false, storage, "10Gi", cpu, "10", memory, "10Gi", pods, "10", gpu, "10")

		// create user and give it admin rolebinding on both subnamespaces
		userA := GenerateE2EUserName("user-a")
		CreateUser(userA, randPrefix)
		GrantTestingUserAdmin(userA, nsC)

		// verify before update
		FieldShouldContain("subnamespace", nsB, nsC, ".status.total.free.pods", "10")

		// move resources
		CreateUpdateQuota("updatequota-from-"+nsC+"-to-"+nsA, nsC, nsA, userA,
			storage, "0", cpu, "0", memory, "0", pods, "10", gpu, "0")

		// verify after update
		FieldShouldContain("subnamespace", nsB, nsC, ".status.total.free.pods", "0")
		FieldShouldContain("subnamespace", nsA, nsB, ".status.total.free.pods", "15")
		FieldShouldContain("subnamespace", nsRoot, nsA, ".status.total.free.pods", "35")
	})

	It("should move resources from the root namespace to a resourcepool", func() {
		nsA := GenerateE2EName("a", testPrefix, randPrefix)
		nsB := GenerateE2EName("b", testPrefix, randPrefix)
		nsC := GenerateE2EName("c", testPrefix, randPrefix)

		CreateSubnamespace(nsA, nsRoot, randPrefix, false, storage, "50Gi", cpu, "50", memory, "50Gi", pods, "50", gpu, "50")
		CreateSubnamespace(nsB, nsA, randPrefix, false, storage, "25Gi", cpu, "25", memory, "25Gi", pods, "25", gpu, "25")
		CreateSubnamespace(nsC, nsB, randPrefix, true, storage, "10Gi", cpu, "10", memory, "10Gi", pods, "10", gpu, "10")

		// verify before update
		FieldShouldContain("subnamespace", nsB, nsC, ".status.total.free.pods", "10")

		// move resources
		CreateUpdateQuota("updatequota-from-"+nsRoot+"-to-"+nsC, nsRoot, nsC, "", "pods", "10")

		// verify after update
		FieldShouldContain("subnamespace", nsB, nsC, ".status.total.free.pods", "20")
	})

	It("should move resources from one resourcepool to another resourcepool", func() {
		nsA := GenerateE2EName("a", testPrefix, randPrefix)
		nsB := GenerateE2EName("b", testPrefix, randPrefix)
		nsC := GenerateE2EName("c", testPrefix, randPrefix)

		CreateSubnamespace(nsA, nsRoot, randPrefix, false, storage, "50Gi", cpu, "50", memory, "50Gi", pods, "50", gpu, "50")
		CreateSubnamespace(nsB, nsA, randPrefix, true, storage, "25Gi", cpu, "25", memory, "25Gi", pods, "25", gpu, "25")
		CreateSubnamespace(nsC, nsA, randPrefix, true, storage, "10Gi", cpu, "10", memory, "10Gi", pods, "10", gpu, "10")

		// verify before update
		FieldShouldContain("subnamespace", nsA, nsC, ".status.total.free.pods", "10")

		// move resources
		CreateUpdateQuota("updatequota-from-"+nsB+"-to-"+nsC, nsB, nsC, "", "pods", "10")

		// verify after update
		FieldShouldContain("subnamespace", nsA, nsC, ".status.total.free.pods", "20")
	})

	It("should not move resources between subnamespaces from different secondary roots", func() {
		nsA := GenerateE2EName("a", testPrefix, randPrefix)
		nsB := GenerateE2EName("b", testPrefix, randPrefix)
		nsC := GenerateE2EName("c", testPrefix, randPrefix)
		nsD := GenerateE2EName("d", testPrefix, randPrefix)

		CreateSubnamespace(nsA, nsRoot, randPrefix, false, storage, "50Gi", cpu, "50", memory, "50Gi", pods, "50", gpu, "50")
		CreateSubnamespace(nsB, nsRoot, randPrefix, false, storage, "50Gi", cpu, "50", memory, "50Gi", pods, "50", gpu, "50")
		AnnotateNSSecondaryRoot(nsA)
		AnnotateNSSecondaryRoot(nsB)

		CreateSubnamespace(nsC, nsA, randPrefix, true, storage, "25Gi", cpu, "25", memory, "25Gi", pods, "25", gpu, "25")
		CreateSubnamespace(nsD, nsB, randPrefix, true, storage, "25Gi", cpu, "25", memory, "25Gi", pods, "25", gpu, "25")

		// verify before update
		FieldShouldContain("subnamespace", nsA, nsC, ".status.total.free.pods", "25")

		// move resources
		ShouldNotCreateUpdateQuota("updatequota-from-"+nsC+"-to-"+nsD, nsC, nsD, "", "pods", "10")

		// verify after update
		FieldShouldContain("subnamespace", nsA, nsC, ".status.total.free.pods", "25")
	})
	It("should allow the creation of an updatequota with a user in a permitted group", func() {
		nsA := GenerateE2EName("a", testPrefix, randPrefix)
		nsB := GenerateE2EName("b", testPrefix, randPrefix)
		CreateSubnamespace(nsA, nsRoot, randPrefix, false, storage, "50Gi", cpu, "50", memory, "50Gi", pods, "50", gpu, "50")
		CreateSubnamespace(nsB, nsRoot, randPrefix, false, storage, "25Gi", cpu, "25", memory, "25Gi", pods, "25", gpu, "25")

		userA := GenerateE2EUserName("user-a")

		CreateUser(userA, randPrefix)
		CreateGroup("test", userA, randPrefix)
		GrantTestingUserAdmin(userA, nsA)

		CreateUpdateQuota("updatequota-from-"+nsA+"-to-"+nsB, nsA, nsB, userA, "pods", "10")

		FieldShouldContain("subnamespace", nsRoot, nsB, ".status.total.free.pods", "35")
		CleanupTestGroup("test")

	})
})
