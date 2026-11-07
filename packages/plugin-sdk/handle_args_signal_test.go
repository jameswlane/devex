package sdk_test

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"syscall"
	"time"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// signalFixtureEnv re-executes the test binary as a plugin process that calls
// sdk.HandleArgs, so signal delivery is observed across a real process boundary.
const signalFixtureEnv = "DEVEX_SDK_HANDLE_ARGS_SIGNAL_FIXTURE"

type blockingLegacyPlugin struct{}

func (blockingLegacyPlugin) Info() sdk.PluginInfo { return sdk.PluginInfo{Name: "legacy"} }
func (blockingLegacyPlugin) Execute(string, []string) error {
	fmt.Println("ready")
	time.Sleep(time.Minute)
	fmt.Println("legacy plugin was not terminated")
	return nil
}

type blockingContextPlugin struct{ blockingLegacyPlugin }

func (blockingContextPlugin) ExecuteContext(ctx context.Context, _ string, _ []string) error {
	fmt.Println("ready")
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(time.Minute):
		return fmt.Errorf("context was not cancelled")
	}
}

// runSignalFixture runs inside the child process and never returns.
func runSignalFixture(kind string) {
	var plugin sdk.Plugin = blockingLegacyPlugin{}
	if kind == "contextual" {
		plugin = blockingContextPlugin{}
	}
	sdk.HandleArgs(plugin, []string{"run"})
	os.Exit(0)
}

var _ = Describe("HandleArgs signal handling", func() {
	// start launches the fixture, waits until the plugin is blocked inside its
	// command, and returns a channel that closes once the process has been reaped.
	start := func(kind string) (*exec.Cmd, *bytes.Buffer, <-chan struct{}) {
		if runtime.GOOS == "windows" {
			Skip("POSIX signals")
		}
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		DeferCleanup(cancel)
		cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestSDK$")
		cmd.WaitDelay = time.Second
		cmd.Env = append(os.Environ(), signalFixtureEnv+"="+kind)
		var stderr bytes.Buffer
		cmd.Stderr = &stderr
		stdout, err := cmd.StdoutPipe()
		Expect(err).NotTo(HaveOccurred())
		Expect(cmd.Start()).To(Succeed())
		reader := bufio.NewReader(stdout)
		line, readErr := reader.ReadString('\n')
		done := make(chan struct{})
		go func() {
			_, _ = io.Copy(io.Discard, reader)
			_ = cmd.Wait()
			close(done)
		}()
		DeferCleanup(func() { _ = cmd.Process.Kill(); <-done })
		Expect(readErr).NotTo(HaveOccurred())
		Expect(line).To(Equal("ready\n"))
		return cmd, &stderr, done
	}

	wait := func(cmd *exec.Cmd, done <-chan struct{}) *os.ProcessState {
		Eventually(done).WithTimeout(10 * time.Second).Should(BeClosed())
		return cmd.ProcessState
	}

	It("keeps the default SIGTERM termination for a legacy Execute plugin", func() {
		cmd, _, done := start("legacy")
		Expect(cmd.Process.Signal(syscall.SIGTERM)).To(Succeed())
		state := wait(cmd, done)
		status, ok := state.Sys().(syscall.WaitStatus)
		Expect(ok).To(BeTrue())
		Expect(status.Signaled()).To(BeTrue(), "legacy plugin must be terminated by SIGTERM, got %v", state)
		Expect(status.Signal()).To(Equal(syscall.SIGTERM))
	})

	It("cancels a contextual plugin's context on SIGTERM and exits with an error", func() {
		cmd, stderr, done := start("contextual")
		Expect(cmd.Process.Signal(syscall.SIGTERM)).To(Succeed())
		state := wait(cmd, done)
		Expect(state.ExitCode()).To(Equal(1))
		Expect(stderr.String()).To(ContainSubstring("Error: context canceled"))
	})
})
