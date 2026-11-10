package templateexec_test

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/jameswlane/devex/apps/cli/internal/state"
	"github.com/jameswlane/devex/apps/cli/internal/templateexec"
	"github.com/jameswlane/devex/apps/cli/internal/templates"
	devexpaths "github.com/jameswlane/devex/packages/plugin-sdk/paths"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type resumeExecutor struct {
	calls  []string
	fail   bool
	cancel context.CancelFunc
}

func (e *resumeExecutor) ExecuteStep(_ context.Context, step templates.PlannedStep) (map[string]any, error) {
	e.calls = append(e.calls, step.ID)
	if step.ID == "failed" && e.fail {
		if e.cancel != nil {
			e.cancel()
			return nil, context.Canceled
		}
		return nil, errors.New("step failed")
	}
	return nil, nil
}

var _ = Describe("Resuming persisted runs", func() {
	var store *state.BoltStore
	var runtime *templateexec.ExecutionRuntime
	var executor *resumeExecutor
	var plan templates.ExecutionPlan
	BeforeEach(func() {
		var err error
		store, err = state.OpenBoltStore(filepath.Join(GinkgoT().TempDir(), "runs.db"), 0600)
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(store.Close)
		executor = &resumeExecutor{fail: true}
		runtime = templateexec.NewExecutionRuntime(store, nil)
		runtime.Executors[templates.StepTypeGroup] = executor
		plan = templates.ExecutionPlan{ID: "first", TemplateID: "resume", Action: templates.LifecycleInstall, Steps: []templates.PlannedStep{
			{ID: "safe", Type: templates.StepTypeGroup, Idempotent: true},
			{ID: "unsafe", Type: templates.StepTypeGroup},
			{ID: "failed", Type: templates.StepTypeGroup, Idempotent: true},
			{ID: "pending", Type: templates.StepTypeGroup, Idempotent: true},
		}}
	})
	It("authenticates even unused secret inputs and remains stable across attempt identity changes", func() {
		plan.RawInputs = map[string]any{"pin": "1234"}
		first, err := templateexec.PlanFingerprint(plan)
		Expect(err).NotTo(HaveOccurred())
		normalized := plan
		normalized.ID = ""
		normalized.CreatedAt = time.Time{}
		data, err := json.Marshal(struct {
			Plan      templates.ExecutionPlan
			RawInputs map[string]any
		}{normalized, normalized.RawInputs})
		Expect(err).NotTo(HaveOccurred())
		Expect(first).To(HavePrefix("hmac-sha256:"))
		Expect(first).NotTo(Equal(fmt.Sprintf("hmac-sha256:%x", sha256.Sum256(data))))
		plan.ID = "new-attempt"
		plan.CreatedAt = time.Now()
		same, err := templateexec.PlanFingerprint(plan)
		Expect(err).NotTo(HaveOccurred())
		Expect(same).To(Equal(first))
		plan.RawInputs["pin"] = "1235"
		changed, err := templateexec.PlanFingerprint(plan)
		Expect(err).NotTo(HaveOccurred())
		Expect(changed).NotTo(Equal(first))
	})
	It("rejects an old run after the installation key has been reset", func() {
		_, _ = runtime.Execute(context.Background(), plan, templateexec.ExecuteOptions{})
		dirs, err := devexpaths.Resolve()
		Expect(err).NotTo(HaveOccurred())
		Expect(os.Remove(filepath.Join(dirs.State, "resume.key"))).To(Succeed())
		plan.ID = "after-reset"
		_, err = runtime.Execute(context.Background(), plan, templateexec.ExecuteOptions{ResumeRunID: "first"})
		Expect(err).To(MatchError(ContainSubstring("fingerprint differs")))
	})
	It("skips only successful idempotent steps and supports repeated resumed failures", func() {
		_, err := runtime.Execute(context.Background(), plan, templateexec.ExecuteOptions{})
		Expect(err).To(MatchError("step failed"))
		plan.ID = "second"
		_, err = runtime.Execute(context.Background(), plan, templateexec.ExecuteOptions{ResumeRunID: "first"})
		Expect(err).To(MatchError("step failed"))
		Expect(executor.calls).To(Equal([]string{"safe", "unsafe", "failed", "unsafe", "failed"}))
		plan.ID = "third"
		executor.fail = false
		_, err = runtime.Execute(context.Background(), plan, templateexec.ExecuteOptions{ResumeRunID: "second"})
		Expect(err).NotTo(HaveOccurred())
		Expect(executor.calls).To(Equal([]string{"safe", "unsafe", "failed", "unsafe", "failed", "unsafe", "failed", "pending"}))
		run, err := store.GetRun(context.Background(), "third")
		Expect(err).NotTo(HaveOccurred())
		Expect(run.Status).To(Equal(state.RunStatusSucceeded))
		Expect(run.ResumeOf).To(Equal("second"))
	})
	It("rejects changed plans before executing or recording a new attempt", func() {
		_, _ = runtime.Execute(context.Background(), plan, templateexec.ExecuteOptions{})
		plan.ID = "changed"
		plan.Steps[0].Request.Args = []string{"changed-secret"}
		_, err := runtime.Execute(context.Background(), plan, templateexec.ExecuteOptions{ResumeRunID: "first"})
		Expect(err).To(MatchError(ContainSubstring("fingerprint differs")))
		Expect(executor.calls).To(HaveLen(3))
		_, err = store.GetRun(context.Background(), "changed")
		Expect(errors.Is(err, state.ErrNotFound)).To(BeTrue())
	})
	It("persists canceled run and operation status and retries interrupted work", func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		executor.cancel = cancel
		_, err := runtime.Execute(ctx, plan, templateexec.ExecuteOptions{})
		Expect(errors.Is(err, context.Canceled)).To(BeTrue())
		run, err := store.GetRun(context.Background(), "first")
		Expect(err).NotTo(HaveOccurred())
		Expect(run.Status).To(Equal(state.RunStatusCanceled))
		ops, err := store.ListOperationsByRun(context.Background(), "first")
		Expect(err).NotTo(HaveOccurred())
		Expect(ops[2].Status).To(Equal(state.OperationStatusCanceled))
		executor.fail = false
		plan.ID = "resumed"
		_, err = runtime.Execute(context.Background(), plan, templateexec.ExecuteOptions{ResumeRunID: "first"})
		Expect(err).NotTo(HaveOccurred())
	})
	It("recovers a dead process lock and marks its running run abandoned", func() {
		host, err := os.Hostname()
		Expect(err).NotTo(HaveOccurred())
		Expect(store.CreateRun(context.Background(), state.TemplateRun{ID: "killed", TemplateID: "resume", Status: state.RunStatusRunning})).To(Succeed())
		Expect(store.AcquireLifecycleLock(context.Background(), state.LifecycleLock{TemplateID: "resume", RunID: "killed", Action: templates.LifecycleInstall, Hostname: host, PID: 1 << 30}, state.LockOptions{})).To(Succeed())
		executor.fail = false
		_, err = runtime.Execute(context.Background(), plan, templateexec.ExecuteOptions{})
		Expect(err).NotTo(HaveOccurred())
		run, err := store.GetRun(context.Background(), "killed")
		Expect(err).NotTo(HaveOccurred())
		Expect(run.Status).To(Equal(state.RunStatusAbandoned))
	})
	It("rejects a live lock and recovers an expired remote lock", func() {
		lock := state.LifecycleLock{TemplateID: "resume", RunID: "other", Action: templates.LifecycleInstall, PID: os.Getpid()}
		Expect(store.AcquireLifecycleLock(context.Background(), lock, state.LockOptions{})).To(Succeed())
		_, err := runtime.Execute(context.Background(), plan, templateexec.ExecuteOptions{})
		Expect(errors.Is(err, state.ErrLifecycleLockActive)).To(BeTrue())
		Expect(store.ReleaseLifecycleLock(context.Background(), "resume", "other")).To(Succeed())
		lock.Hostname = "remote-host"
		Expect(store.AcquireLifecycleLock(context.Background(), lock, state.LockOptions{})).To(Succeed())
		executor.fail = false
		_, err = runtime.Execute(context.Background(), plan, templateexec.ExecuteOptions{LockStaleTimeout: time.Nanosecond})
		Expect(err).NotTo(HaveOccurred())
	})
	It("keeps old records inspectable while rejecting unsafe resume", func() {
		Expect(store.CreateRun(context.Background(), state.TemplateRun{ID: "legacy", TemplateID: "resume", Status: state.RunStatusFailed})).To(Succeed())
		runs, err := store.ListRuns(context.Background(), state.RunFilter{})
		Expect(err).NotTo(HaveOccurred())
		Expect(runs).To(HaveLen(1))
		_, err = runtime.Execute(context.Background(), plan, templateexec.ExecuteOptions{ResumeRunID: "legacy"})
		Expect(err).To(MatchError(ContainSubstring("predates")))
	})
})
