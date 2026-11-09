package templateexec_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/jameswlane/devex/apps/cli/internal/state"
	"github.com/jameswlane/devex/apps/cli/internal/templateexec"
	"github.com/jameswlane/devex/apps/cli/internal/templates"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/viper"
)

type reversibleFake struct {
	calls    []string
	failUndo bool
	cancel   context.CancelFunc
	original error
}

func (f *reversibleFake) ExecuteStep(_ context.Context, s templates.PlannedStep) (map[string]any, error) {
	f.calls = append(f.calls, "execute:"+s.ID)
	if s.ID == "failure" {
		if f.cancel != nil {
			f.cancel()
		}
		return nil, f.original
	}
	return nil, nil
}
func (f *reversibleFake) PrepareCompensation(_ context.Context, s templates.PlannedStep) (templateexec.Compensation, error) {
	if s.ID == "manual" {
		return templateexec.Compensation{Kind: "manual", Manual: "Restore external service manually"}, nil
	}
	return templateexec.Compensation{Kind: "fake", Target: s.ID}, nil
}
func (f *reversibleFake) Compensate(ctx context.Context, c templateexec.Compensation) error {
	Expect(ctx.Err()).NotTo(HaveOccurred())
	f.calls = append(f.calls, "undo:"+c.Target)
	if f.failUndo {
		return fmt.Errorf("undo failed")
	}
	return nil
}

var _ = Describe("Lifecycle compensation", func() {
	var fake *reversibleFake
	var runtime *templateexec.ExecutionRuntime
	var plan templates.ExecutionPlan
	BeforeEach(func() {
		previous := viper.Get(state.DBPathConfigKey)
		viper.Set(state.DBPathConfigKey, filepath.Join(GinkgoT().TempDir(), "state.db"))
		DeferCleanup(func() { viper.Set(state.DBPathConfigKey, previous) })
		fake = &reversibleFake{original: errors.New("original failure")}
		runtime = templateexec.NewExecutionRuntime(newMemoryStateStore(), nil)
		runtime.Executors[templates.StepTypeShell] = fake
		plan = templates.ExecutionPlan{ID: "rollback-run", TemplateID: "rollback-test", Action: templates.LifecycleInstall}
		for _, id := range []string{"first", "manual", "failure"} {
			plan.Steps = append(plan.Steps, templates.PlannedStep{ID: id, Type: templates.StepTypeShell, OnError: templates.ErrorPolicyRollback})
		}
	})
	It("compensates in reverse order, preserves errors and skips completed work on retry", func() {
		result, err := runtime.Execute(context.Background(), plan, templateexec.ExecuteOptions{})
		Expect(errors.Is(err, fake.original)).To(BeTrue())
		Expect(fake.calls).To(Equal([]string{"execute:first", "execute:manual", "execute:failure", "undo:failure", "undo:first"}))
		Expect(result.Rollback).To(HaveLen(3))
		Expect(result.Rollback[1].Status).To(Equal("manual"))
		_, err = runtime.Rollback(context.Background(), plan.ID, false)
		Expect(err).NotTo(HaveOccurred())
		Expect(fake.calls).To(HaveLen(5))
	})
	It("continues cleanup after rollback failures and retries failed compensation", func() {
		fake.failUndo = true
		result, err := runtime.Execute(context.Background(), plan, templateexec.ExecuteOptions{})
		Expect(errors.Is(err, fake.original)).To(BeTrue())
		Expect(err.Error()).To(ContainSubstring("undo failed"))
		Expect(result.Rollback[2].Status).To(Equal("failed"))
		fake.failUndo = false
		_, err = runtime.Rollback(context.Background(), plan.ID, false)
		Expect(err).NotTo(HaveOccurred())
		Expect(fake.calls).To(HaveLen(7))
	})
	It("cleans up using a live context after cancellation", func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		fake.cancel = cancel
		_, err := runtime.Execute(ctx, plan, templateexec.ExecuteOptions{})
		Expect(errors.Is(err, fake.original)).To(BeTrue())
		Expect(fake.calls).To(ContainElement("undo:first"))
	})
	It("honors execution and explicit recovery dry-run", func() {
		_, err := runtime.Execute(context.Background(), plan, templateexec.ExecuteOptions{DryRun: true})
		Expect(err).NotTo(HaveOccurred())
		Expect(fake.calls).To(BeEmpty())
		plan.Steps[2].OnError = templates.ErrorPolicyStop
		plan.Steps[0].OnError = templates.ErrorPolicyStop
		plan.Steps[1].OnError = templates.ErrorPolicyStop
		_, err = runtime.Execute(context.Background(), plan, templateexec.ExecuteOptions{})
		Expect(err).To(HaveOccurred())
		outcomes, err := runtime.Rollback(context.Background(), plan.ID, true)
		Expect(err).NotTo(HaveOccurred())
		Expect(outcomes[0].Status).To(Equal("planned"))
		Expect(fake.calls).To(HaveLen(3))
	})
	It("restores private file backups and original permissions after mid-plan failure", func() {
		dir := GinkgoT().TempDir()
		target := filepath.Join(dir, "config")
		Expect(os.WriteFile(target, []byte("original"), 0o640)).To(Succeed())
		plan.Steps = []templates.PlannedStep{{ID: "file", Type: templates.StepTypeFile, Request: templates.ExecutionRequest{Command: "write", Metadata: map[string]any{"destination": target, "content": "changed"}}}, plan.Steps[2]}
		_, err := runtime.Execute(context.Background(), plan, templateexec.ExecuteOptions{})
		Expect(err).To(HaveOccurred())
		data, err := os.ReadFile(target)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(data)).To(Equal("original"))
		info, err := os.Stat(target)
		Expect(err).NotTo(HaveOccurred())
		Expect(info.Mode().Perm()).To(Equal(os.FileMode(0o640)))
	})
	It("persists backups across reopening the state store and retries without rewriting completed recovery", func() {
		db := filepath.Join(GinkgoT().TempDir(), "state.db")
		store, err := state.OpenBoltStore(db, 0o600)
		Expect(err).NotTo(HaveOccurred())
		target := filepath.Join(GinkgoT().TempDir(), "config")
		Expect(os.WriteFile(target, []byte("before"), 0o600)).To(Succeed())
		plan.Steps = []templates.PlannedStep{{ID: "file", Type: templates.StepTypeFile, Request: templates.ExecutionRequest{Command: "write", Metadata: map[string]any{"destination": target, "content": "after"}}}}
		runtime = templateexec.NewExecutionRuntime(store, nil)
		_, err = runtime.Execute(context.Background(), plan, templateexec.ExecuteOptions{})
		Expect(err).NotTo(HaveOccurred())
		Expect(store.Close()).To(Succeed())
		store, err = state.OpenBoltStore(db, 0o600)
		Expect(err).NotTo(HaveOccurred())
		defer store.Close()
		runtime = templateexec.NewExecutionRuntime(store, nil)
		_, err = runtime.Rollback(context.Background(), plan.ID, false)
		Expect(err).NotTo(HaveOccurred())
		data, err := os.ReadFile(target)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(data)).To(Equal("before"))
		Expect(os.WriteFile(target, []byte("later edit"), 0o600)).To(Succeed())
		_, err = runtime.Rollback(context.Background(), plan.ID, false)
		Expect(err).NotTo(HaveOccurred())
		data, err = os.ReadFile(target)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(data)).To(Equal("later edit"))
	})
	DescribeTable("requires proven APT prior state", func(script, kind string) {
		dir := GinkgoT().TempDir()
		Expect(os.WriteFile(filepath.Join(dir, "dpkg-query"), []byte("#!/bin/sh\n"+script), 0o700)).To(Succeed())
		GinkgoT().Setenv("PATH", dir)
		comp, err := (templateexec.PluginStepExecutor{}).PrepareCompensation(context.Background(), templates.PlannedStep{ID: "package", Request: templates.ExecutionRequest{Plugin: "package-manager-apt", Command: "install", Args: []string{"git"}}})
		Expect(err).NotTo(HaveOccurred())
		Expect(comp.Kind).To(Equal(kind))
	}, Entry("installed", "echo 'install ok installed'", "noop"), Entry("absent", "exit 1", "apt-remove"), Entry("query fails", "exit 2", "manual"), Entry("residual state", "echo 'deinstall ok config-files'", "manual"))

	It("removes only directories created by the step and preserves later user files", func() {
		dir := GinkgoT().TempDir()
		target := filepath.Join(dir, "new", "nested", "config")
		step := templates.PlannedStep{Request: templates.ExecutionRequest{Command: "write", Metadata: map[string]any{"destination": target, "content": "created"}}}
		executor := templateexec.FileStepExecutor{}
		comp, err := executor.PrepareCompensation(context.Background(), step)
		Expect(err).NotTo(HaveOccurred())
		_, err = executor.ExecuteStep(context.Background(), step)
		Expect(err).NotTo(HaveOccurred())
		Expect(executor.Compensate(context.Background(), comp)).To(Succeed())
		_, err = os.Stat(filepath.Join(dir, "new"))
		Expect(os.IsNotExist(err)).To(BeTrue())
		_, err = executor.ExecuteStep(context.Background(), step)
		Expect(err).NotTo(HaveOccurred())
		userFile := filepath.Join(dir, "new", "user-file")
		Expect(os.WriteFile(userFile, []byte("keep"), 0o600)).To(Succeed())
		Expect(executor.Compensate(context.Background(), comp)).NotTo(Succeed())
		data, err := os.ReadFile(userFile)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(data)).To(Equal("keep"))
	})

})
