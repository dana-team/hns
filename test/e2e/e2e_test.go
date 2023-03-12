package e2e

import (
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const (
	namspacePrefix = "e2e-test-"
	// A 1s timeout was too short; 2s *seems* stable and also matches the Ginkgo default
	defTimeout = 2
	// For the operations that involves propagation, 5s seems to be a more stable time choice
	propagationTime = 5
	// For the operations that involves deletion, 10s seems to be a more stable time
	cleanupTimeout = 10
)

const (
	storage = "basic.storageclass.storage.k8s.io/requests.storage"
	cpu     = "cpu"
	memory  = "memory"
	pods    = "pods"
	gpu     = "requests.nvidia.com/gpu"
)

const rqDepth = 2

func TestE2e(t *testing.T) {
	RegisterFailHandler(Fail)

	SetDefaultEventuallyTimeout(time.Second * 2)
	RunSpecs(t, "HNS Suite")
}
