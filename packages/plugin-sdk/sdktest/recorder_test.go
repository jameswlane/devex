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
			Args: []string{"--", "manager", "install", "two words", "", "quote'argument"},
		}}))
	})

	It("uses a deterministic unprivileged SDK account", func() {
		resolver := sdk.NewPrivilegeResolver()
		Expect(resolver.Elevated).To(BeFalse())
		identity, err := resolver.ResolveUser(nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(identity).To(Equal(&sdk.UserIdentity{
			Username: "devex-test-user", UID: "1000", GID: "1000", HomeDir: os.Getenv("HOME"),
		}))
	})

	It("records scoped authorization probes separately from execution", func() {
		Expect(sdk.ExecOperationWithContext(context.Background(), sdk.OperationPrivilege{Scope: sdk.ScopeSystem}, "manager", "install")).To(Succeed())
		Expect(recorder.Calls("sudo")).To(Equal([]sdktest.Call{
			{Args: []string{"-n", "true"}},
			{Args: []string{"--", "manager", "install"}},
		}))
		Expect(recorder.Calls("manager")).To(Equal([]sdktest.Call{
			{Args: []string{"install"}, ElevationRequested: true},
		}))
	})

	It("supports explicit authorization and preserves scripted authorization denial", func() {
		sdk.DefaultPrivilegeAdapter = sdk.UnixSudoAdapter{
			Authorize: func(ctx context.Context, sudo string) error {
				return sdk.NewCommandContext(ctx, sudo, "-v").Run()
			},
		}
		privilege := sdk.OperationPrivilege{Scope: sdk.ScopeSystem}
		Expect(sdk.ExecOperationWithContext(context.Background(), privilege, "manager", "list")).To(Succeed())
		Expect(recorder.Calls("sudo")[0].Args).To(Equal([]string{"-v"}))
		recorder.Reset()
		recorder.Script("sudo", []string{"-v"}, sdktest.Response{ExitCode: 7})

		err := sdk.ExecOperationWithContext(context.Background(), privilege, "manager", "list")
		var auth *sdk.AuthorizationError
		Expect(err).To(BeAssignableToTypeOf(auth))
		Expect(sdk.ExitStatus(err)).To(Equal(7))
		Expect(recorder.Calls("sudo")).To(Equal([]sdktest.Call{{Args: []string{"-v"}}}))
		Expect(recorder.Calls("manager")).To(BeEmpty())
	})

	It("does not treat a failed noninteractive probe as target denial", func() {
		recorder.Script("sudo", []string{"-n", "true"}, sdktest.Response{ExitCode: 1})
		recorder.Script("manager", []string{"install"}, sdktest.Response{ExitCode: 42})
		err := sdk.ExecOperationWithContext(context.Background(), sdk.OperationPrivilege{Scope: sdk.ScopeSystem}, "manager", "install")
		Expect(err).To(MatchError("exit status 42"))
		Expect(recorder.Calls("manager")).To(HaveLen(1))
	})

	It("rejects harmless absolute host commands at SDK execution boundaries", func() {
		const hostCommand = "/bin/printf"
		_, err := sdk.ExecCommandOutputWithContext(context.Background(), hostCommand, "escaped")
		Expect(err).To(MatchError(ContainSubstring("not a registered fake")))
		_, err = sdk.RunCommand(hostCommand, "escaped")
		Expect(err).To(MatchError(ContainSubstring("not a registered fake")))
		Expect(sdk.ExecCommandWithContext(context.Background(), false, hostCommand, "escaped")).To(MatchError(ContainSubstring("not a registered fake")))
		Expect(sdk.ExecCommandWithContext(context.Background(), true, hostCommand, "escaped")).To(MatchError("exit status 127"))
		for _, scope := range []sdk.OperationScope{sdk.ScopeUser, sdk.ScopeSystem} {
			err = sdk.ExecOperationWithContext(context.Background(), sdk.OperationPrivilege{Scope: scope}, hostCommand, "escaped")
			Expect(err).To(HaveOccurred())
		}
		Expect(recorder.Calls("manager")).To(BeEmpty())
	})

	It("rejects unregistered commands and absolute targets inside fake sudo itself", func() {
		for _, target := range []string{"printf", "/bin/printf", "/usr/bin/manager"} {
			output, err := exec.CommandContext(context.Background(), "sudo", "--", target, "escaped").CombinedOutput()
			Expect(err).To(MatchError("exit status 127"))
			Expect(string(output)).To(Equal("sdktest: sudo target is not a registered fake\n"))
		}
	})

	It("preserves cancellation before a registered command starts", func() {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		Expect(sdk.ExecCommandWithContext(ctx, false, "manager", "install")).To(MatchError(context.Canceled))
		Expect(recorder.Calls("manager")).To(BeEmpty())
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
