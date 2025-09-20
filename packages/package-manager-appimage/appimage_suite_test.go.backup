package appimage

import (
	"testing"

	"github.com/jameswlane/devex/pkg/testhelper"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/jameswlane/devex/pkg/fs"
)

func TestAppImage(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "AppImage Installer Suite")
}

var originalFilesystem interface{}

var _ = BeforeSuite(func() {
	// Store the original filesystem
	originalFilesystem = fs.AppFs
})

var _ = AfterSuite(func() {
	// Restore the original filesystem
	if originalFilesystem != nil {
		fs.UseOsFs()
	}
})

// Set up test logging suppression for all tests in this suite
var _ = BeforeEach(func() {
	testhelper.SuppressLogs()
})
