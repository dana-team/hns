package testutils

import (
	danav1 "github.com/dana-team/hns/api/v1"
	. "github.com/onsi/gomega"
	"strconv"
	"strings"
)

const (
	namspacePrefix  = "e2e-test-"
	propagationTime = 5
)

// GenerateName generates a name for a namespace and subnamespace
func GenerateE2EName(nm string) string {
	prefix := namspacePrefix + "subnamespace-"
	snsName := prefix + nm

	return snsName
}

// GenerateE2EUserName generates a name for a namespace and subnamespace
func GenerateE2EUserName(nm string) string {
	prefix := namspacePrefix + "user-"
	snsName := prefix + nm

	return snsName
}

// GrantTestingUserAdmin gives admin rolebinding to a user on a namespace
func GrantTestingUserAdmin(user, ns string) {
	MustRun("kubectl create rolebinding", "test-admin-"+user+"-"+ns, "--user", user, "--namespace", ns, "--clusterrole admin")
}

// AnnotateNSSecondaryRoot annotates a namespace as secondary root
func AnnotateNSSecondaryRoot(ns string) {
	MustRun("kubectl annotate --overwrite ns", ns, danav1.IsSecondaryRoot+"="+danav1.True)
}

// CreateRootNS creates/updates a root name with a given name
// and with the required labels
func CreateRootNS(nm string, rqDepth int) {
	rootNS := generateRootNSManifest(nm, strconv.Itoa(rqDepth))
	MustApplyYAML(rootNS)
	labelTestingNs(nm)
	RunShouldContain(nm, propagationTime, "kubectl get ns", nm)
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
func CreateSubnamespace(nm, nsnm string, isRp bool, args ...string) {
	sns := generateSNSManifest(nm, nsnm, strconv.FormatBool(isRp), args...)
	MustApplyYAML(sns)
	RunShouldContain(nm, propagationTime, "kubectl get subnamespace -n", nsnm)
	labelTestingNs(nm)
}

// ShouldNotCreateSubnamespace should not be able to create the specified Subnamespace
//in the parent namespace and with the given resources
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
func CreateUser(u string) {
	user := generateUserManifest(u)
	MustApplyYAML(user)
	RunShouldContain(u, propagationTime, "kubectl get users")
	labelTestingUsers(u)
}

// CreatePod creates the specified Pod
func CreatePod(name, ns string) {
	pod := generatePodManifest(name, ns)
	MustApplyYAML(pod)
	RunShouldContain(name, propagationTime, "kubectl get pods -n" + ns)
	labelTestingPods(pod)
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
    ` + danav1.Aggragator + nm + `: ` + nm + `
    ` + danav1.Hns + `: "true"` + `
  annotations:
    openshift.io/display-name: ` + nm + `
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
func generatePodManifest(name, ns string) string {
	return `# temp file created by user_pod.go
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
	- containerPort: 80`
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
