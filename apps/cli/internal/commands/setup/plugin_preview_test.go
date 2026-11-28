package setup

import (
	"context"
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/jameswlane/devex/apps/cli/internal/bootstrap"
	"github.com/jameswlane/devex/apps/cli/internal/types"
	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
)

var _ = Describe("Plugin step previews on the Confirmation screen", func() {
	var (
		model *SetupModel
		calls []pluginStep
		reply func(pluginStep) (sdk.Description, error)
	)

	step := pluginStep{plugin: "package-manager-docker", command: "ensure-installed"}
	dockerItem := func() planItem {
		s := step
		return planItem{app: types.CrossPlatformApp{Name: "docker"}, group: planDependency, reason: "databases: PostgreSQL", pluginStep: &s}
	}
	dockerDescription := sdk.Description{
		SchemaVersion: sdk.DescriptionSchemaVersion,
		Plugin:        "package-manager-docker",
		Command:       "ensure-installed",
		Actions: []sdk.PlannedAction{
			{Kind: sdk.ActionRepository, Target: "https://download.docker.com/linux/ubuntu noble stable", Scope: sdk.ScopeSystem},
			{Kind: sdk.ActionPackage, Target: "docker-ce", Scope: sdk.ScopeSystem},
			{Kind: sdk.ActionService, Target: "docker", Scope: sdk.ScopeSystem, Detail: "enable and start"},
			{Kind: sdk.ActionGroup, Target: "docker", Scope: sdk.ScopeSystem, Detail: "add you to the group"},
			{Kind: sdk.ActionFile, Target: "/home/dev/.bashrc", Scope: sdk.ScopeUser, Detail: "append activation"},
		},
		Downloads: []sdk.PlannedDownload{{
			URL:          "https://download.docker.com/linux/ubuntu/gpg",
			Verification: sdk.ArtifactVerification{Policy: "required", SHA256: "1500c1f5"},
		}},
		Notes: []string{"Log out and back in for the docker group"},
	}

	defaultDescribePluginStep := describePluginStep

	BeforeEach(func() {
		calls = nil
		reply = func(pluginStep) (sdk.Description, error) { return dockerDescription, nil }
		original := describePluginStep
		describePluginStep = func(_ context.Context, _ *bootstrap.PluginBootstrap, s pluginStep) (sdk.Description, error) {
			calls = append(calls, s)
			return reply(s)
		}
		DeferCleanup(func() { describePluginStep = original })
		model = &SetupModel{}
	})

	It("lists every change the plugin describes, with downloads, scope and notes", func() {
		model.refreshPluginPreviews(context.Background(), []planItem{dockerItem()})

		Expect(model.planItemDetails(dockerItem())).To(Equal([]string{
			"      plugin: package-manager-docker ensure-installed",
			"      Download: https://download.docker.com/linux/ubuntu/gpg (verified by sha256 1500c1f5)",
			"      Repository: https://download.docker.com/linux/ubuntu noble stable (system, needs sudo)",
			"      Package: docker-ce (system, needs sudo)",
			"      Service: docker — enable and start (system, needs sudo)",
			"      Group: docker — add you to the group (system, needs sudo)",
			"      File: /home/dev/.bashrc — append activation (user)",
			"      Note: Log out and back in for the docker group",
		}))
	})

	It("shows signature verification the way catalog entries carry it", func() {
		Expect(describeVerification(sdk.MiseInstallVerification())).To(Equal("verified by signature from key " + sdk.MiseReleaseKeyFingerprint))
		Expect(describeVerification(sdk.ArtifactVerification{})).To(Equal("not verified"))
	})

	It("says when the plugin would change nothing", func() {
		reply = func(pluginStep) (sdk.Description, error) {
			return sdk.Description{SchemaVersion: 1, Plugin: step.plugin, Command: step.command, AlreadySatisfied: true}, nil
		}
		model.refreshPluginPreviews(context.Background(), []planItem{dockerItem()})
		Expect(model.planItemDetails(dockerItem())).To(Equal([]string{"      plugin: package-manager-docker ensure-installed — nothing to change"}))
	})

	DescribeTable("falls back to one line marked as not previewed",
		func(err error, reason string) {
			reply = func(pluginStep) (sdk.Description, error) { return sdk.Description{}, err }
			model.refreshPluginPreviews(context.Background(), []planItem{dockerItem()})
			Expect(model.planItemDetails(dockerItem())).To(Equal([]string{
				"      plugin: Run package-manager-docker ensure-installed — not previewed (" + reason + ")",
			}))
		},
		Entry("plugin without describe support", sdk.ErrDescribeUnsupported, "the plugin can't preview this step"),
		Entry("plugins not bootstrapped yet", errPluginsNotReady, "plugins are not installed yet"),
		Entry("slow plugin", context.DeadlineExceeded, "the preview timed out"),
		Entry("broken plugin", errors.New("invalid description JSON"), "the preview failed: invalid description JSON"),
	)

	It("keeps plugin text and failure reasons on one safe line", func() {
		reply = func(pluginStep) (sdk.Description, error) {
			return sdk.Description{}, errors.New("describe failed: exit status 1: Error: boom\nsecond line\n")
		}
		model.refreshPluginPreviews(context.Background(), []planItem{dockerItem()})
		Expect(model.planItemDetails(dockerItem())).To(Equal([]string{
			"      plugin: Run package-manager-docker ensure-installed — not previewed (the preview failed: describe failed: exit status 1: Error: boom)",
		}))

		reply = func(pluginStep) (sdk.Description, error) {
			return sdk.Description{SchemaVersion: 1, Plugin: step.plugin, Command: step.command,
				Actions: []sdk.PlannedAction{{Kind: sdk.ActionFile, Target: "/tmp/x\x1b[2J\nfake", Scope: sdk.ScopeUser}},
				Notes:   []string{"log out\rnow"}}, nil
		}
		model.refreshPluginPreviews(context.Background(), []planItem{dockerItem()})
		Expect(model.planItemDetails(dockerItem())).To(Equal([]string{
			"      plugin: package-manager-docker ensure-installed",
			"      File: /tmp/x [2J fake (user)",
			"      Note: log out now",
		}))
	})

	It("marks a step as not previewed when plugins are not bootstrapped", func() {
		describePluginStep = defaultDescribePluginStep
		model.refreshPluginPreviews(context.Background(), []planItem{dockerItem()})
		Expect(model.planItemDetails(dockerItem())[0]).To(ContainSubstring("not previewed (plugins are not installed yet)"))
	})

	It("describes each step once and skips steps that will not run", func() {
		installed := dockerItem()
		installed.app.Name = "mise"
		installed.pluginStep = &pluginStep{plugin: "package-manager-mise", command: "ensure-installed"}
		blocked := dockerItem()
		blocked.app.Name = "other"
		blocked.pluginStep = &pluginStep{plugin: "other", command: "ensure-installed"}
		blocked.channelErr = errors.New("bad channel")
		model.selections.alreadyInstalled = map[string]bool{"mise": true}

		model.refreshPluginPreviews(context.Background(), []planItem{dockerItem(), dockerItem(), installed, blocked})
		Expect(calls).To(Equal([]pluginStep{step}))
	})

	It("shows the plugin's description instead of the catalog install method", func() {
		item := dockerItem()
		model.refreshPluginPreviews(context.Background(), []planItem{item})
		item.app.Linux = types.OSConfig{InstallMethod: "apt", InstallCommand: "docker.io"}
		details := model.planItemDetails(item)
		Expect(details).NotTo(ContainElement(ContainSubstring("apt: docker.io")))
		Expect(details).To(ContainElement("      Package: docker-ce (system, needs sudo)"))
	})
})
