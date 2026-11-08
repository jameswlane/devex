package tui

import (
	"context"
	"errors"
	"io"
	"sync"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/types"
	"github.com/jameswlane/devex/packages/plugin-sdk/sdktest"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/cobra"
)

type lifecycleProgram struct {
	run    func() (tea.Model, error)
	killed chan struct{}
	once   sync.Once
}

func (p *lifecycleProgram) Run() (tea.Model, error) { return p.run() }
func (p *lifecycleProgram) Send(tea.Msg)            {}
func (p *lifecycleProgram) Kill()                   { p.once.Do(func() { close(p.killed) }) }

var _ = Describe("Installation program lifecycle", func() {
	It("returns a command error for one failed app and retains both successful registrations", func() {
		recorder := sdktest.NewRecorder(GinkgoT(), "install-result-tool")
		recorder.Script("install-result-tool", []string{"bad"}, sdktest.Response{ExitCode: 1})
		apps := make([]types.CrossPlatformApp, 0, 3)
		for _, name := range []string{"first", "bad", "last"} {
			apps = append(apps, types.CrossPlatformApp{Name: name, AllPlatforms: types.OSConfig{InstallMethod: "generic", InstallCommand: "install-result-tool " + name}})
		}
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		model := NewModel(apps)
		p := tea.NewProgram(model, tea.WithInput(nil), tea.WithOutput(io.Discard), tea.WithoutRenderer(), tea.WithoutSignalHandler())
		repo := &MockRepository{}
		installer := NewStreamingInstaller(p, repo, ctx, config.CrossPlatformSettings{})
		defer installer.cancel()
		installer.performanceAnalyzer = nil
		command := &cobra.Command{Use: "install", SilenceUsage: true, SilenceErrors: true, RunE: func(*cobra.Command, []string) error {
			return runInstallationProgram(ctx, cancel, model, p, func() error { return installer.InstallApps(ctx, apps, config.CrossPlatformSettings{}) })
		}}
		command.SetArgs([]string{})
		err := command.Execute()
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("bad"))
		Expect(recorder.Calls("install-result-tool")).To(HaveLen(3))
		Expect(repo.installedApps).To(ConsistOf("first", "last"))
	})
	It("returns worker panics and quits the TUI", func() {
		ctx, cancel := context.WithCancel(context.Background())
		model := NewModel(nil)
		p := tea.NewProgram(model, tea.WithInput(nil), tea.WithOutput(io.Discard), tea.WithoutRenderer(), tea.WithoutSignalHandler())
		err := runInstallationProgram(ctx, cancel, model, p, func() error { panic("worker exploded") })
		Expect(err).To(MatchError(ContainSubstring("worker exploded")))
	})
	It("joins the cancelled worker even if it reports success after quit", func() {
		ctx, cancel := context.WithCancel(context.Background())
		started := make(chan struct{})
		finished := make(chan struct{})
		p := &lifecycleProgram{killed: make(chan struct{}), run: func() (tea.Model, error) { <-started; return nil, nil }}
		err := runInstallationProgram(ctx, cancel, NewModel(nil), p, func() error {
			close(started)
			<-ctx.Done()
			close(finished)
			return nil
		})
		Expect(errors.Is(err, context.Canceled)).To(BeTrue())
		Expect(finished).To(BeClosed())
		Expect(p.killed).To(BeClosed())
	})
	It("preserves worker failures when the TUI fails", func() {
		ctx, cancel := context.WithCancel(context.Background())
		workerErr, tuiErr := errors.New("first app failed"), errors.New("terminal failed")
		p := &lifecycleProgram{killed: make(chan struct{}), run: func() (tea.Model, error) { return nil, tuiErr }}
		err := runInstallationProgram(ctx, cancel, NewModel(nil), p, func() error { <-ctx.Done(); return workerErr })
		Expect(errors.Is(err, workerErr)).To(BeTrue())
		Expect(errors.Is(err, tuiErr)).To(BeTrue())
	})
	It("preserves partial failures when the user quits during a later installation", func() {
		ctx, cancel := context.WithCancel(context.Background())
		started := make(chan struct{})
		workerErr := errors.New("bad: package unavailable; first installed successfully")
		p := &lifecycleProgram{killed: make(chan struct{}), run: func() (tea.Model, error) { <-started; return nil, nil }}
		err := runInstallationProgram(ctx, cancel, NewModel(nil), p, func() error {
			close(started)
			<-ctx.Done()
			return workerErr
		})
		Expect(errors.Is(err, context.Canceled)).To(BeTrue())
		Expect(errors.Is(err, workerErr)).To(BeTrue())
	})

	It("recovers a TUI panic after killing the program and joining its worker", func() {
		ctx, cancel := context.WithCancel(context.Background())
		workerErr := errors.New("partial install failure")
		p := &lifecycleProgram{killed: make(chan struct{}), run: func() (tea.Model, error) { panic("render failed") }}
		err := runInstallationProgram(ctx, cancel, NewModel(nil), p, func() error { <-ctx.Done(); return workerErr })
		Expect(errors.Is(err, workerErr)).To(BeTrue())
		Expect(err.Error()).To(ContainSubstring("render failed"))
		Expect(p.killed).To(BeClosed())
	})
	It("returns nil only when installation finishes successfully", func() {
		ctx, cancel := context.WithCancel(context.Background())
		model := NewModel(nil)
		p := tea.NewProgram(model, tea.WithInput(nil), tea.WithOutput(io.Discard), tea.WithoutRenderer(), tea.WithoutSignalHandler())
		Expect(runInstallationProgram(ctx, cancel, model, p, func() error { return nil })).To(Succeed())
	})
})
