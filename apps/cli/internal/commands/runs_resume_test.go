package commands

import (
	"bytes"
	"context"
	"path/filepath"

	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/mocks"
	"github.com/jameswlane/devex/apps/cli/internal/state"
	"github.com/jameswlane/devex/apps/cli/internal/types"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/viper"
)

var _ = Describe("Run resume command", func() {
	BeforeEach(func() {
		viper.Reset()
		DeferCleanup(viper.Reset)
		viper.Set(state.DBPathConfigKey, filepath.Join(GinkgoT().TempDir(), "runs.db"))
	})
	It("replays a canceled application selection and links the new attempt", func() {
		apps := []types.CrossPlatformApp{{Name: "app", AllPlatforms: types.OSConfig{InstallMethod: "generic", InstallCommand: "fake"}}}
		settings := config.CrossPlatformSettings{ApplicationOverrides: apps}
		run, err := state.StartApplicationRun(context.Background(), apps, settings)
		Expect(err).NotTo(HaveOccurred())
		Expect(run.Finish(context.Background(), context.Canceled)).To(Succeed())
		oldPrepare, oldInstall := prepareApplicationInstallers, installApplication
		DeferCleanup(func() { prepareApplicationInstallers = oldPrepare; installApplication = oldInstall })
		prepareApplicationInstallers = func(context.Context, pluginBootstrapProvider) error { return nil }
		calls := 0
		installApplication = func(_ context.Context, app types.CrossPlatformApp, _ config.CrossPlatformSettings, _ types.Repository) error {
			calls++
			Expect(app.Name).To(Equal("app"))
			return nil
		}
		repo := mocks.NewMockRepository()
		Expect(repo.AddApp("app")).To(Succeed())
		oldNative := nativeApplicationInstalled
		nativeApplicationInstalled = func(context.Context, types.CrossPlatformApp) (bool, error) { return true, nil }
		DeferCleanup(func() { nativeApplicationInstalled = oldNative })
		cmd := newRunsResumeCmd(repo, settings, nil)
		cmd.SetOut(&bytes.Buffer{})
		cmd.SetArgs([]string{run.ID})
		Expect(cmd.Execute()).To(Succeed())
		Expect(calls).To(Equal(1))
		store, err := openTemplateStateStore()
		Expect(err).NotTo(HaveOccurred())
		defer store.Close()
		runs, err := store.ListRuns(context.Background(), state.RunFilter{})
		Expect(err).NotTo(HaveOccurred())
		Expect(runs).To(HaveLen(2))
		Expect(runs[1].ResumeOf).To(Equal(run.ID))
		Expect(runs[1].Status).To(Equal(state.RunStatusSucceeded))
	})
	It("rejects inherited dry-run before opening state or executing work", func() {
		viper.Set("dry-run", true)
		cmd := newRunsResumeCmd(mocks.NewMockRepository(), config.CrossPlatformSettings{}, nil)
		cmd.SetArgs([]string{"missing"})
		Expect(cmd.Execute()).To(MatchError(ContainSubstring("resume does not support --dry-run")))
	})
	It("rejects a changed application definition without invoking an installer", func() {
		apps := []types.CrossPlatformApp{{Name: "app"}}
		settings := config.CrossPlatformSettings{ApplicationOverrides: apps}
		run, err := state.StartApplicationRun(context.Background(), apps, settings)
		Expect(err).NotTo(HaveOccurred())
		Expect(run.Finish(context.Background(), context.Canceled)).To(Succeed())
		settings.DebugMode = true
		cmd := newRunsResumeCmd(mocks.NewMockRepository(), settings, nil)
		cmd.SetArgs([]string{run.ID})
		Expect(cmd.Execute()).To(MatchError(ContainSubstring("fingerprint differs")))
	})
})
