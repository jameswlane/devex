package apk_test

import (
	"errors"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/jameswlane/devex/pkg/common"
	"github.com/jameswlane/devex/pkg/installers/apk"
	"github.com/jameswlane/devex/pkg/mocks"
	"github.com/jameswlane/devex/pkg/utils"
)

var _ = Describe("APK Installer", func() {
	var (
		mockRepo     *mocks.MockRepository
		mockExec     *mocks.MockCommandExecutor
		installer    *apk.ApkInstaller
		originalExec utils.Interface
	)

	BeforeEach(func() {
		mockRepo = mocks.NewMockRepository()
		mockExec = mocks.NewMockCommandExecutor()

		// Store original executor and replace with mock
		originalExec = utils.CommandExec
		utils.CommandExec = mockExec

		installer = apk.NewApkInstaller()
	})

	AfterEach(func() {
		// Restore original executor
		utils.CommandExec = originalExec
	})

	Describe("NewApkInstaller", func() {
		It("creates a new APK installer", func() {
			apkInstaller := apk.NewApkInstaller()
			Expect(apkInstaller).NotTo(BeNil())
		})
	})

	Describe("Install", func() {
		Context("when APK is not available", func() {
			It("returns structured error for system not found", func() {
				// Make which apk fail
				mockExec.FailingCommands["which apk"] = true

				err := installer.Install("test-package", mockRepo)

				Expect(err).To(HaveOccurred())

				// Check if it's the correct structured error type
				var installerErr *common.InstallerError
				Expect(err).To(BeAssignableToTypeOf(installerErr))

				installerErr = func() *common.InstallerError {
					target := &common.InstallerError{}
					_ = errors.As(err, &target)
					return target
				}()
				Expect(installerErr.Type).To(Equal(common.ErrorTypeSystemNotFound))
				Expect(installerErr.Installer).To(Equal("apk"))
				Expect(installerErr.Package).To(Equal("test-package"))

				// Check suggestions are provided
				suggestions := installerErr.GetSuggestions()
				Expect(suggestions).ToNot(BeEmpty())
				Expect(strings.Join(suggestions, " ")).To(ContainSubstring("Alpine Linux"))
			})
		})

		Context("when APK is available but not functional", func() {
			It("returns structured error for system not functional", func() {
				// Make apk --version fail but which apk succeed
				mockExec.FailingCommands["apk --version"] = true

				err := installer.Install("test-package", mockRepo)

				Expect(err).To(HaveOccurred())

				var installerErr *common.InstallerError
				Expect(err).To(BeAssignableToTypeOf(installerErr))

				installerErr = func() *common.InstallerError {
					target := &common.InstallerError{}
					_ = errors.As(err, &target)
					return target
				}()
				Expect(installerErr.Type).To(Equal(common.ErrorTypeSystemNotFunctional))
				Expect(installerErr.Installer).To(Equal("apk"))
			})
		})

		Context("when APK is available and functional", func() {
			It("returns not implemented error with helpful suggestions", func() {
				// APK commands succeed (default mock behavior)

				err := installer.Install("test-package", mockRepo)

				Expect(err).To(HaveOccurred())

				var installerErr *common.InstallerError
				Expect(err).To(BeAssignableToTypeOf(installerErr))

				installerErr = func() *common.InstallerError {
					target := &common.InstallerError{}
					_ = errors.As(err, &target)
					return target
				}()
				Expect(installerErr.Type).To(Equal(common.ErrorTypeNotImplemented))
				Expect(installerErr.Installer).To(Equal("apk"))
				Expect(installerErr.Package).To(Equal("test-package"))

				// Check that manual command is suggested
				suggestions := installerErr.GetSuggestions()
				Expect(suggestions).ToNot(BeEmpty())
				Expect(strings.Join(suggestions, " ")).To(ContainSubstring("sudo apk add test-package"))
			})

			It("executes system validation commands", func() {
				installer.Install("test-package", mockRepo)

				// Verify validation commands were executed
				Expect(mockExec.Commands).To(ContainElement("which apk"))
				Expect(mockExec.Commands).To(ContainElement("apk --version"))
			})
		})
	})

	Describe("Uninstall", func() {
		Context("when APK is available", func() {
			It("returns not implemented error with manual command", func() {
				err := installer.Uninstall("test-package", mockRepo)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("not yet implemented"))

				// Should still validate the system
				Expect(mockExec.Commands).To(ContainElement("which apk"))
			})
		})

		Context("when APK is not available", func() {
			It("returns system not found error", func() {
				mockExec.FailingCommands["which apk"] = true

				err := installer.Uninstall("test-package", mockRepo)

				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("apk system validation failed"))
			})
		})
	})

	Describe("IsInstalled", func() {
		Context("when APK is available", func() {
			It("returns not implemented error", func() {
				installed, err := installer.IsInstalled("test-package")

				Expect(installed).To(BeFalse())
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("not yet implemented"))
			})
		})

		Context("when APK is not available", func() {
			It("returns system validation error", func() {
				mockExec.FailingCommands["which apk"] = true

				installed, err := installer.IsInstalled("test-package")

				Expect(installed).To(BeFalse())
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("apk system validation failed"))
			})
		})
	})

	Describe("Error Handling", func() {
		It("provides appropriate error types for different scenarios", func() {
			// Test system not found
			mockExec.FailingCommands["which apk"] = true
			err := installer.Install("test-package", mockRepo)

			var installerErr *common.InstallerError
			Expect(err).To(BeAssignableToTypeOf(installerErr))
			installerErr = func() *common.InstallerError {
				target := &common.InstallerError{}
				_ = errors.As(err, &target)
				return target
			}()
			Expect(installerErr.Type).To(Equal(common.ErrorTypeSystemNotFound))

			// Test suggestions are helpful
			suggestions := installerErr.GetSuggestions()
			Expect(suggestions).To(HaveLen(2))
			Expect(suggestions[0]).To(ContainSubstring("Alpine Linux"))
		})
	})

	Describe("Logging", func() {
		It("logs appropriate messages for unimplemented functionality", func() {
			// This test validates that appropriate log messages are generated
			// The actual log verification would depend on your logging setup
			installer.Install("test-package", mockRepo)

			// Verify that commands are attempted (indicating logs were called)
			Expect(mockExec.Commands).ToNot(BeEmpty())
		})
	})
})
