//go:build darwin

package sdk_test

import (
	"errors"
	"os"
	"path/filepath"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("macOS native denial smoke", func() {
	It("does not run the target after authorization cancellation", func() {
		if os.Getenv("DEVEX_MACOS_AUTH_SMOKE") != "1" {
			Skip("set DEVEX_MACOS_AUTH_SMOKE=1 on an interactive non-root macOS host")
		}
		marker := filepath.Join(GinkgoT().TempDir(), "target-ran")
		err := sdk.ExecOperationWithContext(
			GinkgoT().Context(),
			sdk.OperationPrivilege{Scope: sdk.ScopeSystem},
			"/bin/sh",
			"-c",
			"printf ran > \"$1\"",
			"devex-auth-smoke",
			marker,
		)
		var authorization *sdk.AuthorizationError
		Expect(errors.As(err, &authorization)).To(BeTrue())
		Expect(authorization.Kind).To(Equal(sdk.AuthorizationDenied))
		Expect(authorization.NativeExitCode).To(Equal(-128))
		Expect(sdk.ExitStatus(err)).To(Equal(1))
		Expect(marker).NotTo(BeAnExistingFile())
	})
})
