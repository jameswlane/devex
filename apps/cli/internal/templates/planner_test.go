package templates_test

import (
	"context"
	"os"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v3"

	"github.com/jameswlane/devex/apps/cli/internal/templates"
	"github.com/jameswlane/devex/apps/cli/internal/types"
)

type testAppResolver struct {
	apps map[string]*types.AppConfig
}

func (r testAppResolver) ResolveTemplateApp(ref string) (*types.AppConfig, error) {
	return r.apps[ref], nil
}

var _ = Describe("Template Runtime Planner", func() {
	var (
		tempDir string
		manager *templates.TemplateManager
		spec    templates.TemplateSpec
	)

	BeforeEach(func() {
		var err error
		tempDir, err = os.MkdirTemp("", "devex-template-runtime")
		Expect(err).ToNot(HaveOccurred())
		manager = templates.NewTemplateManager(tempDir)

		spec = templates.TemplateSpec{
			SchemaVersion: templates.TemplateSchemaVersion,
			ID:            "databases",
			Name:          "Databases",
			Version:       "1.0.0",
			Inputs: map[string]templates.TemplateInput{
				"databases": {
					Type:     "multiselect",
					Required: true,
					Options: []templates.TemplateInputOption{
						{Label: "PostgreSQL", Value: "postgres"},
						{Label: "Redis", Value: "redis"},
					},
				},
				"postgres_name": {
					Type:    "text",
					Default: "devex-postgres",
					Validation: &templates.TemplateInputValidation{
						Pattern: "^[a-zA-Z0-9_.-]+$",
					},
				},
				"postgres_password": {
					Type:      "text",
					Sensitive: true,
					Default:   "secret",
				},
			},
			Lifecycle: map[templates.LifecycleAction][]templates.TemplateStepSpec{
				templates.LifecycleInstall: {
					{
						ID:    "docker",
						Order: 10,
						If:    `contains(inputs.databases, "postgres")`,
						Plugin: &templates.TemplatePluginAction{
							Name:    "package-manager-docker",
							Command: "ensure-installed",
						},
					},
					{
						ID:        "postgres",
						Order:     20,
						DependsOn: []string{"docker"},
						If:        `contains(inputs.databases, "postgres")`,
						App: &templates.TemplateAppAction{
							Name:      "PostgreSQL",
							Category:  "databases",
							Installer: "docker",
							Command:   "install",
							Args: []string{
								"postgres:16",
								"--name={{ inputs.postgres_name }}",
							},
						},
						Resources: []templates.TemplateResource{
							{Type: "container", Name: "{{ inputs.postgres_name }}", ExternalID: "{{ inputs.postgres_name }}"},
						},
					},
				},
				templates.LifecycleMaintain: {
					{
						ID:    "postgres-restart",
						Order: 10,
						If:    `has_resource("postgres")`,
						Plugin: &templates.TemplatePluginAction{
							Name:    "package-manager-docker",
							Command: "restart",
							Args:    []string{"{{ resource.postgres.external_id }}"},
						},
					},
				},
			},
		}
	})

	AfterEach(func() {
		if tempDir != "" {
			_ = os.RemoveAll(tempDir)
		}
	})

	It("resolves inputs using CLI, file, env, and defaults precedence", func() {
		answersPath := filepath.Join(tempDir, "answers.yaml")
		data, err := yaml.Marshal(map[string]any{
			"databases":     []string{"redis"},
			"postgres_name": "from-file",
		})
		Expect(err).ToNot(HaveOccurred())
		Expect(os.WriteFile(answersPath, data, 0600)).To(Succeed())
		Expect(os.Setenv("DEVEX_INPUT_POSTGRES_NAME", "from-env")).To(Succeed())
		defer os.Unsetenv("DEVEX_INPUT_POSTGRES_NAME")

		inputs, err := templates.ResolveInputs(context.Background(), spec, templates.InputResolverOptions{
			Answers:        []string{"databases=postgres"},
			AnswerFiles:    []string{answersPath},
			NonInteractive: true,
		})

		Expect(err).ToNot(HaveOccurred())
		Expect(inputs["databases"]).To(Equal([]string{"postgres"}))
		Expect(inputs["postgres_name"]).To(Equal("from-file"))
		Expect(inputs["postgres_password"]).To(Equal("secret"))
	})

	It("fails missing required inputs in non-interactive mode", func() {
		_, err := templates.ResolveInputs(context.Background(), spec, templates.InputResolverOptions{
			NonInteractive: true,
		})
		Expect(err).To(MatchError(ContainSubstring("required input 'databases' is missing")))
	})

	It("evaluates conditions and interpolates planned requests", func() {
		planner := templates.TemplatePlanner{
			Manager: manager,
			Now:     func() time.Time { return time.Unix(10, 0) },
		}
		plan, err := planner.Plan(context.Background(), writeSpec(tempDir, spec), templates.PlanOptions{
			Action: templates.LifecycleInstall,
			InputOptions: templates.InputResolverOptions{
				Answers:        []string{"databases=postgres", "postgres_name=db-local"},
				NonInteractive: true,
			},
		})

		Expect(err).ToNot(HaveOccurred())
		Expect(plan.TemplateID).To(Equal("databases"))
		Expect(plan.Inputs["postgres_password"]).To(Equal("[REDACTED]"))
		Expect(plan.Steps).To(HaveLen(2))
		Expect(plan.Steps[0].ID).To(Equal("docker"))
		Expect(plan.Steps[1].Request.Plugin).To(Equal("package-manager-docker"))
		Expect(plan.Steps[1].Request.Args).To(Equal([]string{"postgres:16", "--name=db-local"}))
		Expect(plan.Steps[1].Resources[0].ExternalID).To(Equal("db-local"))
	})

	It("keeps executable sensitive derivatives raw but builds redacted snapshots", func() {
		spec.Lifecycle[templates.LifecycleInstall] = []templates.TemplateStepSpec{
			{
				ID:    "secret-step",
				Order: 10,
				Plugin: &templates.TemplatePluginAction{
					Name:    "package-manager-docker",
					Command: "run",
					Args: []string{
						"--password={{ inputs.postgres_password }}",
					},
					Env: map[string]string{
						"POSTGRES_PASSWORD": "{{ inputs.postgres_password }}",
					},
					Metadata: map[string]any{
						"connection": "postgres://postgres:{{ inputs.postgres_password }}@localhost/postgres",
					},
				},
				Resources: []templates.TemplateResource{
					{
						Type:       "container",
						Name:       "postgres",
						ExternalID: "postgres",
						Metadata: map[string]any{
							"password": "{{ inputs.postgres_password }}",
						},
					},
				},
			},
			{
				ID:    "secret-file",
				Order: 20,
				File: &templates.TemplateFileAction{
					Operation:   "write",
					Destination: "/tmp/postgres.env",
					Content:     "POSTGRES_PASSWORD={{ inputs.postgres_password }}",
				},
			},
		}
		planner := templates.TemplatePlanner{Manager: manager}

		plan, err := planner.Plan(context.Background(), writeSpec(tempDir, spec), templates.PlanOptions{
			Action: templates.LifecycleInstall,
			InputOptions: templates.InputResolverOptions{
				Answers:        []string{"databases=postgres", "postgres_password=super-secret"},
				NonInteractive: true,
			},
		})

		Expect(err).ToNot(HaveOccurred())
		Expect(plan.Steps[0].Request.Args).To(ContainElement("--password=super-secret"))
		Expect(plan.Steps[0].Request.Env["POSTGRES_PASSWORD"]).To(Equal("super-secret"))
		Expect(plan.Steps[0].Request.Metadata["connection"]).To(Equal("postgres://postgres:super-secret@localhost/postgres"))
		Expect(plan.Steps[1].Request.Args).To(ContainElement("--content=POSTGRES_PASSWORD=super-secret"))

		redacted := templates.RedactPlan(spec, *plan)
		Expect(redacted.Inputs["postgres_password"]).To(Equal("[REDACTED]"))
		Expect(redacted.Steps[0].Request.Args).To(ContainElement("--password=[REDACTED]"))
		Expect(redacted.Steps[0].Request.Env["POSTGRES_PASSWORD"]).To(Equal("[REDACTED]"))
		Expect(redacted.Steps[0].Request.Metadata["connection"]).To(Equal("postgres://postgres:[REDACTED]@localhost/postgres"))
		Expect(redacted.Steps[0].Resources[0].Metadata["password"]).To(Equal("[REDACTED]"))
		Expect(redacted.Steps[1].Request.Args).To(ContainElement("--content=POSTGRES_PASSWORD=[REDACTED]"))
	})

	It("uses recorded resources for has_resource conditions", func() {
		planner := templates.TemplatePlanner{Manager: manager}
		plan, err := planner.Plan(context.Background(), writeSpec(tempDir, spec), templates.PlanOptions{
			Action: templates.LifecycleMaintain,
			InputOptions: templates.InputResolverOptions{
				Answers:        []string{"databases=postgres"},
				NonInteractive: true,
			},
			KnownResources: map[string]templates.ResourceDeclaration{
				"postgres": {Type: "container", Name: "db-local", ExternalID: "db-local"},
			},
		})

		Expect(err).ToNot(HaveOccurred())
		Expect(plan.Steps).To(HaveLen(1))
		Expect(plan.Steps[0].Request.Args).To(Equal([]string{"db-local"}))
	})

	It("interpolates and reuses resource metadata", func() {
		spec.Inputs["compose_file"] = templates.TemplateInput{
			Type:    "text",
			Default: "compose.prod.yaml",
		}
		spec.Lifecycle[templates.LifecycleInstall] = []templates.TemplateStepSpec{
			{
				ID:    "compose-stack",
				Order: 10,
				Plugin: &templates.TemplatePluginAction{
					Name:    "package-manager-docker",
					Command: "compose",
					Args:    []string{"up", "--file={{ inputs.compose_file }}"},
				},
				Resources: []templates.TemplateResource{
					{
						Type:       "compose_project",
						Name:       "devex-server",
						ExternalID: "devex-server",
						Metadata: map[string]any{
							"compose_file": "{{ inputs.compose_file }}",
						},
					},
				},
			},
		}
		spec.Lifecycle[templates.LifecycleMaintain] = []templates.TemplateStepSpec{
			{
				ID:    "compose-pull",
				Order: 10,
				If:    `has_resource("compose-stack")`,
				Plugin: &templates.TemplatePluginAction{
					Name:    "package-manager-docker",
					Command: "compose",
					Args: []string{
						"pull",
						"--file={{ resource.compose-stack.metadata.compose_file }}",
						"--project={{ resource.compose-stack.external_id }}",
					},
				},
			},
		}

		planner := templates.TemplatePlanner{Manager: manager}
		installPlan, err := planner.Plan(context.Background(), writeSpec(tempDir, spec), templates.PlanOptions{
			Action: templates.LifecycleInstall,
			InputOptions: templates.InputResolverOptions{
				Answers:        []string{"databases=postgres", "compose_file=compose.prod.yaml"},
				NonInteractive: true,
			},
		})
		Expect(err).ToNot(HaveOccurred())
		Expect(installPlan.Steps[0].Resources[0].Metadata["compose_file"]).To(Equal("compose.prod.yaml"))

		maintainPlan, err := planner.Plan(context.Background(), writeSpec(tempDir, spec), templates.PlanOptions{
			Action: templates.LifecycleMaintain,
			InputOptions: templates.InputResolverOptions{
				Answers:        []string{"databases=postgres"},
				NonInteractive: true,
			},
			KnownResources: map[string]templates.ResourceDeclaration{
				"compose-stack": {
					Type:       "compose_project",
					Name:       "devex-server",
					ExternalID: "devex-server",
					Metadata: map[string]any{
						"compose_file": "compose.prod.yaml",
					},
				},
			},
		})
		Expect(err).ToNot(HaveOccurred())
		Expect(maintainPlan.Steps[0].Request.Args).To(Equal([]string{
			"pull",
			"--file=compose.prod.yaml",
			"--project=devex-server",
		}))
	})

	It("resolves app refs through the planner resolver", func() {
		spec.Lifecycle[templates.LifecycleInstall] = []templates.TemplateStepSpec{
			{
				ID:    "git",
				Order: 10,
				App:   &templates.TemplateAppAction{Ref: "git"},
			},
		}
		planner := templates.TemplatePlanner{
			Manager: manager,
			AppResolver: testAppResolver{apps: map[string]*types.AppConfig{
				"git": {
					BaseConfig:     types.BaseConfig{Name: "Git", Category: "development"},
					InstallMethod:  "apt",
					InstallCommand: "git",
				},
			}},
		}

		plan, err := planner.Plan(context.Background(), writeSpec(tempDir, spec), templates.PlanOptions{
			Action: templates.LifecycleInstall,
			InputOptions: templates.InputResolverOptions{
				Answers:        []string{"databases=postgres"},
				NonInteractive: true,
			},
		})

		Expect(err).ToNot(HaveOccurred())
		Expect(plan.Steps[0].Request.Plugin).To(Equal("package-manager-apt"))
		Expect(plan.Steps[0].Request.Args).To(Equal([]string{"git"}))
	})

	It("rejects dangerous shell input after interpolation", func() {
		spec.Inputs["message"] = templates.TemplateInput{Type: "text", Required: true}
		spec.Lifecycle[templates.LifecycleInstall] = []templates.TemplateStepSpec{
			{
				ID:    "echo",
				Shell: &templates.TemplateShellAction{Command: "echo", Args: []string{"{{ inputs.message }}"}},
			},
		}
		planner := templates.TemplatePlanner{Manager: manager}

		_, err := planner.Plan(context.Background(), writeSpec(tempDir, spec), templates.PlanOptions{
			Action: templates.LifecycleInstall,
			InputOptions: templates.InputResolverOptions{
				Answers:        []string{"databases=postgres", "message=; rm -rf /"},
				NonInteractive: true,
			},
		})

		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("shell argument must not start with a shell operator"))
	})

	It("allows shell metacharacters inside typed argument data", func() {
		spec.Inputs["message"] = templates.TemplateInput{Type: "text", Required: true}
		spec.Lifecycle[templates.LifecycleInstall] = []templates.TemplateStepSpec{
			{
				ID:    "echo",
				Shell: &templates.TemplateShellAction{Command: "echo", Args: []string{"--message={{ inputs.message }}"}},
			},
		}
		planner := templates.TemplatePlanner{Manager: manager}

		plan, err := planner.Plan(context.Background(), writeSpec(tempDir, spec), templates.PlanOptions{
			Action: templates.LifecycleInstall,
			InputOptions: templates.InputResolverOptions{
				Answers:        []string{"databases=postgres", "message=greeting; farewell"},
				NonInteractive: true,
			},
		})

		Expect(err).ToNot(HaveOccurred())
		Expect(plan.Steps[0].Request.Args).To(Equal([]string{"--message=greeting; farewell"}))
	})

	It("rejects dangerous file and service inputs after interpolation", func() {
		spec.Inputs["path"] = templates.TemplateInput{Type: "text", Required: true}
		spec.Inputs["service"] = templates.TemplateInput{Type: "text", Required: true}
		planner := templates.TemplatePlanner{Manager: manager}

		spec.Lifecycle[templates.LifecycleInstall] = []templates.TemplateStepSpec{
			{
				ID: "file",
				File: &templates.TemplateFileAction{
					Operation:   "write",
					Destination: "{{ inputs.path }}",
					Content:     "hello",
				},
			},
		}
		_, err := planner.Plan(context.Background(), writeSpec(tempDir, spec), templates.PlanOptions{
			Action: templates.LifecycleInstall,
			InputOptions: templates.InputResolverOptions{
				Answers:        []string{"databases=postgres", "path=../profile", "service=docker"},
				NonInteractive: true,
			},
		})
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("path traversal"))

		spec.Lifecycle[templates.LifecycleInstall] = []templates.TemplateStepSpec{
			{
				ID:      "service",
				Service: &templates.TemplateServiceAction{Name: "{{ inputs.service }}", Action: "restart"},
			},
		}
		_, err = planner.Plan(context.Background(), writeSpec(tempDir, spec), templates.PlanOptions{
			Action: templates.LifecycleInstall,
			InputOptions: templates.InputResolverOptions{
				Answers:        []string{"databases=postgres", "path=/tmp/profile", "service=docker;rm"},
				NonInteractive: true,
			},
		})
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("service name"))
	})

	It("validates generated plugins for app, plugin, and shell steps", func() {
		spec.Lifecycle[templates.LifecycleInstall] = []templates.TemplateStepSpec{
			{
				ID: "app",
				App: &templates.TemplateAppAction{
					Name:      "PostgreSQL",
					Category:  "databases",
					Installer: "docker",
					Args:      []string{"postgres:16"},
				},
			},
			{
				ID:     "plugin",
				Plugin: &templates.TemplatePluginAction{Name: "package-manager-docker", Command: "status"},
			},
			{
				ID:    "shell",
				Shell: &templates.TemplateShellAction{Command: "echo", Args: []string{"hello"}},
			},
		}
		checked := map[string]bool{}
		planner := templates.TemplatePlanner{
			Manager: manager,
			PluginAvailable: func(_ context.Context, name string) bool {
				checked[name] = true
				return name != "tool-shell"
			},
		}

		_, err := planner.Plan(context.Background(), writeSpec(tempDir, spec), templates.PlanOptions{
			Action: templates.LifecycleInstall,
			InputOptions: templates.InputResolverOptions{
				Answers:        []string{"databases=postgres"},
				NonInteractive: true,
			},
			ValidatePlugins: true,
		})

		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("plugin 'tool-shell' is not available"))
		Expect(checked).To(HaveKey("package-manager-docker"))
		Expect(checked).To(HaveKey("tool-shell"))
	})

	It("plans built-in database lifecycle requests with non-interactive answers", func() {
		planner := templates.TemplatePlanner{Manager: manager}
		inputs := templates.InputResolverOptions{
			Answers: []string{
				"databases=postgres,redis,mysql",
				"postgres_name=devex-postgres",
				"redis_name=devex-redis",
				"mysql_name=devex-mysql",
				"mysql_password=devex-secret",
			},
			NonInteractive: true,
		}

		installPlan, err := planner.Plan(context.Background(), "databases", templates.PlanOptions{
			Action:       templates.LifecycleInstall,
			InputOptions: inputs,
		})
		Expect(err).ToNot(HaveOccurred())
		Expect(stepRequest(installPlan, "postgres").Command).To(Equal("install"))
		Expect(stepRequest(installPlan, "postgres").Args).To(Equal([]string{
			"postgres:16",
			"--name=devex-postgres",
			"--port=127.0.0.1:5432:5432",
			"--env=POSTGRES_HOST_AUTH_METHOD=trust",
			"--volume=devex-postgres-data:/var/lib/postgresql/data",
			"--restart=unless-stopped",
			"--detach",
		}))
		Expect(stepRequest(installPlan, "redis").Args).To(ContainElements("redis:7", "--name=devex-redis", "--detach"))
		Expect(stepRequest(installPlan, "mysql").Args).To(ContainElements("mysql:8", "--name=devex-mysql", "--env=MYSQL_ROOT_PASSWORD=devex-secret"))

		resources := map[string]templates.ResourceDeclaration{
			"postgres": {Type: "container", Name: "devex-postgres", ExternalID: "devex-postgres"},
			"redis":    {Type: "container", Name: "devex-redis", ExternalID: "devex-redis"},
			"mysql":    {Type: "container", Name: "devex-mysql", ExternalID: "devex-mysql"},
		}

		maintainPlan, err := planner.Plan(context.Background(), "databases", templates.PlanOptions{
			Action:         templates.LifecycleMaintain,
			InputOptions:   inputs,
			KnownResources: resources,
		})
		Expect(err).ToNot(HaveOccurred())
		Expect(stepRequest(maintainPlan, "postgres-pull").Args).To(Equal([]string{"postgres:16"}))
		Expect(stepRequest(maintainPlan, "postgres-restart").Args).To(Equal([]string{"devex-postgres"}))

		statusPlan, err := planner.Plan(context.Background(), "databases", templates.PlanOptions{
			Action:         templates.LifecycleStatus,
			InputOptions:   inputs,
			KnownResources: resources,
		})
		Expect(err).ToNot(HaveOccurred())
		Expect(stepRequest(statusPlan, "redis-status").Command).To(Equal("status"))
		Expect(stepRequest(statusPlan, "redis-status").Args).To(Equal([]string{"devex-redis"}))

		removePlan, err := planner.Plan(context.Background(), "databases", templates.PlanOptions{
			Action:         templates.LifecycleRemove,
			InputOptions:   inputs,
			KnownResources: resources,
		})
		Expect(err).ToNot(HaveOccurred())
		Expect(stepRequest(removePlan, "mysql-remove").Args).To(Equal([]string{"devex-mysql", "--volumes"}))
	})

	It("plans built-in factorio server lifecycle requests with non-interactive answers", func() {
		planner := templates.TemplatePlanner{Manager: manager}
		inputs := templates.InputResolverOptions{
			Answers:        []string{"container_name=devex-factorio", "server_port=34197"},
			NonInteractive: true,
		}

		installPlan, err := planner.Plan(context.Background(), "factorio-server", templates.PlanOptions{
			Action:       templates.LifecycleInstall,
			InputOptions: inputs,
		})
		Expect(err).ToNot(HaveOccurred())
		Expect(stepRequest(installPlan, "factorio").Args).To(Equal([]string{
			"factoriotools/factorio:stable",
			"--name=devex-factorio",
			"--port=34197:34197/udp",
			"--volume=devex-factorio-data:/factorio",
			"--restart=unless-stopped",
			"--detach",
		}))

		resources := map[string]templates.ResourceDeclaration{
			"factorio": {Type: "container", Name: "devex-factorio", ExternalID: "devex-factorio"},
		}
		maintainPlan, err := planner.Plan(context.Background(), "factorio-server", templates.PlanOptions{
			Action:         templates.LifecycleMaintain,
			InputOptions:   inputs,
			KnownResources: resources,
		})
		Expect(err).ToNot(HaveOccurred())
		Expect(stepRequest(maintainPlan, "factorio-pull").Args).To(Equal([]string{"factoriotools/factorio:stable"}))
		Expect(stepRequest(maintainPlan, "factorio-restart").Args).To(Equal([]string{"devex-factorio"}))

		statusPlan, err := planner.Plan(context.Background(), "factorio-server", templates.PlanOptions{
			Action:         templates.LifecycleStatus,
			InputOptions:   inputs,
			KnownResources: resources,
		})
		Expect(err).ToNot(HaveOccurred())
		Expect(stepRequest(statusPlan, "factorio-status").Args).To(Equal([]string{"devex-factorio"}))

		removePlan, err := planner.Plan(context.Background(), "factorio-server", templates.PlanOptions{
			Action:         templates.LifecycleRemove,
			InputOptions:   inputs,
			KnownResources: resources,
		})
		Expect(err).ToNot(HaveOccurred())
		Expect(stepRequest(removePlan, "factorio-remove").Args).To(Equal([]string{"devex-factorio", "--volumes"}))
	})

	It("plans built-in server stack compose lifecycle requests with non-interactive answers", func() {
		planner := templates.TemplatePlanner{Manager: manager}
		inputs := templates.InputResolverOptions{
			Answers:        []string{"project_name=devex-server", "compose_file=docker-compose.yml"},
			NonInteractive: true,
		}

		installPlan, err := planner.Plan(context.Background(), "server-stack", templates.PlanOptions{
			Action:       templates.LifecycleInstall,
			InputOptions: inputs,
		})
		Expect(err).ToNot(HaveOccurred())
		Expect(stepRequest(installPlan, "compose-stack").Args).To(Equal([]string{
			"up",
			"--file=docker-compose.yml",
			"--project=devex-server",
			"--detach",
		}))

		resources := map[string]templates.ResourceDeclaration{
			"compose-stack": {
				Type:       "compose_project",
				Name:       "devex-server",
				ExternalID: "devex-server",
				Metadata: map[string]any{
					"compose_file": "docker-compose.yml",
				},
			},
		}
		maintainPlan, err := planner.Plan(context.Background(), "server-stack", templates.PlanOptions{
			Action:         templates.LifecycleMaintain,
			InputOptions:   inputs,
			KnownResources: resources,
		})
		Expect(err).ToNot(HaveOccurred())
		Expect(stepRequest(maintainPlan, "compose-pull").Args).To(Equal([]string{"pull", "--file=docker-compose.yml", "--project=devex-server"}))
		Expect(stepRequest(maintainPlan, "compose-restart").Args).To(Equal([]string{"up", "--file=docker-compose.yml", "--project=devex-server", "--detach"}))

		statusPlan, err := planner.Plan(context.Background(), "server-stack", templates.PlanOptions{
			Action:         templates.LifecycleStatus,
			InputOptions:   inputs,
			KnownResources: resources,
		})
		Expect(err).ToNot(HaveOccurred())
		Expect(stepRequest(statusPlan, "compose-status").Args).To(Equal([]string{"ps", "--file=docker-compose.yml", "--project=devex-server"}))

		removePlan, err := planner.Plan(context.Background(), "server-stack", templates.PlanOptions{
			Action:         templates.LifecycleRemove,
			InputOptions:   inputs,
			KnownResources: resources,
		})
		Expect(err).ToNot(HaveOccurred())
		Expect(stepRequest(removePlan, "compose-remove").Args).To(Equal([]string{"down", "--file=docker-compose.yml", "--project=devex-server", "--volumes"}))
	})
})

func writeSpec(tempDir string, spec templates.TemplateSpec) string {
	GinkgoHelper()
	path := filepath.Join(tempDir, spec.ID+".yaml")
	data, err := yaml.Marshal(spec)
	Expect(err).ToNot(HaveOccurred())
	Expect(os.WriteFile(path, data, 0600)).To(Succeed())
	return path
}

func stepRequest(plan *templates.ExecutionPlan, stepID string) templates.ExecutionRequest {
	GinkgoHelper()
	for _, step := range plan.Steps {
		if step.ID == stepID {
			return step.Request
		}
	}
	Fail("missing planned step " + stepID)
	return templates.ExecutionRequest{}
}
