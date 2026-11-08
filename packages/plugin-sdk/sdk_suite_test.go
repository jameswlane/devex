package sdk_test

import (
	"os"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestSDK(t *testing.T) {
	if kind := os.Getenv(signalFixtureEnv); kind != "" {
		runSignalFixture(kind)
	}
	RegisterFailHandler(Fail)
	RunSpecs(t, "Plugin SDK Suite")
}

var _ = BeforeEach(func() { GinkgoT().Setenv("CI", ""); GinkgoT().Setenv("DEVEX_UNATTENDED", "") })
