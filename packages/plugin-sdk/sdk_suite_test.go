package sdk_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestSDK(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Plugin SDK Suite")
}

var _ = BeforeEach(func() { GinkgoT().Setenv("CI", ""); GinkgoT().Setenv("DEVEX_UNATTENDED", "") })
