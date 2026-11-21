package templateexec_test

import (
	"context"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/jameswlane/devex/apps/cli/internal/state"
	"github.com/jameswlane/devex/apps/cli/internal/templateexec"
	"github.com/jameswlane/devex/apps/cli/internal/templates"
)

// The shipped starters run end to end against a recording plugin executor and
// a real bbolt state store: install records resources, and maintain, status
// and remove act on those recorded resources without any answers.
var _ = Describe("Shipped starter lifecycles with a fake plugin executor", func() {
	var (
		store    *state.BoltStore
		plugins  *recordingPluginExecutor
		runtime  *templateexec.ExecutionRuntime
		planWith func(id string, action templates.LifecycleAction, goos string, answers []string, known map[string]templates.ResourceDeclaration) templates.ExecutionPlan
	)

	BeforeEach(func() {
		var err error
		store, err = state.OpenBoltStore(filepath.Join(GinkgoT().TempDir(), "state.db"), 0o600)
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(store.Close)
		plugins = &recordingPluginExecutor{}
		runtime = templateexec.NewExecutionRuntime(store, plugins)
		planWith = func(id string, action templates.LifecycleAction, goos string, answers []string, known map[string]templates.ResourceDeclaration) templates.ExecutionPlan {
			planner := templates.TemplatePlanner{
				Manager:         templates.NewTemplateManager(GinkgoT().TempDir()),
				PluginAvailable: func(context.Context, string) bool { return true },
				System:          map[string]string{"os": goos},
			}
			plan, err := planner.Plan(context.Background(), id, templates.PlanOptions{
				Action:          action,
				InputOptions:    templates.InputResolverOptions{Answers: answers, NonInteractive: true},
				KnownResources:  known,
				ValidatePlugins: true,
			})
			Expect(err).NotTo(HaveOccurred())
			return *plan
		}
	})

	execute := func(plan templates.ExecutionPlan) *templateexec.ExecutionResult {
		plugins.calls = nil
		result, err := runtime.Execute(context.Background(), plan, templateexec.ExecuteOptions{Concurrency: 4})
		Expect(err).NotTo(HaveOccurred())
		Expect(result.StepsExecuted).To(Equal(len(plan.Steps)))
		return result
	}

	callsOf := func() [][]string {
		calls := make([][]string, 0, len(plugins.calls))
		for _, call := range plugins.calls {
			calls = append(calls, append([]string{call.name}, call.args...))
		}
		return calls
	}

	recorded := func(runID string) map[string]templates.ResourceDeclaration {
		resources, err := store.ListResourcesByRun(context.Background(), runID)
		Expect(err).NotTo(HaveOccurred())
		known := map[string]templates.ResourceDeclaration{}
		for _, resource := range resources {
			known[resource.StepID] = templates.ResourceDeclaration{
				Type: resource.Type, Name: resource.Name, ExternalID: resource.ExternalID, Metadata: resource.Metadata,
			}
		}
		return known
	}

	It("runs the data science containers and maintains the recorded image", func() {
		install := execute(planWith("data-science", templates.LifecycleInstall, "linux", []string{"components=jupyter,mlflow", "notebook_stack=datascience", "jupyter_port=9999"}, nil))
		Expect(callsOf()).To(ContainElements(
			[]string{"package-manager-docker", "ensure-installed", "--add-user"},
			[]string{"package-manager-docker", "install", "quay.io/jupyter/datascience-notebook:lab-4.6.6", "--name=devex-jupyter", "--port=127.0.0.1:9999:8888", "--volume=devex-jupyter-work:/home/jovyan/work", "--restart=unless-stopped", "--detach"},
		))
		Expect(callsOf()[0]).To(Equal([]string{"package-manager-docker", "ensure-installed", "--add-user"}), "Docker Engine must be ready before any container starts")
		known := recorded(install.RunID)
		Expect(known).To(HaveKey("jupyter"))
		Expect(known).To(HaveKey("mlflow"))

		execute(planWith("data-science", templates.LifecycleMaintain, "linux", nil, known))
		Expect(callsOf()).To(ContainElements(
			[]string{"package-manager-docker", "pull", "quay.io/jupyter/datascience-notebook:lab-4.6.6"},
			[]string{"package-manager-docker", "restart", "devex-jupyter"},
			[]string{"package-manager-docker", "pull", "ghcr.io/mlflow/mlflow:v3.18.0"},
		))

		execute(planWith("data-science", templates.LifecycleStatus, "linux", nil, known))
		Expect(callsOf()).To(ConsistOf(
			[]string{"package-manager-docker", "status", "devex-jupyter"},
			[]string{"package-manager-docker", "status", "devex-mlflow"},
		))

		execute(planWith("data-science", templates.LifecycleRemove, "linux", nil, known))
		Expect(callsOf()).To(ConsistOf(
			[]string{"package-manager-docker", "remove", "devex-mlflow", "--force"},
			[]string{"package-manager-docker", "remove", "devex-jupyter", "--force"},
		))
	})

	It("installs and removes the pinned DevOps tools through mise", func() {
		install := execute(planWith("devops", templates.LifecycleInstall, "linux", []string{"tools=kubectl,helm", "helm_version=3.19"}, nil))
		Expect(callsOf()).To(HaveLen(3))
		Expect(callsOf()[0]).To(Equal([]string{"package-manager-mise", "ensure-installed"}), "mise must be installed before any tool")
		Expect(callsOf()[1:]).To(ConsistOf(
			[]string{"package-manager-mise", "install", "kubectl@1.37"},
			[]string{"package-manager-mise", "install", "helm@3.19"},
		))
		known := recorded(install.RunID)

		execute(planWith("devops", templates.LifecycleMaintain, "linux", nil, known))
		Expect(callsOf()).To(ConsistOf(
			[]string{"package-manager-mise", "update", "kubectl@1.37"},
			[]string{"package-manager-mise", "update", "helm@3.19"},
		))

		execute(planWith("devops", templates.LifecycleStatus, "linux", nil, known))
		Expect(callsOf()).To(ConsistOf(
			[]string{"package-manager-mise", "is-installed", "kubectl@1.37"},
			[]string{"package-manager-mise", "is-installed", "helm@3.19"},
		))

		execute(planWith("devops", templates.LifecycleRemove, "linux", nil, known))
		Expect(callsOf()).To(ConsistOf(
			[]string{"package-manager-mise", "remove", "kubectl@1.37"},
			[]string{"package-manager-mise", "remove", "helm@3.19"},
		))
	})

	It("prepares iOS on macOS and removes it again", func() {
		install := execute(planWith("mobile", templates.LifecycleInstall, "darwin", []string{"targets=ios"}, nil))
		Expect(callsOf()).To(Equal([][]string{{"package-manager-brew", "install", "--formula", "cocoapods"}}))

		execute(planWith("mobile", templates.LifecycleRemove, "darwin", nil, recorded(install.RunID)))
		Expect(callsOf()).To(Equal([][]string{{"package-manager-brew", "remove", "--formula", "cocoapods"}}))
	})

	It("installs Godot from Flathub on Linux and removes it again", func() {
		install := execute(planWith("game-development", templates.LifecycleInstall, "linux", []string{"engine=godot"}, nil))
		Expect(callsOf()).To(Equal([][]string{
			{"package-manager-flatpak", "ensure-installed"},
			{"package-manager-flatpak", "add-flathub"},
			{"package-manager-flatpak", "install", "org.godotengine.Godot"},
		}))

		execute(planWith("game-development", templates.LifecycleRemove, "linux", nil, recorded(install.RunID)))
		Expect(callsOf()).To(Equal([][]string{{"package-manager-flatpak", "remove", "org.godotengine.Godot"}}))
	})
})
