package mcpserver_test

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/viper"

	"github.com/jameswlane/devex/apps/cli/internal/agentapi"
	"github.com/jameswlane/devex/apps/cli/internal/catalog"
	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/mcpserver"
	"github.com/jameswlane/devex/apps/cli/internal/platform"
)

type fakeExecutor struct {
	mu          sync.Mutex
	installed   [][]string
	uninstalled [][]string
	options     []mcpserver.UninstallOptions
	outcomes    []mcpserver.Outcome
	untracked   bool
	block       bool
}

func (f *fakeExecutor) Install(ctx context.Context, apps []string) ([]mcpserver.Outcome, error) {
	f.mu.Lock()
	f.installed = append(f.installed, append([]string(nil), apps...))
	f.mu.Unlock()
	if f.block {
		<-ctx.Done()
		return nil, ctx.Err()
	}
	if f.outcomes != nil {
		return f.outcomes, nil
	}
	outcomes := []mcpserver.Outcome{}
	for _, app := range apps {
		outcomes = append(outcomes, mcpserver.Outcome{App: app, Status: "installed"})
	}
	return outcomes, nil
}

func (f *fakeExecutor) PlanUninstall(_ context.Context, apps []string, options mcpserver.UninstallOptions) ([]mcpserver.UninstallTarget, error) {
	if f.untracked {
		return nil, &agentapi.Error{Category: "not_found", Message: "application git is not tracked by DevEx"}
	}
	targets := []mcpserver.UninstallTarget{}
	for _, app := range apps {
		config, data := "kept", "kept"
		if options.RemoveConfig {
			config = "removed"
		}
		if options.RemoveData {
			data = "removed"
		}
		targets = append(targets, mcpserver.UninstallTarget{Name: app, Method: "apt", Config: config, Data: data})
	}
	return targets, nil
}

func (f *fakeExecutor) Uninstall(_ context.Context, apps []string, options mcpserver.UninstallOptions) ([]mcpserver.Outcome, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.uninstalled = append(f.uninstalled, append([]string(nil), apps...))
	f.options = append(f.options, options)
	outcomes := make([]mcpserver.Outcome, 0, len(apps))
	for _, app := range apps {
		outcomes = append(outcomes, mcpserver.Outcome{App: app, Status: "removed"})
	}
	return outcomes, nil
}

var _ = Describe("MCP mutating tools", func() {
	var (
		ctx      context.Context
		executor *fakeExecutor
		options  mcpserver.Options
		now      time.Time
		host     platform.Platform
	)

	BeforeEach(func() {
		ctx = context.Background()
		viper.Reset()
		DeferCleanup(viper.Reset)
		settings := config.CrossPlatformSettings{HomeDir: GinkgoT().TempDir(), Catalog: &catalog.Result{}}
		Expect(config.LoadCatalogOffline(ctx, settings)).To(Succeed())
		executor = &fakeExecutor{}
		now = time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC)
		host = platform.Platform{OS: "linux", Distribution: "ubuntu", Version: "24.04"}
		options = mcpserver.Options{
			Version: "test", Settings: settings, Mutations: true, Executor: executor,
			Clock:          func() time.Time { return now },
			DetectPlatform: func(context.Context) (*platform.Platform, error) { h := host; return &h, nil },
		}
	})

	connect := func() *mcp.ClientSession {
		server := mcpserver.New(options)
		serverTransport, clientTransport := mcp.NewInMemoryTransports()
		serverSession, err := server.Connect(ctx, serverTransport, nil)
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(serverSession.Close)
		session, err := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil).Connect(ctx, clientTransport, nil)
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(session.Close)
		return session
	}
	call := func(session *mcp.ClientSession, name string, args any) *mcp.CallToolResult {
		result, err := session.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: args})
		Expect(err).NotTo(HaveOccurred())
		return result
	}
	decode := func(result *mcp.CallToolResult, target any) {
		Expect(result.IsError).To(BeFalse(), "tool error: %v", result.Content)
		data, err := json.Marshal(result.StructuredContent)
		Expect(err).NotTo(HaveOccurred())
		Expect(json.Unmarshal(data, target)).To(Succeed())
	}
	errorText := func(result *mcp.CallToolResult) string {
		Expect(result.IsError).To(BeTrue())
		return result.Content[0].(*mcp.TextContent).Text
	}
	prepare := func(session *mcp.ClientSession, apps ...string) mcpserver.PreparedPlan {
		var plan mcpserver.PreparedPlan
		decode(call(session, "prepare_install", map[string]any{"apps": apps}), &plan)
		return plan
	}

	It("does not register mutating tools unless mutations are enabled", func() {
		options.Mutations = false
		session := connect()
		tools, err := session.ListTools(ctx, nil)
		Expect(err).NotTo(HaveOccurred())
		for _, tool := range tools.Tools {
			Expect(mcpserver.MutatingTools).NotTo(ContainElement(tool.Name))
		}
		for _, name := range mcpserver.MutatingTools {
			_, err := session.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: map[string]any{"plan_id": "x", "digest": "y"}})
			Expect(err).To(MatchError(ContainSubstring("unknown tool")), name)
		}
	})

	It("annotates the mutating tools", func() {
		tools, err := connect().ListTools(ctx, nil)
		Expect(err).NotTo(HaveOccurred())
		byName := map[string]*mcp.Tool{}
		for _, tool := range tools.Tools {
			byName[tool.Name] = tool
		}
		Expect(byName).To(HaveKey("install_apps"))
		Expect(byName["install_apps"].Annotations.ReadOnlyHint).To(BeFalse())
		Expect(*byName["uninstall_apps"].Annotations.DestructiveHint).To(BeTrue())
		Expect(byName["prepare_install"].Annotations.ReadOnlyHint).To(BeTrue())
	})

	It("requires named applications and never plans defaults or everything", func() {
		session := connect()
		Expect(errorText(call(session, "prepare_install", map[string]any{}))).To(ContainSubstring("name at least one application or category"))
		Expect(errorText(call(session, "prepare_uninstall", map[string]any{"apps": []string{}}))).To(ContainSubstring("name at least one application"))
	})

	It("installs exactly the prepared applications once", func() {
		session := connect()
		plan := prepare(session, "git")
		Expect(plan.PlanID).NotTo(BeEmpty())
		Expect(plan.Digest).To(HaveLen(64))
		Expect(plan.Install.Apps).To(ConsistOf(HaveField("Name", "git")))
		Expect(plan.Confirmation).To(ContainSubstring("explicit approval"))
		Expect(executor.installed).To(BeEmpty())

		var out mcpserver.ExecuteOutput
		decode(call(session, "install_apps", map[string]any{"plan_id": plan.PlanID, "digest": plan.Digest}), &out)
		Expect(out.Result).To(Equal("completed"))
		Expect(out.Outcomes).To(ConsistOf(mcpserver.Outcome{App: "git", Status: "installed"}))
		Expect(executor.installed).To(Equal([][]string{{"git"}}))

		Expect(errorText(call(session, "install_apps", map[string]any{"plan_id": plan.PlanID, "digest": plan.Digest}))).To(ContainSubstring("already used"))
		Expect(executor.installed).To(HaveLen(1))
	})

	DescribeTable("rejects invalid confirmations before any effect",
		func(mutate func(*mcpserver.PreparedPlan, *time.Time, *platform.Platform), tool, message string) {
			session := connect()
			plan := prepare(session, "git")
			mutate(&plan, &now, &host)
			Expect(errorText(call(session, tool, map[string]any{"plan_id": plan.PlanID, "digest": plan.Digest}))).To(ContainSubstring(message))
			Expect(executor.installed).To(BeEmpty())
			Expect(executor.uninstalled).To(BeEmpty())
		},
		Entry("missing plan_id", func(p *mcpserver.PreparedPlan, _ *time.Time, _ *platform.Platform) { p.PlanID = "" }, "install_apps", "plan_id and digest"),
		Entry("unknown plan", func(p *mcpserver.PreparedPlan, _ *time.Time, _ *platform.Platform) { p.PlanID = "0123" }, "install_apps", "unknown or already used"),
		Entry("wrong digest", func(p *mcpserver.PreparedPlan, _ *time.Time, _ *platform.Platform) { p.Digest = "deadbeef" }, "install_apps", "digest does not match"),
		Entry("expired", func(_ *mcpserver.PreparedPlan, t *time.Time, _ *platform.Platform) { *t = t.Add(11 * time.Minute) }, "install_apps", "expired"),
		Entry("other operation", func(_ *mcpserver.PreparedPlan, _ *time.Time, _ *platform.Platform) {}, "uninstall_apps", "plan is for install"),
		Entry("platform changed", func(_ *mcpserver.PreparedPlan, _ *time.Time, h *platform.Platform) { h.Version = "26.04" }, "install_apps", "changed since the plan"),
	)

	It("rejects a plan whose application definitions changed", func() {
		session := connect()
		plan := prepare(session, "git")
		viper.Set("applications", []any{map[string]any{"name": "git", "description": "changed", "category": "Dependency", "linux": map[string]any{"install_method": "apt", "install_command": "git-other"}}})

		Expect(errorText(call(session, "install_apps", map[string]any{"plan_id": plan.PlanID, "digest": plan.Digest}))).To(ContainSubstring("changed since the plan"))
		Expect(executor.installed).To(BeEmpty())
	})

	It("reports partial failure with redacted errors and every planned application", func() {
		executor.outcomes = []mcpserver.Outcome{
			{App: "git", Status: "installed"},
			{App: "curl", Status: "failed", Error: "download https://user:hunter2@example.com/x?token=abc failed: password=hunter2\nsecond line"},
		}
		session := connect()
		plan := prepare(session, "git", "curl", "wget")

		var out mcpserver.ExecuteOutput
		decode(call(session, "install_apps", map[string]any{"plan_id": plan.PlanID, "digest": plan.Digest}), &out)
		Expect(out.Result).To(Equal("partial_failure"))
		Expect(out.Outcomes).To(ContainElement(mcpserver.Outcome{App: "wget", Status: "not_run"}))
		data, err := json.Marshal(out)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(data)).NotTo(ContainSubstring("hunter2"))
		Expect(string(data)).NotTo(ContainSubstring("token=abc"))
		Expect(string(data)).NotTo(ContainSubstring("second line"))
	})

	It("stops a confirmed install when the client cancels", func() {
		executor.block = true
		session := connect()
		plan := prepare(session, "git")
		callCtx, cancel := context.WithCancel(ctx)
		go func() {
			defer GinkgoRecover()
			Eventually(func() int { executor.mu.Lock(); defer executor.mu.Unlock(); return len(executor.installed) }).Should(Equal(1))
			cancel()
		}()
		_, err := session.CallTool(callCtx, &mcp.CallToolParams{Name: "install_apps", Arguments: map[string]any{"plan_id": plan.PlanID, "digest": plan.Digest}})
		Expect(err).To(HaveOccurred())
	})

	It("uninstalls the prepared applications and keeps configuration and data by default", func() {
		session := connect()
		var plan mcpserver.PreparedPlan
		decode(call(session, "prepare_uninstall", map[string]any{"apps": []string{"git"}}), &plan)
		Expect(plan.Uninstall).To(ConsistOf(mcpserver.UninstallTarget{Name: "git", Method: "apt", Config: "kept", Data: "kept"}))
		Expect(plan.NotIncluded).To(ContainElement(ContainSubstring("Dependent applications are never removed")))

		var out mcpserver.ExecuteOutput
		decode(call(session, "uninstall_apps", map[string]any{"plan_id": plan.PlanID, "digest": plan.Digest}), &out)
		Expect(out.Result).To(Equal("completed"))
		Expect(executor.uninstalled).To(Equal([][]string{{"git"}}))
		Expect(executor.options).To(Equal([]mcpserver.UninstallOptions{{}}))
	})

	It("removes configuration only when the plan says so", func() {
		session := connect()
		var plan mcpserver.PreparedPlan
		decode(call(session, "prepare_uninstall", map[string]any{"apps": []string{"git"}, "remove_config": true}), &plan)
		Expect(plan.Uninstall[0].Config).To(Equal("removed"))
		var out mcpserver.ExecuteOutput
		decode(call(session, "uninstall_apps", map[string]any{"plan_id": plan.PlanID, "digest": plan.Digest}), &out)
		Expect(executor.options).To(Equal([]mcpserver.UninstallOptions{{RemoveConfig: true}}))
	})

	It("refuses to plan uninstalling untracked applications", func() {
		executor.untracked = true
		Expect(errorText(call(connect(), "prepare_uninstall", map[string]any{"apps": []string{"git"}}))).To(ContainSubstring("not_found: application git is not tracked"))
	})
})

var _ = Describe("Sanitize", func() {
	It("redacts credentials, query strings and secret values and bounds the length", func() {
		Expect(mcpserver.Sanitize("GET https://a:b@host/p?x=1 token: abc")).To(Equal("GET https://[redacted]@host/p?[redacted] token: [redacted]"))
		long := ""
		for range 400 {
			long += "x"
		}
		Expect(len(mcpserver.Sanitize(long))).To(BeNumerically("<=", mcpserver.MaxErrorLength+len("…")))
	})
})
