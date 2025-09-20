package installers_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/installers"
	"github.com/jameswlane/devex/apps/cli/internal/mocks"
	"github.com/jameswlane/devex/apps/cli/internal/types"
	"github.com/jameswlane/devex/apps/cli/internal/utils"
)

var _ = Describe("Installers Package", func() {
	var (
		repo      *mocks.MockRepository
		mockUtils *mocks.MockUtils
		settings  config.CrossPlatformSettings
	)

	BeforeEach(func() {
		repo = mocks.NewMockRepository()
		mockUtils = mocks.NewMockUtils()
		utils.CommandExec = mockUtils // Replace the real CommandExec with the mock
		settings = config.CrossPlatformSettings{}
	})

	Describe("InstallApp", func() {
		It("returns an error for an unsupported install method", func() {
			app := types.AppConfig{
				BaseConfig: types.BaseConfig{
					Name: "unsupported-app",
				},
				InstallMethod:  "unsupported",
				InstallCommand: "some command",
			}

			err := installers.InstallApp(app, settings, repo)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("is not supported on this platform"))
		})
	})

})
