package utils_test

import (
	"os"
	"testing"

	"github.com/jameswlane/devex/apps/cli/internal/testhelper"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestUtils(t *testing.T) {
	t.Parallel()
	RegisterFailHandler(Fail)
	RunSpecs(t, "Utils Suite")
}

// Set up test logging suppression for all tests in this suite
var _ = BeforeEach(func() {
	testhelper.SuppressLogs()
	// CI runners set CI=true, which turns on unattended mode and adds
	// DEVEX_UNATTENDED=1 to operation argv. Specs set it explicitly when they
	// cover unattended behaviour.
	// TestUtils runs in parallel with other tests, which rules out t.Setenv.
	for _, key := range []string{"CI", "DEVEX_UNATTENDED"} {
		previous, had := os.LookupEnv(key)
		Expect(os.Unsetenv(key)).To(Succeed())
		DeferCleanup(func() {
			if had {
				Expect(os.Setenv(key, previous)).To(Succeed())
			}
		})
	}
})
