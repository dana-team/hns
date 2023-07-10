package testutils

import (
	danav1 "github.com/dana-team/hns/api/v1"
	. "github.com/onsi/gomega"
	"math/rand"
	"strconv"
	"strings"
	"time"
)

const (
	namspacePrefix   = "e2e"
	randStringLength = 8

	// we use 120 seconds here because some tests run in parallel and there may be heavy load and time may be needed
	propagationTime   = 120
	eventuallyTimeout = 120
)

// GenerateE2EName generates a name for a namespace and subnamespace
func GenerateE2EName(nm, testPrefix, randPrefix string) string {
	prefix := namspacePrefix + "-" + testPrefix + "-" + randPrefix + "-"
	snsName := prefix + nm

	return snsName
}

// GenerateE2EUserName generates a name for a namespace and subnamespace
func GenerateE2EUserName(nm string) string {
	prefix := namspacePrefix + "-" + RandStr() + "-user-"
	snsName := prefix + nm

	return snsName
}

// RandStr generates a random string
func RandStr() string {
	rand.Seed(time.Now().UnixNano())

	var charset = []byte("abcdefghijklmnopqrstuvwxyz0123456789")

	b := make([]byte, randStringLength)
	for i := range b {
		// randomly select 1 character from given charset
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}

// GrantTestingUserAdmin gives admin rolebinding to a user on a namespace
func GrantTestingUserAdmin(user, ns string) {
	MustRun("kubectl create rolebinding", "test-admin-"+user+"-"+ns, "--user", user, "--namespace", ns, "--clusterrole admin")
}

// AnnotateNSSecondaryRoot annotates a namespace as secondary root
func AnnotateNSSecondaryRoot(ns string) {
	MustRun("kubectl annotate --overwrite ns", ns, danav1.IsSecondaryRoot+"="+danav1.True)
}

// AnnotateNSDefaultAnnotation annotates a namespace with the default annotations
func AnnotateNSDefaultAnnotation(ns string) {
	MustRun("kubectl annotate --overwrite ns", ns, danav1.DefaultAnnotations[0]+"='[{\"key\":\"test\",\"value\":\"true\",\"effect\":\"NoSchedule\"}]'")
	MustRun("kubectl annotate --overwrite ns", ns, danav1.DefaultAnnotations[1]+"="+"testServer")
}

// CreateRootNS creates/updates a root name with a given name
// and with the required labels
func CreateRootNS(nm, randPrefix string, rqDepth int) {
	rootNS := generateRootNSManifest(nm, strconv.Itoa(rqDepth))
	MustApplyYAML(rootNS)
	RunShouldContain(nm, propagationTime, "kubectl get ns", nm)
	LabelTestingNs(nm, randPrefix)

}

// CreateResourceQuota creates/updates a ResourceQuota object in a given
// namespace and with the given resources.
func CreateResourceQuota(nm, nsnm string, args ...string) {
	rq := generateRQManifest(nm, nsnm, args...)
	MustApplyYAML(rq)
	RunShouldContain(nm, propagationTime, "kubectl get resourcequota -n", nsnm)
}

// CreateSubnamespace creates/updates the specified Subnamespace in the parent namespace with canned testing
// labels making it easier to look up and delete later, and with the given resources
func CreateSubnamespace(nm, nsnm, randPrefix string, isRp bool, args ...string) {
	sns := generateSNSManifest(nm, nsnm, strconv.FormatBool(isRp), args...)
	MustApplyYAML(sns)
	RunShouldContain(nm, propagationTime, "kubectl get subnamespace -n", nsnm)
	RunShouldContain(nm, propagationTime, "kubectl get namespace")
	FieldShouldContain("subnamespace", nsnm, nm, ".metadata.annotations", danav1.CrqPointer)
	LabelTestingNs(nm, randPrefix)
}

// ShouldNotCreateSubnamespace should not be able to create the specified Subnamespace
// in the parent namespace and with the given resources
func ShouldNotCreateSubnamespace(nm, nsnm string, isRp bool, args ...string) {
	sns := generateSNSManifest(nm, nsnm, strconv.FormatBool(isRp), args...)
	MustNotApplyYAML(sns)
	RunShouldNotContain(nm, propagationTime, "kubectl get subnamespace -n", nsnm)
}

// ShouldNotUpdateSubnamespace should not be able to create the specified Subnamespace
// in the parent namespace and with the given resources
func ShouldNotUpdateSubnamespace(nm, nsnm string, isRp bool, args ...string) {
	sns := generateSNSManifest(nm, nsnm, strconv.FormatBool(isRp), args...)
	MustNotApplyYAML(sns)
}

// CreateUpdateQuota creates the specified UpdateQuota in the parent namespace with canned testing
// labels making it easier to look up and delete later, and with the given resources
func CreateUpdateQuota(nm, nsnm, dsnm, user string, args ...string) {
	upq := generateUPQManifest(nm, nsnm, dsnm, args...)
	if user != "" {
		MustApplyYAMLAsUser(upq, user)
	} else {
		MustApplyYAML(upq)

	}
	MustApplyYAMLAsUser(upq, user)
	RunShouldContain(nm, propagationTime, "kubectl get updatequota -n", nsnm)
}

// CreateMigrationHierarchy creates the specified MigrationHierarchy
func CreateMigrationHierarchy(currentns, tons string) string {
	name := "from" + currentns + "to" + tons
	mh := generateMigrartionHierarchyManifest(name, currentns, tons)
	MustApplyYAML(mh)
	RunShouldContain(name, propagationTime, "kubectl get migrationhierarchy")
	return name
}

// ShouldNotCreateMigrationHierarchy should not be able to create the specified MigrationHierarchy
func ShouldNotCreateMigrationHierarchy(currentns, tons string) {
	name := "from" + currentns + "to" + tons
	mh := generateMigrartionHierarchyManifest(name, currentns, tons)
	MustNotApplyYAML(mh)
	RunShouldNotContain(name, propagationTime, "kubectl get migrationhierarchy")
}

// ShouldNotCreateUpdateQuota should not be able to create the specified UpdateQuota
// in the parent namespace and with the given resources
func ShouldNotCreateUpdateQuota(nm, nsnm, dsnm, user string, args ...string) {
	upq := generateUPQManifest(nm, nsnm, dsnm, args...)
	if user != "" {
		MustNotApplyYAMLAsUser(upq, user)
	} else {
		MustNotApplyYAML(upq)

	}
	RunShouldNotContain(nm, propagationTime, "kubectl get updatequota -n", nsnm)
}

// CreateUser creates the specified User
func CreateUser(u, randPrefix string) {
	user := generateUserManifest(u)
	MustApplyYAML(user)
	RunShouldContain(u, propagationTime, "kubectl get users")
	labelTestingUsers(u, randPrefix)
}

// CreatePod creates a pod in the specified namespace with the required cpu and memory(Gi)
func CreatePod(ns, name, randPrefix, cpu, memory string) {
	pod := generatePodManifest(ns, name, cpu, memory)
	MustApplyYAML(pod)
	RunShouldContain(name, propagationTime, "kubectl get pod -n", ns)
	LabelTestingNs(ns, randPrefix)
}

// generateRootNSManifest generates a namespace manifest with the
// parameters needed to indicate a root namespace
func generateRootNSManifest(nm string, rqDepth string) string {
	return `# temp file created by root_ns_test.go
apiVersion: v1
kind: Namespace
metadata:
  name: ` + nm + `
  labels:
    ` + danav1.Hns + `: "true"` + `
  annotations:
    ` + danav1.DisplayName + `: ` + nm + `
    ` + danav1.OpenShiftDisplayName + `: ` + nm + `
    ` + danav1.Role + `: ` + danav1.Root + `
    ` + danav1.RootCrqSelector + `: ` + nm + `
    ` + danav1.Depth + `: "0"` + `
    ` + danav1.RqDepth + `: ` + `"` + rqDepth + `"`
}

// generateSNSManifest generates a Subnamespace manifest
func generateSNSManifest(nm, nsnm, isRp string, args ...string) string {
	return `# temp file created by sns_test.go
apiVersion: dana.hns.io/v1
kind: Subnamespace
metadata:
  name: ` + nm + `
  namespace: ` + nsnm + `
  labels:
    ` + danav1.ResourcePool + `: "` + isRp + `"
spec:
  resourcequota:
    hard: ` + argsToResourceListString(4, args...)
}

// generateRQManifest generates a ResourceQuota manifest
func generateRQManifest(nm, nsnm string, args ...string) string {
	return `# temp file created by rq_test.go
apiVersion: v1
kind: ResourceQuota
metadata:
  name: ` + nm + `
  namespace: ` + nsnm + `
spec:
  hard: ` + argsToResourceListString(2, args...)
}

// generateUPQManifest generates an UpdateQuota manifest
func generateUPQManifest(nm, nsnm, dsnm string, args ...string) string {
	return `# temp file created by upq_test.go
apiVersion: dana.hns.io/v1
kind: Updatequota
metadata:
  name: ` + nm + `
  namespace: ` + nsnm + `
spec:
  destns: ` + dsnm + `
  sourcens: ` + nsnm + `
  resourcequota:
    hard: ` + argsToResourceListString(4, args...)
}

// generateMigrartionHierarchyManifest generates an MigrartionHierarchy manifest
func generateMigrartionHierarchyManifest(nm, currentns, tons string) string {
	return `# temp file created by migrationhierarchy_test.go
apiVersion: dana.hns.io/v1
kind: MigrationHierarchy
metadata:
  name: ` + nm + `
spec:
  currentns: ` + currentns + `
  tons: ` + tons
}

// generateUserManifest generates an User manifest
func generateUserManifest(nm string) string {
	return `# temp file created by user_test.go
apiVersion: user.openshift.io/v1
kind: User
metadata:
  name: ` + nm + `
groups: 
  - e2e-test`
}

// generatePodManifest generates an Pod manifest
func generatePodManifest(ns, name, cpu, memory string) string {
	return `# temp file created by pod_test.go
apiVersion: v1
kind: Pod
metadata:
  name: ` + name + `
  namespace: ` + ns + `
spec:
  containers:
  - name: nginx
    image: nginx:1.14.2
    ports:
    - containerPort: 80
    resources:
      requests:
        cpu: ` + cpu + `
        memory: ` + memory + `Gi
      limits:
        cpu: ` + cpu + `
        memory: ` + memory + `Gi
`
}

// argsToResourceListString provides a convenient way to specify a resource list
// in hard limits/usages for RQ instances, or limits/requests for pod
// containers by interpreting even-numbered args as resource names (e.g.
// "secrets") and odd-valued args as quantities (e.g. "5", "1Gb", etc). The
// level controls how much indention in the output (0 indicates no indention).
func argsToResourceListString(level int, args ...string) string {
	Expect(len(args)%2).Should(Equal(0), "Need even number of arguments, not %d", len(args))
	rl := ``
	for i := 0; i < len(args); i += 2 {
		rl += `
` + strings.Repeat("  ", level) + args[i] + `: "` + args[i+1] + `"`
	}
	return rl
}
