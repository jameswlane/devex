package snap_test

import (
	"testing"

	"github.com/jameswlane/devex/pkg/testhelper"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestSnap(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Snap Installer Suite")
}

var _ = BeforeSuite(func() {
	// Suite-wide setup if needed
})

var _ = AfterSuite(func() {
	// Suite-wide cleanup if needed
})

// Set up test logging suppression for all tests in this suite
var _ = BeforeEach(func() {
	testhelper.SuppressLogs()
})
