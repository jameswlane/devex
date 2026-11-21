package main_test

import (
	"errors"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	main "github.com/jameswlane/devex/packages/package-manager-winget"
	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
)

type exitStatus int

func (e exitStatus) Error() string { return fmt.Sprintf("exit status %d", int(e)) }
func (e exitStatus) ExitCode() int { return int(e) }

var _ = Describe("winget HRESULT exit statuses", func() {
	It("reads the unsigned status Windows reports", func() {
		code, ok := main.HResult(fmt.Errorf("wrapped: %w", exitStatus(int(uint32(0x8A150014)))))
		Expect(ok).To(BeTrue())
		Expect(code).To(Equal(uint32(0x8A150014)))
	})

	It("reads a status reported as a negative 32-bit value", func() {
		negative := int32(-1978335212) // 0x8A150014
		code, ok := main.HResult(exitStatus(int(negative)))
		Expect(ok).To(BeTrue())
		Expect(code).To(Equal(uint32(0x8A150014)))
	})

	It("has no status for errors without a native process", func() {
		_, ok := main.HResult(errors.New("plain"))
		Expect(ok).To(BeFalse())
		_, ok = main.HResult(sdk.NewAuthorizationDeniedError("denied", 1, exitStatus(1)))
		Expect(ok).To(BeFalse())
	})
})
