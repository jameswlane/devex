package nixflake_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/jameswlane/devex/pkg/installers/nixflake"
	"github.com/jameswlane/devex/pkg/mocks"
)

var _ = Describe("NixFlake Installer", func() {
	Describe("NewNixFlakeInstaller", func() {
		It("should create a new installer instance", func() {
			installer := nixflake.NewNixFlakeInstaller()
			Expect(installer).ToNot(BeNil())
		})
	})

	Describe("Installer Methods", func() {
		var installer *nixflake.NixFlakeInstaller
		var mockRepo *mocks.MockRepository

		BeforeEach(func() {
			installer = nixflake.NewNixFlakeInstaller()
			mockRepo = &mocks.MockRepository{}
		})

		Context("Install", func() {
			It("should return an error for unimplemented installer", func() {
				err := installer.Install("test-package", mockRepo)
				Expect(err).To(HaveOccurred())
			})
		})

		Context("Uninstall", func() {
			It("should return an error for unimplemented installer", func() {
				err := installer.Uninstall("test-package", mockRepo)
				Expect(err).To(HaveOccurred())
			})
		})

		Context("IsInstalled", func() {
			It("should return an error for unimplemented installer", func() {
				_, err := installer.IsInstalled("test-package")
				Expect(err).To(HaveOccurred())
			})
		})
	})
})
