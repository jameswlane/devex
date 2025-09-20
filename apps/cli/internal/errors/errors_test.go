package errors_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/jameswlane/devex/apps/cli/internal/errors"
)

var _ = Describe("InstallerError", func() {
	It("formats the error message correctly", func() {
		err := &errors.InstallerError{
			Installer: "test-installer",
			Err:       errors.ErrInstallFailed,
		}
		Expect(err.Error()).To(Equal("installer 'test-installer' failed: installation failed"))
	})

	It("unwraps the underlying error", func() {
		baseErr := errors.ErrInstallFailed
		err := &errors.InstallerError{
			Installer: "test-installer",
			Err:       baseErr,
		}
		Expect(err.Unwrap()).To(Equal(baseErr))
	})
})

var _ = Describe("Wrap", func() {
	It("returns nil if the input error is nil", func() {
		err := errors.Wrap(nil, "context")
		Expect(err).To(BeNil())
	})

	It("wraps an error with additional context", func() {
		baseErr := errors.New("base error")
		err := errors.Wrap(baseErr, "additional context")
		Expect(err.Error()).To(Equal("additional context: base error"))
	})
})

var _ = Describe("New", func() {
	It("creates a new error with the specified message", func() {
		err := errors.New("test error")
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(Equal("test error"))
	})
})

var _ = Describe("Is", func() {
	It("matches the target error correctly", func() {
		baseErr := errors.New("base error")
		wrappedErr := errors.Wrap(baseErr, "context")
		Expect(errors.Is(wrappedErr, baseErr)).To(BeTrue())
	})

	It("returns false if the error does not match the target", func() {
		baseErr := errors.New("base error")
		otherErr := errors.New("other error")
		Expect(errors.Is(baseErr, otherErr)).To(BeFalse())
	})
})

var _ = Describe("As", func() {
	It("asserts the error type correctly", func() {
		instErr := &errors.InstallerError{
			Installer: "test-installer",
			Err:       errors.ErrInstallFailed,
		}
		var targetErr *errors.InstallerError
		Expect(errors.As(instErr, &targetErr)).To(BeTrue())
		Expect(targetErr.Installer).To(Equal("test-installer"))
	})
})

var _ = Describe("Unwrap", func() {
	It("retrieves the underlying error", func() {
		baseErr := errors.New("base error")
		wrappedErr := errors.Wrap(baseErr, "context")
		Expect(errors.Unwrap(wrappedErr)).To(Equal(baseErr))
	})
})
