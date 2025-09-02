package xbps_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/jameswlane/devex/pkg/installers/xbps"
	"github.com/jameswlane/devex/pkg/mocks"
)

var _ = Describe("Xbps Installer", func() {
	Describe("NewXbpsInstaller", func() {
		It("should create a new installer instance", func() {
			installer := xbps.NewXbpsInstaller()
			Expect(installer).ToNot(BeNil())
		})
	})

	Describe("Installer Methods", func() {
		var installer *xbps.XbpsInstaller
		var mockRepo *mocks.MockRepository

		BeforeEach(func() {
			installer = xbps.NewXbpsInstaller()
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
