package sdktest_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

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

	It("isolates XDG directories, temporary files, and desktop session state", func() {
		// Simulate a live desktop session and host XDG layout before isolating.
		cleared := []string{
			sdk.UnattendedEnv, "CI", "NONINTERACTIVE", "DEBIAN_FRONTEND",
			"XDG_CURRENT_DESKTOP", "XDG_SESSION_DESKTOP", "XDG_SESSION_TYPE", "DESKTOP_SESSION",
			"DBUS_SESSION_BUS_ADDRESS", "DISPLAY", "WAYLAND_DISPLAY",
		}
		for _, key := range cleared {
			GinkgoT().Setenv(key, "host-value")
		}
		for _, key := range []string{"XDG_CONFIG_HOME", "XDG_DATA_HOME", "XDG_STATE_HOME", "XDG_CACHE_HOME", "XDG_RUNTIME_DIR"} {
			GinkgoT().Setenv(key, "/host/"+key)
		}
		hostTemp := GinkgoT().TempDir()
		GinkgoT().Setenv("TMPDIR", hostTemp)
		isolated := sdktest.NewRecorder(GinkgoT(), "manager")

		home := os.Getenv("HOME")
		for key, dir := range map[string]string{
			"XDG_CONFIG_HOME": filepath.Join(home, ".config"),
			"XDG_DATA_HOME":   filepath.Join(home, ".local", "share"),
			"XDG_STATE_HOME":  filepath.Join(home, ".local", "state"),
			"XDG_CACHE_HOME":  filepath.Join(home, ".cache"),
		} {
			Expect(os.Getenv(key)).To(Equal(dir), key)
			Expect(dir).To(BeADirectory())
		}
		root := filepath.Dir(home)
		Expect(os.Getenv("XDG_RUNTIME_DIR")).To(Equal(filepath.Join(root, "run")))
		Expect(os.Getenv("XDG_RUNTIME_DIR")).To(BeADirectory())
		Expect(os.TempDir()).To(Equal(filepath.Join(root, "tmp")))
		Expect(os.TempDir()).NotTo(Equal(hostTemp))
		Expect(os.TempDir()).To(BeADirectory())
		for _, key := range cleared {
			Expect(os.Getenv(key)).To(BeEmpty(), key)
		}
		Expect(sdk.PathsForHome("").Backups()).To(HavePrefix(home + string(filepath.Separator)))
		Expect(sdk.NewCommandContext(context.Background(), "manager").Run()).To(Succeed())
		Expect(isolated.Environment("manager", "NONINTERACTIVE")).To(Equal([]string{""}))
		Expect(isolated.Environment("manager", "DEBIAN_FRONTEND")).To(Equal([]string{""}))
	})

	It("lists advertised command names in declaration order", func() {
		info := sdk.PluginInfo{Commands: []sdk.PluginCommand{{Name: "install"}, {Name: "list"}}}
		Expect(sdktest.CommandNames(info)).To(Equal([]string{"install", "list"}))
		Expect(sdktest.CommandNames(sdk.PluginInfo{})).To(BeEmpty())
	})

	It("captures output larger than the pipe buffer without blocking", func() {
		line := strings.Repeat("x", 1023) + "\n"
		output := sdktest.CaptureStdout(GinkgoT(), func() {
			for range 2048 {
				fmt.Print(line)
			}
		})
		Expect(output).To(HaveLen(2 * 1024 * 1024))
		Expect(output).To(Equal(strings.Repeat(line, 2048)))
	})

	It("restores standard output when the captured function fails", func() {
		original := os.Stdout
		Expect(func() {
			sdktest.CaptureStdout(GinkgoT(), func() { panic("boom") })
		}).To(PanicWith("boom"))
		Expect(os.Stdout).To(BeIdenticalTo(original))
	})

	It("restores standard input after cleanup", func() {
		original := os.Stdin
		DeferCleanup(func() { Expect(os.Stdin).To(BeIdenticalTo(original)) })
		sdktest.ProvideStdin(GinkgoT(), "")
		Expect(os.Stdin).NotTo(BeIdenticalTo(original))
	})

	It("supplies interactive input and captures standard output", func() {
		sdktest.ProvideStdin(GinkgoT(), "y\n")
		var response string
		output := sdktest.CaptureStdout(GinkgoT(), func() {
			fmt.Print("Continue? ")
			_, err := fmt.Scanln(&response)
			Expect(err).NotTo(HaveOccurred())
		})
		Expect(response).To(Equal("y"))
		Expect(output).To(Equal("Continue? "))
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
			{Args: []string{"-n", "--", "manager", "install"}},
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

	It("uses a fallback response only for argv without an exact script", func() {
		recorder.Script("manager", []string{"extract", "known"}, sdktest.Response{Stdout: "scripted\n"})
		recorder.Fallback("manager", sdktest.Response{ExitCode: 9, Stdout: "partial\n", Stderr: "fallback failure\n"})

		output, err := sdk.ExecCommandOutputWithContext(context.Background(), "manager", "extract", "known")
		Expect(err).NotTo(HaveOccurred())
		Expect(output).To(Equal("scripted\n"))

		var stdout, stderr bytes.Buffer
		command := sdk.NewCommandContext(context.Background(), "manager", "extract", "/tmp/random-123")
		command.Stdout, command.Stderr = &stdout, &stderr
		Expect(command.Run()).To(MatchError("exit status 9"))
		Expect(stdout.String()).To(Equal("partial\n"))
		Expect(stderr.String()).To(Equal("fallback failure\n"))

		// Scripts added after a fallback still take precedence.
		recorder.Script("manager", []string{"late"}, sdktest.Response{})
		Expect(sdk.NewCommandContext(context.Background(), "manager", "late").Run()).To(Succeed())
		Expect(recorder.Calls("manager")).To(HaveLen(3))
	})

	It("rejects fallbacks for unregistered fakes, sudo, and invalid exit codes", func() {
		for _, invalid := range []func(*sdktest.Recorder){
			func(r *sdktest.Recorder) { r.Fallback("unregistered", sdktest.Response{}) },
			func(r *sdktest.Recorder) { r.Fallback("sudo", sdktest.Response{ExitCode: 1}) },
			func(r *sdktest.Recorder) { r.Fallback("manager", sdktest.Response{ExitCode: 256}) },
			func(r *sdktest.Recorder) { r.Fallback("manager", sdktest.Response{ExitCode: -1}) },
		} {
			spy := &fatalSpy{TestingT: GinkgoT()}
			Expect(func() { invalid(sdktest.NewRecorder(spy, "manager")) }).To(PanicWith(errFatal))
		}
	})

	Describe("standard input capture", func() {
		run := func(input string, args ...string) {
			command := sdk.NewCommandContext(context.Background(), "manager", args...)
			command.Stdin = strings.NewReader(input)
			Expect(command.Run()).To(Succeed())
		}

		It("records exact payloads for an opted-in argv", func() {
			recorder.CaptureStdin("manager", []string{"load", "/"})
			payloads := []string{
				"[org/gnome]\nkey='value'\n",
				"no trailing newline",
				"",
				"\n\n  indented \\ backslash\t-n\n",
			}
			for _, payload := range payloads {
				run(payload, "load", "/")
			}
			Expect(recorder.Stdin("manager", []string{"load", "/"})).To(Equal(payloads))
			Expect(recorder.Calls("manager")).To(HaveLen(len(payloads)))
		})

		It("keeps scripted responses for a captured argv", func() {
			recorder.CaptureStdin("manager", []string{"load"})
			recorder.Script("manager", []string{"load"}, sdktest.Response{ExitCode: 3})
			command := sdk.NewCommandContext(context.Background(), "manager", "load")
			command.Stdin = strings.NewReader("payload\n")
			Expect(command.Run()).To(MatchError("exit status 3"))
			Expect(recorder.Stdin("manager", []string{"load"})).To(Equal([]string{"payload\n"}))
		})

		It("records the empty input of an invocation without a stdin source", func() {
			recorder.CaptureStdin("manager", []string{"load"})
			Expect(sdk.NewCommandContext(context.Background(), "manager", "load").Run()).To(Succeed())
			Expect(recorder.Stdin("manager", []string{"load"})).To(Equal([]string{""}))
		})

		It("never reads stdin for argv that is not opted in", func() {
			recorder.CaptureStdin("manager", []string{"load"})
			reader, writer, err := os.Pipe()
			Expect(err).NotTo(HaveOccurred())
			defer func() { _ = reader.Close(); _ = writer.Close() }()
			// The writer stays open: a fake that read this stdin would block forever.
			command := sdk.NewCommandContext(context.Background(), "manager", "load", "other")
			command.Stdin = reader
			Expect(command.Run()).To(Succeed())
			Expect(recorder.Stdin("manager", []string{"load"})).To(BeEmpty())
		})

		It("clears captured input on reset", func() {
			recorder.CaptureStdin("manager", []string{"load"})
			run("first\n", "load")
			recorder.Reset()
			Expect(recorder.Stdin("manager", []string{"load"})).To(BeEmpty())
			run("second\n", "load")
			Expect(recorder.Stdin("manager", []string{"load"})).To(Equal([]string{"second\n"}))
		})

		It("rejects captures for unregistered fakes, sudo, duplicates, and unrequested reads", func() {
			for _, invalid := range []func(*sdktest.Recorder){
				func(r *sdktest.Recorder) { r.CaptureStdin("unregistered", nil) },
				func(r *sdktest.Recorder) { r.CaptureStdin("sudo", []string{"-v"}) },
				func(r *sdktest.Recorder) { r.CaptureStdin("manager", nil); r.CaptureStdin("manager", nil) },
				func(r *sdktest.Recorder) { r.Stdin("manager", []string{"never-captured"}) },
			} {
				spy := &fatalSpy{TestingT: GinkgoT()}
				Expect(func() { invalid(sdktest.NewRecorder(spy, "manager")) }).To(PanicWith(errFatal))
			}
		})
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

var errFatal = errors.New("fatal")

// fatalSpy turns Fatalf into a recoverable panic so misuse checks are observable.
type fatalSpy struct{ sdktest.TestingT }

func (s *fatalSpy) Fatalf(string, ...any) { panic(errFatal) }
