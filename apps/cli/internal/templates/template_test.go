package templates_test

import (
	"os"
	"path/filepath"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/jameswlane/devex/apps/cli/internal/templates"
)

func TestTemplates(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Templates Suite")
}

var _ = Describe("Template System", func() {
	var (
		templateManager *templates.TemplateManager
		tempDir         string
	)

	BeforeEach(func() {
		var err error
		tempDir, err = os.MkdirTemp("", "devex-templates-test")
		Expect(err).ToNot(HaveOccurred())

		templateManager = templates.NewTemplateManager(tempDir)
	})

	AfterEach(func() {
		if tempDir != "" {
			os.RemoveAll(tempDir)
		}
	})

	Describe("TemplateManager", func() {
		It("should create a new template manager", func() {
			Expect(templateManager).ToNot(BeNil())
		})

		It("should get available built-in templates", func() {
			templates, err := templateManager.GetAvailableTemplates()
			Expect(err).ToNot(HaveOccurred())
			Expect(len(templates)).To(BeNumerically(">=", 4)) // flat and directory built-in templates

			// Check that we have the expected templates
			templateNames := make([]string, len(templates))
			for i, template := range templates {
				templateNames[i] = template.Metadata.Name
			}

			expectedTemplates := []string{
				"web-development",
				"minimal",
				"backend",
				"full-stack",
			}

			for _, expected := range expectedTemplates {
				Expect(templateNames).To(ContainElement(expected))
			}
		})

		It("should validate template structure", func() {
			templates, err := templateManager.GetAvailableTemplates()
			Expect(err).ToNot(HaveOccurred())

			for _, template := range templates {
				err := templateManager.ValidateTemplate(template)
				Expect(err).ToNot(HaveOccurred(), "Template %s should be valid", template.Metadata.Name)

				// Check required metadata
				Expect(template.Metadata.Name).ToNot(BeEmpty())
				Expect(template.Metadata.Description).ToNot(BeEmpty())
				Expect(template.Metadata.Category).ToNot(BeEmpty())
				Expect(template.Metadata.Platforms).ToNot(BeEmpty())

				// Check that all templates have at least one application
				Expect(template.Applications).ToNot(BeEmpty())

				// Check environment settings
				Expect(template.Environment.Shell).ToNot(BeEmpty())
				Expect(template.Environment.Editor).ToNot(BeEmpty())
			}
		})

		It("should get a specific template by name", func() {
			template, err := templateManager.GetTemplate("web-development")
			Expect(err).ToNot(HaveOccurred())
			Expect(template).ToNot(BeNil())
			Expect(template.Metadata.Name).To(Equal("web-development"))
			Expect(template.Metadata.Category).To(Equal("development"))
			Expect(template.Metadata.Icon).To(Equal("🌐"))
		})

		It("should return error for non-existent template", func() {
			template, err := templateManager.GetTemplate("non-existent")
			Expect(err).To(HaveOccurred())
			Expect(template).To(BeNil())
			Expect(err.Error()).To(ContainSubstring("not found"))
		})
	})

	Describe("Built-in Templates", func() {
		var availableTemplates []templates.Template

		BeforeEach(func() {
			var err error
			availableTemplates, err = templateManager.GetAvailableTemplates()
			Expect(err).ToNot(HaveOccurred())
		})

		It("should have a web development template", func() {
			webTemplate := findTemplate(availableTemplates, "web-development")
			Expect(webTemplate).ToNot(BeNil())
			Expect(webTemplate.Metadata.Description).To(ContainSubstring("React, Vue, Angular"))
			Expect(webTemplate.Metadata.Difficulty).To(Equal("intermediate"))
			Expect(webTemplate.Applications).To(ContainElement(HaveField("Name", "node")))
			Expect(webTemplate.Applications).To(ContainElement(HaveField("Name", "vscode")))
			Expect(webTemplate.Environment.Languages).To(ContainElement("node"))
		})

		It("should have a minimal template", func() {
			minimalTemplate := findTemplate(availableTemplates, "minimal")
			Expect(minimalTemplate).ToNot(BeNil())
			Expect(minimalTemplate.Metadata.Description).To(ContainSubstring("essential development tools"))
			Expect(minimalTemplate.Metadata.Difficulty).To(Equal("beginner"))
			Expect(minimalTemplate.Applications).To(HaveLen(1)) // Only vim
		})

		It("should load directory templates through the legacy API", func() {
			backendTemplate := findTemplate(availableTemplates, "backend")
			Expect(backendTemplate).ToNot(BeNil())
			Expect(backendTemplate.Metadata.Description).To(ContainSubstring("Backend development environment"))
			Expect(backendTemplate.Applications).To(ContainElement(HaveField("Name", "postgresql")))
		})
	})

	Describe("Template Specs", func() {
		It("should load flat and directory templates as canonical specs", func() {
			specs, err := templateManager.GetAvailableTemplateSpecs()
			Expect(err).ToNot(HaveOccurred())
			Expect(len(specs)).To(BeNumerically(">=", 7))

			Expect(findSpec(specs, "minimal")).ToNot(BeNil())
			Expect(findSpec(specs, "web-development")).ToNot(BeNil())
			Expect(findSpec(specs, "backend")).ToNot(BeNil())
			Expect(findSpec(specs, "full-stack")).ToNot(BeNil())
			Expect(findSpec(specs, "databases")).ToNot(BeNil())
			Expect(findSpec(specs, "server-stack")).ToNot(BeNil())
			Expect(findSpec(specs, "factorio-server")).ToNot(BeNil())
		})

		It("should convert legacy flat templates to install lifecycle steps", func() {
			spec, err := templateManager.GetTemplateSpec("minimal")
			Expect(err).ToNot(HaveOccurred())

			Expect(spec.SchemaVersion).To(Equal(templates.TemplateSchemaVersion))
			Expect(spec.ID).To(Equal("minimal"))
			Expect(spec.Lifecycle).To(HaveKey(templates.LifecycleInstall))
			Expect(spec.Lifecycle[templates.LifecycleInstall]).To(HaveLen(1))
			Expect(spec.Lifecycle[templates.LifecycleInstall][0].App).ToNot(BeNil())
			Expect(spec.Lifecycle[templates.LifecycleInstall][0].App.Name).To(Equal("vim"))
		})

		It("should convert directory templates to install lifecycle steps", func() {
			spec, err := templateManager.GetTemplateSpec("backend")
			Expect(err).ToNot(HaveOccurred())

			Expect(spec.SchemaVersion).To(Equal(templates.TemplateSchemaVersion))
			Expect(spec.ID).To(Equal("backend"))
			Expect(spec.Categories).To(ContainElement("backend"))
			Expect(spec.Lifecycle[templates.LifecycleInstall]).ToNot(BeEmpty())
			Expect(findStepApp(spec.Lifecycle[templates.LifecycleInstall], "postgresql")).ToNot(BeNil())
		})

		It("should load user canonical template specs", func() {
			userTemplatesDir := filepath.Join(tempDir, ".devex", "templates")
			Expect(os.MkdirAll(userTemplatesDir, 0750)).To(Succeed())

			templateFile := filepath.Join(userTemplatesDir, "databases.yaml")
			Expect(os.WriteFile(templateFile, []byte(`
schema_version: devex.template/v1
id: databases
name: Databases
version: 1.0.0
description: Database template
inputs:
  db_password:
    type: text
    prompt: Database password
    sensitive: true
lifecycle:
  install:
    - id: postgres
      order: 10
      app:
        name: PostgreSQL
        category: databases
        installer: docker
        command: install
        args:
          - postgres:16
`), 0600)).To(Succeed())

			spec, err := templateManager.GetTemplateSpec("databases")
			Expect(err).ToNot(HaveOccurred())
			Expect(spec.Inputs["db_password"].Sensitive).To(BeTrue())
			Expect(spec.Lifecycle[templates.LifecycleInstall]).To(HaveLen(1))
		})

		It("should reject specs with unsupported schema versions", func() {
			spec := templates.TemplateSpec{
				SchemaVersion: "devex.template/v0",
				ID:            "bad",
				Name:          "Bad",
				Version:       "1.0.0",
				Lifecycle: map[templates.LifecycleAction][]templates.TemplateStepSpec{
					templates.LifecycleInstall: {},
				},
			}

			err := templateManager.ValidateTemplateSpec(spec)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("unsupported template schema version"))
		})

		It("should reject specs with duplicate step IDs", func() {
			spec := templates.TemplateSpec{
				SchemaVersion: templates.TemplateSchemaVersion,
				ID:            "bad",
				Name:          "Bad",
				Version:       "1.0.0",
				Lifecycle: map[templates.LifecycleAction][]templates.TemplateStepSpec{
					templates.LifecycleInstall: {
						{
							ID: "same",
							Plugin: &templates.TemplatePluginAction{
								Name:    "package-manager-docker",
								Command: "status",
							},
						},
						{
							ID: "same",
							Plugin: &templates.TemplatePluginAction{
								Name:    "package-manager-docker",
								Command: "status",
							},
						},
					},
				},
			}

			err := templateManager.ValidateTemplateSpec(spec)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("duplicate step id"))
		})

		It("should allow the same step ID in different lifecycle actions", func() {
			spec := templates.TemplateSpec{
				SchemaVersion: templates.TemplateSchemaVersion,
				ID:            "database",
				Name:          "Database",
				Version:       "1.0.0",
				Lifecycle: map[templates.LifecycleAction][]templates.TemplateStepSpec{
					templates.LifecycleInstall: {
						{
							ID: "postgres",
							Plugin: &templates.TemplatePluginAction{
								Name:    "package-manager-docker",
								Command: "install",
							},
						},
					},
					templates.LifecycleMaintain: {
						{
							ID: "postgres",
							Plugin: &templates.TemplatePluginAction{
								Name:    "package-manager-docker",
								Command: "restart",
							},
						},
					},
				},
			}

			err := templateManager.ValidateTemplateSpec(spec)
			Expect(err).ToNot(HaveOccurred())
		})

		It("should reject specs with invalid input types", func() {
			spec := templates.TemplateSpec{
				SchemaVersion: templates.TemplateSchemaVersion,
				ID:            "bad-input",
				Name:          "Bad Input",
				Version:       "1.0.0",
				Inputs: map[string]templates.TemplateInput{
					"databases": {
						Type:   "multselect",
						Prompt: "Choose databases",
					},
				},
				Lifecycle: map[templates.LifecycleAction][]templates.TemplateStepSpec{
					templates.LifecycleInstall: {
						{
							ID: "postgres",
							Plugin: &templates.TemplatePluginAction{
								Name:    "package-manager-docker",
								Command: "install",
							},
						},
					},
				},
			}

			err := templateManager.ValidateTemplateSpec(spec)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("invalid type"))
		})

		It("should reject specs with multiple primary actions in one step", func() {
			spec := templates.TemplateSpec{
				SchemaVersion: templates.TemplateSchemaVersion,
				ID:            "bad",
				Name:          "Bad",
				Version:       "1.0.0",
				Lifecycle: map[templates.LifecycleAction][]templates.TemplateStepSpec{
					templates.LifecycleInstall: {
						{
							ID: "bad-step",
							App: &templates.TemplateAppAction{
								Name: "App",
							},
							Plugin: &templates.TemplatePluginAction{
								Name:    "package-manager-docker",
								Command: "status",
							},
						},
					},
				},
			}

			err := templateManager.ValidateTemplateSpec(spec)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("exactly one primary action"))
		})

		It("should slugify legacy display names when converting to specs", func() {
			legacy := templates.Template{
				Metadata: templates.TemplateMetadata{
					Name:        "Web Development",
					Version:     "1.0.0",
					Description: "Web development template",
					Category:    "development",
					Platforms:   []string{"linux"},
				},
			}

			spec := templates.ConvertLegacyTemplateToSpec(legacy)
			Expect(spec.ID).To(Equal("web-development"))
			Expect(spec.Name).To(Equal("Web Development"))
		})

		It("should not add a blank category when converting legacy templates", func() {
			legacy := templates.Template{
				Metadata: templates.TemplateMetadata{
					Name:        "Uncategorized",
					Version:     "1.0.0",
					Description: "Template without a category",
					Platforms:   []string{"linux"},
				},
			}

			spec := templates.ConvertLegacyTemplateToSpec(legacy)
			Expect(spec.Categories).To(BeEmpty())
		})
	})

	Describe("Template Categories", func() {
		It("should have proper category distribution", func() {
			templates, err := templateManager.GetAvailableTemplates()
			Expect(err).ToNot(HaveOccurred())

			categories := make(map[string]int)
			for _, template := range templates {
				categories[template.Metadata.Category]++
			}

			// Check that we have development category templates
			Expect(categories["development"]).To(BeNumerically(">", 0))
		})
	})

	Describe("Template Validation", func() {
		It("should reject template with missing name", func() {
			invalidTemplate := templates.Template{
				Metadata: templates.TemplateMetadata{
					Description: "Test template",
					Category:    "test",
				},
			}

			err := templateManager.ValidateTemplate(invalidTemplate)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("name is required"))
		})

		It("should reject template with invalid platform", func() {
			invalidTemplate := templates.Template{
				Metadata: templates.TemplateMetadata{
					Name:        "test",
					Description: "Test template",
					Category:    "test",
					Platforms:   []string{"invalid-platform"},
				},
			}

			err := templateManager.ValidateTemplate(invalidTemplate)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("invalid platform"))
		})

		It("should reject template with invalid difficulty", func() {
			invalidTemplate := templates.Template{
				Metadata: templates.TemplateMetadata{
					Name:        "test",
					Description: "Test template",
					Category:    "test",
					Platforms:   []string{"linux"},
					Difficulty:  "invalid-difficulty",
				},
			}

			err := templateManager.ValidateTemplate(invalidTemplate)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("invalid difficulty"))
		})
	})

	Describe("User Templates", func() {
		It("should save and load user templates", func() {
			userTemplate := templates.Template{
				Metadata: templates.TemplateMetadata{
					Name:        "custom-test",
					Version:     "1.0.0",
					Description: "Custom test template",
					Category:    "development",
					Platforms:   []string{"linux"},
					Difficulty:  "beginner",
				},
				Environment: templates.EnvironmentTemplate{
					Shell:  "bash",
					Editor: "vim",
				},
			}

			// Save the template
			err := templateManager.SaveTemplate(userTemplate)
			Expect(err).ToNot(HaveOccurred())

			// Verify file was created
			userTemplatesDir := filepath.Join(tempDir, ".devex", "templates")
			templateFile := filepath.Join(userTemplatesDir, "custom-test.yaml")
			Expect(templateFile).To(BeAnExistingFile())

			// Load templates and verify our custom template is included
			templates, err := templateManager.GetAvailableTemplates()
			Expect(err).ToNot(HaveOccurred())

			customTemplate := findTemplate(templates, "custom-test")
			Expect(customTemplate).ToNot(BeNil())
			Expect(customTemplate.Metadata.Description).To(Equal("Custom test template"))
		})

		It("should skip canonical template specs in the legacy template loader", func() {
			userTemplatesDir := filepath.Join(tempDir, ".devex", "templates")
			Expect(os.MkdirAll(userTemplatesDir, 0750)).To(Succeed())

			templateFile := filepath.Join(userTemplatesDir, "canonical.yaml")
			Expect(os.WriteFile(templateFile, []byte(`
schema_version: devex.template/v1
id: canonical
name: Canonical
version: 1.0.0
lifecycle:
  install:
    - id: noop
      plugin:
        name: package-manager-docker
        command: status
`), 0600)).To(Succeed())

			templates, err := templateManager.GetAvailableTemplates()
			Expect(err).ToNot(HaveOccurred())
			Expect(findTemplate(templates, "canonical")).To(BeNil())

			spec, err := templateManager.GetTemplateSpec("canonical")
			Expect(err).ToNot(HaveOccurred())
			Expect(spec.ID).To(Equal("canonical"))
		})
	})
})

// Helper function to find a template by name
func findTemplate(templates []templates.Template, name string) *templates.Template {
	for _, template := range templates {
		if template.Metadata.Name == name {
			return &template
		}
	}
	return nil
}

func findSpec(specs []templates.TemplateSpec, id string) *templates.TemplateSpec {
	for _, spec := range specs {
		if spec.ID == id {
			return &spec
		}
	}
	return nil
}

func findStepApp(steps []templates.TemplateStepSpec, name string) *templates.TemplateAppAction {
	for _, step := range steps {
		if step.App != nil && step.App.Name == name {
			return step.App
		}
	}
	return nil
}
