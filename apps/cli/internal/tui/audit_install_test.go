package tui

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/jameswlane/devex/apps/cli/internal/log"

	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/fs"
	"github.com/jameswlane/devex/apps/cli/internal/state"
	"github.com/jameswlane/devex/apps/cli/internal/types"
	"github.com/jameswlane/devex/packages/plugin-sdk/sdktest"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/viper"
)

type auditCancelExecutor struct {
	CommandExecutor
	cancel context.CancelFunc
}

func (e auditCancelExecutor) ExecuteCommand(ctx context.Context, command string) (*exec.Cmd, error) {
	e.cancel()
	return e.CommandExecutor.ExecuteCommand(ctx, command)
}

var _ = Describe("TUI installation audit", func() {
	BeforeEach(func() {
		viper.Reset()
		DeferCleanup(viper.Reset)
		viper.Set(state.DBPathConfigKey, filepath.Join(GinkgoT().TempDir(), "state.db"))
	})
	read := func() []state.AuditEvent {
		path, err := state.ConfiguredDBPath()
		Expect(err).NotTo(HaveOccurred())
		store, err := state.OpenBoltStore(path, 0o600)
		Expect(err).NotTo(HaveOccurred())
		defer store.Close()
		events, err := store.ListAudit(state.AuditFilter{Mode: "tui"})
		Expect(err).NotTo(HaveOccurred())
		return events
	}
	DescribeTable("records installation outcomes with fake native commands", func(outcome string) {
		recorder := sdktest.NewRecorder(GinkgoT(), "audit-tool")
		if outcome == "failed" {
			recorder.Script("audit-tool", []string{"install"}, sdktest.Response{ExitCode: 1})
		}
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		var executor CommandExecutor = NewDefaultCommandExecutor()
		if outcome == "canceled" {
			executor = auditCancelExecutor{executor, cancel}
		}
		installer := NewStreamingInstallerWithExecutor(nil, &MockRepository{}, ctx, executor, config.CrossPlatformSettings{})
		DeferCleanup(installer.cancel)
		installer.performanceAnalyzer = nil
		app := types.CrossPlatformApp{Name: "audit-app", AllPlatforms: types.OSConfig{InstallMethod: "generic", InstallCommand: "audit-tool install"}}
		err := installer.InstallApps(state.WithAuditVersion(ctx, "test-version"), []types.CrossPlatformApp{app}, config.CrossPlatformSettings{})
		if outcome == "succeeded" {
			Expect(err).NotTo(HaveOccurred())
		} else {
			Expect(err).To(HaveOccurred())
		}
		events := read()
		Expect(events).To(HaveLen(12))
		Expect(events[len(events)-1].Status).To(Equal(state.OperationStatus(outcome)))
		Expect(events[0].Actor).To(Equal("devex-test-user"))
		Expect(events[0].ToolVersion).To(Equal("test-version"))
		stages := map[string]int{}
		for _, event := range events {
			stages[event.StepID]++
			Expect(event.RunID).To(Equal(events[0].RunID))
			Expect(event.PlanID).To(Equal(events[0].PlanID))
		}
		Expect(stages).To(Equal(map[string]int{"application": 2, "pre-install": 2, "install": 2, "config-files": 2, "post-install": 2, "registration": 2}))
		calls := recorder.Calls("audit-tool")
		if outcome == "canceled" {
			Expect(calls).To(BeEmpty())
		} else {
			Expect(calls).To(HaveLen(1))
		}
	}, Entry("success", "succeeded"), Entry("failure", "failed"), Entry("mid-install cancellation", "canceled"))
	It("completes installation when audit storage is unwritable and logs the failure", func() {
		sdktest.NewRecorder(GinkgoT(), "audit-tool")
		file := filepath.Join(GinkgoT().TempDir(), "file")
		Expect(os.WriteFile(file, nil, 0o600)).To(Succeed())
		viper.Set(state.DBPathConfigKey, filepath.Join(file, "state.db"))
		var logs bytes.Buffer
		log.InitDefaultLogger(&logs)
		DeferCleanup(log.InitTestLogger)
		installer := NewStreamingInstaller(nil, &MockRepository{}, context.Background(), config.CrossPlatformSettings{})
		DeferCleanup(installer.cancel)
		installer.performanceAnalyzer = nil
		app := types.CrossPlatformApp{Name: "audit-app", AllPlatforms: types.OSConfig{InstallMethod: "generic", InstallCommand: "audit-tool"}}
		Expect(installer.InstallApps(context.Background(), []types.CrossPlatformApp{app}, config.CrossPlatformSettings{})).To(Succeed())
		Expect(logs.String()).To(ContainSubstring("Installation audit write failed"))
	})

	It("audits successful config copies and optional failed hooks without losing registration", func() {
		recorder := sdktest.NewRecorder(GinkgoT(), "audit-tool", "bash")
		recorder.Script("bash", []string{"-c", "audit-hook"}, sdktest.Response{ExitCode: 1})
		oldFS := fs.AppFs
		fs.UseOsFs()
		DeferCleanup(func() { fs.SetFs(oldFS) })
		dir := GinkgoT().TempDir()
		// config_files destinations must be inside $HOME (the suite's temporary HOME).
		source := filepath.Join(dir, "source")
		destination := filepath.Join(os.Getenv("HOME"), ".config", "audit-config", "destination")
		Expect(os.WriteFile(source, []byte("configuration"), 0o600)).To(Succeed())
		app := types.CrossPlatformApp{Name: "audit-app", AllPlatforms: types.OSConfig{InstallMethod: "generic", InstallCommand: "audit-tool", ConfigFiles: []types.ConfigFile{{Source: source, Destination: destination}}, PostInstall: []types.InstallCommand{{Shell: "audit-hook"}}}}
		installer := NewStreamingInstaller(nil, &MockRepository{}, context.Background(), config.CrossPlatformSettings{})
		DeferCleanup(installer.cancel)
		installer.performanceAnalyzer = nil
		Expect(installer.InstallApps(context.Background(), []types.CrossPlatformApp{app}, config.CrossPlatformSettings{})).To(HaveOccurred())
		content, err := os.ReadFile(destination)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(content)).To(Equal("configuration"))
		results := map[string]state.OperationStatus{}
		for _, event := range read() {
			if event.Event == "result" {
				results[event.StepID] = event.Status
			}
		}
		Expect(results["config-files"]).To(Equal(state.OperationStatusSucceeded))
		Expect(results["post-install"]).To(Equal(state.OperationStatusFailed))
		Expect(results["registration"]).To(Equal(state.OperationStatusSucceeded))
		Expect(results["application"]).To(Equal(state.OperationStatusSucceeded))
	})
})
