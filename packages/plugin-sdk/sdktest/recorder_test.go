package sdktest_test

import (
	"bytes"
	"context"
	"os"
	"os/exec"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
	"github.com/jameswlane/devex/packages/plugin-sdk/sdktest"
)

var _ = Describe("Recorder", func() {
	var (
		recorder     *sdktest.Recorder
		originalHome string
	)

	BeforeEach(func() {
		originalHome = os.Getenv("HOME")
		recorder = sdktest.NewRecorder(GinkgoT(), "manager")
	})

	It("isolates home, identity, and executable lookup", func() {
		Expect(os.Getenv("HOME")).NotTo(Equal(originalHome))
		Expect(os.Getenv("HOME")).To(BeADirectory())
		Expect(os.Getenv("USER")).To(Equal("devex-test-user"))
		Expect(os.Getenv("LOGNAME")).To(Equal("devex-test-user"))
		_, err := exec.LookPath("sh")
		Expect(err).To(HaveOccurred())
	})

	It("records exact argument boundaries and elevation", func() {
		err := sdk.ExecCommandWithContext(context.Background(), true, "manager", "install", "two words", "", "quote'argument")
		Expect(err).NotTo(HaveOccurred())

		Expect(recorder.Calls("manager")).To(Equal([]sdktest.Call{{
			Args:               []string{"install", "two words", "", "quote'argument"},
			ElevationRequested: true,
		}}))
		Expect(recorder.Calls("sudo")).To(Equal([]sdktest.Call{{
			Args: []string{"manager", "install", "two words", "", "quote'argument"},
		}}))
	})

	It("does not mark direct invocations as elevated", func() {
		Expect(sdk.ExecCommandWithContext(context.Background(), false, "manager", "search", "term")).To(Succeed())
		Expect(recorder.Calls("manager")).To(Equal([]sdktest.Call{{
			Args: []string{"search", "term"},
		}}))
	})

	It("scripts stdout, stderr, and exit status for exact argv", func() {
		recorder.Script("manager", []string{"remove", "pkg"}, sdktest.Response{
			ExitCode: 23,
			Stdout:   "removed partially\n",
			Stderr:   "native failure\n",
		})

		var stdout, stderr bytes.Buffer
		cmd := exec.CommandContext(context.Background(), "manager", "remove", "pkg")
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr
		err := cmd.Run()

		Expect(err).To(MatchError("exit status 23"))
		Expect(stdout.String()).To(Equal("removed partially\n"))
		Expect(stderr.String()).To(Equal("native failure\n"))
	})

	It("uses a successful empty response for unmatched argv", func() {
		recorder.Script("manager", []string{"search", "expected"}, sdktest.Response{ExitCode: 1})

		output, err := sdk.ExecCommandOutputWithContext(context.Background(), "manager", "search", "other")

		Expect(err).NotTo(HaveOccurred())
		Expect(output).To(BeEmpty())
	})

	It("resets calls without removing responses", func() {
		recorder.Script("manager", []string{"list"}, sdktest.Response{Stdout: "installed\n"})
		_, err := sdk.ExecCommandOutputWithContext(context.Background(), "manager", "list")
		Expect(err).NotTo(HaveOccurred())
		recorder.Reset()
		Expect(recorder.Calls("manager")).To(BeEmpty())

		output, err := sdk.ExecCommandOutputWithContext(context.Background(), "manager", "list")
		Expect(err).NotTo(HaveOccurred())
		Expect(output).To(Equal("installed\n"))
	})
})
