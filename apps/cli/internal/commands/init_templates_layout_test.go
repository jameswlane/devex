package commands

import (
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/platform"
	"github.com/jameswlane/devex/apps/cli/internal/templates"
	"github.com/jameswlane/devex/apps/cli/internal/types"
)

var _ = Describe("Template initialization layout", func() {
	DescribeTable("writes loadable canonical application files",
		func(additive bool) {
			home := GinkgoT().TempDir()
			configDir := filepath.Join(home, ".local/share/devex/config")
			template := templates.Template{
				Metadata: templates.TemplateMetadata{Name: "layout", Additive: additive},
				Applications: []types.AppConfig{{
					BaseConfig:    types.BaseConfig{Name: "template-tool", Description: "Template tool", Category: "development"},
					InstallMethod: "apt", InstallCommand: "template-tool",
				}},
			}
			Expect(generateConfigFromTemplate(template, configDir, platform.DetectionResult{})).To(Succeed())
			Expect(filepath.Join(configDir, "applications.yaml")).NotTo(BeAnExistingFile())
			Expect(filepath.Join(configDir, "applications/development/template-tool.yaml")).To(BeAnExistingFile())
			apps, err := (config.FileApplicationLoader{}).LoadApplications([]string{configDir})
			Expect(err).NotTo(HaveOccurred())
			Expect(apps).To(ContainElement(HaveField("Name", "template-tool")))
			if additive {
				Expect(apps).To(ContainElement(SatisfyAll(HaveField("Name", "git"), HaveField("Linux.InstallCommand", "git"))))
			} else {
				Expect(apps).To(HaveLen(1))
			}
			validator := config.NewConfigValidator(home, true)
			Expect(validator.ValidateDirectoryStructure(configDir)).To(Succeed())
			Expect(validator.GetWarnings()).To(BeEmpty())
		},
		Entry("additive templates retain the shipped Git definition", true),
		Entry("replacement templates write only their own applications", false),
	)
})
