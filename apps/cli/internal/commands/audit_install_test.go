package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/jameswlane/devex/apps/cli/internal/fs"
	"github.com/jameswlane/devex/apps/cli/internal/log"
	sdk "github.com/jameswlane/devex/packages/plugin-sdk"

	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/installers"
	"github.com/jameswlane/devex/apps/cli/internal/mocks"
	"github.com/jameswlane/devex/apps/cli/internal/state"
	"github.com/jameswlane/devex/apps/cli/internal/types"
	"github.com/jameswlane/devex/packages/plugin-sdk/sdktest"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/viper"
)

var _ = Describe("Plain installation audit", func() {
	BeforeEach(func() {
		viper.Reset()
		DeferCleanup(viper.Reset)
		viper.Set(state.DBPathConfigKey, filepath.Join(GinkgoT().TempDir(), "state.db"))
		old := prepareApplicationInstallers
		prepareApplicationInstallers = func(context.Context, pluginBootstrapProvider) error { return nil }
		DeferCleanup(func() { prepareApplicationInstallers = old })
		installers.EnableTestMode()
		DeferCleanup(installers.DisableTestMode)
	})
	DescribeTable("exports complete operation histories", func(outcome string) {
		recorder := sdktest.NewRecorder(GinkgoT(), "bash")
		if outcome == "failed" {
			recorder.Script("bash", []string{"-c", "audit-hook"}, sdktest.Response{ExitCode: 1})
		}
		app := types.CrossPlatformApp{Name: "audit-app", AllPlatforms: types.OSConfig{InstallMethod: "apt", InstallCommand: "audit-package", PreInstall: []types.InstallCommand{{Shell: "audit-hook"}}}}
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		if outcome == "canceled" {
			original := sdk.NewCommandContext
			sdk.NewCommandContext = func(ctx context.Context, name string, args ...string) *exec.Cmd {
				cancel()
				return original(ctx, name, args...)
			}
			DeferCleanup(func() { sdk.NewCommandContext = original })
		}
		var output bytes.Buffer
		err := runPlainInstall(state.WithAuditVersion(ctx, "test-version"), &output, []types.CrossPlatformApp{app}, mocks.NewMockRepository(), config.CrossPlatformSettings{}, nil)
		if outcome == "succeeded" {
			Expect(err).NotTo(HaveOccurred())
		} else {
			Expect(err).To(HaveOccurred())
		}
		cmd := NewRunsCmd()
		output.Reset()
		cmd.SetOut(&output)
		cmd.SetArgs([]string{"audit", "--format", "json", "--app", "audit-app", "--mode", "plain"})
		Expect(cmd.Execute()).To(Succeed())
		var events []state.AuditEvent
		Expect(json.Unmarshal(output.Bytes(), &events)).To(Succeed())
		Expect(events).To(HaveLen(12))
		Expect(events[len(events)-1].Status).To(Equal(state.OperationStatus(outcome)))
		Expect(events[0].Actor).To(Equal("devex-test-user"))
		Expect(events[0].ToolVersion).To(Equal("test-version"))
		stages := map[string]int{}
		for _, event := range events {
			stages[event.StepID]++
			Expect(event.RunID).To(Equal(events[0].RunID))
		}
		Expect(stages).To(Equal(map[string]int{"application": 2, "pre-install": 2, "install": 2, "config-files": 2, "post-install": 2, "registration": 2}))
		calls := recorder.Calls("bash")
		if outcome == "canceled" {
			Expect(calls).To(BeEmpty())
		} else {
			Expect(calls).To(HaveLen(1))
		}
	}, Entry("success", "succeeded"), Entry("failure", "failed"), Entry("cancellation", "canceled"))
	It("copies and audits configuration files through the plain lifecycle", func() {
		sdktest.NewRecorder(GinkgoT())
		oldFS := fs.AppFs
		fs.UseOsFs()
		DeferCleanup(func() { fs.SetFs(oldFS) })
		dir := GinkgoT().TempDir()
		// config_files destinations must be inside $HOME (the suite's temporary HOME).
		source := filepath.Join(dir, "source")
		destination := filepath.Join(os.Getenv("HOME"), ".config", "audit-config", "destination")
		Expect(os.WriteFile(source, []byte("configuration"), 0o600)).To(Succeed())
		app := types.CrossPlatformApp{Name: "audit-config", AllPlatforms: types.OSConfig{InstallMethod: "apt", InstallCommand: "audit-package", ConfigFiles: []types.ConfigFile{{Source: source, Destination: destination}}}}
		Expect(runPlainInstall(context.Background(), &bytes.Buffer{}, []types.CrossPlatformApp{app}, mocks.NewMockRepository(), config.CrossPlatformSettings{}, nil)).To(Succeed())
		content, err := os.ReadFile(destination)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(content)).To(Equal("configuration"))
		cmd := NewRunsCmd()
		var output bytes.Buffer
		cmd.SetOut(&output)
		cmd.SetArgs([]string{"audit", "--event", "result", "--status", "succeeded"})
		Expect(cmd.Execute()).To(Succeed())
		var events []state.AuditEvent
		Expect(json.Unmarshal(output.Bytes(), &events)).To(Succeed())
		stages := make([]string, 0, len(events))
		for _, event := range events {
			stages = append(stages, event.StepID)
		}
		Expect(stages).To(ContainElement("config-files"))
	})
	It("completes installation when audit storage is unwritable and logs the failure", func() {
		sdktest.NewRecorder(GinkgoT())
		file := filepath.Join(GinkgoT().TempDir(), "file")
		Expect(os.WriteFile(file, nil, 0o600)).To(Succeed())
		viper.Set(state.DBPathConfigKey, filepath.Join(file, "state.db"))
		var logs bytes.Buffer
		log.InitDefaultLogger(&logs)
		DeferCleanup(log.InitTestLogger)
		app := types.CrossPlatformApp{Name: "audit-app", AllPlatforms: types.OSConfig{InstallMethod: "apt", InstallCommand: "audit-package"}}
		Expect(runPlainInstall(context.Background(), &bytes.Buffer{}, []types.CrossPlatformApp{app}, mocks.NewMockRepository(), config.CrossPlatformSettings{}, nil)).To(Succeed())
		Expect(logs.String()).To(ContainSubstring("Installation audit write failed"))
	})

	It("rejects unsupported export formats before opening a database", func() {
		cmd := NewRunsCmd()
		cmd.SetArgs([]string{"audit", "--format", "yaml"})
		Expect(cmd.Execute()).To(MatchError(ContainSubstring("unsupported audit format")))
	})
})
