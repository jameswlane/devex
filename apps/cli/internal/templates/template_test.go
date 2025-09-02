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
			Expect(templates).To(HaveLen(2)) // 2 built-in templates (converted to YAML)

			// Check that we have the expected templates
			templateNames := make([]string, len(templates))
			for i, template := range templates {
				templateNames[i] = template.Metadata.Name
			}

			expectedTemplates := []string{
				"web-development",
				"minimal",
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
