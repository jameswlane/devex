package installers_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/jameswlane/devex/pkg/config"
	"github.com/jameswlane/devex/pkg/installers"
	"github.com/jameswlane/devex/pkg/mocks"
	"github.com/jameswlane/devex/pkg/types"
	"github.com/jameswlane/devex/pkg/utils"
)

var _ = Describe("Installers Package", func() {
	var (
		repo      *mocks.MockRepository
		mockUtils *mocks.MockUtils
		settings  config.Settings
	)

	BeforeEach(func() {
		repo = mocks.NewMockRepository()
		mockUtils = mocks.NewMockUtils()
		utils.CommandExec = mockUtils // Replace the real CommandExec with the mock
		settings = config.Settings{}
	})

	Describe("InstallApp", func() {
		It("returns an error for an unsupported install method", func() {
			app := types.AppConfig{
				Name:           "unsupported-app",
				InstallMethod:  "unsupported",
				InstallCommand: "some command",
			}

			err := installers.InstallApp(app, settings, repo)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("unsupported install method"))
		})
	})

	Describe("RemoveConflictingPackages", func() {
		It("removes conflicting packages successfully", func() {
			conflicts := []string{"conflict1", "conflict2"}
			err := installers.RemoveConflictingPackages(conflicts)
			Expect(err).ToNot(HaveOccurred())

			// Verify the correct command was executed
			Expect(mockUtils.Commands).To(ContainElement("sudo apt-get remove -y conflict1 conflict2"))
		})
	})
})
