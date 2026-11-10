package templateexec_test

import (
	"context"
	"path/filepath"

	"github.com/jameswlane/devex/apps/cli/internal/state"
	"github.com/jameswlane/devex/apps/cli/internal/templateexec"
	"github.com/jameswlane/devex/apps/cli/internal/templates"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type concurrentStepExecutor struct {
	entered chan string
	release chan struct{}
}

func (e concurrentStepExecutor) ExecuteStep(ctx context.Context, step templates.PlannedStep) (map[string]any, error) {
	e.entered <- step.ID
	select {
	case <-e.release:
		return nil, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

var _ = Describe("Concurrent template journal", func() {
	It("journals overlapping steps and inherits both on resume", func() {
		store, err := state.OpenBoltStore(filepath.Join(GinkgoT().TempDir(), "parallel.db"), 0600)
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(store.Close)
		entered := make(chan string, 2)
		release := make(chan struct{})
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		runtime := templateexec.NewExecutionRuntime(store, nil)
		runtime.Executors[templates.StepTypePlugin] = concurrentStepExecutor{entered, release}
		plan := templates.ExecutionPlan{ID: "first", TemplateID: "parallel", Action: templates.LifecycleInstall, Steps: []templates.PlannedStep{
			{ID: "one", Type: templates.StepTypePlugin, Idempotent: true, Request: templates.ExecutionRequest{Plugin: "package-manager-mise"}},
			{ID: "two", Type: templates.StepTypePlugin, Idempotent: true, Request: templates.ExecutionRequest{Plugin: "package-manager-flatpak"}},
		}}
		done := make(chan error, 1)
		go func() { _, err := runtime.Execute(ctx, plan, templateexec.ExecuteOptions{Concurrency: 2}); done <- err }()
		Eventually(entered).Should(Receive())
		Eventually(entered).Should(Receive())
		close(release)
		Eventually(done).Should(Receive(Succeed()))
		ops, err := store.ListOperationsByRun(ctx, "first")
		Expect(err).NotTo(HaveOccurred())
		Expect(ops).To(HaveLen(2))
		for _, op := range ops {
			Expect(op.Status).To(Equal(state.OperationStatusSucceeded))
		}
		// Mark the parent interrupted after both steps finished to exercise resume.
		Expect(store.CompleteRun(ctx, "first", state.RunStatusFailed, "interrupted")).To(Succeed())
		plan.ID = "second"
		result, err := runtime.Execute(ctx, plan, templateexec.ExecuteOptions{Concurrency: 2, ResumeRunID: "first"})
		Expect(err).NotTo(HaveOccurred())
		Expect(result.StepsExecuted).To(BeZero())
		Expect(entered).NotTo(Receive())
	})
})
