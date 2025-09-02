package stream_test

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/jameswlane/devex/apps/cli/internal/installer/stream"
)

// Program interface that matches what we need from tea.Program
type Program interface {
	Send(tea.Msg)
}

// Mock TUI program for testing
type mockProgram struct {
	messages []stream.LogMessage
}

func (p *mockProgram) Send(msg tea.Msg) {
	if logMsg, ok := msg.(stream.LogMessage); ok {
		p.messages = append(p.messages, logMsg)
	}
}

// Mock input handler for testing
type mockInputHandler struct {
	responses map[string]string
}

func newMockInputHandler() *mockInputHandler {
	return &mockInputHandler{
		responses: make(map[string]string),
	}
}

func (h *mockInputHandler) RequestInput(ctx context.Context, prompt string, timeout time.Duration) string {
	if response, exists := h.responses[prompt]; exists {
		return response
	}
	return ""
}

func (h *mockInputHandler) SetResponse(prompt, response string) {
	h.responses[prompt] = response
}

// Mock stdin writer
type mockStdinWriter struct {
	written []string
}

func (w *mockStdinWriter) Write(p []byte) (int, error) {
	w.written = append(w.written, string(p))
	return len(p), nil
}

func (w *mockStdinWriter) Close() error {
	return nil
}

func TestStream(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Stream Manager Suite")
}

var _ = Describe("Stream Manager", func() {
	var (
		manager      *stream.Manager
		mockProg     *mockProgram
		ctx          context.Context
		cancel       context.CancelFunc
		inputHandler *mockInputHandler
	)

	BeforeEach(func() {
		ctx, cancel = context.WithCancel(context.Background())
		mockProg = &mockProgram{}
		manager = stream.New(mockProg, ctx)
		inputHandler = newMockInputHandler()
	})

	AfterEach(func() {
		if cancel != nil {
			cancel()
		}
	})

	Describe("Output Streaming", func() {
		It("should stream clean output", func() {
			input := "Hello World\nThis is a test"
			reader := strings.NewReader(input)

			manager.StreamOutput(reader, "TEST")

			// Should have logged messages
			Expect(len(mockProg.messages) >= 1).To(BeTrue())
		})

		It("should filter progress lines", func() {
			input := "Reading database... 50%\nReading database... done\nOther output"
			reader := strings.NewReader(input)

			manager.StreamOutput(reader, "APT")

			// Should filter incomplete progress but show complete ones
			found := false
			for _, msg := range mockProg.messages {
				if strings.Contains(msg.Message, "done") {
					found = true
					break
				}
			}
			Expect(found).To(BeTrue())
		})

		It("should handle context cancellation", func() {
			// Create a reader that would block
			reader := &blockingReader{make(chan []byte, 1)}

			// Cancel context before streaming
			cancel()

			// Should return quickly without hanging
			done := make(chan bool, 1)
			go func() {
				manager.StreamOutput(reader, "TEST")
				done <- true
			}()

			select {
			case <-done:
				// Good, returned quickly
			case <-time.After(1 * time.Second):
				Fail("StreamOutput did not respect context cancellation")
			}
		})
	})

	Describe("Input Monitoring", func() {
		It("should detect password prompts", func() {
			input := "Enter password for user:\nSudo password required:"
			reader := strings.NewReader(input)
			stdin := &mockStdinWriter{}

			inputHandler.SetResponse("Enter password for user:", "testpass")

			manager.MonitorForInput(reader, stdin, inputHandler)

			// Should have written password to stdin
			Expect(len(stdin.written)).To(BeNumerically(">", 0))
		})

		It("should ignore non-password prompts", func() {
			input := "Processing files...\nInstallation complete"
			reader := strings.NewReader(input)
			stdin := &mockStdinWriter{}

			manager.MonitorForInput(reader, stdin, inputHandler)

			// Should not have written anything
			Expect(len(stdin.written)).To(Equal(0))
		})
	})

	Describe("Terminal Output Cleaning", func() {
		It("should remove ANSI escape sequences", func() {
			input := "\x1b[32mGreen text\x1b[0m"
			cleaned := stream.CleanTerminalOutput(input)
			Expect(cleaned).To(Equal("Green text"))
		})

		It("should remove cursor positioning", func() {
			input := "\x1b[2AMove up\x1b[1B"
			cleaned := stream.CleanTerminalOutput(input)
			Expect(cleaned).To(Equal("Move up"))
		})

		It("should remove carriage returns", func() {
			input := "Progress: 50%\rProgress: 100%"
			cleaned := stream.CleanTerminalOutput(input)
			Expect(cleaned).To(Equal("Progress: 50%Progress: 100%"))
		})

		It("should preserve normal text", func() {
			input := "Normal text with spaces and 123 numbers"
			cleaned := stream.CleanTerminalOutput(input)
			Expect(cleaned).To(Equal(input))
		})

		It("should remove control characters but preserve tabs", func() {
			input := "Text\twith\x01tab\x02and\x03control\x04chars"
			cleaned := stream.CleanTerminalOutput(input)
			Expect(cleaned).To(Equal("Text\twithtabandcontrolchars"))
		})
	})

	Describe("Logging", func() {
		It("should log with different levels", func() {
			levels := []string{"INFO", "ERROR", "WARN", "DEBUG"}

			for _, level := range levels {
				manager.Log(level, fmt.Sprintf("Test %s message", level))
			}

			// Should have messages for each level
			Expect(len(mockProg.messages)).To(Equal(len(levels)))
		})

		It("should handle logging without TUI program", func() {
			nilManager := stream.New(nil, ctx)

			// Should not panic
			Expect(func() {
				nilManager.Log("INFO", "Test message")
			}).ToNot(Panic())
		})

		It("should handle context cancellation during logging", func() {
			cancel()

			// Should not panic or hang
			Expect(func() {
				manager.Log("INFO", "Test message")
			}).ToNot(Panic())
		})
	})

	Describe("Configuration", func() {
		It("should use custom configuration", func() {
			config := stream.Config{
				MaxLogLines:         500,
				InputTimeout:        10 * time.Second,
				InitializationDelay: 1 * time.Second,
			}

			customManager := stream.NewWithConfig(mockProg, ctx, config)
			Expect(customManager).ToNot(BeNil())
		})

		It("should provide default configuration", func() {
			config := stream.DefaultConfig()
			Expect(config.MaxLogLines).To(Equal(1000))
			Expect(config.InputTimeout).To(Equal(30 * time.Second))
		})
	})
})

// Helper types for testing
type blockingReader struct {
	ch chan []byte
}

func (r *blockingReader) Read(p []byte) (int, error) {
	select {
	case data := <-r.ch:
		n := copy(p, data)
		return n, nil
	default:
		return 0, nil // Would normally block, but return immediately for testing
	}
}
