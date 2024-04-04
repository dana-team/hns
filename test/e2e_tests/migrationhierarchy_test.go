package e2e_tests

import (
	danav1 "github.com/dana-team/hns/api/v1"
	. "github.com/dana-team/hns/test/testutils"
	. "github.com/onsi/ginkgo/v2"
)

var _ = Describe("MigrationHierarchy", func() {
	testPrefix := "mh-test"
	var randPrefix string
	var nsRoot string

	BeforeEach(func() {
		randPrefix = RandStr()

		CleanupTestNamespaces(randPrefix)
		CleanupTestMigrationHierarchies(randPrefix)

		nsRoot = GenerateE2EName("root", testPrefix, randPrefix)
		CreateRootNS(nsRoot, randPrefix, rqDepth)
		CreateResourceQuota(nsRoot, nsRoot, storage, "100Gi", cpu, "100", memory, "100Gi", pods, "100", gpu, "100")
	})

	AfterEach(func() {
		CleanupTestNamespaces(randPrefix)
		CleanupTestMigrationHierarchies(randPrefix)

	})

	It("should migrate subnamespace that have a CRQ or its direct parent have a CRQ,", func() {
		nsA := GenerateE2EName("a", testPrefix, randPrefix)
		nsB := GenerateE2EName("b", testPrefix, randPrefix)
		nsC := GenerateE2EName("c", testPrefix, randPrefix)
		nsD := GenerateE2EName("d", testPrefix, randPrefix)
		nsE := GenerateE2EName("e", testPrefix, randPrefix)
		nsF := GenerateE2EName("f", testPrefix, randPrefix)

		// create hierarchy
		CreateSubnamespace(nsA, nsRoot, randPrefix, false, storage, "50Gi", cpu, "50", memory, "50Gi", pods, "50", gpu, "50")
		CreateSubnamespace(nsB, nsA, randPrefix, false, storage, "25Gi", cpu, "25", memory, "25Gi", pods, "25", gpu, "25")
		CreateSubnamespace(nsC, nsB, randPrefix, false, storage, "10Gi", cpu, "10", memory, "10Gi", pods, "10", gpu, "10")
		CreateSubnamespace(nsD, nsC, randPrefix, false, storage, "5Gi", cpu, "5", memory, "5Gi", pods, "5", gpu, "5")
		CreateSubnamespace(nsE, nsC, randPrefix, false, storage, "5Gi", cpu, "5", memory, "5Gi", pods, "5", gpu, "5")
		CreateSubnamespace(nsF, nsD, randPrefix, false, storage, "1Gi", cpu, "1", memory, "1Gi", pods, "1", gpu, "1")

		mhName := CreateMigrationHierarchy(nsF, nsE, "")

		// make sure the subnamespace was migrated and the parent was updated
		FieldShouldContain("subnamespace", nsE, nsF, ".metadata.namespace", nsE)
		FieldShouldContain("namespace", "", nsF, ".metadata.labels", danav1.Parent+":"+nsE)

		// verify phase is complete before labeling it
		FieldShouldContain("migrationhierarchy", "", mhName, ".status.phase", "Complete")
		LabelTestingMigrationHierarchies(mhName, randPrefix)
	})

	It("should add a requester annotation to the migrationhierarchy object with the account name", func() {
		nsA := GenerateE2EName("a", testPrefix, randPrefix)
		nsB := GenerateE2EName("b", testPrefix, randPrefix)
		CreateSubnamespace(nsA, nsRoot, randPrefix, false, storage, "50Gi", cpu, "50", memory, "50Gi", pods, "50", gpu, "50")
		CreateSubnamespace(nsB, nsRoot, randPrefix, false, storage, "25Gi", cpu, "25", memory, "25Gi", pods, "25", gpu, "25")

		userA := GenerateE2EUserName("user-a")
		CreateUser(userA, randPrefix)
		GrantTestingUserClusterAdmin(userA)

		mhName := CreateMigrationHierarchy(nsB, nsA, userA)

		// verify phase is complete before labeling it
		FieldShouldContain("migrationhierarchy", "", mhName, ".status.phase", "Complete")
		FieldShouldContain("migrationhierarchy", "", mhName, ".metadata.annotations.requester", userA)
		LabelTestingMigrationHierarchies(mhName, randPrefix)
	})

	It("should migrate subnamespace that doesn't have a CRQ and their direct parent doesn't have a CRQ,", func() {
		nsA := GenerateE2EName("a", testPrefix, randPrefix)
		nsB := GenerateE2EName("b", testPrefix, randPrefix)
		nsC := GenerateE2EName("c", testPrefix, randPrefix)
		nsD := GenerateE2EName("d", testPrefix, randPrefix)

		// create hierarchy
		CreateSubnamespace(nsA, nsRoot, randPrefix, false, storage, "50Gi", cpu, "50", memory, "50Gi", pods, "50", gpu, "50")
		CreateSubnamespace(nsB, nsA, randPrefix, false, storage, "25Gi", cpu, "25", memory, "25Gi", pods, "25", gpu, "25")
		CreateSubnamespace(nsC, nsA, randPrefix, false, storage, "25Gi", cpu, "25", memory, "25Gi", pods, "25", gpu, "25")
		CreateSubnamespace(nsD, nsB, randPrefix, false, storage, "10Gi", cpu, "10", memory, "10Gi", pods, "10", gpu, "10")

		mhName := CreateMigrationHierarchy(nsD, nsC, "")

		// verify phase is complete before labeling it
		FieldShouldContain("migrationhierarchy", "", mhName, ".status.phase", "Complete")
		FieldShouldContain("subnamespace", nsC, nsD, ".metadata.namespace", nsC)
		FieldShouldContain("namespace", "", nsD, ".metadata.labels", danav1.Parent+":"+nsC)
		LabelTestingMigrationHierarchies(mhName, randPrefix)
	})

	It("should migrate subnamespace that does have a CRQ under a subnamespace that doesn't have a CRQ", func() {
		nsA := GenerateE2EName("a", testPrefix, randPrefix)
		nsB := GenerateE2EName("b", testPrefix, randPrefix)
		nsC := GenerateE2EName("c", testPrefix, randPrefix)
		nsD := GenerateE2EName("d", testPrefix, randPrefix)
		nsE := GenerateE2EName("e", testPrefix, randPrefix)
		nsF := GenerateE2EName("f", testPrefix, randPrefix)

		// create hierarchy
		CreateSubnamespace(nsA, nsRoot, randPrefix, false, storage, "50Gi", cpu, "50", memory, "50Gi", pods, "50", gpu, "50")
		CreateSubnamespace(nsB, nsA, randPrefix, false, storage, "25Gi", cpu, "25", memory, "25Gi", pods, "25", gpu, "25")
		CreateSubnamespace(nsC, nsA, randPrefix, false, storage, "25Gi", cpu, "25", memory, "25Gi", pods, "25", gpu, "25")
		CreateSubnamespace(nsD, nsB, randPrefix, false, storage, "10Gi", cpu, "10", memory, "10Gi", pods, "10", gpu, "10")
		CreateSubnamespace(nsE, nsD, randPrefix, false, storage, "2Gi", cpu, "2", memory, "2Gi", pods, "2", gpu, "2")
		CreateSubnamespace(nsF, nsE, randPrefix, false, storage, "1Gi", cpu, "1", memory, "1Gi", pods, "1", gpu, "1")

		mhName := CreateMigrationHierarchy(nsD, nsA, "")

		// verify phase is complete before labeling it
		FieldShouldContain("migrationhierarchy", "", mhName, ".status.phase", "Complete")
		FieldShouldContain("subnamespace", nsA, nsD, ".metadata.namespace", nsA)
		FieldShouldContain("namespace", "", nsD, ".metadata.labels", danav1.Parent+":"+nsA)

		FieldShouldContain("resourcequota", nsD, nsD, ".spec.hard.pods", "10")
		RunShouldNotContain(nsD, propagationTime, "kubectl get clusterresourcequota")

		LabelTestingMigrationHierarchies(mhName, randPrefix)
	})

	It("should migrate a subnamespace that does not have a CRQ under a subnamespace that does have a CRQ", func() {
		nsA := GenerateE2EName("a", testPrefix, randPrefix)
		nsB := GenerateE2EName("b", testPrefix, randPrefix)
		nsC := GenerateE2EName("c", testPrefix, randPrefix)
		nsD := GenerateE2EName("d", testPrefix, randPrefix)
		nsE := GenerateE2EName("e", testPrefix, randPrefix)
		nsF := GenerateE2EName("f", testPrefix, randPrefix)

		// create hierarchy
		CreateSubnamespace(nsB, nsRoot, randPrefix, false, storage, "25Gi", cpu, "25", memory, "25Gi", pods, "25", gpu, "25")
		CreateSubnamespace(nsD, nsB, randPrefix, false, storage, "10Gi", cpu, "10", memory, "10Gi", pods, "10", gpu, "10")
		CreateSubnamespace(nsE, nsD, randPrefix, false, storage, "2Gi", cpu, "2", memory, "2Gi", pods, "2", gpu, "2")
		CreateSubnamespace(nsF, nsE, randPrefix, false, storage, "1Gi", cpu, "1", memory, "1Gi", pods, "1", gpu, "1")
		CreateSubnamespace(nsA, nsRoot, randPrefix, false, storage, "50Gi", cpu, "50", memory, "50Gi", pods, "50", gpu, "50")
		CreateSubnamespace(nsC, nsA, randPrefix, false, storage, "25Gi", cpu, "25", memory, "25Gi", pods, "25", gpu, "25")

		mhName := CreateMigrationHierarchy(nsA, nsF, "")

		// verify phase is complete before labeling it
		FieldShouldContain("migrationhierarchy", "", mhName, ".status.phase", "Complete")
		FieldShouldContain("subnamespace", nsF, nsA, ".metadata.namespace", nsF)
		FieldShouldContain("namespace", "", nsA, ".metadata.labels", danav1.Parent+":"+nsF)

		FieldShouldNotContain("resourcequota", nsA, nsA, ".spec.hard.pods", "50")
		FieldShouldContain("clusterresourcequota", "", nsA, ".spec.quota.hard.pods", "50")

		LabelTestingMigrationHierarchies(mhName, randPrefix)
	})

	It("should migrate subnamespace with children, including the children migration", func() {
		nsA := GenerateE2EName("a", testPrefix, randPrefix)
		nsB := GenerateE2EName("b", testPrefix, randPrefix)
		nsC := GenerateE2EName("c", testPrefix, randPrefix)
		nsD := GenerateE2EName("d", testPrefix, randPrefix)
		nsE := GenerateE2EName("e", testPrefix, randPrefix)
		nsF := GenerateE2EName("f", testPrefix, randPrefix)
		nsG := GenerateE2EName("g", testPrefix, randPrefix)
		nsH := GenerateE2EName("h", testPrefix, randPrefix)

		// create hierarchy
		CreateSubnamespace(nsA, nsRoot, randPrefix, false, storage, "50Gi", cpu, "50", memory, "50Gi", pods, "50", gpu, "50")
		CreateSubnamespace(nsB, nsA, randPrefix, false, storage, "25Gi", cpu, "25", memory, "25Gi", pods, "25", gpu, "25")
		CreateSubnamespace(nsC, nsB, randPrefix, false, storage, "10Gi", cpu, "10", memory, "10Gi", pods, "10", gpu, "10")
		CreateSubnamespace(nsD, nsC, randPrefix, false, storage, "5Gi", cpu, "5", memory, "5Gi", pods, "5", gpu, "5")
		CreateSubnamespace(nsE, nsC, randPrefix, false, storage, "5Gi", cpu, "5", memory, "5Gi", pods, "5", gpu, "5")
		CreateSubnamespace(nsF, nsD, randPrefix, false, storage, "2Gi", cpu, "2", memory, "2Gi", pods, "2", gpu, "2")
		CreateSubnamespace(nsG, nsF, randPrefix, false, storage, "1Gi", cpu, "1", memory, "1Gi", pods, "1", gpu, "1")
		CreateSubnamespace(nsH, nsG, randPrefix, false, storage, "1Gi", cpu, "1", memory, "1Gi", pods, "1", gpu, "1")

		mhName := CreateMigrationHierarchy(nsF, nsE, "")

		// make sure the subnamespace was migrated and the parent was updated
		FieldShouldContain("subnamespace", nsE, nsF, ".metadata.namespace", nsE)
		FieldShouldContain("namespace", "", nsF, ".metadata.labels", danav1.Parent+":"+nsE)

		// make sure the subnamespace childrens were migrated and the parent was updated
		FieldShouldContain("subnamespace", nsF, nsG, ".metadata.namespace", nsF)
		FieldShouldContain("namespace", "", nsG, ".metadata.labels", danav1.Parent+":"+nsF)

		FieldShouldContain("subnamespace", nsG, nsH, ".metadata.namespace", nsG)
		FieldShouldContain("namespace", "", nsH, ".metadata.labels", danav1.Parent+":"+nsG)

		// verify phase is complete before labeling it
		FieldShouldContain("migrationhierarchy", "", mhName, ".status.phase", "Complete")
		LabelTestingMigrationHierarchies(mhName, randPrefix)
	})

	It("should migrate depth 5 subnamespace or lower to depth 3 subnamespace", func() {
		nsA := GenerateE2EName("a", testPrefix, randPrefix)
		nsB := GenerateE2EName("b", testPrefix, randPrefix)
		nsC := GenerateE2EName("c", testPrefix, randPrefix)
		nsD := GenerateE2EName("d", testPrefix, randPrefix)
		nsE := GenerateE2EName("e", testPrefix, randPrefix)

		// create hierarchy
		CreateSubnamespace(nsA, nsRoot, randPrefix, false, storage, "50Gi", cpu, "50", memory, "50Gi", pods, "50", gpu, "50")
		CreateSubnamespace(nsB, nsA, randPrefix, false, storage, "25Gi", cpu, "25", memory, "25Gi", pods, "25", gpu, "25")
		CreateSubnamespace(nsC, nsB, randPrefix, false, storage, "25Gi", cpu, "25", memory, "25Gi", pods, "25", gpu, "25")
		CreateSubnamespace(nsD, nsC, randPrefix, false, storage, "10Gi", cpu, "10", memory, "10Gi", pods, "10", gpu, "10")
		CreateSubnamespace(nsE, nsD, randPrefix, false, storage, "5Gi", cpu, "5", memory, "5Gi", pods, "5", gpu, "5")

		mhName := CreateMigrationHierarchy(nsE, nsC, "")

		// make sure the subnamespace was migrated and the parent was updated
		FieldShouldContain("subnamespace", nsC, nsE, ".metadata.namespace", nsC)
		FieldShouldContain("namespace", "", nsE, ".metadata.labels", danav1.Parent+":"+nsC)

		// verify phase is complete before labeling it
		FieldShouldContain("migrationhierarchy", "", mhName, ".status.phase", "Complete")
		LabelTestingMigrationHierarchies(mhName, randPrefix)
	})

	It("should not migrate a subnamespaces to a resourcepool", func() {
		nsA := GenerateE2EName("a", testPrefix, randPrefix)
		nsB := GenerateE2EName("b", testPrefix, randPrefix)
		nsC := GenerateE2EName("c", testPrefix, randPrefix)
		nsD := GenerateE2EName("d", testPrefix, randPrefix)
		nsE := GenerateE2EName("e", testPrefix, randPrefix)
		nsF := GenerateE2EName("f", testPrefix, randPrefix)
		nsG := GenerateE2EName("g", testPrefix, randPrefix)

		// create hierarchy
		CreateSubnamespace(nsA, nsRoot, randPrefix, false, storage, "50Gi", cpu, "50", memory, "50Gi", pods, "50", gpu, "50")
		CreateSubnamespace(nsB, nsA, randPrefix, false, storage, "25Gi", cpu, "25", memory, "25Gi", pods, "25", gpu, "25")
		CreateSubnamespace(nsC, nsB, randPrefix, false, storage, "10Gi", cpu, "10", memory, "10Gi", pods, "10", gpu, "10")
		CreateSubnamespace(nsD, nsC, randPrefix, false, storage, "5Gi", cpu, "5", memory, "5Gi", pods, "5", gpu, "5")
		CreateSubnamespace(nsE, nsC, randPrefix, false, storage, "5Gi", cpu, "5", memory, "5Gi", pods, "5", gpu, "5")
		CreateSubnamespace(nsF, nsE, randPrefix, true, storage, "2Gi", cpu, "2", memory, "2Gi", pods, "2", gpu, "2")
		CreateSubnamespace(nsG, nsD, randPrefix, false, storage, "2Gi", cpu, "2", memory, "2Gi", pods, "2", gpu, "2")

		// make sure the subnamespace was not migrated and the parent has not been updated
		ShouldNotCreateMigrationHierarchy(nsG, nsF)

		FieldShouldContain("subnamespace", nsD, nsG, ".metadata.namespace", nsD)
		FieldShouldContain("namespace", "", nsG, ".metadata.labels", danav1.Parent+":"+nsD)
	})

	It("should migrate resourcepool with children to subnamespace, including the children migration, they all should remain resourcepools", func() {
		nsA := GenerateE2EName("a", testPrefix, randPrefix)
		nsB := GenerateE2EName("b", testPrefix, randPrefix)
		nsC := GenerateE2EName("c", testPrefix, randPrefix)
		nsD := GenerateE2EName("d", testPrefix, randPrefix)
		nsE := GenerateE2EName("e", testPrefix, randPrefix)
		nsF := GenerateE2EName("f", testPrefix, randPrefix)
		nsG := GenerateE2EName("g", testPrefix, randPrefix)
		nsH := GenerateE2EName("h", testPrefix, randPrefix)

		// create hierarchy
		CreateSubnamespace(nsA, nsRoot, randPrefix, false, storage, "50Gi", cpu, "50", memory, "50Gi", pods, "50", gpu, "50")
		CreateSubnamespace(nsB, nsA, randPrefix, false, storage, "25Gi", cpu, "25", memory, "25Gi", pods, "25", gpu, "25")
		CreateSubnamespace(nsC, nsB, randPrefix, false, storage, "10Gi", cpu, "10", memory, "10Gi", pods, "10", gpu, "10")
		CreateSubnamespace(nsD, nsC, randPrefix, false, storage, "5Gi", cpu, "5", memory, "5Gi", pods, "5", gpu, "5")
		CreateSubnamespace(nsE, nsC, randPrefix, false, storage, "5Gi", cpu, "5", memory, "5Gi", pods, "5", gpu, "5")
		CreateSubnamespace(nsF, nsD, randPrefix, true, storage, "2Gi", cpu, "2", memory, "2Gi", pods, "2", gpu, "2")
		CreateSubnamespace(nsG, nsF, randPrefix, true)
		CreateSubnamespace(nsH, nsG, randPrefix, true)

		mhName := CreateMigrationHierarchy(nsF, nsE, "")

		// make sure the subnamespace was migrated and the parent was updated
		FieldShouldContain("subnamespace", nsE, nsF, ".metadata.namespace", nsE)
		FieldShouldContain("namespace", "", nsF, ".metadata.labels", danav1.Parent+":"+nsE)
		FieldShouldContain("subnamespace", nsE, nsF, ".metadata.labels", danav1.ResourcePool+":true")

		// make sure the subnamespace childrens were migrated and the parent was updated
		FieldShouldContain("subnamespace", nsF, nsG, ".metadata.namespace", nsF)
		FieldShouldContain("namespace", "", nsG, ".metadata.labels", danav1.Parent+":"+nsF)
		FieldShouldContain("subnamespace", nsF, nsG, ".metadata.labels", danav1.ResourcePool+":true")

		FieldShouldContain("subnamespace", nsG, nsH, ".metadata.namespace", nsG)
		FieldShouldContain("namespace", "", nsH, ".metadata.labels", danav1.Parent+":"+nsG)
		FieldShouldContain("subnamespace", nsG, nsH, ".metadata.labels", danav1.ResourcePool+":true")

		// verify phase is complete before labeling it
		FieldShouldContain("migrationhierarchy", "", mhName, ".status.phase", "Complete")
		LabelTestingMigrationHierarchies(mhName, randPrefix)
	})

	It("should migrate resourcepool with children to resourcepool, including the children migration", func() {
		nsA := GenerateE2EName("a", testPrefix, randPrefix)
		nsB := GenerateE2EName("b", testPrefix, randPrefix)
		nsC := GenerateE2EName("c", testPrefix, randPrefix)
		nsD := GenerateE2EName("d", testPrefix, randPrefix)
		nsE := GenerateE2EName("e", testPrefix, randPrefix)
		nsF := GenerateE2EName("f", testPrefix, randPrefix)
		nsG := GenerateE2EName("g", testPrefix, randPrefix)
		nsH := GenerateE2EName("h", testPrefix, randPrefix)
		nsI := GenerateE2EName("i", testPrefix, randPrefix)

		// create hierarchy
		CreateSubnamespace(nsA, nsRoot, randPrefix, false, storage, "50Gi", cpu, "50", memory, "50Gi", pods, "50", gpu, "50")
		CreateSubnamespace(nsB, nsA, randPrefix, false, storage, "25Gi", cpu, "25", memory, "25Gi", pods, "25", gpu, "25")
		CreateSubnamespace(nsC, nsB, randPrefix, false, storage, "10Gi", cpu, "10", memory, "10Gi", pods, "10", gpu, "10")
		CreateSubnamespace(nsD, nsC, randPrefix, false, storage, "5Gi", cpu, "5", memory, "5Gi", pods, "5", gpu, "5")
		CreateSubnamespace(nsE, nsC, randPrefix, false, storage, "5Gi", cpu, "5", memory, "5Gi", pods, "5", gpu, "5")
		CreateSubnamespace(nsI, nsE, randPrefix, true, storage, "2Gi", cpu, "2", memory, "2Gi", pods, "2", gpu, "5")
		CreateSubnamespace(nsF, nsD, randPrefix, true, storage, "2Gi", cpu, "2", memory, "2Gi", pods, "2", gpu, "2")
		CreateSubnamespace(nsG, nsF, randPrefix, true)
		CreateSubnamespace(nsH, nsG, randPrefix, true)

		mhName := CreateMigrationHierarchy(nsF, nsI, "")

		// make sure the subnamespace was migrated and the parent was updated
		FieldShouldContain("subnamespace", nsI, nsF, ".metadata.namespace", nsI)
		FieldShouldContain("namespace", "", nsF, ".metadata.labels", danav1.Parent+":"+nsI)
		FieldShouldContain("subnamespace", nsI, nsF, ".metadata.labels", danav1.ResourcePool+":true")

		// make sure the subnamespace childrens were migrated and the parent was updated
		FieldShouldContain("subnamespace", nsF, nsG, ".metadata.namespace", nsF)
		FieldShouldContain("namespace", "", nsG, ".metadata.labels", danav1.Parent+":"+nsF)
		FieldShouldContain("subnamespace", nsF, nsG, ".metadata.labels", danav1.ResourcePool+":true")

		FieldShouldContain("subnamespace", nsG, nsH, ".metadata.namespace", nsG)
		FieldShouldContain("namespace", "", nsH, ".metadata.labels", danav1.Parent+":"+nsG)
		FieldShouldContain("subnamespace", nsG, nsH, ".metadata.labels", danav1.ResourcePool+":true")

		// verify phase is complete before labeling it
		FieldShouldContain("migrationhierarchy", "", mhName, ".status.phase", "Complete")
		LabelTestingMigrationHierarchies(mhName, randPrefix)
	})

	It("should not migrate subnamespaces to a different secondary root", func() {
		nsA := GenerateE2EName("a", testPrefix, randPrefix)
		nsB := GenerateE2EName("b", testPrefix, randPrefix)
		nsC := GenerateE2EName("c", testPrefix, randPrefix)
		nsD := GenerateE2EName("d", testPrefix, randPrefix)

		// make sure the subnamespace was migrated and the parent was updated
		CreateSubnamespace(nsA, nsRoot, randPrefix, false, storage, "50Gi", cpu, "50", memory, "50Gi", pods, "50", gpu, "50")
		CreateSubnamespace(nsB, nsRoot, randPrefix, false, storage, "50Gi", cpu, "50", memory, "50Gi", pods, "50", gpu, "50")
		CreateSubnamespace(nsC, nsA, randPrefix, true, storage, "25Gi", cpu, "25", memory, "25Gi", pods, "25", gpu, "25")
		CreateSubnamespace(nsD, nsB, randPrefix, true, storage, "25Gi", cpu, "25", memory, "25Gi", pods, "25", gpu, "25")

		// add secondary root annotation
		AnnotateNSSecondaryRoot(nsA)
		AnnotateNSSecondaryRoot(nsB)

		// make sure the subnamespace was not migrated and the parent was updated
		ShouldNotCreateMigrationHierarchy(nsD, nsA)

		FieldShouldContain("subnamespace", nsB, nsD, ".metadata.namespace", nsB)
		FieldShouldContain("namespace", "", nsD, ".metadata.labels", danav1.Parent+":"+nsB)
	})

	It("should not migrate a subnamespace to its existing parent", func() {
		nsA := GenerateE2EName("a", testPrefix, randPrefix)
		nsB := GenerateE2EName("b", testPrefix, randPrefix)
		nsC := GenerateE2EName("c", testPrefix, randPrefix)

		// create hierarchy
		CreateSubnamespace(nsA, nsRoot, randPrefix, false, storage, "50Gi", cpu, "50", memory, "50Gi", pods, "50", gpu, "50")
		CreateSubnamespace(nsB, nsA, randPrefix, false, storage, "25Gi", cpu, "25", memory, "25Gi", pods, "25", gpu, "25")
		CreateSubnamespace(nsC, nsB, randPrefix, false, storage, "10Gi", cpu, "10", memory, "10Gi", pods, "10", gpu, "10")

		// make sure the subnamespace was not migrated and the parent has not been updated
		ShouldNotCreateMigrationHierarchy(nsC, nsB)
	})

	It("should migrate resources together with the subnamespaces", func() {
		nsA := GenerateE2EName("a", testPrefix, randPrefix)
		nsB := GenerateE2EName("b", testPrefix, randPrefix)
		nsC := GenerateE2EName("c", testPrefix, randPrefix)
		nsD := GenerateE2EName("d", testPrefix, randPrefix)
		nsE := GenerateE2EName("e", testPrefix, randPrefix)
		nsF := GenerateE2EName("f", testPrefix, randPrefix)
		nsG := GenerateE2EName("g", testPrefix, randPrefix)
		nsH := GenerateE2EName("h", testPrefix, randPrefix)
		nsI := GenerateE2EName("i", testPrefix, randPrefix)

		// create hierarchy
		CreateSubnamespace(nsA, nsRoot, randPrefix, false, storage, "50Gi", cpu, "50", memory, "50Gi", pods, "50", gpu, "50")
		CreateSubnamespace(nsB, nsA, randPrefix, false, storage, "25Gi", cpu, "25", memory, "25Gi", pods, "25", gpu, "25")
		CreateSubnamespace(nsH, nsA, randPrefix, false, storage, "25Gi", cpu, "25", memory, "25Gi", pods, "25", gpu, "25")
		CreateSubnamespace(nsI, nsH, randPrefix, false, storage, "1Gi", cpu, "1", memory, "1Gi", pods, "1", gpu, "1")
		CreateSubnamespace(nsC, nsB, randPrefix, false, storage, "10Gi", cpu, "10", memory, "10Gi", pods, "10", gpu, "10")
		CreateSubnamespace(nsD, nsC, randPrefix, false, storage, "5Gi", cpu, "5", memory, "5Gi", pods, "5", gpu, "5")
		CreateSubnamespace(nsE, nsC, randPrefix, false, storage, "1Gi", cpu, "1", memory, "1Gi", pods, "1", gpu, "1")
		CreateSubnamespace(nsF, nsD, randPrefix, false, storage, "2Gi", cpu, "2", memory, "2Gi", pods, "2", gpu, "2")
		CreateSubnamespace(nsG, nsF, randPrefix, true, storage, "2Gi", cpu, "2", memory, "2Gi", pods, "2", gpu, "2")

		mhName := CreateMigrationHierarchy(nsD, nsI, "")
		FieldShouldContain("subnamespace", nsH, nsI, ".spec.resourcequota.hard."+cpu, "6")
		FieldShouldContain("subnamespace", nsH, nsI, ".spec.resourcequota.hard."+memory, "6Gi")
		FieldShouldContain("subnamespace", nsH, nsI, ".spec.resourcequota.hard."+pods, "6")

		FieldShouldContain("subnamespace", nsB, nsC, ".spec.resourcequota.hard."+cpu, "5")
		FieldShouldContain("subnamespace", nsB, nsC, ".spec.resourcequota.hard."+memory, "5Gi")
		FieldShouldContain("subnamespace", nsB, nsC, ".spec.resourcequota.hard."+pods, "5")

		// verify phase is complete before labeling it
		FieldShouldContain("migrationhierarchy", "", mhName, ".status.phase", "Complete")
		LabelTestingMigrationHierarchies(mhName, randPrefix)
	})

	It("should not migrate a non-Upper ResourcePool to a Subnamespace", func() {
		nsA := GenerateE2EName("a", testPrefix, randPrefix)
		nsB := GenerateE2EName("b", testPrefix, randPrefix)
		nsC := GenerateE2EName("c", testPrefix, randPrefix)
		nsD := GenerateE2EName("d", testPrefix, randPrefix)
		nsE := GenerateE2EName("e", testPrefix, randPrefix)

		// create hierarchy
		CreateSubnamespace(nsA, nsRoot, randPrefix, false, storage, "50Gi", cpu, "50", memory, "50Gi", pods, "50", gpu, "50")
		CreateSubnamespace(nsB, nsA, randPrefix, false, storage, "25Gi", cpu, "25", memory, "25Gi", pods, "25", gpu, "25")
		CreateSubnamespace(nsC, nsB, randPrefix, true, storage, "10Gi", cpu, "10", memory, "10Gi", pods, "10", gpu, "10")
		CreateSubnamespace(nsD, nsC, randPrefix, true)
		CreateSubnamespace(nsE, nsA, randPrefix, false, storage, "25Gi", cpu, "25", memory, "25Gi", pods, "25", gpu, "25")

		// make sure the subnamespace was not migrated and the parent has not been updated
		ShouldNotCreateMigrationHierarchy(nsD, nsE)
	})

	It("should not migrate a Subnamespace to be under itself", func() {
		nsA := GenerateE2EName("a", testPrefix, randPrefix)
		nsB := GenerateE2EName("b", testPrefix, randPrefix)
		nsC := GenerateE2EName("c", testPrefix, randPrefix)

		// create hierarchy
		CreateSubnamespace(nsA, nsRoot, randPrefix, false, storage, "50Gi", cpu, "50", memory, "50Gi", pods, "50", gpu, "50")
		CreateSubnamespace(nsB, nsA, randPrefix, false, storage, "25Gi", cpu, "25", memory, "25Gi", pods, "25", gpu, "25")
		CreateSubnamespace(nsC, nsB, randPrefix, true, storage, "10Gi", cpu, "10", memory, "10Gi", pods, "10", gpu, "10")

		// make sure the subnamespace was not migrated and the parent has not been updated
		ShouldNotCreateMigrationHierarchy(nsB, nsB)
	})

	It("should not migrate a Subnamespace to be under its own descendant and create a loop", func() {
		nsA := GenerateE2EName("a", testPrefix, randPrefix)
		nsB := GenerateE2EName("b", testPrefix, randPrefix)
		nsC := GenerateE2EName("c", testPrefix, randPrefix)

		// create hierarchy
		CreateSubnamespace(nsA, nsRoot, randPrefix, false, storage, "50Gi", cpu, "50", memory, "50Gi", pods, "50", gpu, "50")
		CreateSubnamespace(nsB, nsA, randPrefix, false, storage, "25Gi", cpu, "25", memory, "25Gi", pods, "25", gpu, "25")
		CreateSubnamespace(nsC, nsB, randPrefix, true, storage, "10Gi", cpu, "10", memory, "10Gi", pods, "10", gpu, "10")

		// make sure the subnamespace was not migrated and the parent has not been updated
		ShouldNotCreateMigrationHierarchy(nsA, nsC)
	})

})
