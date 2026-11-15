package templateexec_test

import (
	"context"
	"os"
	"path/filepath"

	"github.com/jameswlane/devex/apps/cli/internal/security"
	"github.com/jameswlane/devex/apps/cli/internal/state"
	"github.com/jameswlane/devex/apps/cli/internal/templateexec"
	"github.com/jameswlane/devex/apps/cli/internal/templates"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type recordingStepExecutor struct{ ran *[]string }

func (e recordingStepExecutor) ExecuteStep(_ context.Context, step templates.PlannedStep) (map[string]any, error) {
	*e.ran = append(*e.ran, step.ID)
	return nil, nil
}

var _ = Describe("Template plans under a required plan approval", func() {
	It("refuses install and maintain plans before any step and still allows removal", func() {
		_, public, err := security.GenerateApprovalKey()
		Expect(err).NotTo(HaveOccurred())
		teamDir := os.Getenv("DEVEX_TEAM_CONFIG_DIR")
		Expect(os.MkdirAll(teamDir, 0o750)).To(Succeed())
		policy := "version: '1'\napproval:\n  required: true\n  approvers:\n    - id: it\n      public_key: " + public + "\n"
		Expect(os.WriteFile(filepath.Join(teamDir, "policy.yaml"), []byte(policy), 0o600)).To(Succeed())

		store, err := state.OpenBoltStore(filepath.Join(GinkgoT().TempDir(), "approval.db"), 0o600)
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(store.Close)
		var ran []string
		runtime := templateexec.NewExecutionRuntime(store, nil)
		runtime.Executors[templates.StepTypePlugin] = recordingStepExecutor{&ran}
		plan := func(action templates.LifecycleAction) templates.ExecutionPlan {
			return templates.ExecutionPlan{ID: string(action), TemplateID: "approval", Action: action, Steps: []templates.PlannedStep{
				{ID: string(action) + "-step", Type: templates.StepTypePlugin, Idempotent: true, Request: templates.ExecutionRequest{Plugin: "package-manager-mise"}},
			}}
		}

		for _, action := range []templates.LifecycleAction{templates.LifecycleInstall, templates.LifecycleMaintain} {
			_, err := runtime.Execute(context.Background(), plan(action), templateexec.ExecuteOptions{})
			Expect(err).To(MatchError(ContainSubstring("requires an approved plan")))
		}
		Expect(ran).To(BeEmpty())

		_, err = runtime.Execute(context.Background(), plan(templates.LifecycleRemove), templateexec.ExecuteOptions{})
		Expect(err).NotTo(HaveOccurred())
		Expect(ran).To(Equal([]string{"remove-step"}))
	})
})
