package sdk

import (
	"bytes"
	"context"
	"errors"
	"os/exec"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Executable authorization diagnostics", func() {
	It("round trips typed authorization while retaining the process exit error", func() {
		original := &AuthorizationError{Kind: AuthorizationDenied, Reason: "permission rejected", NativeExitCode: 23}
		var diagnostic bytes.Buffer
		writeExecutionError(&diagnostic, original)
		native := exec.CommandContext(context.Background(), "sh", "-c", "exit 23").Run()
		err := decodeExecutionError(diagnostic.String(), native)
		var auth *AuthorizationError
		Expect(errors.As(err, &auth)).To(BeTrue())
		Expect(auth.Kind).To(Equal(AuthorizationDenied))
		Expect(auth.NativeExitCode).To(Equal(23))
		Expect(ExitStatus(err)).To(Equal(23))
		var exit *exec.ExitError
		Expect(errors.As(err, &exit)).To(BeTrue())
	})
	It("bounds retained stderr while keeping trailing authorization diagnostics", func() {
		var tail diagnosticTail
		_, err := tail.Write(bytes.Repeat([]byte("x"), 128*1024))
		Expect(err).NotTo(HaveOccurred())
		writeExecutionError(&tail, &AuthorizationError{Kind: AuthorizationDenied, Reason: "denied", NativeExitCode: 23})
		Expect(len(tail.data)).To(BeNumerically("<=", 64*1024))
		var auth *AuthorizationError
		Expect(errors.As(decodeExecutionError(tail.String(), errors.New("native")), &auth)).To(BeTrue())
	})
	It("keeps ordinary plugin failures and invalid diagnostics as native errors", func() {
		native := errors.New("plugin failed")
		Expect(decodeExecutionError("normal stderr", native)).To(BeIdenticalTo(native))
		Expect(decodeExecutionError(authorizationDiagnosticPrefix+"{}", native)).To(BeIdenticalTo(native))
	})
})
