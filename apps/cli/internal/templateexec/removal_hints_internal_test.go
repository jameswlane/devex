package templateexec

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/jameswlane/devex/apps/cli/internal/templates"
	"github.com/jameswlane/devex/apps/cli/internal/types"
)

var _ = Describe("Recorded removal hints (#757)", func() {
	declaration := templates.ResourceDeclaration{Type: "app", Name: "mise"}

	It("records no guessed removal for a plugin-method app", func() {
		step := templates.PlannedStep{
			App:     &types.AppConfig{InstallMethod: types.InstallMethodPlugin, InstallCommand: "package-manager-mise ensure-installed"},
			Request: templates.ExecutionRequest{Plugin: "package-manager-mise", Command: "ensure-installed"},
		}
		Expect(resourceMetadataWithRemovalHints(step, declaration)).NotTo(HaveKey("removal"))
	})

	It("records no guessed removal for a plugin-method app whose plugin is named package-manager-plugin", func() {
		step := templates.PlannedStep{
			App:     &types.AppConfig{InstallMethod: types.InstallMethodPlugin, InstallCommand: "package-manager-plugin setup"},
			Request: templates.ExecutionRequest{Plugin: "package-manager-plugin", Command: "setup"},
		}
		Expect(resourceMetadataWithRemovalHints(step, declaration)).NotTo(HaveKey("removal"))
	})

	It("records no guessed removal when a sensitive input redacted the install method", func() {
		step := templates.PlannedStep{
			App:     &types.AppConfig{InstallMethod: "[REDACTED]", InstallCommand: "[REDACTED]"},
			Request: templates.ExecutionRequest{Plugin: "package-manager-mise", Command: "ensure-installed"},
		}
		Expect(resourceMetadataWithRemovalHints(step, declaration)).NotTo(HaveKey("removal"))
	})

	It("keeps a removal hint the template declares", func() {
		step := templates.PlannedStep{
			App:     &types.AppConfig{InstallMethod: types.InstallMethodPlugin, InstallCommand: "package-manager-mise ensure-installed"},
			Request: templates.ExecutionRequest{Plugin: "package-manager-mise", Command: "ensure-installed"},
		}
		declared := declaration
		declared.Metadata = map[string]any{"removal": map[string]any{"plugin": "tool-example", "command": "teardown"}}
		Expect(resourceMetadataWithRemovalHints(step, declared)).To(HaveKey("removal"))
	})

	It("still guesses removal for package-manager apps", func() {
		step := templates.PlannedStep{
			App:     &types.AppConfig{InstallMethod: "apt", InstallCommand: "git"},
			Request: templates.ExecutionRequest{Plugin: "package-manager-apt", Command: "install", Args: []string{"git"}},
		}
		Expect(resourceMetadataWithRemovalHints(step, declaration)).To(HaveKey("removal"))
	})
})
