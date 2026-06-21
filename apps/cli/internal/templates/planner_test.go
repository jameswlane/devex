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
})

func writeSpec(tempDir string, spec templates.TemplateSpec) string {
	GinkgoHelper()
	path := filepath.Join(tempDir, spec.ID+".yaml")
	data, err := yaml.Marshal(spec)
	Expect(err).ToNot(HaveOccurred())
	Expect(os.WriteFile(path, data, 0600)).To(Succeed())
	return path
}
