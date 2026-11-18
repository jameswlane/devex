package templates_test

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/jameswlane/devex/apps/cli/assets"
	"github.com/jameswlane/devex/apps/cli/internal/templates"
)

// starterAnswers are the answers that select every component of each
// lifecycle starter, so a plan covers all of its steps on the planned OS.
var starterAnswers = map[string][]string{
	"databases":        {"databases=postgres,redis,mysql", "mysql_password=example"},
	"devops":           {"tools=docker,kubectl,helm,kind,terraform,opentofu,awscli"},
	"mobile":           {"targets=react-native,flutter,android,ios"},
	"data-science":     {"components=jupyter,mlflow"},
	"game-development": {"engine=godot"},
	"web-development":  {},
}

func planStarter(id string, action templates.LifecycleAction, goos string, answers []string, known map[string]templates.ResourceDeclaration, available func(context.Context, string) bool) (*templates.ExecutionPlan, error) {
	planner := templates.TemplatePlanner{
		Manager:         templates.NewTemplateManager(GinkgoT().TempDir()),
		AppResolver:     testAppResolver{},
		PluginAvailable: available,
		System:          map[string]string{"os": goos},
	}
	return planner.Plan(context.Background(), id, templates.PlanOptions{
		Action:          action,
		InputOptions:    templates.InputResolverOptions{Answers: answers, NonInteractive: true},
		KnownResources:  known,
		ValidatePlugins: true,
	})
}

func writeSpecFile(dir string, content string) string {
	path := filepath.Join(dir, "template.yaml")
	Expect(os.WriteFile(path, []byte(content), 0o600)).To(Succeed())
	return path
}

func allPluginsAvailable(context.Context, string) bool { return true }

func stepIDs(plan *templates.ExecutionPlan) []string {
	ids := make([]string, 0, len(plan.Steps))
	for _, step := range plan.Steps {
		ids = append(ids, step.ID)
	}
	return ids
}

func plannedStep(plan *templates.ExecutionPlan, id string) templates.PlannedStep {
	for _, step := range plan.Steps {
		if step.ID == id {
			return step
		}
	}
	Fail("plan has no step " + id)
	return templates.PlannedStep{}
}

// installedResources mimics the state store after an install: every resource a
// planned install step declares becomes a known resource for later actions.
func installedResources(plan *templates.ExecutionPlan) map[string]templates.ResourceDeclaration {
	known := map[string]templates.ResourceDeclaration{}
	for _, step := range plan.Steps {
		if len(step.Resources) > 0 {
			known[step.ID] = step.Resources[0]
		}
	}
	return known
}

var _ = Describe("Shipped lifecycle templates", func() {
	It("loads every embedded template file, so none is skipped as invalid", func() {
		entries, err := fs.ReadDir(assets.Templates, ".")
		Expect(err).NotTo(HaveOccurred())
		manager := templates.NewTemplateManager(GinkgoT().TempDir())
		specs, err := manager.GetAvailableTemplateSpecsWithSources()
		Expect(err).NotTo(HaveOccurred())
		loaded := map[string]bool{}
		for _, spec := range specs {
			Expect(spec.Source).To(Equal("builtin"))
			Expect(manager.ValidateTemplateSpec(spec.Spec)).To(Succeed(), spec.Spec.ID)
			loaded[spec.Spec.ID] = true
		}
		for _, entry := range entries {
			id := strings.TrimSuffix(strings.TrimSuffix(entry.Name(), ".yaml"), ".yml")
			if !entry.IsDir() {
				spec, err := manager.LoadTemplateSpecFile("../../assets/templates/" + entry.Name())
				Expect(err).NotTo(HaveOccurred(), entry.Name())
				id = spec.ID
			}
			Expect(loaded).To(HaveKey(id), "embedded template %s did not load", entry.Name())
		}
	})

	DescribeTable("plans every lifecycle action on each supported OS",
		func(id string, goos string) {
			install, err := planStarter(id, templates.LifecycleInstall, goos, starterAnswers[id], nil, allPluginsAvailable)
			Expect(err).NotTo(HaveOccurred())
			Expect(install.Steps).NotTo(BeEmpty())
			known := installedResources(install)
			Expect(known).NotTo(BeEmpty(), "install must record resources for later actions")

			for _, action := range []templates.LifecycleAction{templates.LifecycleMaintain, templates.LifecycleStatus, templates.LifecycleRemove} {
				manager := templates.NewTemplateManager(GinkgoT().TempDir())
				spec, err := manager.GetTemplateSpec(id)
				Expect(err).NotTo(HaveOccurred())
				if _, defined := spec.Lifecycle[action]; !defined {
					continue
				}
				plan, err := planStarter(id, action, goos, starterAnswers[id], known, allPluginsAvailable)
				Expect(err).NotTo(HaveOccurred(), string(action))
				for _, step := range plan.Steps {
					for _, arg := range step.Request.Args {
						Expect(arg).NotTo(ContainSubstring("<nil>"), "%s step %s has an unresolved value", action, step.ID)
						Expect(arg).NotTo(ContainSubstring("{{"), "%s step %s has an uninterpolated value", action, step.ID)
					}
				}
				if action == templates.LifecycleRemove {
					Expect(plan.Steps).NotTo(BeEmpty(), "remove must undo the install on %s", goos)
				}
			}
		},
		Entry("devops on Linux", "devops", "linux"),
		Entry("devops on macOS", "devops", "darwin"),
		Entry("mobile on Linux", "mobile", "linux"),
		Entry("mobile on macOS", "mobile", "darwin"),
		Entry("data-science on Linux", "data-science", "linux"),
		Entry("data-science on macOS", "data-science", "darwin"),
		Entry("game-development on Linux", "game-development", "linux"),
		Entry("game-development on macOS", "game-development", "darwin"),
		Entry("databases on Linux", "databases", "linux"),
		Entry("web-development on Linux", "web-development", "linux"),
		Entry("web-development on macOS", "web-development", "darwin"),
	)

	Describe("OS guards", func() {
		It("prepares Android with Flatpak on Linux and never plans iOS there", func() {
			plan, err := planStarter("mobile", templates.LifecycleInstall, "linux", starterAnswers["mobile"], nil, allPluginsAvailable)
			Expect(err).NotTo(HaveOccurred())
			Expect(stepIDs(plan)).To(Equal([]string{"mise", "node", "java", "flutter", "flatpak", "flathub", "android-studio-linux"}))
			Expect(plannedStep(plan, "android-studio-linux").Request).To(HaveField("Plugin", "package-manager-flatpak"))
			Expect(plannedStep(plan, "java").Request.Args).To(Equal([]string{"java@temurin-17"}))
		})

		It("uses Homebrew for Android Studio and CocoaPods on macOS", func() {
			plan, err := planStarter("mobile", templates.LifecycleInstall, "darwin", starterAnswers["mobile"], nil, allPluginsAvailable)
			Expect(err).NotTo(HaveOccurred())
			Expect(stepIDs(plan)).To(Equal([]string{"mise", "node", "java", "flutter", "android-studio-macos", "cocoapods"}))
			Expect(plannedStep(plan, "cocoapods").Request.Args).To(Equal([]string{"--formula", "cocoapods"}))
			Expect(plannedStep(plan, "android-studio-macos").Request.Args).To(Equal([]string{"--cask", "android-studio"}))
		})

		It("installs Docker Engine only on Linux and still orders the containers after it", func() {
			linux, err := planStarter("data-science", templates.LifecycleInstall, "linux", starterAnswers["data-science"], nil, allPluginsAvailable)
			Expect(err).NotTo(HaveOccurred())
			Expect(stepIDs(linux)).To(Equal([]string{"docker-engine", "jupyter", "mlflow"}))
			Expect(plannedStep(linux, "jupyter").DependsOn).To(Equal([]string{"docker-engine"}))

			macos, err := planStarter("data-science", templates.LifecycleInstall, "darwin", starterAnswers["data-science"], nil, allPluginsAvailable)
			Expect(err).NotTo(HaveOccurred())
			Expect(stepIDs(macos)).To(Equal([]string{"jupyter", "mlflow"}))
			Expect(plannedStep(macos, "jupyter").DependsOn).To(BeEmpty())

			devops, err := planStarter("devops", templates.LifecycleInstall, "darwin", starterAnswers["devops"], nil, allPluginsAvailable)
			Expect(err).NotTo(HaveOccurred())
			Expect(stepIDs(devops)).NotTo(ContainElement("docker-engine"))
		})

		It("installs the Godot edition the user picked from the platform's package manager", func() {
			linux, err := planStarter("game-development", templates.LifecycleInstall, "linux", []string{"engine=godot", "godot_edition=dotnet"}, nil, allPluginsAvailable)
			Expect(err).NotTo(HaveOccurred())
			Expect(stepIDs(linux)).To(Equal([]string{"flatpak", "flathub", "godot-dotnet-linux"}))
			Expect(plannedStep(linux, "godot-dotnet-linux").Request.Args).To(Equal([]string{"org.godotengine.GodotSharp"}))

			macos, err := planStarter("game-development", templates.LifecycleInstall, "darwin", []string{"engine=godot"}, nil, allPluginsAvailable)
			Expect(err).NotTo(HaveOccurred())
			Expect(stepIDs(macos)).To(Equal([]string{"godot-macos"}))
			Expect(plannedStep(macos, "godot-macos").Request.Args).To(Equal([]string{"--cask", "godot"}))
		})
	})

	Describe("component and version selection", func() {
		It("installs only the DevOps tools the user picked, at the pinned versions", func() {
			plan, err := planStarter("devops", templates.LifecycleInstall, "linux", []string{"tools=kubectl,opentofu", "kubectl_version=1.36"}, nil, allPluginsAvailable)
			Expect(err).NotTo(HaveOccurred())
			Expect(stepIDs(plan)).To(Equal([]string{"mise", "kubectl", "opentofu"}))
			Expect(plannedStep(plan, "kubectl").Request.Args).To(Equal([]string{"kubectl@1.36"}))
			Expect(plannedStep(plan, "opentofu").Request.Args).To(Equal([]string{"opentofu@1.13"}))
		})

		It("rejects a version that is not a version", func() {
			_, err := planStarter("devops", templates.LifecycleInstall, "linux", []string{"tools=kubectl", "kubectl_version=latest;rm"}, nil, allPluginsAvailable)
			Expect(err).To(MatchError(ContainSubstring("kubectl_version")))
		})

		It("picks the notebook image for the chosen languages and keeps it for maintain", func() {
			install, err := planStarter("data-science", templates.LifecycleInstall, "linux", []string{"components=jupyter", "notebook_stack=r"}, nil, allPluginsAvailable)
			Expect(err).NotTo(HaveOccurred())
			Expect(plannedStep(install, "jupyter").Request.Args[0]).To(Equal("quay.io/jupyter/r-notebook:lab-4.6.6"))

			// Maintain runs without answers and must still pull the R image.
			maintain, err := planStarter("data-science", templates.LifecycleMaintain, "linux", nil, installedResources(install), allPluginsAvailable)
			Expect(err).NotTo(HaveOccurred())
			Expect(stepIDs(maintain)).To(Equal([]string{"jupyter-pull", "jupyter-restart"}))
			Expect(plannedStep(maintain, "jupyter-pull").Request.Args).To(Equal([]string{"quay.io/jupyter/r-notebook:lab-4.6.6"}))
		})

		It("passes the MLflow server command through after --", func() {
			plan, err := planStarter("data-science", templates.LifecycleInstall, "linux", []string{"components=mlflow"}, nil, allPluginsAvailable)
			Expect(err).NotTo(HaveOccurred())
			args := plannedStep(plan, "mlflow").Request.Args
			Expect(args).To(ContainElement("--port=127.0.0.1:5000:5000"))
			Expect(args).To(ContainElements("--", "mlflow", "server", "--port", "5000"))
		})

		It("installs nothing for game development until the user opts in to an engine", func() {
			_, err := planStarter("game-development", templates.LifecycleInstall, "linux", nil, nil, allPluginsAvailable)
			Expect(err).To(MatchError(ContainSubstring("required input 'engine'")))

			_, err = planStarter("game-development", templates.LifecycleInstall, "linux", []string{"engine=unity"}, nil, allPluginsAvailable)
			Expect(err).To(HaveOccurred())
		})

		It("installs mise before Node.js for web development", func() {
			plan, err := planStarter("web-development", templates.LifecycleInstall, "linux", nil, nil, allPluginsAvailable)
			Expect(err).NotTo(HaveOccurred())
			Expect(stepIDs(plan)).To(Equal([]string{"mise", "node"}))
			Expect(plannedStep(plan, "mise").Request).To(HaveField("Command", "ensure-installed"))
			Expect(plannedStep(plan, "node").DependsOn).To(Equal([]string{"mise"}))
			Expect(plannedStep(plan, "node").Request.Args).To(Equal([]string{"node@24"}))

			plan, err = planStarter("web-development", templates.LifecycleInstall, "linux", []string{"node_version=22"}, nil, allPluginsAvailable)
			Expect(err).NotTo(HaveOccurred())
			Expect(plannedStep(plan, "node").Request.Args).To(Equal([]string{"node@22"}))

			// The mise plugin rejects aliases such as lts, so the plan must too.
			_, err = planStarter("web-development", templates.LifecycleInstall, "linux", []string{"node_version=lts"}, nil, allPluginsAvailable)
			Expect(err).To(MatchError(ContainSubstring("node_version")))
		})

		It("requires a mobile target instead of installing everything", func() {
			_, err := planStarter("mobile", templates.LifecycleInstall, "linux", nil, nil, allPluginsAvailable)
			Expect(err).To(MatchError(ContainSubstring("required input 'targets'")))
		})
	})

	DescribeTable("fails the plan when a plugin it needs is unavailable",
		func(id string, goos string, answers []string, missing string) {
			available := func(_ context.Context, name string) bool { return name != missing }
			_, err := planStarter(id, templates.LifecycleInstall, goos, answers, nil, available)
			Expect(err).To(MatchError(ContainSubstring("plugin '" + missing + "' is not available")))
		},
		Entry("mise for DevOps tools", "devops", "linux", []string{"tools=kubectl"}, "package-manager-mise"),
		Entry("mise for web development", "web-development", "linux", nil, "package-manager-mise"),
		Entry("Docker for the notebook", "data-science", "darwin", []string{"components=jupyter"}, "package-manager-docker"),
		Entry("Flatpak for Android Studio", "mobile", "linux", []string{"targets=android"}, "package-manager-flatpak"),
		Entry("Homebrew for CocoaPods", "mobile", "darwin", []string{"targets=ios"}, "package-manager-brew"),
		Entry("Flatpak for Godot", "game-development", "linux", []string{"engine=godot"}, "package-manager-flatpak"),
	)
})

var _ = Describe("Planner dependencies on skipped steps", func() {
	It("treats a dependency skipped by its condition as satisfied", func() {
		dir := GinkgoT().TempDir()
		manager := templates.NewTemplateManager(dir)
		path := writeSpecFile(dir, `schema_version: devex.template/v1
id: guarded
name: Guarded
version: 1.0.0
lifecycle:
  install:
    - id: setup
      order: 10
      if: platform("linux")
      plugin:
        name: package-manager-docker
        command: ensure-installed
    - id: app
      order: 20
      depends_on:
        - setup
      plugin:
        name: package-manager-docker
        command: install
        args:
          - redis:7
`)
		planner := templates.TemplatePlanner{Manager: manager, System: map[string]string{"os": "darwin"}}
		plan, err := planner.Plan(context.Background(), path, templates.PlanOptions{Action: templates.LifecycleInstall})
		Expect(err).NotTo(HaveOccurred())
		Expect(stepIDs(plan)).To(Equal([]string{"app"}))
		Expect(plan.Steps[0].DependsOn).To(BeEmpty())

		planner.System = map[string]string{"os": "linux"}
		plan, err = planner.Plan(context.Background(), path, templates.PlanOptions{Action: templates.LifecycleInstall})
		Expect(err).NotTo(HaveOccurred())
		Expect(stepIDs(plan)).To(Equal([]string{"setup", "app"}))
		Expect(plan.Steps[1].DependsOn).To(Equal([]string{"setup"}))
	})
})

var _ = Describe("Required inputs on actions after install", func() {
	It("plans maintain, status and remove from recorded resources without install answers", func() {
		for _, id := range []string{"mobile", "databases", "game-development"} {
			install, err := planStarter(id, templates.LifecycleInstall, "linux", starterAnswers[id], nil, allPluginsAvailable)
			Expect(err).NotTo(HaveOccurred())
			for _, action := range []templates.LifecycleAction{templates.LifecycleMaintain, templates.LifecycleStatus, templates.LifecycleRemove} {
				_, err := planStarter(id, action, "linux", nil, installedResources(install), allPluginsAvailable)
				Expect(err).NotTo(HaveOccurred(), "%s %s", id, action)
			}
		}
	})

	It("still requires an input that the action's own steps use", func() {
		dir := GinkgoT().TempDir()
		path := writeSpecFile(dir, `schema_version: devex.template/v1
id: needs-token
name: Needs Token
version: 1.0.0
inputs:
  token:
    type: text
    required: true
    sensitive: true
lifecycle:
  install:
    - id: app
      plugin:
        name: package-manager-docker
        command: install
        args:
          - redis:7
  maintain:
    - id: rotate
      plugin:
        name: package-manager-docker
        command: restart
        args:
          - --env=TOKEN={{ inputs.token }}
`)
		planner := templates.TemplatePlanner{Manager: templates.NewTemplateManager(dir)}
		_, err := planner.Plan(context.Background(), path, templates.PlanOptions{
			Action:       templates.LifecycleMaintain,
			InputOptions: templates.InputResolverOptions{NonInteractive: true},
		})
		Expect(err).To(MatchError(ContainSubstring("required input 'token'")))
	})
})

var _ = Describe("Required inputs of install steps that won't run", func() {
	It("installs PostgreSQL and Redis without a MySQL password", func() {
		plan, err := planStarter("databases", templates.LifecycleInstall, "linux", []string{"databases=postgres,redis"}, nil, allPluginsAvailable)

		Expect(err).NotTo(HaveOccurred())
		Expect(stepIDs(plan)).To(Equal([]string{"docker-engine", "postgres", "redis"}))
		Expect(plan.RawInputs).NotTo(HaveKey("mysql_password"))
	})

	It("still requires the MySQL password when MySQL is selected", func() {
		_, err := planStarter("databases", templates.LifecycleInstall, "linux", []string{"databases=postgres,mysql"}, nil, allPluginsAvailable)

		Expect(err).To(MatchError(ContainSubstring("required input 'mysql_password' is missing")))
	})

	It("doesn't prompt for the MySQL password when MySQL isn't selected", func() {
		planner := templates.TemplatePlanner{
			Manager:     templates.NewTemplateManager(GinkgoT().TempDir()),
			AppResolver: testAppResolver{},
			System:      map[string]string{"os": "linux"},
		}
		var prompted []string
		_, err := planner.Plan(context.Background(), "databases", templates.PlanOptions{
			Action: templates.LifecycleInstall,
			InputOptions: templates.InputResolverOptions{
				Answers: []string{"databases=postgres"},
				Prompt: func(_ context.Context, key string, _ templates.TemplateInput) (any, error) {
					prompted = append(prompted, key)
					return "secret", nil
				},
			},
		})

		Expect(err).NotTo(HaveOccurred())
		Expect(prompted).To(BeEmpty())
	})

	It("keeps an input required when the condition that would skip its step reads an unanswered required input", func() {
		dir := GinkgoT().TempDir()
		path := writeSpecFile(dir, `schema_version: devex.template/v1
id: gated
name: Gated
version: 1.0.0
inputs:
  mode:
    type: text
    required: true
  token:
    type: text
    required: true
lifecycle:
  install:
    - id: app
      if: equals(inputs.mode, "remote")
      plugin:
        name: package-manager-docker
        command: install
        args:
          - --env=TOKEN={{ inputs.token }}
          - redis:7
`)
		planner := templates.TemplatePlanner{Manager: templates.NewTemplateManager(dir)}
		var prompted []string
		_, err := planner.Plan(context.Background(), path, templates.PlanOptions{
			Action: templates.LifecycleInstall,
			InputOptions: templates.InputResolverOptions{
				Prompt: func(_ context.Context, key string, _ templates.TemplateInput) (any, error) {
					prompted = append(prompted, key)
					return "remote", nil
				},
			},
		})

		Expect(err).NotTo(HaveOccurred())
		Expect(prompted).To(ConsistOf("mode", "token"))
	})
})
