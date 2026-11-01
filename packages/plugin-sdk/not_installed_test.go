package sdk

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Not-installed query results", func() {
	It("round trips a not-installed result and exits with ExitCodeNotInstalled", func() {
		original := &NotInstalledError{Packages: []string{"bat", "fzf"}}
		Expect(ExitStatus(fmt.Errorf("is-installed: %w", original))).To(Equal(ExitCodeNotInstalled))

		var diagnostic bytes.Buffer
		writeExecutionError(&diagnostic, fmt.Errorf("is-installed: %w", original))
		native := exec.CommandContext(context.Background(), "sh", "-c", "exit 3").Run()
		Expect(native).To(HaveOccurred())

		err := decodeExecutionError(diagnostic.String(), native)
		var missing *NotInstalledError
		Expect(errors.As(err, &missing)).To(BeTrue())
		Expect(missing.Packages).To(Equal([]string{"bat", "fzf"}))
		Expect(err).To(MatchError("not installed: bat, fzf"))
		Expect(errors.Is(err, native)).To(BeTrue())
	})

	It("keeps a failed query as a plain error", func() {
		var diagnostic bytes.Buffer
		writeExecutionError(&diagnostic, errors.New("dpkg-query: database locked"))
		native := exec.CommandContext(context.Background(), "sh", "-c", "exit 1").Run()

		err := decodeExecutionError(diagnostic.String(), native)
		var missing *NotInstalledError
		Expect(errors.As(err, &missing)).To(BeFalse())
		Expect(decodeExecutionError(notInstalledDiagnosticPrefix+`{"packages":[]}`, native)).To(BeIdenticalTo(native))
	})
})
