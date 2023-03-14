package e2e

import (
	danav1 "github.com/dana-team/hns/api/v1"
	. "github.com/dana-team/hns/pkg/testutils"
	. "github.com/onsi/ginkgo/v2"
)

var _ = Describe("Subnamespaces", func() {
	nsRoot := GenerateE2EName("root")

	BeforeEach(func() {
		CleanupTestNamespaces()

		//set up root namespace
		CreateRootNS(nsRoot, rqDepth)
		CreateResourceQuota(nsRoot, nsRoot, storage, "100Gi", cpu, "100", memory, "100Gi", pods, "100", gpu, "100")
	})

	AfterEach(func() {
		CleanupTestNamespaces()
	})

	It("should create and delete a subnamespace", func() {
		nsA := GenerateE2EName("a")
		nsB := GenerateE2EName("b")
		nsC := GenerateE2EName("c")

		By("creating a subnamespace and a resourcequota for a subnamespace in a high hierarchy")
		CreateSubnamespace(nsA, nsRoot, false, storage, "50Gi", cpu, "50", memory, "50Gi", pods, "50", gpu, "50")
		CreateSubnamespace(nsB, nsA, false, storage, "25Gi", cpu, "25", memory, "25Gi", pods, "25", gpu, "25")

		// verify
		FieldShouldContain("resourcequota", nsA, nsA, ".metadata.name", nsA)
		FieldShouldContain("resourcequota", nsB, nsB, ".metadata.name", nsB)

		By("creating a subnamespace and a clusterresourcequota for a subnamespace in a lower hierarchy")
		CreateSubnamespace(nsC, nsB, false, storage, "10Gi", cpu, "10", memory, "10Gi", pods, "10", gpu, "10")
		FieldShouldContain("clusterresourcequota", "", nsC, ".metadata.name", nsC)

		// delete subnamespace
		MustRun("kubectl delete subnamespace", nsC, "-n", nsB)
		MustNotRun("kubectl get ns", nsC)
	})

	It("should create a subnamespace and namespace with the needed labels and annotations", func() {
		nsA := GenerateE2EName("a")
		nsB := GenerateE2EName("b")
		nsC := GenerateE2EName("c")

		CreateSubnamespace(nsA, nsRoot, false, storage, "50Gi", cpu, "50", memory, "50Gi", pods, "50", gpu, "50")
		CreateSubnamespace(nsB, nsA, false, storage, "25Gi", cpu, "25", memory, "25Gi", pods, "25", gpu, "25")
		CreateSubnamespace(nsC, nsB, false, storage, "10Gi", cpu, "10", memory, "10Gi", pods, "10", gpu, "10")

		// verify namespace labels
		FieldShouldContain("namespace", "", nsA, ".metadata.labels", danav1.Role+":"+danav1.NoRole)
		FieldShouldContain("namespace", "", nsB, ".metadata.labels", danav1.Role+":"+danav1.NoRole)
		FieldShouldContain("namespace", "", nsC, ".metadata.labels", danav1.Role+":"+danav1.Leaf)

		FieldShouldContain("namespace", "", nsA, ".metadata.labels", danav1.Aggragator+nsA+":true")
		FieldShouldContain("namespace", "", nsB, ".metadata.labels", danav1.Aggragator+nsB+":true")
		FieldShouldContain("namespace", "", nsC, ".metadata.labels", danav1.Aggragator+nsC+":true")

		FieldShouldContain("namespace", "", nsC, ".metadata.labels", danav1.Hns+":true")
		FieldShouldContain("namespace", "", nsC, ".metadata.labels", danav1.Parent+":"+nsB)
		FieldShouldContain("namespace", "", nsC, ".metadata.labels", danav1.ResourcePool+":false")

		// verify namespace annotations
		FieldShouldContain("namespace", "", nsC, ".metadata.annotations", danav1.Role+":"+danav1.Leaf)
		FieldShouldContain("namespace", "", nsC, ".metadata.annotations", "openshift.io/display-name:"+nsRoot+"/"+nsA+"/"+nsB+"/"+nsC)
		FieldShouldContain("namespace", "", nsC, ".metadata.annotations", "openshift.io/display-name:"+nsRoot+"/"+nsA+"/"+nsB+"/"+nsC)

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
		FieldShouldContain("subnamespace", nsB, nsC, ".metadata.annotations", danav1.IsRq+":"+danav1.False)
		FieldShouldContain("subnamespace", nsB, nsC, ".metadata.annotations", danav1.IsUpperRp+":"+danav1.False)
		FieldShouldContain("subnamespace", nsB, nsC, ".metadata.annotations", "openshift.io/display-name:"+nsRoot+"/"+nsA+"/"+nsB+"/"+nsC)
	})

	It("should update the role of a subnamespace after it creates children", func() {
		nsA := GenerateE2EName("a")
		nsB := GenerateE2EName("b")

		CreateSubnamespace(nsA, nsRoot, false, storage, "50Gi", cpu, "50", memory, "50Gi", pods, "50", gpu, "50")

		// verify before child is created
		FieldShouldContain("namespace", "", nsA, ".metadata.labels", danav1.Role+":"+danav1.Leaf)
		FieldShouldContain("namespace", "", nsA, ".metadata.annotations", danav1.Role+":"+danav1.Leaf)

		CreateSubnamespace(nsB, nsA, false, storage, "25Gi", cpu, "25", memory, "25Gi", pods, "25", gpu, "25")

		// verify after child is created
		FieldShouldContain("namespace", "", nsA, ".metadata.labels", danav1.Role+":"+danav1.NoRole)
		FieldShouldContain("namespace", "", nsA, ".metadata.annotations", danav1.Role+":"+danav1.NoRole)
	})

	It("should update subnamespace with new resources", func() {
		nsA := GenerateE2EName("a")
		nsB := GenerateE2EName("b")
		nsC := GenerateE2EName("c")

		CreateSubnamespace(nsA, nsRoot, false, storage, "50Gi", cpu, "50", memory, "50Gi", pods, "50", gpu, "50")
		CreateSubnamespace(nsB, nsA, false, storage, "25Gi", cpu, "25", memory, "25Gi", pods, "25", gpu, "25")
		CreateSubnamespace(nsC, nsB, false, storage, "10Gi", cpu, "10", memory, "10Gi", pods, "10", gpu, "10")

		// verify before update
		FieldShouldContain("subnamespace", nsB, nsC, ".spec.resourcequota.hard.pods", "10")
		FieldShouldContain("subnamespace", nsA, nsB, ".status.total.allocated.pods", "10")
		FieldShouldContain("subnamespace", nsA, nsB, ".status.total.free.pods", "15")

		// update subnamespace with new resource values
		CreateSubnamespace(nsC, nsB, false, storage, "20Gi", cpu, "20", memory, "20Gi", pods, "20", gpu, "20")

		// verify after update
		FieldShouldContain("subnamespace", nsB, nsC, ".spec.resourcequota.hard.pods", "20")
		FieldShouldContain("subnamespace", nsA, nsB, ".status.total.allocated.pods", "20")
		FieldShouldContain("subnamespace", nsA, nsB, ".status.total.free.pods", "5")
	})

	It("should update the status of the subnamespace correctly", func() {
		nsA := GenerateE2EName("a")
		nsB := GenerateE2EName("b")
		nsC := GenerateE2EName("c")

		CreateSubnamespace(nsA, nsRoot, false, storage, "50Gi", cpu, "50", memory, "50Gi", pods, "50", gpu, "50")
		CreateSubnamespace(nsB, nsA, false, storage, "25Gi", cpu, "25", memory, "25Gi", pods, "25", gpu, "25")
		CreateSubnamespace(nsC, nsA, false, storage, "10Gi", cpu, "10", memory, "10Gi", pods, "10", gpu, "10")

		// verify
		ComplexFieldShouldContain("subnamespace", nsRoot, nsA, "'{{range.status.namespaces}}{{.namespace}}{{\"\\n\"}}{{end}}'", nsB)
		ComplexFieldShouldContain("subnamespace", nsRoot, nsA, "'{{range.status.namespaces}}{{.namespace}}{{\"\\n\"}}{{end}}'", nsC)

		FieldShouldContain("subnamespace", nsRoot, nsA, ".status.phase", "Created")
		ComplexFieldShouldContain("subnamespace", nsRoot, nsA, "'{{range.status.namespaces}}{{.resourcequota.hard.pods}}{{\"\\n\"}}{{end}}'", "25")
		ComplexFieldShouldContain("subnamespace", nsRoot, nsA, "'{{range.status.namespaces}}{{.resourcequota.hard.pods}}{{\"\\n\"}}{{end}}'", "10")
		FieldShouldContain("subnamespace", nsRoot, nsA, ".status.total.allocated.pods", "35")
		FieldShouldContain("subnamespace", nsRoot, nsA, ".status.total.free.pods", "15")
	})

	It("should not allow creating a subnamespace if a subnamespace of the same name already exists", func() {
		nsA := GenerateE2EName("a")
		nsB := GenerateE2EName("b")
		nsC := GenerateE2EName("c")
		nsD := GenerateE2EName("c")

		CreateSubnamespace(nsA, nsRoot, false, storage, "50Gi", cpu, "50", memory, "50Gi", pods, "50", gpu, "50")
		CreateSubnamespace(nsB, nsRoot, false, storage, "50Gi", cpu, "50", memory, "50Gi", pods, "50", gpu, "50")
		CreateSubnamespace(nsC, nsA, false, storage, "25Gi", cpu, "25", memory, "25Gi", pods, "25", gpu, "25")

		// creation of subnamespace should fail
		ShouldNotCreateSubnamespace(nsD, nsB, false, storage, "25Gi", cpu, "25", memory, "25Gi", pods, "25", gpu, "25")
	})

	It("should fail to delete a subnamespace which is not a leaf", func() {
		nsA := GenerateE2EName("a")
		nsB := GenerateE2EName("b")

		CreateSubnamespace(nsA, nsRoot, false, storage, "50Gi", cpu, "50", memory, "50Gi", pods, "50", gpu, "50")
		CreateSubnamespace(nsB, nsA, false, storage, "25Gi", cpu, "25", memory, "25Gi", pods, "25", gpu, "25")

		MustNotRun("kubectl delete subnamespace -n", nsRoot, nsA)
	})

	It("should fail to create a subnamespace which requests more resources than its parent to allocate", func() {
		nsA := GenerateE2EName("a")
		nsB := GenerateE2EName("b")
		nsC := GenerateE2EName("c")
		nsD := GenerateE2EName("d")
		nsE := GenerateE2EName("e")
		nsF := GenerateE2EName("f")

		CreateSubnamespace(nsA, nsRoot, false, storage, "50Gi", cpu, "50", memory, "50Gi", pods, "50", gpu, "50")
		CreateSubnamespace(nsB, nsA, false, storage, "40Gi", cpu, "40", memory, "40Gi", pods, "40", gpu, "40")
		CreateSubnamespace(nsC, nsB, false, storage, "30Gi", cpu, "30", memory, "30Gi", pods, "30", gpu, "30")

		CreateSubnamespace(nsD, nsC, false, storage, "10Gi", cpu, "10", memory, "10Gi", pods, "10", gpu, "10")
		CreateSubnamespace(nsE, nsC, false, storage, "10Gi", cpu, "10", memory, "10Gi", pods, "10", gpu, "10")
		ShouldNotCreateSubnamespace(nsF, nsC, false, storage, "11Gi", cpu, "11", memory, "11Gi", pods, "11", gpu, "11")
	})

	It("should fail to update a subnamespace to request more resources than its parent has to allocate", func() {
		nsA := GenerateE2EName("a")
		nsB := GenerateE2EName("b")
		nsC := GenerateE2EName("c")
		nsD := GenerateE2EName("d")
		nsE := GenerateE2EName("e")
		nsF := GenerateE2EName("f")

		CreateSubnamespace(nsA, nsRoot, false, storage, "50Gi", cpu, "50", memory, "50Gi", pods, "50", gpu, "50")
		CreateSubnamespace(nsB, nsA, false, storage, "40Gi", cpu, "40", memory, "40Gi", pods, "40", gpu, "40")
		CreateSubnamespace(nsC, nsB, false, storage, "30Gi", cpu, "30", memory, "30Gi", pods, "30", gpu, "30")

		CreateSubnamespace(nsD, nsC, false, storage, "10Gi", cpu, "10", memory, "10Gi", pods, "10", gpu, "10")
		CreateSubnamespace(nsE, nsC, false, storage, "10Gi", cpu, "10", memory, "10Gi", pods, "10", gpu, "10")
		CreateSubnamespace(nsF, nsC, false, storage, "9Gi", cpu, "9", memory, "9Gi", pods, "9", gpu, "9")

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
		nsA := GenerateE2EName("a")
		ShouldNotCreateSubnamespace(nsA, nsRoot, false, cpu, "50", memory, "50Gi", pods, "50", gpu, "50")
		ShouldNotCreateSubnamespace(nsA, nsRoot, false, storage, "50Gi", memory, "50Gi", pods, "50", gpu, "50")
		ShouldNotCreateSubnamespace(nsA, nsRoot, false, storage, "50Gi", cpu, "50", pods, "50", gpu, "50")
		ShouldNotCreateSubnamespace(nsA, nsRoot, false, storage, "50Gi", cpu, "50", memory, "50", gpu, "50")
		ShouldNotCreateSubnamespace(nsA, nsRoot, false, storage, "50Gi", cpu, "50", memory, "50Gi", pods, "50")
		ShouldNotCreateSubnamespace(nsA, nsRoot, false)
	})

	It("should not allow to update a subnamesapce to have less resources than allocated to its children", func() {
		nsA := GenerateE2EName("a")
		nsB := GenerateE2EName("b")
		nsC := GenerateE2EName("c")

		CreateSubnamespace(nsA, nsRoot, false, storage, "50Gi", cpu, "50", memory, "50Gi", pods, "50", gpu, "50")
		CreateSubnamespace(nsB, nsA, false, storage, "40Gi", cpu, "40", memory, "40Gi", pods, "40", gpu, "40")
		CreateSubnamespace(nsC, nsB, false, storage, "30Gi", cpu, "30", memory, "30Gi", pods, "30", gpu, "30")

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

})