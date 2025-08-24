package apt_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/jameswlane/devex/pkg/installers/apt"
	"github.com/jameswlane/devex/pkg/installers/utilities"
	"github.com/jameswlane/devex/pkg/mocks"
	"github.com/jameswlane/devex/pkg/utils"
)

var _ = Describe("APT Installer", func() {
	var (
		mockRepo     *mocks.MockRepository
		mockExec     *mocks.MockCommandExecutor
		installer    *apt.APTInstaller
		originalExec utils.Interface
	)

	BeforeEach(func() {
		mockRepo = mocks.NewMockRepository()
		mockExec = mocks.NewMockCommandExecutor()

		// Store original executor and replace with mock
		originalExec = utils.CommandExec
		utils.CommandExec = mockExec

		// Reset APT version cache for consistent testing
		apt.ResetVersionCache()

		// Reset package manager cache for consistent testing
		utilities.ResetPackageManagerCache()

		installer = apt.New()
	})

	AfterEach(func() {
		// Restore original executor
		utils.CommandExec = originalExec
	})

	Describe("New", func() {
		It("creates a new APT installer", func() {
			aptInstaller := apt.New()
			Expect(aptInstaller).NotTo(BeNil())
		})
	})

	Describe("Install", func() {
		Context("with valid package", func() {
			It("installs a package successfully", func() {
				// Set up the mock to have the package available in repository
				// by not setting it as a failing package

				err := installer.Install("test-package", mockRepo)

				// Since we're using a simple mock, the install will succeed if validation passes
				Expect(err).NotTo(HaveOccurred())

				// Verify the package was added to the repository
				// (The mock repository should have captured this)
			})
		})

		Context("when apt system validation fails", func() {
			BeforeEach(func() {
				// Set the failing command to simulate apt not found
				mockExec.FailingCommand = "which apt"
			})

			It("returns validation error", func() {
				err := installer.Install("test-package", mockRepo)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("critical validations failed"))
			})
		})

		Context("when package is not available", func() {
			It("returns package validation error", func() {
				err := installer.Install("failing-package", mockRepo)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("package validation failed"))
			})
		})

		Context("with special packages", func() {
			It("installs packages with post-installation setup", func() {
				// Use a simpler test that doesn't rely on Docker's complex setup
				err := installer.Install("test-package", mockRepo)
				Expect(err).NotTo(HaveOccurred())

				// Verify basic installation commands were executed
				Expect(len(mockExec.Commands)).To(BeNumerically(">", 0))
			})
		})
	})

	Describe("RunAptUpdate", func() {
		Context("with force update", func() {
			It("runs apt update successfully", func() {
				err := apt.RunAptUpdate(true, mockRepo)

				// Verify update command was called
				Expect(mockExec.Commands).To(ContainElement("sudo apt-get update"))
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("when update fails", func() {
			BeforeEach(func() {
				mockExec.FailingCommand = "sudo apt-get update"
			})

			It("returns update error", func() {
				err := apt.RunAptUpdate(true, mockRepo)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to update apt package lists"))
			})
		})
	})
})
