package e2e

import (
	"fmt"

	danav1 "github.com/dana-team/hns/api/v1"
	. "github.com/dana-team/hns/pkg/testutils"
	. "github.com/onsi/ginkgo/v2"
)

var _ = Describe("ResourcePool", func() {
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

	It("should create and delete a resourcepool under a subnamespace", func() {
		nsA := GenerateE2EName("a")
		nsB := GenerateE2EName("b")
		nsC := GenerateE2EName("c")

		CreateSubnamespace(nsA, nsRoot, false, storage, "50Gi", cpu, "50", memory, "50Gi", pods, "50", gpu, "50")
		CreateSubnamespace(nsB, nsA, false, storage, "25Gi", cpu, "25", memory, "25Gi", pods, "25", gpu, "25")
		CreateSubnamespace(nsC, nsB, true, storage, "10Gi", cpu, "10", memory, "10Gi", pods, "10", gpu, "10")

		// verify
		//FieldShouldContain("namespace", "", nsC, ".metadata.labels", danav1.ResourcePool+":true") # TODO: remove from comment after fixing bug
		FieldShouldContain("subnamespace", nsB, nsC, ".metadata.labels", danav1.ResourcePool+":true")
		FieldShouldContain("subnamespace", nsB, nsC, ".metadata.annotations", danav1.IsRq+":"+danav1.False)
		FieldShouldContain("subnamespace", nsB, nsC, ".metadata.annotations", danav1.IsUpperRp+":"+danav1.True)

		// delete subnamespace
		MustRun("kubectl delete subnamespace", nsC, "-n", nsB)
		MustNotRun("kubectl get ns", nsC)
	})

	It("should create a resourcepool under a resourcepool and update the labels accordingly", func() {
		nsA := GenerateE2EName("a")
		nsB := GenerateE2EName("b")
		nsC := GenerateE2EName("c")
		nsD := GenerateE2EName("d")

		CreateSubnamespace(nsA, nsRoot, false, storage, "50Gi", cpu, "50", memory, "50Gi", pods, "50", gpu, "50")
		CreateSubnamespace(nsB, nsA, false, storage, "25Gi", cpu, "25", memory, "25Gi", pods, "25", gpu, "25")
		CreateSubnamespace(nsC, nsB, true, storage, "10Gi", cpu, "10", memory, "10Gi", pods, "10", gpu, "10")
		CreateSubnamespace(nsD, nsC, true)

		// verify
		FieldShouldContain("subnamespace", nsC, nsD, ".metadata.labels", danav1.ResourcePool+":true")
		//FieldShouldContain("subnamespace", nsC, nsD, ".metadata.annotations", danav1.IsRq+":"+danav1.False) # TODO: remove from comment after fixing bug
		FieldShouldContain("subnamespace", nsC, nsD, ".metadata.annotations", danav1.IsUpperRp+":"+danav1.False)
		FieldShouldContain("subnamespace", nsC, nsD, ".metadata.annotations", danav1.UpperRp+":"+nsC)
	})

	It("should update the resources of an upper resourcepool", func() {
		nsA := GenerateE2EName("a")
		nsB := GenerateE2EName("b")
		nsC := GenerateE2EName("c")
		nsD := GenerateE2EName("d")

		CreateSubnamespace(nsA, nsRoot, false, storage, "50Gi", cpu, "50", memory, "50Gi", pods, "50", gpu, "50")
		CreateSubnamespace(nsB, nsA, false, storage, "25Gi", cpu, "25", memory, "25Gi", pods, "25", gpu, "25")
		CreateSubnamespace(nsC, nsB, true, storage, "10Gi", cpu, "10", memory, "10Gi", pods, "10", gpu, "10")
		CreateSubnamespace(nsD, nsC, true)

		// verify before update
		FieldShouldContain("subnamespace", nsB, nsC, ".status.total.free.pods", "10")

		// update
		CreateSubnamespace(nsC, nsB, true, storage, "10Gi", cpu, "10", memory, "10Gi", pods, "20", gpu, "10")

		// verify after update
		FieldShouldContain("subnamespace", nsB, nsC, ".status.total.free.pods", "20")
	})

	It("should delete a resourcepool if it's a leaf", func() {
		nsA := GenerateE2EName("a")
		nsB := GenerateE2EName("b")
		nsC := GenerateE2EName("c")
		nsD := GenerateE2EName("d")

		CreateSubnamespace(nsA, nsRoot, false, storage, "50Gi", cpu, "50", memory, "50Gi", pods, "50", gpu, "50")
		CreateSubnamespace(nsB, nsA, false, storage, "25Gi", cpu, "25", memory, "25Gi", pods, "25", gpu, "25")
		CreateSubnamespace(nsC, nsB, true, storage, "10Gi", cpu, "10", memory, "10Gi", pods, "10", gpu, "10")
		CreateSubnamespace(nsD, nsC, true)

		MustRun("kubectl delete subnamespace -n", nsC, nsD)
	})

	It("should create a clusterresourcequota for a resourcepool regardless of its depth only if it's upper", func() {
		nsA := GenerateE2EName("a")
		nsB := GenerateE2EName("b")
		nsC := GenerateE2EName("c")
		nsD := GenerateE2EName("d")

		CreateSubnamespace(nsA, nsRoot, false, storage, "50Gi", cpu, "50", memory, "50Gi", pods, "50", gpu, "50")
		CreateSubnamespace(nsB, nsA, true, storage, "25Gi", cpu, "25", memory, "25Gi", pods, "25", gpu, "25")
		CreateSubnamespace(nsC, nsB, true)
		CreateSubnamespace(nsD, nsC, true)

		RunShouldNotContain(nsA, propagationTime, "kubectl get clusterresourcequota")
		FieldShouldContain("clusterresourcequota", "", nsB, ".metadata.name", nsB)
		RunShouldNotContain(nsC, propagationTime, "kubectl get clusterresourcequota")
		RunShouldNotContain(nsD, propagationTime, "kubectl get clusterresourcequota")
	})

	It("should turn all descendants of a subnamespace to resourcepool when the sns turns to resourcepool", func() {
		nsA := GenerateE2EName("a")
		nsB := GenerateE2EName("b")
		nsC := GenerateE2EName("c")
		nsD := GenerateE2EName("d")
		nsE := GenerateE2EName("e")
		nsF := GenerateE2EName("f")
		nsG := GenerateE2EName("g")
		nsH := GenerateE2EName("h")
		nsI := GenerateE2EName("i")
		nsJ := GenerateE2EName("i")

		CreateSubnamespace(nsA, nsRoot, false, storage, "50Gi", cpu, "50", memory, "50Gi", pods, "50", gpu, "50")
		CreateSubnamespace(nsB, nsA, false, storage, "25Gi", cpu, "25", memory, "25Gi", pods, "25", gpu, "25")
		CreateSubnamespace(nsC, nsB, false, storage, "10Gi", cpu, "10", memory, "10Gi", pods, "10", gpu, "10")
		CreateSubnamespace(nsD, nsC, false, storage, "10Gi", cpu, "10", memory, "10Gi", pods, "10", gpu, "10")
		CreateSubnamespace(nsE, nsD, false, storage, "5Gi", cpu, "5", memory, "5Gi", pods, "5", gpu, "5")
		CreateSubnamespace(nsF, nsD, false, storage, "5Gi", cpu, "5", memory, "5Gi", pods, "5", gpu, "5")
		CreateSubnamespace(nsG, nsE, false, storage, "2Gi", cpu, "2", memory, "2Gi", pods, "2", gpu, "2")
		CreateSubnamespace(nsH, nsE, false, storage, "2Gi", cpu, "2", memory, "2Gi", pods, "2", gpu, "2")
		CreateSubnamespace(nsI, nsF, false, storage, "2Gi", cpu, "2", memory, "2Gi", pods, "2", gpu, "2")
		CreateSubnamespace(nsJ, nsF, false, storage, "2Gi", cpu, "2", memory, "2Gi", pods, "2", gpu, "2")

		// verify before change
		FieldShouldContain("subnamespace", nsC, nsD, ".metadata.labels", danav1.ResourcePool+":false")
		FieldShouldContain("subnamespace", nsD, nsE, ".metadata.labels", danav1.ResourcePool+":false")
		FieldShouldContain("subnamespace", nsD, nsF, ".metadata.labels", danav1.ResourcePool+":false")
		FieldShouldContain("subnamespace", nsE, nsG, ".metadata.labels", danav1.ResourcePool+":false")
		FieldShouldContain("subnamespace", nsE, nsH, ".metadata.labels", danav1.ResourcePool+":false")
		FieldShouldContain("subnamespace", nsF, nsI, ".metadata.labels", danav1.ResourcePool+":false")
		FieldShouldContain("subnamespace", nsF, nsJ, ".metadata.labels", danav1.ResourcePool+":false")

		FieldShouldContain("clusterresourcequota", "", nsD, ".metadata.name", nsD)
		FieldShouldContain("clusterresourcequota", "", nsE, ".metadata.name", nsE)
		FieldShouldContain("clusterresourcequota", "", nsF, ".metadata.name", nsF)
		FieldShouldContain("clusterresourcequota", "", nsG, ".metadata.name", nsG)
		FieldShouldContain("clusterresourcequota", "", nsH, ".metadata.name", nsH)
		FieldShouldContain("clusterresourcequota", "", nsI, ".metadata.name", nsI)
		FieldShouldContain("clusterresourcequota", "", nsJ, ".metadata.name", nsJ)

		// turn nsD to ResourcePool
		CreateSubnamespace(nsD, nsC, true, storage, "10Gi", cpu, "10", memory, "10Gi", pods, "10", gpu, "10")

		// verify labels and annotations after change
		FieldShouldContain("subnamespace", nsC, nsD, ".metadata.labels", danav1.ResourcePool+":true")
		FieldShouldContain("subnamespace", nsC, nsD, ".metadata.annotations", danav1.IsUpperRp+":"+danav1.True)

		FieldShouldContain("subnamespace", nsD, nsE, ".metadata.labels", danav1.ResourcePool+":true")
		FieldShouldContain("subnamespace", nsD, nsE, ".metadata.annotations", danav1.IsUpperRp+":"+danav1.False)
		FieldShouldContain("subnamespace", nsD, nsE, ".metadata.annotations", danav1.UpperRp+":"+nsD)

		FieldShouldContain("subnamespace", nsD, nsF, ".metadata.labels", danav1.ResourcePool+":true")
		FieldShouldContain("subnamespace", nsD, nsF, ".metadata.annotations", danav1.IsUpperRp+":"+danav1.False)
		FieldShouldContain("subnamespace", nsD, nsF, ".metadata.annotations", danav1.UpperRp+":"+nsD)

		FieldShouldContain("subnamespace", nsE, nsG, ".metadata.labels", danav1.ResourcePool+":true")
		FieldShouldContain("subnamespace", nsE, nsG, ".metadata.annotations", danav1.IsUpperRp+":"+danav1.False)
		FieldShouldContain("subnamespace", nsE, nsG, ".metadata.annotations", danav1.UpperRp+":"+nsD)

		FieldShouldContain("subnamespace", nsE, nsH, ".metadata.labels", danav1.ResourcePool+":true")
		FieldShouldContain("subnamespace", nsE, nsH, ".metadata.annotations", danav1.IsUpperRp+":"+danav1.False)
		FieldShouldContain("subnamespace", nsE, nsH, ".metadata.annotations", danav1.UpperRp+":"+nsD)

		FieldShouldContain("subnamespace", nsF, nsI, ".metadata.labels", danav1.ResourcePool+":true")
		FieldShouldContain("subnamespace", nsF, nsI, ".metadata.annotations", danav1.IsUpperRp+":"+danav1.False)
		FieldShouldContain("subnamespace", nsF, nsI, ".metadata.annotations", danav1.UpperRp+":"+nsD)

		FieldShouldContain("subnamespace", nsF, nsJ, ".metadata.labels", danav1.ResourcePool+":true")
		FieldShouldContain("subnamespace", nsF, nsJ, ".metadata.annotations", danav1.IsUpperRp+":"+danav1.False)
		FieldShouldContain("subnamespace", nsF, nsJ, ".metadata.annotations", danav1.UpperRp+":"+nsD)

		// verify clusterresourcequotas are deleted
		FieldShouldContain("clusterresourcequota", "", nsD, ".metadata.name", nsD)
		RunShouldNotContain(nsE, propagationTime, "kubectl get clusterresourcequota")
		RunShouldNotContain(nsF, propagationTime, "kubectl get clusterresourcequota")
		RunShouldNotContain(nsG, propagationTime, "kubectl get clusterresourcequota")
		RunShouldNotContain(nsH, propagationTime, "kubectl get clusterresourcequota")
		RunShouldNotContain(nsI, propagationTime, "kubectl get clusterresourcequota")
		RunShouldNotContain(nsJ, propagationTime, "kubectl get clusterresourcequota")
	})
	It("should turn only the upper resourcepool into a subnamespace when converting", func() {
		nsA := GenerateE2EName("a")
		nsB := GenerateE2EName("b")
		nsC := GenerateE2EName("c")
		nsD := GenerateE2EName("d")
		nsE := GenerateE2EName("e")
		nsF := GenerateE2EName("f")

		// create hierarchy
		CreateSubnamespace(nsA, nsRoot, false, storage, "50Gi", cpu, "50", memory, "50Gi", pods, "50", gpu, "50")
		CreateSubnamespace(nsB, nsA, false, storage, "25Gi", cpu, "25", memory, "25Gi", pods, "25", gpu, "25")
		CreateSubnamespace(nsC, nsB, true, storage, "10Gi", cpu, "10", memory, "10Gi", pods, "10", gpu, "10")
		CreateSubnamespace(nsD, nsC, true)
		CreateSubnamespace(nsE, nsC, true)
		CreateSubnamespace(nsF, nsC, true)

		// create pods in leafs
		podName := "example"
		CreatePod(nsD, podName)
		CreatePod(nsE, podName)
		CreatePod(nsF, podName)

		// make sure the pods are up
		RunShouldContain(podName, propagationTime, "kubectl get pods -n"+nsD)
		RunShouldContain(podName, propagationTime, "kubectl get pods -n"+nsE)
		RunShouldContain(podName, propagationTime, "kubectl get pods -n"+nsF)

		// convert upper resource pool to subnamespace
		CreateSubnamespace(nsC, nsB, false, storage, "10Gi", cpu, "10", memory, "10Gi", pods, "10", gpu, "10")

		// make sure the subnamespace was converted and its children weren't
		FieldShouldContain("subnamespace", nsB, nsC, ".metadata.labels", danav1.ResourcePool+":false")
		FieldShouldContain("subnamespace", nsC, nsD, ".metadata.labels", danav1.ResourcePool+":true")
		FieldShouldContain("subnamespace", nsC, nsE, ".metadata.labels", danav1.ResourcePool+":true")
		FieldShouldContain("subnamespace", nsC, nsF, ".metadata.labels", danav1.ResourcePool+":true")

		// make sure the top resourcepools have crq and is upper set to true
		FieldShouldContain("subnamespace", nsC, nsD, ".metadata.annotations", danav1.IsUpperRp+":"+danav1.True)
		FieldShouldContain("subnamespace", nsC, nsE, ".metadata.annotations", danav1.IsUpperRp+":"+danav1.True)
		FieldShouldContain("subnamespace", nsC, nsF, ".metadata.annotations", danav1.IsUpperRp+":"+danav1.True)

		FieldShouldContain("clusterresourcequota", "", nsD, ".metadata.name", nsD)
		FieldShouldContain("clusterresourcequota", "", nsE, ".metadata.name", nsE)
		FieldShouldContain("clusterresourcequota", "", nsF, ".metadata.name", nsF)

		// make sure the subnamespace has the upper resource pool in its status
		ComplexFieldShouldContain("subnamespace", nsB, nsC, "'{{range.status.namespaces}}{{.namespace}}{{\"\\n\"}}{{end}}'", nsD)
		ComplexFieldShouldContain("subnamespace", nsB, nsC, "'{{range.status.namespaces}}{{.namespace}}{{\"\\n\"}}{{end}}'", nsE)
		ComplexFieldShouldContain("subnamespace", nsB, nsC, "'{{range.status.namespaces}}{{.namespace}}{{\"\\n\"}}{{end}}'", nsF)
	})
	It("should sum the children workloads correctly", func() {
		nsA := GenerateE2EName("a")
		nsB := GenerateE2EName("b")
		nsC := GenerateE2EName("c")
		nsD := GenerateE2EName("d")
		nsE := GenerateE2EName("e")
		nsF := GenerateE2EName("f")
		nsG := GenerateE2EName("g")
		nsH := GenerateE2EName("h")
		nsI := GenerateE2EName("i")
		nsJ := GenerateE2EName("j")
		nsK := GenerateE2EName("k")

		// create hierarchy
		CreateSubnamespace(nsA, nsRoot, false, storage, "100Gi", cpu, "100", memory, "100Gi", pods, "100", gpu, "100")
		CreateSubnamespace(nsB, nsA, false, storage, "50Gi", cpu, "50", memory, "50Gi", pods, "50", gpu, "50")
		CreateSubnamespace(nsC, nsB, true, storage, "25Gi", cpu, "25", memory, "25Gi", pods, "25", gpu, "25")
		CreateSubnamespace(nsD, nsC, true)
		CreateSubnamespace(nsE, nsC, true)
		CreateSubnamespace(nsF, nsD, true)
		CreateSubnamespace(nsG, nsD, true)
		CreateSubnamespace(nsH, nsD, true)
		CreateSubnamespace(nsI, nsE, true)
		CreateSubnamespace(nsJ, nsE, true)
		CreateSubnamespace(nsK, nsE, true)

		// create pods in leafs
		podName := "example"
		CreatePod(nsF, podName)
		CreatePod(nsF, podName+"2")
		CreatePod(nsF, podName+"3")
		CreatePod(nsG, podName)
		CreatePod(nsG, podName+"2")
		CreatePod(nsH, podName)
		CreatePod(nsI, podName)
		CreatePod(nsI, podName+"2")
		CreatePod(nsI, podName+"3")
		CreatePod(nsJ, podName)
		CreatePod(nsJ, podName+"2")
		CreatePod(nsK, podName)

		// make sure the pods are up
		RunShouldContain(podName, propagationTime, "kubectl get pods -n"+nsF)
		RunShouldContain(podName+"2", propagationTime, "kubectl get pods -n"+nsF)
		RunShouldContain(podName+"3", propagationTime, "kubectl get pods -n"+nsF)
		RunShouldContain(podName, propagationTime, "kubectl get pods -n"+nsG)
		RunShouldContain(podName+"2", propagationTime, "kubectl get pods -n"+nsG)
		RunShouldContain(podName, propagationTime, "kubectl get pods -n"+nsH)
		RunShouldContain(podName, propagationTime, "kubectl get pods -n"+nsI)
		RunShouldContain(podName+"2", propagationTime, "kubectl get pods -n"+nsI)
		RunShouldContain(podName+"3", propagationTime, "kubectl get pods -n"+nsI)
		RunShouldContain(podName, propagationTime, "kubectl get pods -n"+nsJ)
		RunShouldContain(podName+"2", propagationTime, "kubectl get pods -n"+nsJ)
		RunShouldContain(podName, propagationTime, "kubectl get pods -n"+nsK)

		// convert upper resource pool to subnamespace
		CreateSubnamespace(nsC, nsB, false, storage, "25Gi", cpu, "25", memory, "25Gi", pods, "25", gpu, "25")

		// make sure the subnamespaces was converted and its children weren't
		FieldShouldContain("subnamespace", nsB, nsC, ".metadata.labels", danav1.ResourcePool+":false")
		FieldShouldContain("subnamespace", nsC, nsD, ".metadata.labels", danav1.ResourcePool+":true")
		FieldShouldContain("subnamespace", nsC, nsE, ".metadata.labels", danav1.ResourcePool+":true")
		// make sure the top resourcepools have crq and is upper set to true
		FieldShouldContain("subnamespace", nsC, nsD, ".metadata.annotations", danav1.IsUpperRp+":"+danav1.True)
		FieldShouldContain("subnamespace", nsC, nsE, ".metadata.annotations", danav1.IsUpperRp+":"+danav1.True)

		FieldShouldContain("clusterresourcequota", "", nsD, ".metadata.name", nsD)
		FieldShouldContain("clusterresourcequota", "", nsE, ".metadata.name", nsE)

		// make sure the crqs has the proper values
		FieldShouldContain("clusterresourcequota", "", nsD, ".spec.quota.hard.pods", fmt.Sprint(6))
		FieldShouldContain("clusterresourcequota", "", nsD, ".spec.quota.hard.cpu", "600m")
		FieldShouldContain("clusterresourcequota", "", nsD, ".spec.quota.hard.memory", "1200M")
		FieldShouldContain("clusterresourcequota", "", nsE, ".spec.quota.hard.pods", fmt.Sprint(6))
		FieldShouldContain("clusterresourcequota", "", nsE, ".spec.quota.hard.cpu", "600m")
		FieldShouldContain("clusterresourcequota", "", nsE, ".spec.quota.hard.memory", "1200M")

		// make sure the subnamespace has the upper resource pool in its status
		ComplexFieldShouldContain("subnamespace", nsB, nsC, "'{{range.status.namespaces}}{{.namespace}}{{\"\\n\"}}{{end}}'", nsD)
		ComplexFieldShouldContain("subnamespace", nsB, nsC, "'{{range.status.namespaces}}{{.namespace}}{{\"\\n\"}}{{end}}'", nsE)

	})
	It("should delete the crq and update annotations for upper resource pools when converting their father", func() {
		nsA := GenerateE2EName("a")
		nsB := GenerateE2EName("b")
		nsC := GenerateE2EName("c")
		nsD := GenerateE2EName("d")
		nsE := GenerateE2EName("e")
		nsF := GenerateE2EName("f")
		nsG := GenerateE2EName("g")
		nsH := GenerateE2EName("h")
		nsI := GenerateE2EName("i")
		nsJ := GenerateE2EName("j")
		nsK := GenerateE2EName("k")

		// create hierarchy
		CreateSubnamespace(nsA, nsRoot, false, storage, "100Gi", cpu, "100", memory, "100Gi", pods, "100", gpu, "100")
		CreateSubnamespace(nsB, nsA, false, storage, "50Gi", cpu, "50", memory, "50Gi", pods, "50", gpu, "50")
		CreateSubnamespace(nsC, nsB, true, storage, "25Gi", cpu, "25", memory, "25Gi", pods, "25", gpu, "25")
		CreateSubnamespace(nsD, nsC, true)
		CreateSubnamespace(nsE, nsC, true)
		CreateSubnamespace(nsF, nsD, true)
		CreateSubnamespace(nsG, nsD, true)
		CreateSubnamespace(nsH, nsD, true)
		CreateSubnamespace(nsI, nsE, true)
		CreateSubnamespace(nsJ, nsE, true)
		CreateSubnamespace(nsK, nsE, true)

		// create pods in leafs
		podName := "example"
		CreatePod(nsF, podName)
		CreatePod(nsF, podName+"2")
		CreatePod(nsF, podName+"3")
		CreatePod(nsG, podName)
		CreatePod(nsG, podName+"2")
		CreatePod(nsH, podName)
		CreatePod(nsI, podName)
		CreatePod(nsI, podName+"2")
		CreatePod(nsI, podName+"3")
		CreatePod(nsJ, podName)
		CreatePod(nsJ, podName+"2")
		CreatePod(nsK, podName)

		// make sure the pods are up
		RunShouldContain(podName, propagationTime, "kubectl get pods -n"+nsF)
		RunShouldContain(podName+"2", propagationTime, "kubectl get pods -n"+nsF)
		RunShouldContain(podName+"3", propagationTime, "kubectl get pods -n"+nsF)
		RunShouldContain(podName, propagationTime, "kubectl get pods -n"+nsG)
		RunShouldContain(podName+"2", propagationTime, "kubectl get pods -n"+nsG)
		RunShouldContain(podName, propagationTime, "kubectl get pods -n"+nsH)
		RunShouldContain(podName, propagationTime, "kubectl get pods -n"+nsI)
		RunShouldContain(podName+"2", propagationTime, "kubectl get pods -n"+nsI)
		RunShouldContain(podName+"3", propagationTime, "kubectl get pods -n"+nsI)
		RunShouldContain(podName, propagationTime, "kubectl get pods -n"+nsJ)
		RunShouldContain(podName+"2", propagationTime, "kubectl get pods -n"+nsJ)
		RunShouldContain(podName, propagationTime, "kubectl get pods -n"+nsK)

		// convert upper resource pool to subnamespace
		CreateSubnamespace(nsC, nsB, false, storage, "25Gi", cpu, "25", memory, "25Gi", pods, "25", gpu, "25")

		// convert it back
		CreateSubnamespace(nsC, nsB, true)

		// make sure the upper resourcepool and its sons have the correct annotaions
		FieldShouldContain("subnamespace", nsB, nsC, ".metadata.annotations", danav1.IsUpperRp+":"+danav1.True)
		FieldShouldContain("subnamespace", nsC, nsD, ".metadata.annotations", danav1.IsUpperRp+":"+danav1.False)
		FieldShouldContain("subnamespace", nsC, nsD, ".metadata.annotations", danav1.UpperRp+":"+nsC)
		FieldShouldContain("subnamespace", nsC, nsE, ".metadata.annotations", danav1.IsUpperRp+":"+danav1.False)
		FieldShouldContain("subnamespace", nsC, nsE, ".metadata.annotations", danav1.UpperRp+":"+nsC)

		// make sure the crqs were deleted
		RunShouldNotContain(nsD, propagationTime, "kubectl get clusterresourcequota")
		RunShouldNotContain(nsE, propagationTime, "kubectl get clusterresourcequota")

	})
	It("should not create a subnamespace under a resourcepool", func() {
		nsA := GenerateE2EName("a")
		nsB := GenerateE2EName("b")
		nsC := GenerateE2EName("c")
		nsD := GenerateE2EName("d")

		CreateSubnamespace(nsA, nsRoot, false, storage, "50Gi", cpu, "50", memory, "50Gi", pods, "50", gpu, "50")
		CreateSubnamespace(nsB, nsA, true, storage, "25Gi", cpu, "25", memory, "25Gi", pods, "25", gpu, "25")
		CreateSubnamespace(nsC, nsB, true)

		ShouldNotCreateSubnamespace(nsD, nsB, false, storage, "10Gi", cpu, "10", memory, "10Gi", pods, "10", gpu, "10")
	})
	It("should show an upper resource pool children", func() {
		nsA := GenerateE2EName("a")
		nsB := GenerateE2EName("b")
		nsC := GenerateE2EName("c")

		CreateSubnamespace(nsA, nsRoot, false, storage, "50Gi", cpu, "50", memory, "50Gi", pods, "50", gpu, "50")
		CreateSubnamespace(nsB, nsA, true, storage, "25Gi", cpu, "25", memory, "25Gi", pods, "25", gpu, "25")
		CreateSubnamespace(nsC, nsB, true)

		ComplexFieldShouldContain("subnamespace", nsA, nsB, "'{{range.status.namespaces}}{{.namespace}}{{\"\\n\"}}{{end}}'", nsC)

	})
})
