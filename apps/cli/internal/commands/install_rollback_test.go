package commands

import (
	"bytes"
	"context"
	"errors"
	"path/filepath"

	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/mocks"
	"github.com/jameswlane/devex/apps/cli/internal/state"
	"github.com/jameswlane/devex/apps/cli/internal/types"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/viper"
)

type failingRegistrationRepository struct{ types.Repository }

func (f failingRegistrationRepository) Set(string, string) error {
	return errors.New("tracking failure")
}

var _ = Describe("Rollback application batches", func() {
	DescribeTable("retains normal registration and stops on failure", func(fail bool) {
		oldPrepare, oldInstall := prepareApplicationInstallers, installApplication
		oldPath := viper.Get(state.DBPathConfigKey)
		DeferCleanup(func() {
			prepareApplicationInstallers, installApplication = oldPrepare, oldInstall
			viper.Set(state.DBPathConfigKey, oldPath)
		})
		viper.Set(state.DBPathConfigKey, filepath.Join(GinkgoT().TempDir(), "state.db"))
		prepareApplicationInstallers = func(context.Context, pluginBootstrapProvider) error { return nil }
		original := errors.New("batch install failure")
		var calls []string
		installApplication = func(_ context.Context, app types.CrossPlatformApp, _ config.CrossPlatformSettings, _ types.Repository) error {
			calls = append(calls, app.Name)
			if fail && app.Name == "second" {
				return original
			}
			return nil
		}
		apps := make([]types.CrossPlatformApp, 0, 3)
		for _, name := range []string{"first", "second", "third"} {
			apps = append(apps, types.CrossPlatformApp{Name: name, AllPlatforms: types.OSConfig{InstallMethod: "docker", InstallCommand: name}})
		}
		settings := config.CrossPlatformSettings{ApplicationOverrides: apps}
		var output bytes.Buffer
		repo := mocks.NewMockRepository()
		err := runRollbackBatch(context.Background(), &output, apps, repo, settings, nil)
		if !fail {
			Expect(err).NotTo(HaveOccurred())
			Expect(calls).To(Equal([]string{"first", "second", "third"}))
			app, err := repo.GetApp("first")
			Expect(err).NotTo(HaveOccurred())
			Expect(app).NotTo(BeNil())
			definition, err := repo.Get("installed-app-definition:first")
			Expect(err).NotTo(HaveOccurred())
			Expect(definition).To(ContainSubstring("first"))
			return
		}
		Expect(errors.Is(err, original)).To(BeTrue())
		Expect(calls).To(Equal([]string{"first", "second"}))
		Expect(output.String()).To(ContainSubstring("manual"))
		Expect(output.String()).To(ContainSubstring("Lifecycle run:"))
	}, Entry("failure", true), Entry("success", false))
	It("records registration failures and requests compensation after package execution", func() {
		oldPrepare, oldInstall := prepareApplicationInstallers, installApplication
		oldPath := viper.Get(state.DBPathConfigKey)
		DeferCleanup(func() {
			prepareApplicationInstallers, installApplication = oldPrepare, oldInstall
			viper.Set(state.DBPathConfigKey, oldPath)
		})
		viper.Set(state.DBPathConfigKey, filepath.Join(GinkgoT().TempDir(), "state.db"))
		prepareApplicationInstallers = func(context.Context, pluginBootstrapProvider) error { return nil }
		installApplication = func(context.Context, types.CrossPlatformApp, config.CrossPlatformSettings, types.Repository) error {
			return nil
		}
		apps := []types.CrossPlatformApp{{Name: "test", AllPlatforms: types.OSConfig{InstallMethod: "docker", InstallCommand: "test"}}}
		var output bytes.Buffer
		err := runRollbackBatch(context.Background(), &output, apps, failingRegistrationRepository{mocks.NewMockRepository()}, config.CrossPlatformSettings{ApplicationOverrides: apps}, nil)
		Expect(err).To(MatchError("tracking failure"))
		Expect(output.String()).To(ContainSubstring("manual"))
		store, err := openTemplateStateStore()
		Expect(err).NotTo(HaveOccurred())
		defer store.Close()
		runs, err := store.ListRuns(context.Background(), state.RunFilter{})
		Expect(err).NotTo(HaveOccurred())
		Expect(runs).To(HaveLen(1))
		Expect(runs[0].Status).To(Equal(state.RunStatusFailed))
		Expect(runs[0].Error).To(Equal("tracking failure"))
	})

})

var _ = Describe("Recorded rollback JSON", func() {
	It("emits only structured outcomes for a dry-run without executing compensation", func() {
		oldPath, oldDry := viper.Get(state.DBPathConfigKey), viper.Get("dry-run")
		DeferCleanup(func() { viper.Set(state.DBPathConfigKey, oldPath); viper.Set("dry-run", oldDry) })
		db := filepath.Join(GinkgoT().TempDir(), "state.db")
		viper.Set(state.DBPathConfigKey, db)
		viper.Set("dry-run", true)
		store, err := state.OpenBoltStore(db, 0o600)
		Expect(err).NotTo(HaveOccurred())
		Expect(store.CreateRun(context.Background(), state.TemplateRun{ID: "json-run", TemplateID: "test", Status: state.RunStatusFailed})).To(Succeed())
		Expect(store.RecordOperation(context.Background(), state.OperationRecord{ID: "op", RunID: "json-run", TemplateID: "test", StepID: "manual", Outputs: map[string]any{"lifecycle_compensation": map[string]any{"kind": "manual", "manual": "restore service"}}})).To(Succeed())
		Expect(store.Close()).To(Succeed())
		cmd := NewRollbackCmd(mocks.NewMockRepository(), config.CrossPlatformSettings{})
		var output bytes.Buffer
		cmd.SetOut(&output)
		cmd.SetArgs([]string{"--run", "json-run", "--format", "json"})
		Expect(cmd.Execute()).To(Succeed())
		Expect(output.String()).To(MatchJSON(`[{"step_id":"manual","status":"manual","detail":"restore service"}]`))
	})
})
