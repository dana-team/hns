package e2e_tests

import (
	danav1 "github.com/dana-team/hns/api/v1"
	. "github.com/dana-team/hns/test/testutils"
	. "github.com/onsi/ginkgo/v2"
)

var _ = Describe("Subnamespaces", func() {
	testPrefix := "sns-test"
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

	It("should create and delete a subnamespace", func() {
		nsA := GenerateE2EName("a", testPrefix, randPrefix)
		nsB := GenerateE2EName("b", testPrefix, randPrefix)
		nsC := GenerateE2EName("c", testPrefix, randPrefix)

		By("creating a subnamespace and a resourcequota for a subnamespace in a high hierarchy")
		CreateSubnamespace(nsA, nsRoot, randPrefix, false, storage, "50Gi", cpu, "50", memory, "50Gi", pods, "50", gpu, "50")
		CreateSubnamespace(nsB, nsA, randPrefix, false, storage, "25Gi", cpu, "25", memory, "25Gi", pods, "25", gpu, "25")

		// verify
		FieldShouldContain("resourcequota", nsA, nsA, ".metadata.name", nsA)
		FieldShouldContain("resourcequota", nsB, nsB, ".metadata.name", nsB)

		By("creating a subnamespace and a clusterresourcequota for a subnamespace in a lower hierarchy")
		CreateSubnamespace(nsC, nsB, randPrefix, false, storage, "10Gi", cpu, "10", memory, "10Gi", pods, "10", gpu, "10")
		FieldShouldContain("clusterresourcequota", "", nsC, ".metadata.name", nsC)

		// delete namespace
		MustRun("kubectl delete namespace", nsC, "-n", nsB)
	})

	It("should create a subnamespace and namespace with the needed labels and annotations", func() {
		nsA := GenerateE2EName("a", testPrefix, randPrefix)
		nsB := GenerateE2EName("b", testPrefix, randPrefix)
		nsC := GenerateE2EName("c", testPrefix, randPrefix)

		CreateSubnamespace(nsA, nsRoot, randPrefix, false, storage, "50Gi", cpu, "50", memory, "50Gi", pods, "50", gpu, "50")
		CreateSubnamespace(nsB, nsA, randPrefix, false, storage, "25Gi", cpu, "25", memory, "25Gi", pods, "25", gpu, "25")
		CreateSubnamespace(nsC, nsB, randPrefix, false, storage, "10Gi", cpu, "10", memory, "10Gi", pods, "10", gpu, "10")

		// verify namespace labels
		FieldShouldContain("namespace", "", nsA, ".metadata.labels", danav1.Role+":"+danav1.NoRole)
		FieldShouldContain("namespace", "", nsB, ".metadata.labels", danav1.Role+":"+danav1.NoRole)
		FieldShouldContain("namespace", "", nsC, ".metadata.labels", danav1.Role+":"+danav1.Leaf)

		FieldShouldContain("namespace", "", nsC, ".metadata.labels", danav1.Hns+":true")
		FieldShouldContain("namespace", "", nsC, ".metadata.labels", danav1.Parent+":"+nsB)
		FieldShouldContain("namespace", "", nsC, ".metadata.labels", danav1.ResourcePool+":false")

		FieldShouldContain("namespace", "", nsC, ".metadata.labels", nsA+":true")
		FieldShouldContain("namespace", "", nsC, ".metadata.labels", nsB+":true")
		FieldShouldContain("namespace", "", nsC, ".metadata.labels", nsC+":true")

		// verify namespace annotations
		FieldShouldContain("namespace", "", nsC, ".metadata.annotations", danav1.Role+":"+danav1.Leaf)
		FieldShouldContain("namespace", "", nsC, ".metadata.annotations",
			danav1.DisplayName+":"+nsRoot+"/"+nsA+"/"+nsB+"/"+nsC)
		FieldShouldContain("namespace", "", nsC, ".metadata.annotations",
			danav1.DisplayName+":"+nsRoot+"/"+nsA+"/"+nsB+"/"+nsC)

		FieldShouldContain("namespace", "", nsC, ".metadata.annotations", danav1.CrqSelector+"-0:"+nsRoot)
		FieldShouldContain("namespace", "", nsC, ".metadata.annotations", danav1.CrqSelector+"-1:"+nsA)
		FieldShouldContain("namespace", "", nsC, ".metadata.annotations", danav1.CrqSelector+"-2:"+nsB)
		FieldShouldContain("namespace", "", nsC, ".metadata.annotations", danav1.CrqSelector+"-3:"+nsC)

		FieldShouldContain("namespace", "", nsC, ".metadata.annotations", danav1.Depth+":3")
		FieldShouldContain("namespace", "", nsC, ".metadata.annotations", danav1.SnsPointer+":"+nsC)

		// verify subnamespace labels
		FieldShouldContain("subnamespace", nsB, nsC, ".metadata.labels", danav1.ResourcePool+":false")

		// verify subnamespace annotations
		FieldShouldContain("subnamespace", nsRoot, nsA, ".metadata.annotations", danav1.IsRq+":"+danav1.True)
		FieldShouldContain("subnamespace", nsA, nsB, ".metadata.annotations", danav1.IsRq+":"+danav1.True)
		FieldShouldContain("subnamespace", nsB, nsC, ".metadata.annotations", danav1.CrqPointer+":"+nsC)
		FieldShouldContain("subnamespace", nsB, nsC, ".metadata.annotations", danav1.IsRq+":"+danav1.False)
		FieldShouldContain("subnamespace", nsB, nsC, ".metadata.annotations", danav1.IsUpperRp+":"+danav1.False)
		FieldShouldContain("subnamespace", nsB, nsC, ".metadata.annotations",
			danav1.DisplayName+":"+nsRoot+"/"+nsA+"/"+nsB+"/"+nsC)
	})

	It("should update the role of a subnamespace after it creates children", func() {
		nsA := GenerateE2EName("a", testPrefix, randPrefix)
		nsB := GenerateE2EName("b", testPrefix, randPrefix)

		CreateSubnamespace(nsA, nsRoot, randPrefix, false, storage, "50Gi", cpu, "50", memory, "50Gi", pods, "50", gpu, "50")

		// verify before child is created
		FieldShouldContain("namespace", "", nsA, ".metadata.labels", danav1.Role+":"+danav1.Leaf)
		FieldShouldContain("namespace", "", nsA, ".metadata.annotations", danav1.Role+":"+danav1.Leaf)

		CreateSubnamespace(nsB, nsA, randPrefix, false, storage, "25Gi", cpu, "25", memory, "25Gi", pods, "25", gpu, "25")

		// verify after child is created
		FieldShouldContain("namespace", "", nsA, ".metadata.labels", danav1.Role+":"+danav1.NoRole)
		FieldShouldContain("namespace", "", nsA, ".metadata.annotations", danav1.Role+":"+danav1.NoRole)
	})

	It("should update subnamespace with new resources", func() {
		nsA := GenerateE2EName("a", testPrefix, randPrefix)
		nsB := GenerateE2EName("b", testPrefix, randPrefix)
		nsC := GenerateE2EName("c", testPrefix, randPrefix)

		CreateSubnamespace(nsA, nsRoot, randPrefix, false, storage, "50Gi", cpu, "50", memory, "50Gi", pods, "50", gpu, "50")
		CreateSubnamespace(nsB, nsA, randPrefix, false, storage, "25Gi", cpu, "25", memory, "25Gi", pods, "25", gpu, "25")
		CreateSubnamespace(nsC, nsB, randPrefix, false, storage, "10Gi", cpu, "10", memory, "10Gi", pods, "10", gpu, "10")

		// verify before update
		FieldShouldContain("subnamespace", nsB, nsC, ".spec.resourcequota.hard.pods", "10")
		FieldShouldContain("subnamespace", nsA, nsB, ".status.total.allocated.pods", "10")
		FieldShouldContain("subnamespace", nsA, nsB, ".status.total.free.pods", "15")

		// update subnamespace with new resource values
		CreateSubnamespace(nsC, nsB, randPrefix, false, storage, "20Gi", cpu, "20", memory, "20Gi", pods, "20", gpu, "20")

		// verify after update
		FieldShouldContain("subnamespace", nsB, nsC, ".spec.resourcequota.hard.pods", "20")
		FieldShouldContain("subnamespace", nsA, nsB, ".status.total.allocated.pods", "20")
		FieldShouldContain("subnamespace", nsA, nsB, ".status.total.free.pods", "5")
	})

	It("should update the status of the subnamespace correctly", func() {
		nsA := GenerateE2EName("a", testPrefix, randPrefix)
		nsB := GenerateE2EName("b", testPrefix, randPrefix)
		nsC := GenerateE2EName("c", testPrefix, randPrefix)

		CreateSubnamespace(nsA, nsRoot, randPrefix, false, storage, "50Gi", cpu, "50", memory, "50Gi", pods, "50", gpu, "50")
		CreateSubnamespace(nsB, nsA, randPrefix, false, storage, "25Gi", cpu, "25", memory, "25Gi", pods, "25", gpu, "25")
		CreateSubnamespace(nsC, nsA, randPrefix, false, storage, "10Gi", cpu, "10", memory, "10Gi", pods, "10", gpu, "10")

		// verify
		ComplexFieldShouldContain("subnamespace", nsRoot, nsA,
			"'{{range.status.namespaces}}{{.namespace}}{{\"\\n\"}}{{end}}'", nsB)
		ComplexFieldShouldContain("subnamespace", nsRoot, nsA,
			"'{{range.status.namespaces}}{{.namespace}}{{\"\\n\"}}{{end}}'", nsC)

		FieldShouldContain("subnamespace", nsRoot, nsA, ".status.phase", "Created")
		ComplexFieldShouldContain("subnamespace", nsRoot, nsA,
			"'{{range.status.namespaces}}{{.resourcequota.hard.pods}}{{\"\\n\"}}{{end}}'", "25")
		ComplexFieldShouldContain("subnamespace", nsRoot, nsA,
			"'{{range.status.namespaces}}{{.resourcequota.hard.pods}}{{\"\\n\"}}{{end}}'", "10")
		FieldShouldContain("subnamespace", nsRoot, nsA, ".status.total.allocated.pods", "35")
		FieldShouldContain("subnamespace", nsRoot, nsA, ".status.total.free.pods", "15")
	})

	It("should not allow creating a subnamespace if a subnamespace of the same name already exists", func() {
		nsA := GenerateE2EName("a", testPrefix, randPrefix)
		nsB := GenerateE2EName("b", testPrefix, randPrefix)
		nsC := GenerateE2EName("c", testPrefix, randPrefix)
		nsD := GenerateE2EName("c", testPrefix, randPrefix)

		CreateSubnamespace(nsA, nsRoot, randPrefix, false, storage, "50Gi", cpu, "50", memory, "50Gi", pods, "50", gpu, "50")
		CreateSubnamespace(nsB, nsRoot, randPrefix, false, storage, "50Gi", cpu, "50", memory, "50Gi", pods, "50", gpu, "50")
		CreateSubnamespace(nsC, nsA, randPrefix, false, storage, "25Gi", cpu, "25", memory, "25Gi", pods, "25", gpu, "25")

		// creation of subnamespace should fail
		ShouldNotCreateSubnamespace(nsD, nsB, false, storage, "25Gi", cpu, "25", memory, "25Gi", pods, "25", gpu, "25")
	})

	It("should not allow creating a subnamespace with the same name as its parent", func() {
		nsA := GenerateE2EName("a", testPrefix, randPrefix)
		nsB := GenerateE2EName("b", testPrefix, randPrefix)
		nsC := GenerateE2EName("c", testPrefix, randPrefix)
		nsD := GenerateE2EName("b", testPrefix, randPrefix)

		CreateSubnamespace(nsA, nsRoot, randPrefix, false, storage, "50Gi", cpu, "50", memory, "50Gi", pods, "50", gpu, "50")
		CreateSubnamespace(nsB, nsA, randPrefix, false, storage, "50Gi", cpu, "50", memory, "50Gi", pods, "50", gpu, "50")
		CreateSubnamespace(nsC, nsB, randPrefix, false, storage, "25Gi", cpu, "25", memory, "25Gi", pods, "25", gpu, "25")

		// creation of subnamespace should fail
		ShouldNotCreateSubnamespace(nsD, nsB, false, storage, "25Gi", cpu, "25", memory, "25Gi", pods, "25", gpu, "25")
	})

	It("should fail to delete a namespace which is not a leaf", func() {
		nsA := GenerateE2EName("a", testPrefix, randPrefix)
		nsB := GenerateE2EName("b", testPrefix, randPrefix)

		CreateSubnamespace(nsA, nsRoot, randPrefix, false, storage, "50Gi", cpu, "50", memory, "50Gi", pods, "50", gpu, "50")
		CreateSubnamespace(nsB, nsA, randPrefix, false, storage, "25Gi", cpu, "25", memory, "25Gi", pods, "25", gpu, "25")

		FieldShouldContain("namespace", "", nsB, ".metadata.labels", danav1.Parent+":"+nsA)

		MustNotRun("kubectl delete namespace", nsA)
	})

	It("should fail to create a subnamespace which requests more resources than its parent to allocate", func() {
		nsA := GenerateE2EName("a", testPrefix, randPrefix)
		nsB := GenerateE2EName("b", testPrefix, randPrefix)
		nsC := GenerateE2EName("c", testPrefix, randPrefix)
		nsD := GenerateE2EName("d", testPrefix, randPrefix)
		nsE := GenerateE2EName("e", testPrefix, randPrefix)
		nsF := GenerateE2EName("f", testPrefix, randPrefix)

		CreateSubnamespace(nsA, nsRoot, randPrefix, false, storage, "50Gi", cpu, "50", memory, "50Gi", pods, "50", gpu, "50")
		CreateSubnamespace(nsB, nsA, randPrefix, false, storage, "40Gi", cpu, "40", memory, "40Gi", pods, "40", gpu, "40")
		CreateSubnamespace(nsC, nsB, randPrefix, false, storage, "30Gi", cpu, "30", memory, "30Gi", pods, "30", gpu, "30")

		CreateSubnamespace(nsD, nsC, randPrefix, false, storage, "10Gi", cpu, "10", memory, "10Gi", pods, "10", gpu, "10")
		CreateSubnamespace(nsE, nsC, randPrefix, false, storage, "10Gi", cpu, "10", memory, "10Gi", pods, "10", gpu, "10")

		FieldShouldContain("namespace", "", nsE, ".metadata.labels", danav1.Parent+":"+nsC)

		ShouldNotCreateSubnamespace(nsF, nsC, false, storage, "11Gi", cpu, "11", memory, "11Gi", pods, "11", gpu, "11")
	})

	It("should fail to update a subnamespace to request more resources than its parent has to allocate", func() {
		nsA := GenerateE2EName("a", testPrefix, randPrefix)
		nsB := GenerateE2EName("b", testPrefix, randPrefix)
		nsC := GenerateE2EName("c", testPrefix, randPrefix)
		nsD := GenerateE2EName("d", testPrefix, randPrefix)
		nsE := GenerateE2EName("e", testPrefix, randPrefix)
		nsF := GenerateE2EName("f", testPrefix, randPrefix)

		CreateSubnamespace(nsA, nsRoot, randPrefix, false, storage, "50Gi", cpu, "50", memory, "50Gi", pods, "50", gpu, "50")
		CreateSubnamespace(nsB, nsA, randPrefix, false, storage, "40Gi", cpu, "40", memory, "40Gi", pods, "40", gpu, "40")
		CreateSubnamespace(nsC, nsB, randPrefix, false, storage, "30Gi", cpu, "30", memory, "30Gi", pods, "30", gpu, "30")

		CreateSubnamespace(nsD, nsC, randPrefix, false, storage, "10Gi", cpu, "10", memory, "10Gi", pods, "10", gpu, "10")
		CreateSubnamespace(nsE, nsC, randPrefix, false, storage, "10Gi", cpu, "10", memory, "10Gi", pods, "10", gpu, "10")
		CreateSubnamespace(nsF, nsC, randPrefix, false, storage, "9Gi", cpu, "9", memory, "9Gi", pods, "9", gpu, "9")

		// verify before update
		FieldShouldContain("subnamespace", nsC, nsF, ".spec.resourcequota.hard.pods", "9")
		FieldShouldContain("subnamespace", nsB, nsC, ".status.total.allocated.pods", "29")
		FieldShouldContain("subnamespace", nsB, nsC, ".status.total.free.pods", "1")

		// update subnamespace with new resource values
		ShouldNotUpdateSubnamespace(nsF, nsC, false, storage, "11Gi", cpu, "11", memory, "11Gi", pods, "11", gpu, "11")

		// verify after update
		FieldShouldContain("subnamespace", nsC, nsF, ".spec.resourcequota.hard.pods", "9")
		FieldShouldContain("subnamespace", nsB, nsC, ".status.total.allocated.pods", "29")
		FieldShouldContain("subnamespace", nsB, nsC, ".status.total.free.pods", "1")
	})

	It("should not allow to create a subnamespace without all quota parameters", func() {
		nsA := GenerateE2EName("a", testPrefix, randPrefix)
		ShouldNotCreateSubnamespace(nsA, nsRoot, false, cpu, "50", memory, "50Gi", pods, "50", gpu, "50")
		ShouldNotCreateSubnamespace(nsA, nsRoot, false, storage, "50Gi", memory, "50Gi", pods, "50", gpu, "50")
		ShouldNotCreateSubnamespace(nsA, nsRoot, false, storage, "50Gi", cpu, "50", pods, "50", gpu, "50")
		ShouldNotCreateSubnamespace(nsA, nsRoot, false, storage, "50Gi", cpu, "50", memory, "50", gpu, "50")
		ShouldNotCreateSubnamespace(nsA, nsRoot, false, storage, "50Gi", cpu, "50", memory, "50Gi", pods, "50")
		ShouldNotCreateSubnamespace(nsA, nsRoot, false)
	})

	It("should not allow to update a subnamesapce to have less resources than allocated to its children", func() {
		nsA := GenerateE2EName("a", testPrefix, randPrefix)
		nsB := GenerateE2EName("b", testPrefix, randPrefix)
		nsC := GenerateE2EName("c", testPrefix, randPrefix)

		CreateSubnamespace(nsA, nsRoot, randPrefix, false, storage, "50Gi", cpu, "50", memory, "50Gi", pods, "50", gpu, "50")
		CreateSubnamespace(nsB, nsA, randPrefix, false, storage, "40Gi", cpu, "40", memory, "40Gi", pods, "40", gpu, "40")
		CreateSubnamespace(nsC, nsB, randPrefix, false, storage, "30Gi", cpu, "30", memory, "30Gi", pods, "30", gpu, "30")

		// verify before update
		FieldShouldContain("subnamespace", nsB, nsC, ".spec.resourcequota.hard.pods", "30")
		FieldShouldContain("subnamespace", nsA, nsB, ".status.total.allocated.pods", "30")
		FieldShouldContain("subnamespace", nsA, nsB, ".status.total.free.pods", "10")

		// update subnamespace with new resource values
		ShouldNotUpdateSubnamespace(nsB, nsA, false, storage, "20Gi", cpu, "20", memory, "20Gi", pods, "20", gpu, "20")

		// verify after update
		FieldShouldContain("subnamespace", nsB, nsC, ".spec.resourcequota.hard.pods", "30")
		FieldShouldContain("subnamespace", nsA, nsB, ".status.total.allocated.pods", "30")
		FieldShouldContain("subnamespace", nsA, nsB, ".status.total.free.pods", "10")
	})

	It("should update the child namespace with the default annotations and labels of its parent", func() {
		nsA := GenerateE2EName("a", testPrefix, randPrefix)
		nsB := GenerateE2EName("b", testPrefix, randPrefix)

		CreateSubnamespace(nsA, nsRoot, randPrefix, false, storage, "50Gi", cpu, "50", memory, "50Gi", pods, "50", gpu, "50")
		AnnotateNSDefaultAnnotations(nsA)
		LabelNSDefaultLabels(nsA)

		CreateSubnamespace(nsB, nsA, randPrefix, false, storage, "25Gi", cpu, "25", memory, "25Gi", pods, "25", gpu, "25")
		for i := range danav1.DefaultAnnotations {
			FieldShouldContain("namespace", "", nsB, ".metadata.annotations", danav1.DefaultAnnotations[i])
		}

		for i := range danav1.DefaultLabels {
			FieldShouldContain("namespace", "", nsB, ".metadata.labels", danav1.DefaultLabels[i])
		}
	})

	It("should fail when deleting a subnamespace directly", func() {
		nsA := GenerateE2EName("a", testPrefix, randPrefix)

		CreateSubnamespace(nsA, nsRoot, randPrefix, false, storage, "50Gi", cpu, "50", memory, "50Gi", pods, "50", gpu, "50")
		MustNotRun("kubectl delete subnamespace", nsA)

	})

	It("should sync entire quota spec to quota object", func() {
		nsA := GenerateE2EName("a", testPrefix, randPrefix)
		nsB := GenerateE2EName("b", testPrefix, randPrefix)
		nsC := GenerateE2EName("c", testPrefix, randPrefix)

		By("creating a subnamespace and a resourcequota for a subnamespace in a high hierarchy")
		CreateSubnamespaceWithScope(nsA, nsRoot, randPrefix, false, "In", storage,
			"50Gi", cpu, "50", memory, "50Gi", pods, "50", gpu, "50")
		CreateSubnamespaceWithScope(nsB, nsA, randPrefix, false, "In", storage,
			"25Gi", cpu, "25", memory, "25Gi", pods, "25", gpu, "25")

		By("creating a subnamespace and a clusterresourcequota for a subnamespace in a lower hierarchy")
		CreateSubnamespaceWithScope(nsC, nsB, randPrefix, false, "In", storage,
			"10Gi", cpu, "10", memory, "10Gi", pods, "10", gpu, "10")
		ComplexFieldShouldContain("clusterresourcequota", "", nsC,
			"'{{range.spec.quota.scopeSelector.matchExpressions}}{{.operator}}{{\"\\n\"}}{{end}}'", "In")

		// delete namespace
		MustRun("kubectl delete namespace", nsC, "-n", nsB)
	})

})
