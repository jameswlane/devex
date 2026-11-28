package commands

import (
	"context"
	"errors"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v3"

	"github.com/jameswlane/devex/apps/cli/internal/state"
	"github.com/jameswlane/devex/apps/cli/internal/templates"
	"github.com/jameswlane/devex/apps/cli/internal/types"
	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
)

type pluginMethodAppResolver struct{}

func (pluginMethodAppResolver) ResolveTemplateApp(string) (*types.AppConfig, error) {
	return &types.AppConfig{
		BaseConfig:     types.BaseConfig{Name: "mise", Category: "development"},
		InstallMethod:  types.InstallMethodPlugin,
		InstallCommand: "package-manager-mise ensure-installed",
	}, nil
}

var _ = Describe("Template removal of a plugin-method app (#757)", func() {
	It("reports manual removal instead of building a removal plan from recorded resources", func() {
		dir := GinkgoT().TempDir()
		spec := templates.TemplateSpec{
			SchemaVersion: templates.TemplateSchemaVersion,
			ID:            "tools",
			Name:          "Tools",
			Version:       "1.0.0",
			Lifecycle: map[templates.LifecycleAction][]templates.TemplateStepSpec{
				templates.LifecycleInstall: {{ID: "mise", Order: 10, App: &templates.TemplateAppAction{Ref: "mise"}}},
				templates.LifecycleRemove:  {{ID: "mise", Order: 10, App: &templates.TemplateAppAction{Ref: "mise", Command: "remove"}}},
			},
		}
		data, err := yaml.Marshal(spec)
		Expect(err).NotTo(HaveOccurred())
		path := filepath.Join(dir, "tools.yaml")
		Expect(os.WriteFile(path, data, 0o600)).To(Succeed())

		run := state.TemplateRun{ID: "tools-install-1", TemplateID: "tools", TemplateVersion: "1.0.0", Action: templates.LifecycleInstall, Status: state.RunStatusSucceeded}
		resources := []state.ManagedResource{{
			ID: "mise-resource", RunID: run.ID, TemplateID: run.TemplateID, StepID: "mise", Type: "app", Name: "mise",
			Plugin: "package-manager-mise", CurrentState: state.ResourceStatePresent,
			Metadata: map[string]any{"removal": map[string]any{"plugin": "package-manager-mise", "command": "remove", "args": []string{"mise"}}},
		}}
		planner := templates.TemplatePlanner{Manager: templates.NewTemplateManager(dir), AppResolver: pluginMethodAppResolver{}}

		plan, err := planTemplateLifecycle(context.Background(), planner, path, templates.LifecycleRemove, run, resources, knownResourcesByStep(resources), templateLifecycleOptions{NonInteractive: true})
		var manual *sdk.ManualRemovalRequiredError
		Expect(errors.As(err, &manual)).To(BeTrue(), "%v", err)
		Expect(plan).To(BeNil())
	})
})
