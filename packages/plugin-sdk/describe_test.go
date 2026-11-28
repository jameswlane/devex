package sdk_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
	"github.com/jameswlane/devex/packages/plugin-sdk/sdktest"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// describingPlugin installs a package and writes an rc file through
// sdk.Changes, so Execute and Describe share one code path.
type describingPlugin struct {
	installed bool
}

func (p *describingPlugin) Info() sdk.PluginInfo { return sdk.PluginInfo{Name: "fixture"} }

func (p *describingPlugin) Execute(command string, args []string) error {
	return p.run(context.Background(), command, sdk.NewChanges(false))
}

func (p *describingPlugin) Describe(ctx context.Context, command string, args []string) (sdk.Description, error) {
	if command != "ensure-installed" {
		return sdk.Description{}, sdk.ErrDescribeUnsupported
	}
	changes := sdk.NewChanges(true)
	if err := p.run(ctx, command, changes); err != nil {
		return sdk.Description{}, err
	}
	return changes.Description(p.Info().Name, command, args), nil
}

func (p *describingPlugin) run(ctx context.Context, command string, changes *sdk.Changes) error {
	if command != "ensure-installed" {
		return fmt.Errorf("unknown command: %s", command)
	}
	fmt.Println("checking the fixture tool...") // plugin chatter must not reach describe's stdout
	if p.installed {
		return nil
	}
	changes.Download(sdk.PlannedDownload{URL: "https://example.com/install.sh", Verification: sdk.ArtifactVerification{Policy: "required", SHA256: "abc"}})
	if err := changes.Do(sdk.PlannedAction{Kind: sdk.ActionPackage, Target: "fixture-tool", Scope: sdk.ScopeSystem}, func() error {
		return sdk.ExecCommandWithContext(ctx, false, "apt-get", "install", "-y", "fixture-tool")
	}); err != nil {
		return err
	}
	rc := filepath.Join(os.Getenv("HOME"), ".bashrc")
	if err := changes.Do(sdk.PlannedAction{Kind: sdk.ActionFile, Target: rc, Scope: sdk.ScopeUser, Detail: "append activation"}, func() error {
		return os.WriteFile(rc, []byte("activate\n"), 0o600)
	}); err != nil {
		return err
	}
	changes.Note("open a new shell to use fixture-tool")
	return nil
}

type plainPlugin struct{}

func (plainPlugin) Info() sdk.PluginInfo           { return sdk.PluginInfo{Name: "plain"} }
func (plainPlugin) Execute(string, []string) error { return nil }

var _ = Describe("Describe contract", func() {
	var recorder *sdktest.Recorder

	BeforeEach(func() {
		recorder = sdktest.NewRecorder(GinkgoT(), "apt-get")
	})

	describe := func(plugin sdk.Plugin, args ...string) (int, string, string) {
		var stdout, stderr bytes.Buffer
		var status int
		captured := sdktest.CaptureStdout(GinkgoT(), func() {
			status = sdk.RunDescribe(context.Background(), plugin, args, &stdout, &stderr)
		})
		// Anything printed while describing goes to stderr, never stdout.
		Expect(captured).To(BeEmpty())
		return status, stdout.String(), stderr.String()
	}

	It("prints the changes without making them", func() {
		status, stdout, stderr := describe(&describingPlugin{}, "ensure-installed", "--global")
		Expect(status).To(Equal(0), stderr)

		description, err := sdk.ParseDescription([]byte(stdout))
		Expect(err).NotTo(HaveOccurred())
		rc := filepath.Join(os.Getenv("HOME"), ".bashrc")
		Expect(description).To(Equal(sdk.Description{
			SchemaVersion: sdk.DescriptionSchemaVersion,
			Plugin:        "fixture",
			Command:       "ensure-installed",
			Args:          []string{"--global"},
			Actions: []sdk.PlannedAction{
				{Kind: sdk.ActionPackage, Target: "fixture-tool", Scope: sdk.ScopeSystem},
				{Kind: sdk.ActionFile, Target: rc, Scope: sdk.ScopeUser, Detail: "append activation"},
			},
			Downloads: []sdk.PlannedDownload{{URL: "https://example.com/install.sh", Verification: sdk.ArtifactVerification{Policy: "required", SHA256: "abc"}}},
			Notes:     []string{"open a new shell to use fixture-tool"},
		}))

		Expect(recorder.Calls("apt-get")).To(BeEmpty())
		Expect(rc).NotTo(BeAnExistingFile())
	})

	It("runs the same changes when executing", func() {
		plugin := &describingPlugin{}
		Expect(plugin.Execute("ensure-installed", nil)).To(Succeed())
		Expect(recorder.Calls("apt-get")).To(Equal([]sdktest.Call{{Args: []string{"install", "-y", "fixture-tool"}}}))
		Expect(filepath.Join(os.Getenv("HOME"), ".bashrc")).To(BeAnExistingFile())
	})

	It("reports already_satisfied when nothing would change", func() {
		status, stdout, _ := describe(&describingPlugin{installed: true}, "ensure-installed")
		Expect(status).To(Equal(0))
		description, err := sdk.ParseDescription([]byte(stdout))
		Expect(err).NotTo(HaveOccurred())
		Expect(description.AlreadySatisfied).To(BeTrue())
		Expect(description.Actions).To(BeEmpty())
		Expect(stdout).To(ContainSubstring(`"actions":[]`))
	})

	It("exits with the unsupported status for commands it cannot describe", func() {
		status, stdout, stderr := describe(&describingPlugin{}, "remove")
		Expect(status).To(Equal(sdk.DescribeUnsupportedExitCode))
		Expect(stdout).To(BeEmpty())
		Expect(stderr).To(ContainSubstring(`cannot describe "remove"`))
	})

	It("exits with the unsupported status for plugins without Describer", func() {
		status, stdout, _ := describe(plainPlugin{}, "install", "git")
		Expect(status).To(Equal(sdk.DescribeUnsupportedExitCode))
		Expect(stdout).To(BeEmpty())
	})

	It("requires a command to describe", func() {
		status, _, stderr := describe(&describingPlugin{}, []string{}...)
		Expect(status).To(Equal(1))
		Expect(stderr).To(ContainSubstring("Usage"))
	})

	It("advertises describe support in --plugin-info", func() {
		Expect(sdk.PluginInfoOutput(&describingPlugin{}).SupportsDescribe).To(BeTrue())
		Expect(sdk.PluginInfoOutput(plainPlugin{}).SupportsDescribe).To(BeFalse())

		data, err := json.Marshal(sdk.PluginInfoOutput(plainPlugin{}))
		Expect(err).NotTo(HaveOccurred())
		Expect(string(data)).NotTo(ContainSubstring("supports_describe"))
	})
})

var _ = Describe("ParseDescription", func() {
	valid := func() sdk.Description {
		return sdk.Description{
			SchemaVersion: sdk.DescriptionSchemaVersion,
			Plugin:        "p",
			Command:       "c",
			Actions:       []sdk.PlannedAction{{Kind: sdk.ActionService, Target: "docker", Scope: sdk.ScopeSystem}},
			Downloads:     []sdk.PlannedDownload{{URL: "https://example.com/x"}},
		}
	}
	parse := func(description sdk.Description) error {
		data, err := json.Marshal(description)
		Expect(err).NotTo(HaveOccurred())
		_, err = sdk.ParseDescription(data)
		return err
	}

	It("accepts a valid description", func() {
		Expect(parse(valid())).To(Succeed())
	})

	DescribeTable("rejects invalid descriptions",
		func(mutate func(*sdk.Description), message string) {
			description := valid()
			mutate(&description)
			Expect(parse(description)).To(MatchError(ContainSubstring(message)))
		},
		Entry("newer schema", func(d *sdk.Description) { d.SchemaVersion = 2 }, "schema_version 2"),
		Entry("missing plugin", func(d *sdk.Description) { d.Plugin = "" }, "needs plugin"),
		Entry("no changes but not satisfied", func(d *sdk.Description) { d.Actions, d.Downloads = nil, nil }, "lists no changes"),
		Entry("satisfied with changes", func(d *sdk.Description) { d.AlreadySatisfied = true }, "already_satisfied"),
		Entry("unknown kind", func(d *sdk.Description) { d.Actions[0].Kind = "reboot" }, `unknown kind "reboot"`),
		Entry("unknown scope", func(d *sdk.Description) { d.Actions[0].Scope = "root" }, `unknown scope "root"`),
		Entry("empty target", func(d *sdk.Description) { d.Actions[0].Target = " " }, "empty target"),
		Entry("bad download URL", func(d *sdk.Description) { d.Downloads[0].URL = "file:///etc/passwd" }, "invalid URL"),
	)

	It("rejects malformed JSON", func() {
		_, err := sdk.ParseDescription([]byte("Installing...\n{"))
		Expect(err).To(MatchError(ContainSubstring("invalid description JSON")))
	})
})
