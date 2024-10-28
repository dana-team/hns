package testutils

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// The testing label marked on all namespaces created using the testing phase, offering ease when doing cleanups
const testingNamespaceLabel = "dana.hns.io/testNamespace"
const testingMigrationHierarchyLabel = "dana.hns.io/testMigrationHierarchy"
const testingUserLabel = "dana.hns.io/testUser"
const testingGroupLabel = "dana.hns.io/testGroup"
const testingServiceAccountLabel = "dana.hns.io/testServiceAccount"
const notFound = "NotFound"

func FieldShouldContain(resource, ns, nm, field, want string) {
	fieldShouldContainMultipleWithTimeout(1, resource, ns, nm, field, []string{want}, eventuallyTimeout)
}

func ComplexFieldShouldContain(resource, ns, nm, field, want string) {
	complexFieldShouldContainMultipleWithTimeout(1, resource, ns, nm, field, []string{want}, eventuallyTimeout)
}

func ShouldNotExist(resource, ns, want string) {
	EventuallyWithOffset(1, func() bool {
		err := TryRun("kubectl get", resource, want, "-n", ns)
		if err == nil {
			return false
		}
		return strings.Contains(err.Error(), notFound)
	}, eventuallyTimeout).Should(BeTrue())
}

func fieldShouldContainMultipleWithTimeout(offset int, resource, ns, nm, field string, want []string, timeout float64) {
	if ns != "" {
		runShouldContainMultiple(offset+1, want, timeout, "kubectl get", resource, nm, "-n", ns, "-o template --template={{"+field+"}}")
	} else {
		runShouldContainMultiple(offset+1, want, timeout, "kubectl get", resource, nm, "-o template --template={{"+field+"}}")
	}
}

func complexFieldShouldContainMultipleWithTimeout(offset int, resource, ns, nm, field string, want []string, timeout float64) {
	if ns != "" {
		runShouldContainMultiple(offset+1, want, timeout, "kubectl get", resource, nm, "-n", ns, "-o template --template="+field)
	} else {
		runShouldContainMultiple(offset+1, want, timeout, "kubectl get", resource, nm, "-o template --template="+field)
	}
}

func FieldShouldNotContain(resource, ns, nm, field, want string) {
	fieldShouldNotContainMultipleWithTimeout(1, resource, ns, nm, field, []string{want}, eventuallyTimeout)
}

func ComplexFieldShouldNotContain(resource, ns, nm, field, want string) {
	complexFieldShouldNotContainMultipleWithTimeout(1, resource, ns, nm, field, []string{want}, eventuallyTimeout)
}

func fieldShouldNotContainMultipleWithTimeout(offset int, resource, ns, nm, field string, want []string, timeout float64) {
	if ns != "" {
		runShouldNotContainMultiple(offset+1, want, timeout, "kubectl get", resource, nm, "-n", ns, "-o template --template={{"+field+"}}")
	} else {
		runShouldNotContainMultiple(offset+1, want, timeout, "kubectl get", resource, nm, "-o template --template={{"+field+"}}")
	}
}

func complexFieldShouldNotContainMultipleWithTimeout(offset int, resource, ns, nm, field string, want []string, timeout float64) {
	if ns != "" {
		runShouldNotContainMultiple(offset+1, want, timeout, "kubectl get", resource, nm, "-n", ns, "-o template --template="+field)
	} else {
		runShouldNotContainMultiple(offset+1, want, timeout, "kubectl get", resource, nm, "-o template --template="+field)
	}
}

func MustRun(cmdln ...string) {
	mustRunWithTimeout(1, eventuallyTimeout, cmdln...)
}

func mustRunWithTimeout(offset int, timeout float64, cmdln ...string) {
	EventuallyWithOffset(offset+1, func() error {
		return TryRun(cmdln...)
	}, timeout).Should(Succeed(), "Command: %s", cmdln)
}

func MustNotRun(cmdln ...string) {
	mustNotRun(1, cmdln...)
}

func mustNotRun(offset int, cmdln ...string) {
	ConsistentlyWithOffset(offset+1, func() error {
		return TryRun(cmdln...)
	}).ShouldNot(BeNil(), "Command: %s", cmdln)
}

func TryRun(cmdln ...string) error {
	stdout, err := RunCommand(cmdln...)
	if err != nil {
		// Add stdout to the error, since it's the error that gets displayed when a test fails, and it
		// can be very hard looking at the log to see which failures are intended and which are not.
		err = fmt.Errorf("Error: %s\nOutput: %s", err, stdout)
		GinkgoT().Log("Output (failed): ", err)
	} else {
		GinkgoT().Log("Output (passed): ", stdout)
	}
	return err
}

func TryRunQuietly(cmdln ...string) error {
	_, err := RunCommand(cmdln...)
	return err
}

func RunShouldContain(substr string, seconds float64, cmdln ...string) {
	runShouldContainMultiple(1, []string{substr}, seconds, cmdln...)
}

func runShouldContainMultiple(offset int, substrs []string, seconds float64, cmdln ...string) {
	EventuallyWithOffset(offset+1, func() string {
		missing, err := tryRunShouldContainMultiple(substrs, cmdln...)
		if err != nil {
			return "failed: " + err.Error()
		}
		return missing
	}, seconds).Should(beQuiet(), "Command: %s", cmdln)
}

func tryRunShouldContainMultiple(substrs []string, cmdln ...string) (string, error) {
	stdout, err := RunCommand(cmdln...)
	GinkgoT().Log("Output: ", stdout)
	return missAny(substrs, stdout), err
}

// If any of the substrs are missing from teststring, returns a string of the form:
//
//	did not output the expected substring(s): <string1>, <string2>, ...
//	and instead output: teststring
//
// Otherwise returns the empty string.
func missAny(substrs []string, teststring string) string {
	var missing []string
	for _, substr := range substrs {
		if !strings.Contains(teststring, substr) {
			missing = append(missing, substr)
		}
	}
	if len(missing) == 0 {
		return ""
	}
	// This looks *ok* if we're only missing a single multiline string, and ok if we're missing
	// multiple single-line strings. It would look awful if we were missing multiple multiline strings,
	// but that's pretty rare.
	msg := "did not output the expected substring(s): " + strings.Join(missing, ", ") + "\n"
	msg += "and instead output: " + teststring
	return msg
}

func RunShouldNotContain(substr string, seconds float64, cmdln ...string) {
	runShouldNotContain(1, substr, seconds, cmdln...)
}

func runShouldNotContain(offset int, substr string, seconds float64, cmdln ...string) {
	runShouldNotContainMultiple(offset+1, []string{substr}, seconds, cmdln...)
}

func runShouldNotContainMultiple(offset int, substrs []string, seconds float64, cmdln ...string) {
	EventuallyWithOffset(offset+1, func() string {
		stdout, err := RunCommand(cmdln...)
		if err != nil {
			return "failed: " + err.Error()
		}

		for _, substr := range substrs {
			if strings.Contains(stdout, substr) {
				return fmt.Sprintf("included the undesired output %q:\n%s", substr, stdout)
			}
		}

		return ""
	}, seconds).Should(beQuiet(), "Command: %s", cmdln)
}

func MustApplyYAML(s string) {
	filename := writeTempFile(s)
	defer removeFile(filename)
	MustRun("kubectl apply -f", filename)
}

func MustNotApplyYAML(s string) {
	filename := writeTempFile(s)
	defer removeFile(filename)
	MustNotRun("kubectl apply -f", filename)
}

func MustApplyYAMLAsUser(s, u string) {
	filename := writeTempFile(s)
	defer removeFile(filename)
	MustRun("kubectl apply -f", filename, "--as", u)
}

func MustNotApplyYAMLAsUser(s, u string) {
	filename := writeTempFile(s)
	defer removeFile(filename)
	MustNotRun("kubectl apply -f", filename, "--as", u)
}

func ShouldDelete(resource, ns, nm string) {
	MustRun("kubectl delete", resource, nm, "-n", ns)
	ShouldNotExist(resource, ns, nm)
}

// RunCommand passes all arguments to the OS to execute, and returns the combined stdout/stderr
// and error object. By default, each arg to this function may contain strings (e.g. "echo hello
// world"), in which case we split the strings on the spaces (so this would be equivalent to calling
// "echo", "hello", "world"). If you _actually_ need an OS argument with strings in it, pass it as
// an argument to this function surrounded by double quotes (e.g. "echo", "\"hello world\"" will be
// passed to the OS as two args, not three).
func RunCommand(cmdln ...string) (string, error) {
	var args []string
	for _, subcmdln := range cmdln {
		// Any arg that starts and ends in a double quote shouldn't be split further
		if len(subcmdln) > 2 && subcmdln[0] == '"' && subcmdln[len(subcmdln)-1] == '"' {
			args = append(args, subcmdln[1:len(subcmdln)-1])
		} else {
			args = append(args, strings.Split(subcmdln, " ")...)
		}
	}
	prefix := fmt.Sprintf("[%d] Running: ", time.Now().Unix())
	GinkgoT().Log(prefix, args)
	cmd := exec.Command(args[0], args[1:]...)
	// Work around https://github.com/kubernetes/kubectl/issues/1098#issuecomment-929743957:
	cmd.Env = append(os.Environ(), "KUBECTL_COMMAND_HEADERS=false")
	stdout, err := cmd.CombinedOutput()
	return string(stdout), err
}

// LabelTestingNs marks testing namespaces with a label for future search and lookup.
func LabelTestingNs(ns, randPrefix string) {
	MustRun("kubectl label --overwrite ns", ns, randPrefix+"-"+testingNamespaceLabel+"=true")
}

// LabelTestingMigrationHierarchies marks testing migrationhierarchies with a label for future search and lookup.
func LabelTestingMigrationHierarchies(mh, randPrefix string) {
	MustRun("kubectl label --overwrite migrationhierarchy", mh, randPrefix+"-"+testingMigrationHierarchyLabel+"=true")
}

// labelTestingGroups marks testing groups with a label for future search and lookup.
func labelTestingGroup(group, randPrefix string) {
	MustRun("kubectl label --overwrite group", group, randPrefix+"-"+testingGroupLabel+"=true")
}

// labelTestingUsers marks testing users with a label for future search and lookup.
func labelTestingUsers(user, randPrefix string) {
	MustRun("kubectl label --overwrite user", user, randPrefix+"-"+testingUserLabel+"=true")
}

// labelTestingServiceAccount marks testing service accounts with a label for future search and lookup.
func labelTestingServiceAccount(sa, ns, randPrefix string) {
	MustRun("kubectl label --overwrite sa", sa, "-n", ns, randPrefix+"-"+testingServiceAccountLabel+"=true")
}

// CleanupTestNamespaces finds the list of namespaces labeled as test namespaces and delegates
// to cleanupNamespaces function.
func CleanupTestNamespaces(randPrefix string) {
	var nses []string
	EventuallyWithOffset(1, func() error {
		LabelQuery := randPrefix + "-" + testingNamespaceLabel + "=true"
		out, err := RunCommand("kubectl get ns -o custom-columns=:.metadata.name --no-headers=true --sort-by=.metadata.creationTimestamp", "-l", LabelQuery)
		if err != nil {
			return err
		}
		// reverse the order of the slice to ensure LIFO behavior in deletion
		nses = reverseSlice(strings.Split(out, "\n"))
		return nil
	}).Should(Succeed(), "while getting list of namespaces to clean up")
	cleanupNamespaces(nses...)
}

// CleanupTestMigrationHierarchies finds the list of migrationhierarchies labeled as test migrationhierarchies and delegates
// to cleanupMigrationHierarchies function.
func CleanupTestMigrationHierarchies(randPrefix string) {
	var mh []string
	EventuallyWithOffset(1, func() error {
		LabelQuery := randPrefix + "-" + testingMigrationHierarchyLabel + "=true"
		out, err := RunCommand("kubectl get migrationhierarchies -o custom-columns=:.metadata.name --no-headers=true", "-l", LabelQuery)
		if err != nil {
			return err
		}
		// reverse the order of the slice to ensure LIFO behavior in deletion
		mh = reverseSlice(strings.Split(out, "\n"))
		return nil
	}).Should(Succeed(), "while getting list of migrationhierarchies to clean up")
	cleanupMigrationHierarchies(mh...)
}

// CleanupTestUsers finds the list of users labeled as test namespaces and delegates
// to cleanupUsers function
func CleanupTestUsers(randPrefix string) {
	var users []string
	EventuallyWithOffset(1, func() error {
		LabelQuery := randPrefix + "-" + testingUserLabel + "=true"
		out, err := RunCommand("kubectl get users -o custom-columns=:.metadata.name --no-headers=true", "-l", LabelQuery)
		if err != nil {
			return err
		}
		users = strings.Split(out, "\n")
		return nil
	}).Should(Succeed(), "while getting list of users to clean up")
	cleanupUsers(users...)
}

// CleanupTestGroup deletes a specific group
func CleanupTestGroup(groupName string) {
	_, err := RunCommand("kubectl get groups -o custom-columns=:.metadata.name --no-headers=true", groupName)
	if err != nil {
		return
	}
	cleanupGroup(groupName)
}

// reverseSlice takes a slice and returns it in reverse order
func reverseSlice(nss []string) []string {
	for i, j := 0, len(nss)-1; i < j; i, j = i+1, j-1 {
		nss[i], nss[j] = nss[j], nss[i]
	}
	return nss
}

// cleanupNamespaces does everything it can to delete the passed-in namespaces
func cleanupNamespaces(nses ...string) {
	var toDelete []string // exclude missing namespaces
	for _, ns := range nses {
		// Skip any namespace that doesn't actually exist. We only check once (e.g. no retries on
		// errors) but reads are usually pretty reliable.

		if err := TryRunQuietly("kubectl get ns", ns); err != nil {
			continue
		}
		toDelete = append(toDelete, ns)
	}

	// Now, actually delete them
	for _, ns := range toDelete {
		err := TryRun("kubectl delete ns", ns)
		Expect(err).Should(BeNil())
	}
}

// cleanupMigrationHierarchies does everything it can to delete the passed-in migrationhierarchies
func cleanupMigrationHierarchies(mhs ...string) {
	var toDelete []string
	for _, mh := range mhs {

		if err := TryRunQuietly("kubectl get migrationhierarchy", mh); err != nil {
			continue
		}
		toDelete = append(toDelete, mh)
	}

	// Now, actually delete them
	for _, mh := range toDelete {
		err := TryRun("kubectl delete migrationhierarchy", mh)
		Expect(err).Should(BeNil())
	}
}

// cleanupUsers does everything it can to delete the passed-in namespaces
func cleanupUsers(users ...string) {
	var toDelete []string // exclude missing namespaces
	for _, user := range users {
		// Skip any namespace that doesn't actually exist. We only check once (e.g. no retries on
		// errors) but reads are usually pretty reliable.

		if err := TryRunQuietly("kubectl get users", user); err != nil {
			continue
		}
		toDelete = append(toDelete, user)
	}

	// Now, actually delete them
	for _, user := range toDelete {
		err := TryRun("kubectl delete user", user)
		Expect(err).Should(BeNil())
	}
}

// cleanupGroups does everything it can to delete the passed-in namespaces
func cleanupGroup(group string) {

	err := TryRun("kubectl delete group", group)
	Expect(err).Should(BeNil())
}
func writeTempFile(cxt string) string {
	f, err := os.CreateTemp(os.TempDir(), "e2e-test-*.yaml")
	Expect(err).Should(BeNil())
	defer func() {
		err := f.Close()
		Expect(err).Should(BeNil())
	}()

	_, err = io.WriteString(f, cxt)
	Expect(err).Should(BeNil())

	return f.Name()
}

func removeFile(path string) {
	Expect(os.Remove(path)).Should(BeNil())
}

// silencer is a matcher that assumes that an empty string is good, and any
// non-empty string means that test failed. You use it by saying
// `Should(beQuiet())` instead of `Should(Equal(""))`, which both looks
// moderately nicer in the code but more importantly produces much nicer error
// messages if it fails. You should never say `ShouldNot(beQuiet())`.
//
// See https://onsi.github.io/gomega/#adding-your-own-matchers for details.
type silencer struct{}

func beQuiet() silencer { return silencer{} }
func (_ silencer) Match(actual interface{}) (bool, error) {
	diffs := actual.(string)
	return diffs == "", nil
}
func (_ silencer) FailureMessage(actual interface{}) string {
	return actual.(string)
}
func (_ silencer) NegatedFailureMessage(actual interface{}) string {
	return "!!!! you should not put beQuiet() in a ShouldNot matcher !!!!" + actual.(string)
}
