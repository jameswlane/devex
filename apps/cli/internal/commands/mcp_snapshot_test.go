package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"

	"github.com/jameswlane/devex/apps/cli/internal/agentapi"
	"github.com/jameswlane/devex/apps/cli/internal/bootstrap"
	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/installers"
	"github.com/jameswlane/devex/apps/cli/internal/mcpserver"
	"github.com/jameswlane/devex/apps/cli/internal/mocks"
	"github.com/jameswlane/devex/apps/cli/internal/state"
	"github.com/jameswlane/devex/apps/cli/internal/types"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/viper"
)

var _ = Describe("MCP execution snapshots", func() {
	var ctx context.Context
	var e *mcpExecutor
	var source, destination, hook string
	var session *mcp.ClientSession
	BeforeEach(func() {
		ctx = context.Background()
		viper.Reset()
		DeferCleanup(viper.Reset)
		home := GinkgoT().TempDir()
		GinkgoT().Setenv("HOME", home)
		GinkgoT().Setenv("DEVEX_CONFIG_DIR", filepath.Join(home, "config"))
		GinkgoT().Setenv("DEVEX_DATA_DIR", filepath.Join(home, "data"))
		viper.Set(state.DBPathConfigKey, filepath.Join(home, "state.db"))
		source, destination, hook = filepath.Join(home, "source"), filepath.Join(home, "destination"), filepath.Join(home, "hook")
		GinkgoT().Setenv("MCP_HOOK", hook)
		Expect(os.WriteFile(source, []byte("approved"), 0600)).To(Succeed())
		app := types.CrossPlatformApp{Name: "snapshot-tool", Description: "fixture", Category: "Testing", AllPlatforms: types.OSConfig{
			InstallMethod: "apt", InstallCommand: "snapshot-tool", ConfigFiles: []types.ConfigFile{{Source: source, Destination: destination}},
			PostInstall: []types.InstallCommand{{Shell: `touch "$MCP_HOOK"`}},
		}}
		e = &mcpExecutor{repo: mocks.NewMockRepository(), settings: config.CrossPlatformSettings{HomeDir: home, ApplicationOverrides: []types.CrossPlatformApp{app}}, log: &bytes.Buffer{}}
		oldInit, oldPrepare := initializeMCPPlugins, prepareApplicationInstallers
		DeferCleanup(func() { initializeMCPPlugins, prepareApplicationInstallers = oldInit, oldPrepare })
		initializeMCPPlugins = func(context.Context) (*bootstrap.PluginBootstrap, error) { return &bootstrap.PluginBootstrap{}, nil }
		prepareApplicationInstallers = func(context.Context, pluginBootstrapProvider) error { return nil }
		// Only the package-manager boundary is faked; config copying and hooks use
		// the production lifecycle with the real adapter and pinned context.
		installers.EnableTestMode()
		DeferCleanup(installers.DisableTestMode)
		srv := mcpserver.New(mcpserver.Options{Settings: e.settings, Executor: e, Mutations: true})
		st, ct := mcp.NewInMemoryTransports()
		ss, err := srv.Connect(ctx, st, nil)
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(ss.Close)
		session, err = mcp.NewClient(&mcp.Implementation{Name: "snapshot-test", Version: "1"}, nil).Connect(ctx, ct, nil)
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(session.Close)
	})
	call := func(name string, args any) *mcp.CallToolResult {
		r, err := session.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: args})
		Expect(err).NotTo(HaveOccurred())
		return r
	}
	prepare := func() mcpserver.PreparedPlan {
		r := call("prepare_install", map[string]any{"apps": []string{"snapshot-tool"}})
		Expect(r.IsError).To(BeFalse(), "%v", r.Content)
		data, err := json.Marshal(r.StructuredContent)
		Expect(err).NotTo(HaveOccurred())
		var p mcpserver.PreparedPlan
		Expect(json.Unmarshal(data, &p)).To(Succeed())
		return p
	}
	DescribeTable("rejects changed approved inputs before execution", func(change string) {
		key := filepath.Join(os.Getenv("HOME"), "public.key")
		Expect(os.WriteFile(key, []byte("first key"), 0600)).To(Succeed())
		viper.Set("plugin.public-key", key)
		p := prepare()
		switch change {
		case "bytes":
			Expect(os.WriteFile(source, []byte("unapproved"), 0600)).To(Succeed())
		case "policy":
			viper.Set("offline", true)
		case "trust":
			Expect(os.WriteFile(key, []byte("second key"), 0600)).To(Succeed())
		case "registry":
			GinkgoT().Setenv("DEVEX_PLUGIN_REGISTRY_URL", "https://example.invalid/new")
		case "security":
			Expect(os.MkdirAll(filepath.Join(os.Getenv("HOME"), ".devex"), 0700)).To(Succeed())
			// Changing the resolved embedded security settings is covered by this
			// adapter's effective settings, without writing a host policy.
			e.settings.Security.Level = 2
		}
		r := call("install_apps", mcpserver.ExecuteInput{PlanID: p.PlanID, Digest: p.Digest})
		Expect(r.IsError).To(BeTrue(), "%v", r.Content)
		Expect(destination).NotTo(BeAnExistingFile())
		Expect(hook).NotTo(BeAnExistingFile())
	}, Entry("source bytes", "bytes"), Entry("effective policy", "policy"), Entry("public key bytes", "trust"), Entry("registry", "registry"), Entry("security policy", "security"))
	It("executes verified definitions and security while restoring request policy and trust", func() {
		viper.Set("offline", false)
		GinkgoT().Setenv("DEVEX_PLUGIN_REGISTRY_URL", "https://example.invalid/approved")
		snapshot, err := e.ResolveInstall(ctx, agentapi.PlanRequest{Names: []string{"snapshot-tool"}})
		Expect(err).NotTo(HaveOccurred())
		oldInstall := installApplication
		DeferCleanup(func() { installApplication = oldInstall })
		initializeMCPPlugins = func(context.Context) (*bootstrap.PluginBootstrap, error) {
			Expect(viper.MergeConfigMap(map[string]any{"offline": true})).To(Succeed())
			e.settings.Security.Level = 2
			e.settings.ApplicationOverrides[0].AllPlatforms.InstallCommand = "unapproved-command"
			return &bootstrap.PluginBootstrap{}, nil
		}
		installApplication = func(_ context.Context, app types.CrossPlatformApp, settings config.CrossPlatformSettings, _ types.Repository) error {
			Expect(app.AllPlatforms.InstallCommand).To(Equal("snapshot-tool"))
			Expect(settings.ResolvedSecurity).To(Equal(snapshot.Security))
			Expect(viper.GetBool("offline")).To(BeFalse())
			Expect(bootstrap.GetRegistryURL()).To(Equal(snapshot.Trust.PluginRegistryURL))
			return nil
		}
		outcomes, err := e.Install(ctx, snapshot)
		Expect(err).NotTo(HaveOccurred())
		Expect(outcomes[0].Status).To(Equal("installed"))
		Expect(os.Getenv("DEVEX_PLUGIN_REGISTRY_URL")).To(Equal("https://example.invalid/approved"))
		viper.Set("offline", true)
		next, err := e.ResolveInstall(ctx, agentapi.PlanRequest{Names: []string{"snapshot-tool"}})
		Expect(err).NotTo(HaveOccurred())
		Expect(next.Digest).NotTo(Equal(snapshot.Digest))
	})

	It("honors the verified rollback policy through the shared lifecycle", func() {
		e.settings.ApplicationOverrides[0].AllPlatforms = types.OSConfig{InstallMethod: "docker", InstallCommand: "snapshot-tool"}
		viper.Set("rollback-on-failure", true)
		oldInstall := installApplication
		DeferCleanup(func() { installApplication = oldInstall })
		installApplication = func(context.Context, types.CrossPlatformApp, config.CrossPlatformSettings, types.Repository) error {
			return nil
		}
		plan := prepare()
		r := call("install_apps", mcpserver.ExecuteInput{PlanID: plan.PlanID, Digest: plan.Digest})
		Expect(r.IsError).To(BeFalse(), "%v", r.Content)
		data, err := json.Marshal(r.StructuredContent)
		Expect(err).NotTo(HaveOccurred())
		var result mcpserver.ExecuteOutput
		Expect(json.Unmarshal(data, &result)).To(Succeed())
		Expect(result.Result).To(Equal("completed"))
		Expect(result.Note).To(ContainSubstring("Rollback was requested"))
		Expect(e.log.(*bytes.Buffer).String()).To(ContainSubstring("Lifecycle run:"))
	})

	It("returns a failed structured uninstall with approved recovery paths", func() {
		home := os.Getenv("HOME")
		dataPath := filepath.Join(home, "cleanup", "data")
		Expect(os.MkdirAll(filepath.Dir(dataPath), 0700)).To(Succeed())
		Expect(os.WriteFile(dataPath, []byte("retain for retry"), 0600)).To(Succeed())
		app := types.CrossPlatformApp{Name: "cleanup-tool", AllPlatforms: types.OSConfig{InstallMethod: "curlpipe", InstallCommand: "https://example.invalid/install.sh", UninstallCommand: "true", CleanupFiles: []string{dataPath}}}
		definition, err := json.Marshal(app)
		Expect(err).NotTo(HaveOccurred())
		Expect(e.repo.AddApp(app.Name)).To(Succeed())
		Expect(e.repo.Set("installed-app-definition:"+app.Name, string(definition))).To(Succeed())
		r := call("prepare_uninstall", map[string]any{"apps": []string{app.Name}, "remove_data": true})
		Expect(r.IsError).To(BeFalse(), "%v", r.Content)
		encoded, err := json.Marshal(r.StructuredContent)
		Expect(err).NotTo(HaveOccurred())
		var plan mcpserver.PreparedPlan
		Expect(json.Unmarshal(encoded, &plan)).To(Succeed())
		Expect(os.Chmod(filepath.Dir(dataPath), 0500)).To(Succeed())
		DeferCleanup(func() { Expect(os.Chmod(filepath.Dir(dataPath), 0700)).To(Succeed()) })
		r = call("uninstall_apps", mcpserver.ExecuteInput{PlanID: plan.PlanID, Digest: plan.Digest})
		Expect(r.IsError).To(BeFalse(), "%v", r.Content)
		encoded, err = json.Marshal(r.StructuredContent)
		Expect(err).NotTo(HaveOccurred())
		var result mcpserver.ExecuteOutput
		Expect(json.Unmarshal(encoded, &result)).To(Succeed())
		Expect(result.Result).To(Equal("failed"))
		Expect(result.Outcomes).To(HaveLen(1))
		Expect(result.Outcomes[0].Status).To(Equal("failed"))
		Expect(result.Outcomes[0].Error).To(ContainSubstring("package removed"))
		Expect(result.Outcomes[0].Recovery.DataPaths).To(Equal([]string{dataPath}))
		Expect(e.repo.GetApp(app.Name)).NotTo(BeNil())
		Expect(dataPath).To(BeAnExistingFile())
	})

	DescribeTable("handles approved source transitions after verification", func(consumer, transition string) {
		if transition == "read failure" {
			if runtime.GOOS != "linux" {
				Skip("requires Linux /proc/self/mem for a root-independent read error")
			}
			info, err := os.Stat("/proc/self/mem")
			Expect(err).NotTo(HaveOccurred())
			Expect(info.Mode().IsRegular()).To(BeTrue())
			_, err = os.ReadFile("/proc/self/mem")
			Expect(err).To(HaveOccurred(), "the fixture must fail reading even with root privileges")
		}
		if transition == "already missing" {
			Expect(os.Remove(source)).To(Succeed())
		}
		mutated := false
		prepareApplicationInstallers = func(context.Context, pluginBootstrapProvider) error {
			mutated = true
			switch transition {
			case "removed":
				Expect(os.Remove(source)).To(Succeed())
			case "directory":
				Expect(os.Remove(source)).To(Succeed())
				Expect(os.Mkdir(source, 0700)).To(Succeed())
			case "read failure":
				Expect(os.Remove(source)).To(Succeed())
				Expect(os.Symlink("/proc/self/mem", source)).To(Succeed())
			}
			return nil
		}
		shouldFail := transition != "unchanged" && transition != "already missing"
		if consumer == "MCP" {
			plan := prepare()
			r := call("install_apps", mcpserver.ExecuteInput{PlanID: plan.PlanID, Digest: plan.Digest})
			Expect(r.IsError).To(BeFalse(), "%v", r.Content)
			data, err := json.Marshal(r.StructuredContent)
			Expect(err).NotTo(HaveOccurred())
			var result mcpserver.ExecuteOutput
			Expect(json.Unmarshal(data, &result)).To(Succeed())
			Expect(result.Outcomes).To(HaveLen(1))
			if shouldFail {
				Expect(result.Result).To(Equal("failed"))
				Expect(result.Outcomes[0].Status).To(Equal("failed"))
				Expect(result.Outcomes[0].Error).To(ContainSubstring("changed after the plan was confirmed"))
			} else {
				Expect(result.Result).To(Equal("completed"))
				Expect(result.Outcomes[0].Status).To(Equal("installed"))
			}
		} else {
			// Use the real confirmed Cobra command, not only a pinned-context
			// helper: its final verification precedes the mutation callback.
			snapshot, err := e.ResolveInstall(ctx, agentapi.PlanRequest{Names: []string{"snapshot-tool"}})
			Expect(err).NotTo(HaveOccurred())
			cmd := NewInstallCmd(e.repo, e.settings)
			cmd.SetOut(e.log)
			cmd.SetErr(e.log)
			cmd.SetArgs([]string{"snapshot-tool", "--non-interactive", "--expect-plan", snapshot.Digest})
			err = cmd.ExecuteContext(ctx)
			if shouldFail {
				Expect(err).To(MatchError(ContainSubstring("changed after the plan was confirmed")))
			} else {
				Expect(err).NotTo(HaveOccurred())
			}
		}
		Expect(mutated).To(BeTrue())
		if shouldFail {
			Expect(destination).NotTo(BeAnExistingFile())
			Expect(hook).NotTo(BeAnExistingFile())
		} else {
			// An actual Shell action must run on success, proving that the
			// negative cases exercise a reachable post-install hook.
			Expect(hook).To(BeAnExistingFile())
			if transition == "unchanged" {
				Expect(os.ReadFile(destination)).To(BeEquivalentTo("approved"))
			} else {
				Expect(destination).NotTo(BeAnExistingFile())
			}
		}
	}, Entry("MCP removed", "MCP", "removed"), Entry("MCP directory", "MCP", "directory"), Entry("MCP read failure", "MCP", "read failure"), Entry("MCP successful hook control", "MCP", "unchanged"), Entry("MCP already missing optional warning", "MCP", "already missing"),
		Entry("CLI removed", "CLI", "removed"), Entry("CLI directory", "CLI", "directory"), Entry("CLI read failure", "CLI", "read failure"), Entry("CLI successful hook control", "CLI", "unchanged"), Entry("CLI already missing optional warning", "CLI", "already missing"))

	It("refuses changed source bytes after verification at the production lifecycle boundary", func() {
		snapshot, err := e.ResolveInstall(ctx, agentapi.PlanRequest{Names: []string{"snapshot-tool"}})
		Expect(err).NotTo(HaveOccurred())
		prepareApplicationInstallers = func(context.Context, pluginBootstrapProvider) error {
			Expect(os.WriteFile(source, []byte("changed after verification"), 0600)).To(Succeed())
			return nil
		}
		outcomes, err := e.Install(ctx, snapshot)
		Expect(err).To(HaveOccurred())
		Expect(outcomes).To(HaveLen(1))
		Expect(outcomes[0].Status).To(Equal("failed"))
		Expect(outcomes[0].Error).To(ContainSubstring("changed after the plan was confirmed"))
		Expect(destination).NotTo(BeAnExistingFile())
		Expect(hook).NotTo(BeAnExistingFile())
	})
})
