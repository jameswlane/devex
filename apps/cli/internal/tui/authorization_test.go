//go:build !windows

package tui

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/types"
	"github.com/jameswlane/devex/packages/plugin-sdk/sdktest"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Noninteractive installer authorization", func() {
	var recorder *sdktest.Recorder
	var installer *StreamingInstaller
	BeforeEach(func() {
		recorder = sdktest.NewRecorder(GinkgoT(), "apt-get")
		installer = &StreamingInstaller{ctx: context.Background(), config: DefaultInstallerConfig(), executor: NewDefaultCommandExecutor()}
	})
	It("stops before opening the installer when non-TTY authorization fails", func() {
		if os.Geteuid() == 0 {
			Skip("root does not require sudo authorization")
		}
		recorder.Script("sudo", []string{"-n", "-v"}, sdktest.Response{ExitCode: 1})
		stdin, err := os.Open(os.DevNull)
		Expect(err).NotTo(HaveOccurred())
		originalStdin := os.Stdin
		os.Stdin = stdin
		DeferCleanup(func() { os.Stdin = originalStdin; Expect(stdin.Close()).To(Succeed()) })
		apps := []types.CrossPlatformApp{{AllPlatforms: types.OSConfig{InstallMethod: "apt", InstallCommand: "git"}}}
		err = StartInstallation(context.Background(), apps, nil, config.CrossPlatformSettings{})
		Expect(errors.Is(err, ErrSudoAuthorization)).To(BeTrue())
		Expect(recorder.Calls("apt-get")).To(BeEmpty())
	})
	It("fails promptly with guidance when credentials are missing without prompting", func() {
		recorder.Script("sudo", []string{"-n", "true"}, sdktest.Response{ExitCode: 1})
		recorder.Script("sudo", []string{"-n", "--", "apt-get", "update"}, sdktest.Response{ExitCode: 1, Stderr: "sudo: a password is required\n"})
		recorder.Script("sudo", []string{"-n", "-v"}, sdktest.Response{ExitCode: 1})
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		err := installer.executeCommandStream(ctx, "sudo -n apt-get update")
		Expect(errors.Is(err, ErrSudoAuthorization)).To(BeTrue())
		Expect(ctx.Err()).NotTo(HaveOccurred())
		for _, call := range recorder.Calls("sudo") {
			Expect(call.Args[0]).To(Equal("-n"))
		}
		Expect(recorder.Calls("apt-get")).To(BeEmpty())
	})
	It("executes privileged commands after foreground authorization and refreshes noninteractively", func() {
		Expect(exec.CommandContext(context.Background(), "sudo", "-v").Run()).To(Succeed())
		Expect(authorizeSudo(context.Background(), false)).To(Succeed())
		Expect(installer.executeCommandStream(context.Background(), "sudo -n apt-get update")).To(Succeed())
		Expect(recorder.Calls("apt-get")).To(Equal([]sdktest.Call{{Args: []string{"update"}, ElevationRequested: true}}))
	})
	It("never prompts when a catalog pipeline invokes sudo", func() {
		bash, err := exec.LookPath("bash")
		// The recorder deliberately isolates PATH; append the host shell's directory
		// so the recorded fake sudo still wins over a real (possibly passwordless) one.
		if err != nil {
			bash = "/bin/bash"
		}
		GinkgoT().Setenv("PATH", os.Getenv("PATH")+string(os.PathListSeparator)+filepath.Dir(bash))
		recorder.Script("sudo", []string{"-n", "-n", "apt-get", "update"}, sdktest.Response{ExitCode: 1})
		recorder.Script("sudo", []string{"-n", "apt-get", "update"}, sdktest.Response{ExitCode: 1})
		recorder.Script("sudo", []string{"-n", "-v"}, sdktest.Response{ExitCode: 1})
		err = installer.executeCommandStream(context.Background(), "echo ready | sudo apt-get update")
		Expect(err).To(MatchError(ContainSubstring("sudo authorization expired")))
		// The recorded fake, not the host's sudo, must have handled the pipeline.
		Expect(recorder.Calls("sudo")).NotTo(BeEmpty())
		for _, call := range recorder.Calls("sudo") {
			Expect(call.Args[0]).To(Equal("-n"))
		}
	})
	It("kills and reaps a running sudo child on cancellation", func() {
		sudo, err := exec.LookPath("sudo")
		Expect(err).NotTo(HaveOccurred())
		pidFile := filepath.Join(GinkgoT().TempDir(), "pid")
		GinkgoT().Setenv("SUDO_PID_FILE", pidFile)
		Expect(os.WriteFile(sudo, []byte("#!/bin/sh\nif [ \"$2\" = true ]; then exit 0; fi\nprintf '%s' \"$$\" > \"$SUDO_PID_FILE\"\nexec /bin/sleep 60\n"), 0700)).To(Succeed())
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		done := make(chan error, 1)
		go func() { done <- installer.executeCommandStream(ctx, "sudo -n apt-get update") }()
		Eventually(func() bool { _, err := os.Stat(pidFile); return err == nil }).Should(BeTrue())
		data, err := os.ReadFile(pidFile)
		Expect(err).NotTo(HaveOccurred())
		pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
		Expect(err).NotTo(HaveOccurred())
		cancel()
		Eventually(done).Should(Receive(MatchError(context.Canceled)))
		Expect(syscall.Kill(pid, 0)).To(Equal(syscall.ESRCH))
	})
})
