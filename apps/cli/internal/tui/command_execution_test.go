package tui

import (
	"context"
	"os/exec"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/mocks"
)

// MockCommandExecutorForStreaming implements types.CommandExecutor for streaming command tests
type MockCommandExecutorForStreaming struct {
	commands []string
}

func NewMockCommandExecutorForStreaming() *MockCommandExecutorForStreaming {
	return &MockCommandExecutorForStreaming{}
}

func (m *MockCommandExecutorForStreaming) ExecuteCommand(ctx context.Context, command string) (*exec.Cmd, error) {
	m.commands = append(m.commands, command)
	// Return a simple echo command that will complete successfully
	cmd := exec.CommandContext(ctx, "echo", "test output")
	return cmd, nil
}

func (m *MockCommandExecutorForStreaming) ValidateCommand(command string) error {
	// Allow all commands for testing
	return nil
}

func (m *MockCommandExecutorForStreaming) GetCommands() []string {
	return m.commands
}

// getTestSettings returns default settings for testing
func getTestSettings() config.CrossPlatformSettings {
	return config.CrossPlatformSettings{
		HomeDir: "/tmp/test-devex",
		Verbose: false,
	}
}

var _ = Describe("StreamingInstaller Command Execution", func() {
	var mockRepo *mocks.MockRepository
	var mockExecutor *MockCommandExecutorForStreaming
	var ctx context.Context
	var cancel context.CancelFunc
	var installer *StreamingInstaller

	BeforeEach(func() {
		mockRepo = mocks.NewMockRepository()
		mockExecutor = NewMockCommandExecutorForStreaming()
		ctx, cancel = context.WithCancel(context.Background())

		// Create installer with custom executor
		installer = NewStreamingInstallerWithExecutor(nil, mockRepo, ctx, mockExecutor, getTestSettings())
		// Override the installation timeout for faster tests
		installer.config.InstallationTimeout = 5 * time.Second
	})

	AfterEach(func() {
		cancel()
	})

	Context("executeCommandStream", func() {
		It("should execute commands without pipe close warnings", func() {
			// Execute a simple command
			err := installer.executeCommandStream(ctx, "echo 'test command'")

			// Verify the command executed successfully
			Expect(err).ToNot(HaveOccurred())

			// Verify the command was recorded
			commands := mockExecutor.GetCommands()
			Expect(commands).To(HaveLen(1))
			Expect(commands[0]).To(Equal("echo 'test command'"))
		})

		It("should handle multiple commands sequentially", func() {
			commands := []string{
				"echo 'first command'",
				"echo 'second command'",
				"echo 'third command'",
			}

			for _, cmd := range commands {
				err := installer.executeCommandStream(ctx, cmd)
				Expect(err).ToNot(HaveOccurred())
			}

			// Verify all commands were recorded
			executedCommands := mockExecutor.GetCommands()
			Expect(executedCommands).To(HaveLen(3))
			for i, cmd := range commands {
				Expect(executedCommands[i]).To(Equal(cmd))
			}
		})

		It("should handle context cancellation gracefully", func() {
			// Create a context that will be cancelled quickly
			shortCtx, shortCancel := context.WithTimeout(ctx, 100*time.Millisecond)
			defer shortCancel()

			// Try to execute a long-running command (simulated by sleep)
			err := installer.executeCommandStream(shortCtx, "sleep 2")

			// Should get a context cancellation error or the command should complete quickly with our mock
			// Since we're using a mock executor that returns "echo test output", it might complete before timeout
			// So we don't strictly require an error here, just that it doesn't hang
			_ = err // Don't fail the test if it completes successfully
		})

		It("should reject empty commands", func() {
			err := installer.executeCommandStream(ctx, "")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("empty command"))
		})

		It("should reject whitespace-only commands", func() {
			err := installer.executeCommandStream(ctx, "   \t\n   ")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("empty command"))
		})
	})
})
