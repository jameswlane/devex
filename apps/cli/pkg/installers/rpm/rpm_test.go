package rpm_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/jameswlane/devex/pkg/installers/rpm"
	"github.com/jameswlane/devex/pkg/mocks"
)

var _ = Describe("Rpm Installer", func() {
	Describe("NewRpmInstaller", func() {
		It("should create a new installer instance", func() {
			installer := rpm.NewRpmInstaller()
			Expect(installer).ToNot(BeNil())
		})
	})

	Describe("Installer Methods", func() {
		var installer *rpm.RpmInstaller
		var mockRepo *mocks.MockRepository

		BeforeEach(func() {
			installer = rpm.NewRpmInstaller()
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
			It("should work and return false when package not found", func() {
				// Note: IsInstalled is implemented to check RPM packages, so it doesn't return an error
				// when the system validation fails, it returns (false, nil) for not found packages
				installed, err := installer.IsInstalled("test-package")
				if err != nil {
					// This is expected if RPM system is not available in test environment
					GinkgoWriter.Printf("IsInstalled() returned error (expected in test env without RPM): %v", err)
				} else {
					// If no error, package should not be installed
					Expect(installed).To(BeFalse(), "IsInstalled() should return false for non-existent package")
				}
			})
		})
	})
})
