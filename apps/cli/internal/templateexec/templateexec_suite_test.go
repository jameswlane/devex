package templateexec_test

import (
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestTemplateExec(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Template Execution Runtime Suite")
}

var _ = BeforeEach(func() {
	GinkgoT().Setenv("HOME", GinkgoT().TempDir())
	GinkgoT().Setenv("XDG_STATE_HOME", GinkgoT().TempDir())
	GinkgoT().Setenv("DEVEX_STATE_DIR", "")
})
