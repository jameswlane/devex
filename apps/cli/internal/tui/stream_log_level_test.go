//go:build linux

package tui

import (
	"bytes"
	"context"
	"os/exec"
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

	It("returns once the command exits even if a background child keeps its output open", func() {
		var buffer bytes.Buffer
		log.InitDefaultLogger(&buffer)
		DeferCleanup(log.InitTestLogger)

		// The production validator rejects backgrounding, so run the shell
		// directly to model an installer that leaves a daemon behind.
		executor := shellExecutor{script: "echo started; sleep 5 &"}
		installer := NewStreamingInstallerWithExecutor(nil, mocks.NewMockRepository(), context.Background(), executor, config.CrossPlatformSettings{})
		started := time.Now()
		Expect(installer.executeCommandStream(context.Background(), "background-daemon")).To(Succeed())

		Expect(time.Since(started)).To(BeNumerically("<", 4*time.Second))
		Expect(buffer.String()).To(ContainSubstring("started"))
	})
})

// shellExecutor runs a fixed shell script regardless of the requested command.
type shellExecutor struct{ script string }

func (e shellExecutor) ExecuteCommand(ctx context.Context, _ string) (*exec.Cmd, error) {
	return exec.CommandContext(ctx, "sh", "-c", e.script), nil
}

func (e shellExecutor) ValidateCommand(string) error { return nil }
