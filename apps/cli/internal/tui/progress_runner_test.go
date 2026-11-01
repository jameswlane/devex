package tui

import (
	"context"
	"io"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/jameswlane/devex/apps/cli/internal/config"
)

// blockingReader never returns data, like an idle keyboard.
type blockingReader struct{ stop chan struct{} }

func (r blockingReader) Read([]byte) (int, error) {
	<-r.stop
	return 0, io.EOF
}

var _ = Describe("ProgressRunner lifecycle", func() {
	var (
		originalInteractive func() bool
		originalNewProgram  func(tea.Model) *tea.Program
		stopInput           chan struct{}
	)

	BeforeEach(func() {
		originalInteractive = isInteractiveTerminal
		originalNewProgram = newProgressProgram
		stopInput = make(chan struct{})
	})

	AfterEach(func() {
		isInteractiveTerminal = originalInteractive
		newProgressProgram = originalNewProgram
		close(stopInput)
	})

	useTestProgram := func() {
		isInteractiveTerminal = func() bool { return true }
		newProgressProgram = func(model tea.Model) *tea.Program {
			return tea.NewProgram(model,
				tea.WithInput(blockingReader{stop: stopInput}),
				tea.WithOutput(io.Discard),
				tea.WithoutSignalHandler(),
			)
		}
	}

	Context("in an interactive terminal", func() {
		BeforeEach(useTestProgram)

		It("waits for the TUI to exit before Quit returns", func() {
			runner := NewProgressRunner(context.Background(), config.CrossPlatformSettings{})
			runner.start()

			quitReturned := make(chan struct{})
			go func() {
				runner.Quit()
				close(quitReturned)
			}()

			Eventually(quitReturned, 3*time.Second).Should(BeClosed())
			// Done must already be closed when Quit returns: the terminal is restored.
			Expect(runner.Done()).To(BeClosed())
		})

		It("is safe to call Quit more than once", func() {
			runner := NewProgressRunner(context.Background(), config.CrossPlatformSettings{})
			runner.start()

			runner.Quit()
			Expect(runner.Quit).NotTo(Panic())
			Expect(runner.Done()).To(BeClosed())
		})

		It("returns from Quit when the user already closed the TUI", func() {
			runner := NewProgressRunner(context.Background(), config.CrossPlatformSettings{})
			runner.start()

			runner.program.Send(tea.Quit())
			Eventually(runner.Done(), 3*time.Second).Should(BeClosed())

			quitReturned := make(chan struct{})
			go func() {
				runner.Quit()
				close(quitReturned)
			}()
			Eventually(quitReturned, 3*time.Second).Should(BeClosed())
		})
	})

	Context("when stdin or stdout is not a terminal", func() {
		BeforeEach(func() {
			isInteractiveTerminal = func() bool { return false }
			newProgressProgram = func(tea.Model) *tea.Program {
				Fail("the TUI must not be created without a terminal")
				return nil
			}
		})

		It("does not start a TUI", func() {
			runner := NewProgressRunner(context.Background(), config.CrossPlatformSettings{})
			Expect(runner.program).To(BeNil())
			runner.start()

			quitReturned := make(chan struct{})
			go func() {
				runner.Quit()
				close(quitReturned)
			}()
			Eventually(quitReturned, time.Second).Should(BeClosed())
			Expect(runner.Done()).NotTo(BeClosed())
		})
	})

	Context("when Quit is called without running an operation", func() {
		BeforeEach(useTestProgram)

		It("returns immediately", func() {
			runner := NewProgressRunner(context.Background(), config.CrossPlatformSettings{})

			quitReturned := make(chan struct{})
			go func() {
				runner.Quit()
				close(quitReturned)
			}()
			Eventually(quitReturned, time.Second).Should(BeClosed())
		})
	})
})
