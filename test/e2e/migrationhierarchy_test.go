package e2e

import (
	danav1 "github.com/dana-team/hns/api/v1"
	. "github.com/dana-team/hns/pkg/testutils"
	. "github.com/onsi/ginkgo/v2"
)

var _ = Describe("MigrationHierarchy", func() {
	nsRoot := GenerateE2EName("root")

	BeforeEach(func() {
		CleanupTestNamespaces()
		CleanupTestMigrationHierarchies()

		// set up root namespace
		CreateRootNS(nsRoot, rqDepth)
		CreateResourceQuota(nsRoot, nsRoot, storage, "100Gi", cpu, "100", memory, "100Gi", pods, "100", gpu, "100")
	})

	AfterEach(func() {
		CleanupTestNamespaces()
		CleanupTestMigrationHierarchies()
	})

	It("should migrate subnamespace that have a CRQ or its direct parent have a CRQ,", func() {
		nsA := GenerateE2EName("a")
		nsB := GenerateE2EName("b")
		nsC := GenerateE2EName("c")
		nsD := GenerateE2EName("d")
		nsE := GenerateE2EName("e")
		nsF := GenerateE2EName("f")

		// create hierarchy
		CreateSubnamespace(nsA, nsRoot, false, storage, "50Gi", cpu, "50", memory, "50Gi", pods, "50", gpu, "50")
		CreateSubnamespace(nsB, nsA, false, storage, "25Gi", cpu, "25", memory, "25Gi", pods, "25", gpu, "25")
		CreateSubnamespace(nsC, nsB, false, storage, "10Gi", cpu, "10", memory, "10Gi", pods, "10", gpu, "10")
		CreateSubnamespace(nsD, nsC, false, storage, "5Gi", cpu, "5", memory, "5Gi", pods, "5", gpu, "5")
		CreateSubnamespace(nsE, nsC, false, storage, "5Gi", cpu, "5", memory, "5Gi", pods, "5", gpu, "5")
		CreateSubnamespace(nsF, nsD, false, storage, "1Gi", cpu, "1", memory, "1Gi", pods, "1", gpu, "1")

		CreateMigrationHierarchy(nsF, nsE)

		// make sure the subnamespace was migrated and the parent was updated
		FieldShouldContain("subnamespace", nsE, nsF, ".metadata.namespace", nsE)
		FieldShouldContain("namespace", "", nsF, ".metadata.labels", danav1.Parent+":"+nsE)

		// add test label to the migrated ns
		LabelTestingNs(nsF)
	})

	It("should not migrate subnamespace that doesn't have a CRQ and their direct parent doesn't have a CRQ,", func() {
		nsA := GenerateE2EName("a")
		nsB := GenerateE2EName("b")
		nsC := GenerateE2EName("c")
		nsD := GenerateE2EName("d")

		// create hierarchy
		CreateSubnamespace(nsA, nsRoot, false, storage, "50Gi", cpu, "50", memory, "50Gi", pods, "50", gpu, "50")
		CreateSubnamespace(nsB, nsA, false, storage, "25Gi", cpu, "25", memory, "25Gi", pods, "25", gpu, "25")
		CreateSubnamespace(nsC, nsA, false, storage, "25Gi", cpu, "25", memory, "25Gi", pods, "25", gpu, "25")
		CreateSubnamespace(nsD, nsB, false, storage, "10Gi", cpu, "10", memory, "10Gi", pods, "10", gpu, "10")

		// make sure the subnamespace was not migrated and the parent has not been updated
		ShouldNotCreateMigrationHierarchy(nsD, nsC)
		FieldShouldContain("subnamespace", nsB, nsD, ".metadata.namespace", nsB)
		FieldShouldContain("namespace", "", nsD, ".metadata.labels", danav1.Parent+":"+nsB)
	})

	It("should migrate subnamespace with children, including the children migration", func() {
		nsA := GenerateE2EName("a")
		nsB := GenerateE2EName("b")
		nsC := GenerateE2EName("c")
		nsD := GenerateE2EName("d")
		nsE := GenerateE2EName("e")
		nsF := GenerateE2EName("f")
		nsG := GenerateE2EName("g")
		nsH := GenerateE2EName("h")

		// create hierarchy
		CreateSubnamespace(nsA, nsRoot, false, storage, "50Gi", cpu, "50", memory, "50Gi", pods, "50", gpu, "50")
		CreateSubnamespace(nsB, nsA, false, storage, "25Gi", cpu, "25", memory, "25Gi", pods, "25", gpu, "25")
		CreateSubnamespace(nsC, nsB, false, storage, "10Gi", cpu, "10", memory, "10Gi", pods, "10", gpu, "10")
		CreateSubnamespace(nsD, nsC, false, storage, "5Gi", cpu, "5", memory, "5Gi", pods, "5", gpu, "5")
		CreateSubnamespace(nsE, nsC, false, storage, "5Gi", cpu, "5", memory, "5Gi", pods, "5", gpu, "5")
		CreateSubnamespace(nsF, nsD, false, storage, "2Gi", cpu, "2", memory, "2Gi", pods, "2", gpu, "2")
		CreateSubnamespace(nsG, nsF, false, storage, "1Gi", cpu, "1", memory, "1Gi", pods, "1", gpu, "1")
		CreateSubnamespace(nsH, nsG, false, storage, "1Gi", cpu, "1", memory, "1Gi", pods, "1", gpu, "1")

		CreateMigrationHierarchy(nsF, nsE)

		// make sure the subnamespace was migrated and the parent was updated
		FieldShouldContain("subnamespace", nsE, nsF, ".metadata.namespace", nsE)
		FieldShouldContain("namespace", "", nsF, ".metadata.labels", danav1.Parent+":"+nsE)

		// make sure the subnamespace childrens were migrated and the parent was updated
		FieldShouldContain("subnamespace", nsF, nsG, ".metadata.namespace", nsF)
		FieldShouldContain("namespace", "", nsG, ".metadata.labels", danav1.Parent+":"+nsF)

		FieldShouldContain("subnamespace", nsG, nsH, ".metadata.namespace", nsG)
		FieldShouldContain("namespace", "", nsH, ".metadata.labels", danav1.Parent+":"+nsG)

		// add test label to the migrated ns
		LabelTestingNs(nsF)
		LabelTestingNs(nsG)
		LabelTestingNs(nsH)
	})

	It("should not migrate an empty resourcepool to be under subnamespace, should get error phase", func() {
		nsA := GenerateE2EName("a")
		nsB := GenerateE2EName("b")
		nsC := GenerateE2EName("c")
		nsD := GenerateE2EName("d")
		nsE := GenerateE2EName("e")
		nsF := GenerateE2EName("f")
		nsG := GenerateE2EName("g")

		// create hierarchy
		CreateSubnamespace(nsA, nsRoot, false, storage, "50Gi", cpu, "50", memory, "50Gi", pods, "50", gpu, "50")
		CreateSubnamespace(nsB, nsA, false, storage, "25Gi", cpu, "25", memory, "25Gi", pods, "25", gpu, "25")
		CreateSubnamespace(nsC, nsB, false, storage, "10Gi", cpu, "10", memory, "10Gi", pods, "10", gpu, "10")
		CreateSubnamespace(nsD, nsC, false, storage, "5Gi", cpu, "5", memory, "5Gi", pods, "5", gpu, "5")
		CreateSubnamespace(nsE, nsC, false, storage, "5Gi", cpu, "5", memory, "5Gi", pods, "5", gpu, "5")
		CreateSubnamespace(nsF, nsD, true, storage, "2Gi", cpu, "2", memory, "2Gi", pods, "2", gpu, "2")
		CreateSubnamespace(nsG, nsF, true)

		// create the migration hierarchy and return name to mhname
		mhname := CreateMigrationHierarchy(nsG, nsE)

		// make sure the migration hierarchy was created in error phase and did not migrate the subnamespace
		FieldShouldContain("subnamespace", nsF, nsG, ".metadata.namespace", nsF)
		FieldShouldContain("namespace", "", nsG, ".metadata.labels", danav1.Parent+":"+nsF)
		FieldShouldContain("migrationhierarchy", "", mhname, ".status.phase", "Error")
	})

	It("should migrate depth 5 subnamespace or lower to depth 2 subnamespace", func() {
		nsA := GenerateE2EName("a")
		nsB := GenerateE2EName("b")
		nsC := GenerateE2EName("c")
		nsD := GenerateE2EName("d")
		nsE := GenerateE2EName("e")

		// create hierarchy
		CreateSubnamespace(nsA, nsRoot, false, storage, "50Gi", cpu, "50", memory, "50Gi", pods, "50", gpu, "50")
		CreateSubnamespace(nsB, nsA, false, storage, "25Gi", cpu, "25", memory, "25Gi", pods, "25", gpu, "25")
		CreateSubnamespace(nsC, nsA, false, storage, "25Gi", cpu, "25", memory, "25Gi", pods, "25", gpu, "25")
		CreateSubnamespace(nsD, nsB, false, storage, "10Gi", cpu, "10", memory, "10Gi", pods, "10", gpu, "10")
		CreateSubnamespace(nsE, nsD, false, storage, "5Gi", cpu, "5", memory, "5Gi", pods, "5", gpu, "5")

		CreateMigrationHierarchy(nsE, nsC)

		// make sure the subnamespace was migrated and the parent was updated
		FieldShouldContain("subnamespace", nsC, nsE, ".metadata.namespace", nsC)
		FieldShouldContain("namespace", "", nsE, ".metadata.labels", danav1.Parent+":"+nsC)

		// add test label to the migrated ns
		LabelTestingNs(nsE)

	})

	It("should not migrate a subnamespaces to a resourcepool", func() {
		nsA := GenerateE2EName("a")
		nsB := GenerateE2EName("b")
		nsC := GenerateE2EName("c")
		nsD := GenerateE2EName("d")
		nsE := GenerateE2EName("e")
		nsF := GenerateE2EName("f")
		nsG := GenerateE2EName("g")

		// create hierarchy
		CreateSubnamespace(nsA, nsRoot, false, storage, "50Gi", cpu, "50", memory, "50Gi", pods, "50", gpu, "50")
		CreateSubnamespace(nsB, nsA, false, storage, "25Gi", cpu, "25", memory, "25Gi", pods, "25", gpu, "25")
		CreateSubnamespace(nsC, nsB, false, storage, "10Gi", cpu, "10", memory, "10Gi", pods, "10", gpu, "10")
		CreateSubnamespace(nsD, nsC, false, storage, "5Gi", cpu, "5", memory, "5Gi", pods, "5", gpu, "5")
		CreateSubnamespace(nsE, nsC, false, storage, "5Gi", cpu, "5", memory, "5Gi", pods, "5", gpu, "5")
		CreateSubnamespace(nsF, nsE, true, storage, "2Gi", cpu, "2", memory, "2Gi", pods, "2", gpu, "2")
		CreateSubnamespace(nsG, nsD, false, storage, "2Gi", cpu, "2", memory, "2Gi", pods, "2", gpu, "2")

		// make sure the subnamespace was not migrated and the parent has not been updated
		ShouldNotCreateMigrationHierarchy(nsG, nsF)
		FieldShouldContain("subnamespace", nsD, nsG, ".metadata.namespace", nsD)
		FieldShouldContain("namespace", "", nsG, ".metadata.labels", danav1.Parent+":"+nsD)
	})

	It("should migrate resourcepool with children to subnamespace, including the children migration. they all should remain resourcepools", func() {
		nsA := GenerateE2EName("a")
		nsB := GenerateE2EName("b")
		nsC := GenerateE2EName("c")
		nsD := GenerateE2EName("d")
		nsE := GenerateE2EName("e")
		nsF := GenerateE2EName("f")
		nsG := GenerateE2EName("g")
		nsH := GenerateE2EName("h")

		// create hierarchy
		CreateSubnamespace(nsA, nsRoot, false, storage, "50Gi", cpu, "50", memory, "50Gi", pods, "50", gpu, "50")
		CreateSubnamespace(nsB, nsA, false, storage, "25Gi", cpu, "25", memory, "25Gi", pods, "25", gpu, "25")
		CreateSubnamespace(nsC, nsB, false, storage, "10Gi", cpu, "10", memory, "10Gi", pods, "10", gpu, "10")
		CreateSubnamespace(nsD, nsC, false, storage, "5Gi", cpu, "5", memory, "5Gi", pods, "5", gpu, "5")
		CreateSubnamespace(nsE, nsC, false, storage, "5Gi", cpu, "5", memory, "5Gi", pods, "5", gpu, "5")
		CreateSubnamespace(nsF, nsD, true, storage, "2Gi", cpu, "2", memory, "2Gi", pods, "2", gpu, "2")
		CreateSubnamespace(nsG, nsF, true)
		CreateSubnamespace(nsH, nsG, true)

		CreateMigrationHierarchy(nsF, nsE)

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

		// add test label to the migrated ns
		LabelTestingNs(nsF)
		LabelTestingNs(nsG)
		LabelTestingNs(nsH)
	})

	It("should migrate resourcepool with children to resourcepool, including the children migration", func() {
		nsA := GenerateE2EName("a")
		nsB := GenerateE2EName("b")
		nsC := GenerateE2EName("c")
		nsD := GenerateE2EName("d")
		nsE := GenerateE2EName("e")
		nsF := GenerateE2EName("f")
		nsG := GenerateE2EName("g")
		nsH := GenerateE2EName("h")
		nsI := GenerateE2EName("i")

		// create hierarchy
		CreateSubnamespace(nsA, nsRoot, false, storage, "50Gi", cpu, "50", memory, "50Gi", pods, "50", gpu, "50")
		CreateSubnamespace(nsB, nsA, false, storage, "25Gi", cpu, "25", memory, "25Gi", pods, "25", gpu, "25")
		CreateSubnamespace(nsC, nsB, false, storage, "10Gi", cpu, "10", memory, "10Gi", pods, "10", gpu, "10")
		CreateSubnamespace(nsD, nsC, false, storage, "5Gi", cpu, "5", memory, "5Gi", pods, "5", gpu, "5")
		CreateSubnamespace(nsE, nsC, false, storage, "5Gi", cpu, "5", memory, "5Gi", pods, "5", gpu, "5")
		CreateSubnamespace(nsI, nsE, true, storage, "2Gi", cpu, "2", memory, "2Gi", pods, "2", gpu, "5")
		CreateSubnamespace(nsF, nsD, true, storage, "2Gi", cpu, "2", memory, "2Gi", pods, "2", gpu, "2")
		CreateSubnamespace(nsG, nsF, true)
		CreateSubnamespace(nsH, nsG, true)

		CreateMigrationHierarchy(nsF, nsI)

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

		// add test label to the migrated ns
		LabelTestingNs(nsF)
		LabelTestingNs(nsG)
		LabelTestingNs(nsH)
	})

	It("should not migrate subnamespaces to a different secondary root", func() {
		nsA := GenerateE2EName("a")
		nsB := GenerateE2EName("b")
		nsC := GenerateE2EName("c")
		nsD := GenerateE2EName("d")

		// make sure the subnamespace was migrated and the parent was updated
		CreateSubnamespace(nsA, nsRoot, false, storage, "50Gi", cpu, "50", memory, "50Gi", pods, "50", gpu, "50")
		CreateSubnamespace(nsB, nsRoot, false, storage, "50Gi", cpu, "50", memory, "50Gi", pods, "50", gpu, "50")
		CreateSubnamespace(nsC, nsA, true, storage, "25Gi", cpu, "25", memory, "25Gi", pods, "25", gpu, "25")
		CreateSubnamespace(nsD, nsB, true, storage, "25Gi", cpu, "25", memory, "25Gi", pods, "25", gpu, "25")

		// add secondary root annotation
		AnnotateNSSecondaryRoot(nsA)
		AnnotateNSSecondaryRoot(nsB)

		// make sure the subnamespace was not migrated and the parent was updated
		ShouldNotCreateMigrationHierarchy(nsD, nsA)
		FieldShouldContain("subnamespace", nsB, nsD, ".metadata.namespace", nsB)
		FieldShouldContain("namespace", "", nsD, ".metadata.labels", danav1.Parent+":"+nsB)
	})

	It("should not migrate a subnamespace/resourcepool to a subnamespace without enough resources", func() {
		nsA := GenerateE2EName("a")
		nsB := GenerateE2EName("b")
		nsC := GenerateE2EName("c")
		nsD := GenerateE2EName("d")
		nsE := GenerateE2EName("e")
		nsF := GenerateE2EName("f")
		nsH := GenerateE2EName("h")

		// create hierarchy
		CreateSubnamespace(nsA, nsRoot, false, storage, "50Gi", cpu, "50", memory, "50Gi", pods, "50", gpu, "50")
		CreateSubnamespace(nsB, nsA, false, storage, "25Gi", cpu, "25", memory, "25Gi", pods, "25", gpu, "25")
		CreateSubnamespace(nsC, nsB, false, storage, "10Gi", cpu, "10", memory, "10Gi", pods, "10", gpu, "10")
		CreateSubnamespace(nsD, nsC, false, storage, "5Gi", cpu, "5", memory, "5Gi", pods, "5", gpu, "5")
		CreateSubnamespace(nsE, nsC, false, storage, "1Gi", cpu, "1", memory, "1Gi", pods, "1", gpu, "1")
		CreateSubnamespace(nsF, nsD, false, storage, "2Gi", cpu, "2", memory, "2Gi", pods, "2", gpu, "2")
		CreateSubnamespace(nsH, nsF, true, storage, "2Gi", cpu, "2", memory, "2Gi", pods, "2", gpu, "2")

		// make sure the subnamespace was not migrated and the parent has not been updated
		ShouldNotCreateMigrationHierarchy(nsF, nsE)
		FieldShouldContain("subnamespace", nsD, nsF, ".metadata.namespace", nsD)
		FieldShouldContain("namespace", "", nsF, ".metadata.labels", danav1.Parent+":"+nsD)

		// make sure the subnamespace was not migrated and the parent has not been updated
		ShouldNotCreateMigrationHierarchy(nsH, nsE)
		FieldShouldContain("subnamespace", nsF, nsH, ".metadata.namespace", nsF)
		FieldShouldContain("namespace", "", nsH, ".metadata.labels", danav1.Parent+":"+nsF)
	})
})
