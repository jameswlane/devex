package progress_test

import (
	"testing"

	"github.com/jameswlane/devex/apps/cli/internal/testhelper"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestProgress(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Progress Suite")
}

// Set up test logging suppression for all tests in this suite
var _ = BeforeEach(func() {
	testhelper.SuppressLogs()
})
