package utilities_test

import (
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/jameswlane/devex/apps/cli/internal/installers/utilities"
)

// indexOf returns the index of substr in s, or -1 if not found
func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}

var _ = Describe("InstallerError", func() {
	Describe("NewSystemError", func() {
		It("should create system error with suggestions", func() {
			underlying := errors.New("command not found")
			err := utilities.NewSystemError("apt", "APT not available", underlying)

			Expect(err.Error()).ToNot(BeEmpty())
			Expect(err.Suggestions).ToNot(BeEmpty())
			Expect(err.Recoverable).To(BeFalse(), "System errors should not be recoverable")

			// Test error wrapping
			Expect(errors.Is(err, underlying)).To(BeTrue())
		})
	})

	Describe("NewPackageError", func() {
		It("should create package error with operation-specific suggestions", func() {
			underlying := errors.New("package not found")
			err := utilities.NewPackageError("install", "nginx", "apt", underlying)

			Expect(err.Package).To(Equal("nginx"))
			Expect(err.Installer).To(Equal("apt"))
			Expect(err.Operation).To(Equal("install"))

			// Should have install-specific suggestions
			found := false
			for _, suggestion := range err.Suggestions {
				if indexOf(suggestion, "package name") >= 0 {
					found = true
					break
				}
			}
			Expect(found).To(BeTrue(), "Expected install-specific suggestions")
		})
	})

	Describe("NewRepositoryError", func() {
		It("should create repository error with appropriate recoverability", func() {
			underlying := errors.New("database connection failed")
			err := utilities.NewRepositoryError("add", "nginx", "apt", underlying)

			Expect(err.Recoverable).To(BeTrue(), "Repository errors should be recoverable")
			Expect(err.Suggestions).ToNot(BeEmpty())
		})
	})

	Describe("NewNetworkError", func() {
		It("should create network error with retry suggestions", func() {
			underlying := errors.New("connection timeout")
			err := utilities.NewNetworkError("download", "package", "apt", underlying)

			Expect(err.Recoverable).To(BeTrue(), "Network errors should be recoverable")

			// Should have network-specific suggestions
			found := false
			for _, suggestion := range err.Suggestions {
				if indexOf(suggestion, "internet connectivity") >= 0 {
					found = true
					break
				}
			}
			Expect(found).To(BeTrue(), "Expected network-specific suggestions")
		})
	})

	Describe("WrapError", func() {
		It("should wrap existing errors with context", func() {
			original := errors.New("original error")
			wrapped := utilities.WrapError(original, utilities.ErrorTypePackage, "install", "test", "apt")

			Expect(wrapped).ToNot(BeNil())
			Expect(errors.Is(wrapped, original)).To(BeTrue(), "Should maintain error chain")

			installerErr := &utilities.InstallerError{}
			ok := errors.As(wrapped, &installerErr)
			Expect(ok).To(BeTrue(), "Expected InstallerError type")
			Expect(installerErr.Package).To(Equal("test"))
		})

		It("should handle nil error wrapping", func() {
			wrapped := utilities.WrapError(nil, utilities.ErrorTypePackage, "install", "test", "apt")
			Expect(wrapped).To(BeNil(), "Wrapping nil error should return nil")
		})
	})

	Describe("Error type comparison", func() {
		It("should compare error types correctly", func() {
			systemErr := utilities.NewSystemError("apt", "system error", nil)
			packageErr := utilities.NewPackageError("install", "nginx", "apt", nil)

			// Test Is() method with base errors
			Expect(errors.Is(systemErr, utilities.ErrSystemValidation)).To(BeTrue(), "System error should match ErrSystemValidation")
			Expect(errors.Is(packageErr, utilities.ErrInstallationFailed)).To(BeTrue(), "Package error should match ErrInstallationFailed")
			Expect(errors.Is(systemErr, utilities.ErrInstallationFailed)).To(BeFalse(), "System error should not match package error types")
		})

		It("should compare InstallerError types", func() {
			err1 := utilities.NewSystemError("apt", "error1", nil)
			err2 := utilities.NewSystemError("dnf", "error2", nil)
			packageErr := utilities.NewPackageError("install", "nginx", "apt", nil)

			Expect(errors.Is(err1, err2)).To(BeTrue(), "Same type errors should match")
			Expect(errors.Is(err1, packageErr)).To(BeFalse(), "Different type errors should not match")
		})
	})

	Describe("Recoverability determination", func() {
		Context("non-recoverable errors", func() {
			DescribeTable("should identify non-recoverable errors",
				func(errMsg string) {
					underlying := errors.New(errMsg)
					packageErr := utilities.NewPackageError("install", "test", "apt", underlying)
					Expect(packageErr.Recoverable).To(BeFalse(), "Error '%s' should not be recoverable", errMsg)
				},
				Entry("permission denied", "permission denied"),
				Entry("disk full", "disk full"),
				Entry("package does not exist", "package does not exist"),
			)
		})

		Context("recoverable errors", func() {
			DescribeTable("should identify recoverable errors",
				func(errMsg string) {
					underlying := errors.New(errMsg)
					packageErr := utilities.NewPackageError("install", "test", "apt", underlying)
					Expect(packageErr.Recoverable).To(BeTrue(), "Error '%s' should be recoverable", errMsg)
				},
				Entry("network timeout", "network timeout"),
				Entry("temporary failure", "temporary failure"),
				Entry("server unavailable", "server unavailable"),
			)
		})
	})
})
