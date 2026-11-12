//go:build linux

package tui

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/log"
	"github.com/jameswlane/devex/apps/cli/internal/mocks"
	"github.com/jameswlane/devex/packages/plugin-sdk/sdktest"
)

var _ = Describe("Native command output logging", func() {
	It("logs a successful command's stderr progress at info level, not error", func() {
		recorder := sdktest.NewRecorder(GinkgoT(), "tool")
		recorder.Script("tool", []string{"--install"}, sdktest.Response{Stderr: "installing tool...\n1.1%\n"})
		var buffer bytes.Buffer
		log.InitDefaultLogger(&buffer)
		DeferCleanup(log.InitTestLogger)

		installer := NewStreamingInstallerWithExecutor(nil, mocks.NewMockRepository(), context.Background(), NewDefaultCommandExecutor(), config.CrossPlatformSettings{})
		Expect(installer.executeCommandStream(context.Background(), "tool --install")).To(Succeed())

		output := buffer.String()
		Expect(output).To(ContainSubstring("installing tool..."))
		Expect(output).To(ContainSubstring("stream=stderr"))
		Expect(output).NotTo(ContainSubstring("ERRO"))
	})

	It("drains inherited descriptors for the bounded wait before returning", func() {
		var buffer bytes.Buffer
		log.InitDefaultLogger(&buffer)
		DeferCleanup(log.InitTestLogger)

		// The production validator rejects backgrounding, so run the shell
		// directly to model an installer that leaves a daemon behind.
		childPID := registerChildCleanup()
		executor := shellExecutor{script: fmt.Sprintf(`echo started; (sleep 0.1; echo child-ready; exec sleep 30) & echo $! > %q`, childPID)}
		installer := NewStreamingInstallerWithExecutor(nil, mocks.NewMockRepository(), context.Background(), executor, config.CrossPlatformSettings{})
		started := time.Now()
		Expect(installer.executeCommandStream(context.Background(), "background-daemon")).To(Succeed())

		elapsed := time.Since(started)
		Expect(elapsed).To(BeNumerically(">=", 900*time.Millisecond))
		Expect(elapsed).To(BeNumerically("<", 4*time.Second))
		Expect(buffer.String()).To(ContainSubstring("started"))
		Expect(buffer.String()).To(ContainSubstring("child-ready"))
	})

	It("reports a capture cutoff when a slow consumer delays a completed command's output", func() {
		marker := filepath.Join(GinkgoT().TempDir(), "writes-complete")
		executor := shellExecutor{script: fmt.Sprintf(`i=1; while [ "$i" -le 4000 ]; do printf 'line-%%04d\n' "$i"; i=$((i+1)); done; : > "%s"`, marker)}
		run := startGatedCommand(executor)
		writer := run.writer

		Eventually(writer.started, 5*time.Second).Should(BeClosed())
		Eventually(func() bool {
			_, err := os.Stat(marker)
			return err == nil
		}, 5*time.Second).Should(BeTrue(), "the direct command did not finish its output writes")
		time.Sleep(1200 * time.Millisecond)
		writer.Release()
		Eventually(run.result, 5*time.Second).Should(Receive(Succeed()))

		output := writer.String()
		if strings.Contains(output, "output may be incomplete") {
			Expect(output).To(ContainSubstring("WARN"))
			return
		}
		for i := 1; i <= 4000; i++ {
			Expect(output).To(ContainSubstring(fmt.Sprintf("line-%04d", i)))
		}
	})

	It("joins gated logging during cleanup before normal gate release", func() {
		run := startGatedCommand(shellExecutor{script: "echo started; sleep 30"})
		Eventually(run.writer.started, 5*time.Second).Should(BeClosed())
		Expect(run.done).NotTo(BeClosed())

		// Simulate leaving the spec before its normal release/result receive.
		// The registered cleanup also runs if any assertion above fails.
		Expect(run.cleanup()).To(Succeed())
		Expect(run.done).To(BeClosed(), "cleanup must join, not just cancel")
		Expect(run.result).To(Receive(MatchError(context.Canceled)))
		Expect(run.cleanup()).To(Succeed(), "cleanup must tolerate repeated calls")
		run.writer.Release()
	})

	It("runs registered cleanup when the spec returns before gate release", func() {
		var run *gatedCommand
		// Registered first, so this observes the real fixture cleanup afterward.
		DeferCleanup(func() {
			Expect(run.done).To(BeClosed())
			Expect(run.result).To(Receive(MatchError(context.Canceled)))
		})
		run = startGatedCommand(shellExecutor{script: "echo started; sleep 30"})
		Eventually(run.writer.started, 5*time.Second).Should(BeClosed())
		// Return without releasing the gate or receiving a result, as an early
		// failed assertion would, but without an intentionally failing spec.
	})

	It("preserves cancellation and terminates the command's child", func() {
		var buffer bytes.Buffer
		log.InitDefaultLogger(&buffer)
		DeferCleanup(log.InitTestLogger)

		childPID := registerChildCleanup()
		executor := shellExecutor{script: fmt.Sprintf(`sleep 30 & child=$!; echo "$child" > %q; echo started; wait "$child"`, childPID)}
		installer := NewStreamingInstallerWithExecutor(nil, mocks.NewMockRepository(), context.Background(), executor, config.CrossPlatformSettings{})
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		started := time.Now()
		err := installer.executeCommandStream(ctx, "cancel-command")

		Expect(err).To(MatchError(context.DeadlineExceeded))
		Expect(time.Since(started)).To(BeNumerically("<", 3*time.Second))
		Expect(buffer.String()).To(ContainSubstring("started"))
		pid, err := readChildPID(childPID)
		Expect(err).NotTo(HaveOccurred())
		Eventually(func() (bool, error) { return childTerminated(pid) }, time.Second).Should(BeTrue(), "cancellation must kill the descendant, not only return promptly")
	})

	It("preserves a direct command's nonzero exit status", func() {
		var buffer bytes.Buffer
		log.InitDefaultLogger(&buffer)
		DeferCleanup(log.InitTestLogger)

		executor := shellExecutor{script: "echo before-failure; exit 23"}
		installer := NewStreamingInstallerWithExecutor(nil, mocks.NewMockRepository(), context.Background(), executor, config.CrossPlatformSettings{})
		err := installer.executeCommandStream(context.Background(), "failing-command")

		var exitErr *exec.ExitError
		Expect(errors.As(err, &exitErr)).To(BeTrue())
		Expect(exitErr.ExitCode()).To(Equal(23))
		Expect(buffer.String()).To(ContainSubstring("before-failure"))
	})
})

// shellExecutor runs a fixed shell script regardless of the requested command.
type shellExecutor struct{ script string }

func (e shellExecutor) ExecuteCommand(ctx context.Context, _ string) (*exec.Cmd, error) {
	cmd := exec.CommandContext(ctx, "sh", "-c", e.script)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	return cmd, nil
}

func (e shellExecutor) ValidateCommand(string) error { return nil }

type gatedLogWriter struct {
	mu          sync.Mutex
	buffer      bytes.Buffer
	started     chan struct{}
	release     chan struct{}
	once        sync.Once
	releaseOnce sync.Once
}

func newGatedLogWriter() *gatedLogWriter {
	return &gatedLogWriter{started: make(chan struct{}), release: make(chan struct{})}
}

func (w *gatedLogWriter) Write(p []byte) (int, error) {
	w.once.Do(func() {
		close(w.started)
		<-w.release
	})
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.buffer.Write(p)
}

func (w *gatedLogWriter) String() string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.buffer.String()
}

// Release is shared by the normal path and unconditional teardown.
func (w *gatedLogWriter) Release() {
	w.releaseOnce.Do(func() { close(w.release) })
}

type gatedCommand struct {
	writer *gatedLogWriter
	cancel context.CancelFunc
	result chan error
	done   chan struct{}
}

func startGatedCommand(executor shellExecutor) *gatedCommand {
	ctx, cancel := context.WithCancel(context.Background())
	run := &gatedCommand{writer: newGatedLogWriter(), cancel: cancel, result: make(chan error, 1), done: make(chan struct{})}
	log.InitDefaultLogger(run.writer)
	// Register before launching work or making assertions. Restore the global
	// logger only after joining; consuming result in the spec cannot steal done.
	DeferCleanup(func() {
		err := run.cleanup()
		if err == nil {
			log.InitTestLogger()
		}
		Expect(err).NotTo(HaveOccurred())
	})
	installer := NewStreamingInstallerWithExecutor(nil, mocks.NewMockRepository(), ctx, executor, config.CrossPlatformSettings{})
	go func() {
		defer close(run.done)
		run.result <- installer.executeCommandStream(ctx, "gated-command")
	}()
	return run
}

func (run *gatedCommand) cleanup() error {
	run.cancel()
	run.writer.Release()
	select {
	case <-run.done:
		return nil
	case <-time.After(5 * time.Second):
		return fmt.Errorf("gated command did not join after cancel and release")
	}
}

// Register before executing the shell, so even an earlier failed assertion
// cannot skip descendant cleanup. The PID file survives the direct shell exit.
func registerChildCleanup() string {
	path := filepath.Join(GinkgoT().TempDir(), "child.pid")
	DeferCleanup(func() {
		pid, err := readChildPID(path)
		if os.IsNotExist(err) {
			return // The command may not have started.
		}
		if err == nil {
			err = syscall.Kill(pid, syscall.SIGKILL)
			if errors.Is(err, syscall.ESRCH) {
				err = nil
			}
		}
		// Kill first, independently of assertion success, then bound verification.
		Expect(err).NotTo(HaveOccurred())
		Eventually(func() (bool, error) { return childTerminated(pid) }, time.Second).Should(BeTrue())
	})
	return path
}

func readChildPID(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, err
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err == nil && pid <= 1 {
		err = fmt.Errorf("invalid child PID %d", pid)
	}
	return pid, err
}

func childTerminated(pid int) (bool, error) {
	data, err := os.ReadFile(fmt.Sprintf("/proc/%d/stat", pid))
	if os.IsNotExist(err) {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	// An orphan may await init's reaper as a zombie: it has exited and cannot
	// run or retain descriptors. Do not confuse kill(pid, 0) with liveness.
	_, state, ok := strings.Cut(string(data), ") ")
	return ok && strings.HasPrefix(state, "Z "), nil
}
