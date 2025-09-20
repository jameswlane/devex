package performance_test

import (
	"testing"

	"github.com/jameswlane/devex/apps/cli/internal/testhelper"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestPerformance(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Performance Suite")
}

// Set up test logging suppression for all tests in this suite
var _ = BeforeEach(func() {
	testhelper.SuppressLogs()
})
