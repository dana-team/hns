package e2e

import (
	. "github.com/dana-team/hns/pkg/testutils"
	. "github.com/onsi/ginkgo/v2"
)

var _ = PDescribe("ResourcePool", func() {
	nsRoot := GenerateE2EName("root")

	BeforeEach(func() {
		CleanupTestNamespaces()
		CleanupTestUsers()

		// set up root namespace
		CreateRootNS(nsRoot, rqDepth)
		CreateResourceQuota(nsRoot, nsRoot, storage, "100Gi", cpu, "100", memory, "100Gi", pods, "100", gpu, "100")
	})

	AfterEach(func() {
		CleanupTestNamespaces()
		CleanupTestUsers()
	})

	It("should move resources from the root namespace to a subnamespace", func() {
		nsA := GenerateE2EName("a")
		nsB := GenerateE2EName("b")
		nsC := GenerateE2EName("c")

		CreateSubnamespace(nsA, nsRoot, false, storage, "50Gi", cpu, "50", memory, "50Gi", pods, "50", gpu, "50")
		CreateSubnamespace(nsB, nsA, false, storage, "25Gi", cpu, "25", memory, "25Gi", pods, "25", gpu, "25")
		CreateSubnamespace(nsC, nsB, false, storage, "10Gi", cpu, "10", memory, "10Gi", pods, "10", gpu, "10")

		// verify before update
		FieldShouldContain("subnamespace", nsB, nsC, ".status.total.free.pods", "10")

		// move resources
		CreateUpdateQuota("updatequota-from-"+nsRoot+"-to-"+nsC, nsRoot, nsC, "", "pods", "10")

		// verify after update
		FieldShouldContain("subnamespace", nsB, nsC, ".status.total.free.pods", "20")
	})

	It("should not move resources from one sns to another if requesting user doesn't have permissions on both subnamespaces", func() {
		nsA := GenerateE2EName("a")
		nsB := GenerateE2EName("b")
		nsC := GenerateE2EName("c")

		CreateSubnamespace(nsA, nsRoot, false, storage, "50Gi", cpu, "50", memory, "50Gi", pods, "50", gpu, "50")
		CreateSubnamespace(nsB, nsA, false, storage, "25Gi", cpu, "25", memory, "25Gi", pods, "25", gpu, "25")
		CreateSubnamespace(nsC, nsA, false, storage, "10Gi", cpu, "10", memory, "10Gi", pods, "10", gpu, "10")

		// create user and give it admin rolebinding on source subnamespace
		userA := GenerateE2EUserName("user-a")
		CreateUser(userA)
		GrantTestingUserAdmin(userA, nsB)

		// verify before update
		FieldShouldContain("subnamespace", nsA, nsC, ".status.total.free.pods", "10")

		// move resources
		ShouldNotCreateUpdateQuota("updatequota-from-"+nsB+"-to-"+nsC, nsB, nsC, userA, "pods", "10")

		// verify after update
		FieldShouldContain("subnamespace", nsA, nsC, ".status.total.free.pods", "10")
	})

	It("should move resources from one sns to another if requesting user has permissions on both subnamespaces", func() {
		nsA := GenerateE2EName("a")
		nsB := GenerateE2EName("b")
		nsC := GenerateE2EName("c")

		CreateSubnamespace(nsA, nsRoot, false, storage, "50Gi", cpu, "50", memory, "50Gi", pods, "50", gpu, "50")
		CreateSubnamespace(nsB, nsA, false, storage, "25Gi", cpu, "25", memory, "25Gi", pods, "25", gpu, "25")
		CreateSubnamespace(nsC, nsA, false, storage, "10Gi", cpu, "10", memory, "10Gi", pods, "10", gpu, "10")

		// create user and give it admin rolebinding on both subnamespaces
		userA := GenerateE2EUserName("user-a")
		CreateUser(userA)
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
		nsA := GenerateE2EName("a")
		nsB := GenerateE2EName("b")
		nsC := GenerateE2EName("c")

		CreateSubnamespace(nsA, nsRoot, false, storage, "50Gi", cpu, "50", memory, "50Gi", pods, "50", gpu, "50")
		CreateSubnamespace(nsB, nsA, false, storage, "25Gi", cpu, "25", memory, "25Gi", pods, "25", gpu, "25")
		CreateSubnamespace(nsC, nsA, false, storage, "10Gi", cpu, "10", memory, "10Gi", pods, "10", gpu, "10")

		// create user and give it admin rolebinding on both subnamespaces
		userA := GenerateE2EUserName("user-a")
		CreateUser(userA)
		GrantTestingUserAdmin(userA, nsA)

		// verify before update
		FieldShouldContain("subnamespace", nsA, nsC, ".status.total.free.pods", "10")

		// move resources
		CreateUpdateQuota("updatequota-from-"+nsB+"-to-"+nsC, nsB, nsC, userA, "pods", "10")

		// verify after update
		FieldShouldContain("subnamespace", nsA, nsC, ".status.total.free.pods", "20")
	})

	It("should give back resources up the branch even if requesting user has permissions only on source subnamespace", func() {
		nsA := GenerateE2EName("a")
		nsB := GenerateE2EName("b")
		nsC := GenerateE2EName("c")

		CreateSubnamespace(nsA, nsRoot, false, storage, "50Gi", cpu, "50", memory, "50Gi", pods, "50", gpu, "50")
		CreateSubnamespace(nsB, nsA, false, storage, "25Gi", cpu, "25", memory, "25Gi", pods, "25", gpu, "25")
		CreateSubnamespace(nsC, nsB, false, storage, "10Gi", cpu, "10", memory, "10Gi", pods, "10", gpu, "10")

		// create user and give it admin rolebinding on both subnamespaces
		userA := GenerateE2EUserName("user-a")
		CreateUser(userA)
		GrantTestingUserAdmin(userA, nsC)

		// verify before update
		FieldShouldContain("subnamespace", nsB, nsC, ".status.total.free.pods", "10")

		// move resources
		CreateUpdateQuota("updatequota-from-"+nsC+"-to-"+nsA, nsC, nsA, userA, storage, "0", cpu, "0", memory, "0", pods, "10", gpu, "0")

		// verify after update
		FieldShouldContain("subnamespace", nsB, nsC, ".status.total.free.pods", "0")
		FieldShouldContain("subnamespace", nsA, nsB, ".status.total.free.pods", "15")
		FieldShouldContain("subnamespace", nsRoot, nsA, ".status.total.free.pods", "35")
	})

	It("should move resources from the root namespace to a resourcepool", func() {
		nsA := GenerateE2EName("a")
		nsB := GenerateE2EName("b")
		nsC := GenerateE2EName("c")

		CreateSubnamespace(nsA, nsRoot, false, storage, "50Gi", cpu, "50", memory, "50Gi", pods, "50", gpu, "50")
		CreateSubnamespace(nsB, nsA, false, storage, "25Gi", cpu, "25", memory, "25Gi", pods, "25", gpu, "25")
		CreateSubnamespace(nsC, nsB, true, storage, "10Gi", cpu, "10", memory, "10Gi", pods, "10", gpu, "10")

		// verify before update
		FieldShouldContain("subnamespace", nsB, nsC, ".status.total.free.pods", "10")

		// move resources
		CreateUpdateQuota("updatequota-from-"+nsRoot+"-to-"+nsC, nsRoot, nsC, "", "pods", "10")

		// verify after update
		FieldShouldContain("subnamespace", nsB, nsC, ".status.total.free.pods", "20")
	})

	It("should move resources from one resourcepool to another resourcepool", func() {
		nsA := GenerateE2EName("a")
		nsB := GenerateE2EName("b")
		nsC := GenerateE2EName("c")

		CreateSubnamespace(nsA, nsRoot, false, storage, "50Gi", cpu, "50", memory, "50Gi", pods, "50", gpu, "50")
		CreateSubnamespace(nsB, nsA, true, storage, "25Gi", cpu, "25", memory, "25Gi", pods, "25", gpu, "25")
		CreateSubnamespace(nsC, nsA, true, storage, "10Gi", cpu, "10", memory, "10Gi", pods, "10", gpu, "10")

		// verify before update
		FieldShouldContain("subnamespace", nsA, nsC, ".status.total.free.pods", "10")

		// move resources
		CreateUpdateQuota("updatequota-from-"+nsB+"-to-"+nsC, nsB, nsC, "", "pods", "10")

		// verify after update
		FieldShouldContain("subnamespace", nsA, nsC, ".status.total.free.pods", "20")
	})

	It("should not move resources between subnamespaces from different secondary roots", func() {
		nsA := GenerateE2EName("a")
		nsB := GenerateE2EName("b")
		nsC := GenerateE2EName("c")
		nsD := GenerateE2EName("d")

		CreateSubnamespace(nsA, nsRoot, false, storage, "50Gi", cpu, "50", memory, "50Gi", pods, "50", gpu, "50")
		CreateSubnamespace(nsB, nsRoot, false, storage, "50Gi", cpu, "50", memory, "50Gi", pods, "50", gpu, "50")
		AnnotateNSSecondaryRoot(nsA)
		AnnotateNSSecondaryRoot(nsB)

		CreateSubnamespace(nsC, nsA, true, storage, "25Gi", cpu, "25", memory, "25Gi", pods, "25", gpu, "25")
		CreateSubnamespace(nsD, nsB, true, storage, "25Gi", cpu, "25", memory, "25Gi", pods, "25", gpu, "25")

		// verify before update
		FieldShouldContain("subnamespace", nsA, nsC, ".status.total.free.pods", "25")

		// move resources
		ShouldNotCreateUpdateQuota("updatequota-from-"+nsC+"-to-"+nsD, nsC, nsD, "", "pods", "10")

		// verify after update
		FieldShouldContain("subnamespace", nsA, nsC, ".status.total.free.pods", "25")
	})
})
