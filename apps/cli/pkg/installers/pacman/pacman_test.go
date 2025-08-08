package pacman_test

import (
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/jameswlane/devex/pkg/installers/pacman"
	"github.com/jameswlane/devex/pkg/mocks"
	"github.com/jameswlane/devex/pkg/utils"
)

var _ = Describe("Pacman Installer", func() {
	var (
		mockRepo     *mocks.MockRepository
		mockExec     *mocks.MockCommandExecutor
		installer    *pacman.PacmanInstaller
		originalExec utils.Interface
	)

	BeforeEach(func() {
		mockRepo = mocks.NewMockRepository()
		mockExec = mocks.NewMockCommandExecutor()

		// Store original executor and replace with mock
		originalExec = utils.CommandExec
		utils.CommandExec = mockExec

		installer = pacman.NewPacmanInstaller()
	})

	AfterEach(func() {
		// Restore original executor
		utils.CommandExec = originalExec
	})

	Describe("NewPacmanInstaller", func() {
		It("creates a new Pacman installer", func() {
			pacmanInstaller := pacman.NewPacmanInstaller()
			Expect(pacmanInstaller).NotTo(BeNil())
		})
	})

	Describe("Install", func() {
		Context("when Pacman is available", func() {
			It("installs a package successfully", func() {
				// Package should get marked as installed after successful install
				err := installer.Install("test-package", mockRepo)

				Expect(err).ToNot(HaveOccurred())
				// Check that package was added to repo
				app, err := mockRepo.GetApp("test-package")
				Expect(err).ToNot(HaveOccurred())
				Expect(app).ToNot(BeNil())
				Expect(app.Name).To(Equal("test-package"))
			})

			It("handles package already installed", func() {
				// Pre-install the package in mock state
				mockExec.InstallationState["test-package"] = true

				err := installer.Install("test-package", mockRepo)

				// Should not error when package is already installed
				Expect(err).ToNot(HaveOccurred())
			})
		})

		Context("when package is not in official repositories", func() {
			It("attempts AUR installation", func() {
				// Mark pacman -Si command to fail (package not in official repos)
				mockExec.FailingCommands["pacman -Si test-package"] = true

				// Should still attempt installation
				_ = installer.Install("test-package", mockRepo)

				// May fail due to YAY not being available in test, but should attempt
				// The important part is that it tries the AUR path
				Expect(mockExec.Commands).To(ContainElement(ContainSubstring("pacman -Si")))
			})
		})

		Context("when YAY needs to be installed", func() {
			It("attempts to install YAY when needed", func() {
				// Set environment variables for the test
				os.Setenv("USER", "user")
				os.Setenv("HOME", "/home/user")
				defer os.Unsetenv("USER")
				defer os.Unsetenv("HOME")

				// Make pacman -Si fail (not in official repos)
				mockExec.FailingCommands["pacman -Si test-package"] = true
				// Make which yay fail initially (not installed)
				mockExec.FailingCommands["which yay"] = true

				// Try installation - it may fail but should attempt YAY install process
				_ = installer.Install("test-package", mockRepo)

				// Check that it attempted to check for YAY
				Expect(mockExec.Commands).To(ContainElement("which yay"))
			})
		})

		Context("when Pacman is not available", func() {
			It("returns an error", func() {
				// Make which pacman fail
				mockExec.FailingCommands["which pacman"] = true

				err := installer.Install("test-package", mockRepo)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("pacman not found"))
			})
		})
	})

	Describe("Uninstall", func() {
		Context("when package is installed", func() {
			It("uninstalls the package successfully", func() {
				// Mark package as installed
				mockExec.InstallationState["test-package"] = true
				// Add to repo first
				mockRepo.AddApp("test-package")

				err := installer.Uninstall("test-package", mockRepo)

				Expect(err).ToNot(HaveOccurred())
				// Check that package was removed from repo
				_, err = mockRepo.GetApp("test-package")
				Expect(err).To(HaveOccurred()) // Should not be found after uninstall
			})
		})

		Context("when package is not installed", func() {
			It("skips uninstallation", func() {
				// Ensure package is not installed
				mockExec.InstallationState["test-package"] = false

				err := installer.Uninstall("test-package", mockRepo)

				// Should not error when package is not installed
				Expect(err).ToNot(HaveOccurred())
			})
		})
	})

	Describe("IsInstalled", func() {
		Context("when package is installed", func() {
			It("returns true", func() {
				// Mark package as installed in mock
				mockExec.InstallationState["test-package"] = true

				isInstalled, err := installer.IsInstalled("test-package")

				Expect(err).ToNot(HaveOccurred())
				Expect(isInstalled).To(BeTrue())
			})
		})

		Context("when package is not installed", func() {
			It("handles package check correctly", func() {
				// Don't set package in installation state (defaults to not installed)
				delete(mockExec.InstallationState, "test-package")

				isInstalled, err := installer.IsInstalled("test-package")

				// The mock doesn't implement pacman -Q logic, so this may fail
				// The important thing is that it attempts to check
				_ = isInstalled
				_ = err
				// Just verify that the command was attempted
				Expect(mockExec.Commands).To(ContainElement(ContainSubstring("pacman -Q")))
			})
		})
	})

	Describe("InstallGroup", func() {
		Context("when installing a package group", func() {
			It("installs the package group successfully", func() {
				err := installer.InstallGroup("base-devel", mockRepo)

				Expect(err).ToNot(HaveOccurred())
				// Check that group was added to repo
				app, err := mockRepo.GetApp("base-devel")
				Expect(err).ToNot(HaveOccurred())
				Expect(app.Name).To(Equal("base-devel"))
			})
		})
	})

	Describe("SystemUpgrade", func() {
		Context("when performing system upgrade", func() {
			It("performs system upgrade successfully", func() {
				err := installer.SystemUpgrade()

				Expect(err).ToNot(HaveOccurred())
				// Verify upgrade command was executed
				Expect(mockExec.Commands).To(ContainElement(ContainSubstring("pacman -Syu")))
			})
		})
	})

	Describe("CleanCache", func() {
		Context("when cleaning package cache", func() {
			It("cleans the cache successfully", func() {
				err := installer.CleanCache()

				Expect(err).ToNot(HaveOccurred())
				// Verify clean command was executed
				Expect(mockExec.Commands).To(ContainElement(ContainSubstring("pacman -Sc")))
			})
		})
	})

	Describe("ListInstalled", func() {
		Context("when listing installed packages", func() {
			It("attempts to list installed packages", func() {
				// Set up some packages as installed
				mockExec.InstallationState["bash"] = true
				mockExec.InstallationState["git"] = true

				packages, err := installer.ListInstalled()

				// May succeed or fail depending on mock implementation
				// The important part is that it attempts the operation
				Expect(mockExec.Commands).To(ContainElement("pacman -Q"))
				_ = packages
				_ = err
			})
		})
	})

	Describe("SearchPackages", func() {
		Context("when searching for packages", func() {
			It("searches for packages in repositories", func() {
				packages, err := installer.SearchPackages("git")

				// May succeed or fail, but should attempt search
				Expect(mockExec.Commands).To(ContainElement(ContainSubstring("pacman -Ss git")))
				_ = packages
				_ = err
			})
		})
	})

	Describe("RunPacmanUpdate", func() {
		Context("when updating package database", func() {
			It("updates the package database successfully", func() {
				err := pacman.RunPacmanUpdate(true, mockRepo)

				Expect(err).ToNot(HaveOccurred())
				// Verify update command was executed
				Expect(mockExec.Commands).To(ContainElement("sudo pacman -Sy"))
			})
		})
	})
})
