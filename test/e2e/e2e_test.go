package e2e

import (
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const (
	propagationTime = 120
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
