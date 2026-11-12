package mcpserver_test

import (
	"context"
	"encoding/json"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/viper"

	"github.com/jameswlane/devex/apps/cli/internal/catalog"
	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/mcpserver"
	"github.com/jameswlane/devex/apps/cli/internal/platform"
)

var _ = Describe("Read-only MCP server", func() {
	var (
		ctx      context.Context
		settings config.CrossPlatformSettings
		options  mcpserver.Options
	)

	BeforeEach(func() {
		ctx = context.Background()
		viper.Reset()
		DeferCleanup(viper.Reset)
		settings = config.CrossPlatformSettings{HomeDir: GinkgoT().TempDir(), Catalog: &catalog.Result{}}
		Expect(config.LoadCatalogOffline(ctx, settings)).To(Succeed())
		options = mcpserver.Options{
			Version:  "test",
			Settings: settings,
			DetectPlatform: func(context.Context) (*platform.Platform, error) {
				return &platform.Platform{OS: "linux", Distribution: "ubuntu", Architecture: "amd64", PackageManagers: []string{"apt"}}, nil
			},
		}
	})

	connect := func() *mcp.ClientSession {
		server := mcpserver.New(options)
		serverTransport, clientTransport := mcp.NewInMemoryTransports()
		serverSession, err := server.Connect(ctx, serverTransport, nil)
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(serverSession.Close)
		client := mcp.NewClient(&mcp.Implementation{Name: "test-client", Version: "0"}, nil)
		session, err := client.Connect(ctx, clientTransport, nil)
		Expect(err).NotTo(HaveOccurred())
		DeferCleanup(session.Close)
		return session
	}

	call := func(session *mcp.ClientSession, name string, args any) (*mcp.CallToolResult, error) {
		return session.CallTool(ctx, &mcp.CallToolParams{Name: name, Arguments: args})
	}

	structured := func(result *mcp.CallToolResult, target any) {
		Expect(result.IsError).To(BeFalse(), "tool error: %v", result.Content)
		data, err := json.Marshal(result.StructuredContent)
		Expect(err).NotTo(HaveOccurred())
		Expect(json.Unmarshal(data, target)).To(Succeed())
	}

	toolErrorText := func(result *mcp.CallToolResult) string {
		Expect(result.IsError).To(BeTrue())
		Expect(result.Content).NotTo(BeEmpty())
		text, ok := result.Content[0].(*mcp.TextContent)
		Expect(ok).To(BeTrue())
		return text.Text
	}

	It("advertises only read-only tools with schemas and the effective configuration resource", func() {
		session := connect()

		tools, err := session.ListTools(ctx, nil)
		Expect(err).NotTo(HaveOccurred())
		names := make([]string, 0, len(tools.Tools))
		for _, tool := range tools.Tools {
			names = append(names, tool.Name)
			Expect(tool.Annotations).NotTo(BeNil())
			Expect(tool.Annotations.ReadOnlyHint).To(BeTrue(), tool.Name)
			Expect(tool.InputSchema).NotTo(BeNil(), tool.Name)
			Expect(tool.OutputSchema).NotTo(BeNil(), tool.Name)
		}
		Expect(names).To(ConsistOf(mcpserver.ReadOnlyTools))

		resources, err := session.ListResources(ctx, nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(resources.Resources).To(HaveLen(1))
		Expect(resources.Resources[0].URI).To(Equal(mcpserver.EffectiveConfigURI))
		Expect(resources.Resources[0].MIMEType).To(Equal("application/json"))
	})

	DescribeTable("rejects mutating tools by name",
		func(name string) {
			_, err := call(connect(), name, map[string]any{"apps": []string{"git"}})
			Expect(err).To(MatchError(ContainSubstring("unknown tool")))
		},
		Entry("install_apps", "install_apps"),
		Entry("uninstall_apps", "uninstall_apps"),
		Entry("run_plugin", "run_plugin"),
	)

	It("lists bundled applications with a search filter, a cap and truncation", func() {
		session := connect()

		result, err := call(session, "list_apps", map[string]any{"search": "git", "limit": 1})
		Expect(err).NotTo(HaveOccurred())
		var out mcpserver.ListAppsOutput
		structured(result, &out)

		Expect(out.Kind).To(Equal("available"))
		Expect(out.Available).To(HaveLen(1))
		Expect(out.Total).To(BeNumerically(">", 1))
		Expect(out.Truncated).To(BeTrue())
	})

	It("lists categories", func() {
		result, err := call(connect(), "list_apps", map[string]any{"kind": "categories"})
		Expect(err).NotTo(HaveOccurred())
		var out mcpserver.ListAppsOutput
		structured(result, &out)
		Expect(out.Categories).NotTo(BeEmpty())
		Expect(out.Available).To(BeEmpty())
	})

	DescribeTable("rejects invalid list requests before doing any work",
		func(args map[string]any, message string) {
			result, err := call(connect(), "list_apps", args)
			Expect(err).NotTo(HaveOccurred())
			Expect(toolErrorText(result)).To(ContainSubstring(message))
		},
		Entry("unknown kind", map[string]any{"kind": "everything"}, "invalid_request: kind must be"),
		Entry("limit too large", map[string]any{"limit": mcpserver.MaxListLimit + 1}, "invalid_request: limit must be"),
		Entry("negative limit", map[string]any{"limit": -1}, "invalid_request: limit must be"),
		Entry("oversized search", map[string]any{"search": strings.Repeat("a", mcpserver.MaxTextLength+1)}, "invalid_request: search must be"),
	)

	It("rejects arguments of the wrong type through the input schema", func() {
		result, err := call(connect(), "list_apps", map[string]any{"limit": "ten"})
		// The SDK reports schema validation failures as a protocol error or a
		// tool error result depending on where validation fails.
		if err == nil {
			Expect(result.IsError).To(BeTrue())
		}
	})

	It("detects the platform", func() {
		result, err := call(connect(), "detect_platform", map[string]any{})
		Expect(err).NotTo(HaveOccurred())
		var out platform.Platform
		structured(result, &out)
		Expect(out.Distribution).To(Equal("ubuntu"))
		Expect(out.PackageManagers).To(ConsistOf("apt"))
	})

	It("reports an empty status for an empty home without probing the system", func() {
		result, err := call(connect(), "get_status", map[string]any{})
		Expect(err).NotTo(HaveOccurred())
		var out mcpserver.StatusOutput
		structured(result, &out)
		Expect(out.Apps).To(BeEmpty())
	})

	It("reports status for a known application as unknown instead of probing it", func() {
		result, err := call(connect(), "get_status", map[string]any{"apps": []string{"git"}})
		Expect(err).NotTo(HaveOccurred())
		var out mcpserver.StatusOutput
		structured(result, &out)
		Expect(out.Apps).To(HaveLen(1))
		Expect(out.Apps[0].Status).To(Equal("unknown"))
	})

	It("returns a categorized error for unknown applications", func() {
		result, err := call(connect(), "get_status", map[string]any{"apps": []string{"not-a-devex-application"}})
		Expect(err).NotTo(HaveOccurred())
		Expect(toolErrorText(result)).To(HavePrefix("not_found:"))
	})

	It("rejects oversized name lists", func() {
		names := make([]string, mcpserver.MaxNames+1)
		for i := range names {
			names[i] = "git"
		}
		result, err := call(connect(), "plan_install", map[string]any{"apps": names})
		Expect(err).NotTo(HaveOccurred())
		Expect(toolErrorText(result)).To(ContainSubstring("invalid_request: apps may name at most"))
	})

	It("validates the configuration", func() {
		result, err := call(connect(), "validate_config", map[string]any{"strict": true})
		Expect(err).NotTo(HaveOccurred())
		Expect(result.IsError).To(BeFalse())
		Expect(result.StructuredContent).To(HaveKeyWithValue("valid", true))
	})

	It("plans an install without executing anything", func() {
		result, err := call(connect(), "plan_install", map[string]any{"apps": []string{"git"}})
		Expect(err).NotTo(HaveOccurred())
		var plan struct {
			Apps []struct {
				Name   string `json:"name"`
				Method string `json:"method"`
			} `json:"apps"`
			DependencyOrder []string `json:"dependency_order"`
		}
		structured(result, &plan)
		Expect(plan.Apps).To(HaveLen(1))
		Expect(plan.Apps[0].Name).To(Equal("git"))
		Expect(plan.DependencyOrder).To(Equal([]string{"git"}))
	})

	It("serves the effective configuration without scripts, commands or secrets", func() {
		viper.Set("applications", []any{map[string]any{
			"name":        "canary-app",
			"description": "A test application",
			"category":    "Testing",
			"default":     true,
			"linux": map[string]any{
				"install_method":    "curlpipe",
				"install_command":   "https://example.invalid/install.sh?token=SECRET_CANARY_VALUE",
				"uninstall_command": "rm -rf SECRET_CANARY_VALUE",
			},
		}})
		session := connect()

		resource, err := session.ReadResource(ctx, &mcp.ReadResourceParams{URI: mcpserver.EffectiveConfigURI})
		Expect(err).NotTo(HaveOccurred())
		Expect(resource.Contents).To(HaveLen(1))
		Expect(resource.Contents[0].MIMEType).To(Equal("application/json"))
		Expect(resource.Contents[0].Text).To(ContainSubstring("canary-app"))
		Expect(resource.Contents[0].Text).NotTo(ContainSubstring("SECRET_CANARY_VALUE"))

		result, err := call(session, "list_apps", map[string]any{"search": "canary"})
		Expect(err).NotTo(HaveOccurred())
		data, err := json.Marshal(result)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(data)).NotTo(ContainSubstring("SECRET_CANARY_VALUE"))
	})

	It("refuses resources outside the effective configuration", func() {
		session := connect()
		for _, uri := range []string{"file:///etc/passwd", "devex://config/raw", "devex://config/effective/../raw"} {
			_, err := session.ReadResource(ctx, &mcp.ReadResourceParams{URI: uri})
			Expect(err).To(HaveOccurred(), uri)
		}
	})

	It("ends a request at its deadline", func() {
		options.RequestTimeout = 20 * time.Millisecond
		options.DetectPlatform = func(ctx context.Context) (*platform.Platform, error) {
			<-ctx.Done()
			return nil, ctx.Err()
		}

		result, err := call(connect(), "detect_platform", map[string]any{})

		Expect(err).NotTo(HaveOccurred())
		Expect(toolErrorText(result)).To(HavePrefix("timeout:"))
	})

	It("stops a request when the client cancels it", func() {
		started := make(chan struct{})
		options.DetectPlatform = func(ctx context.Context) (*platform.Platform, error) {
			close(started)
			<-ctx.Done()
			return nil, ctx.Err()
		}
		session := connect()
		callCtx, cancel := context.WithCancel(ctx)
		go func() {
			<-started
			cancel()
		}()

		_, err := session.CallTool(callCtx, &mcp.CallToolParams{Name: "detect_platform", Arguments: map[string]any{}})

		Expect(err).To(HaveOccurred())
	})

	Context("when a request is queued behind a slow request", func() {
		var (
			release chan struct{}
			started chan struct{}
			calls   atomic.Int32
		)

		BeforeEach(func() {
			release = make(chan struct{})
			started = make(chan struct{})
			calls.Store(0)
			options.DetectPlatform = func(context.Context) (*platform.Platform, error) {
				if calls.Add(1) == 1 {
					close(started)
					// The first request ignores its deadline and holds the
					// server until the spec releases it.
					<-release
				}
				return &platform.Platform{OS: "linux"}, nil
			}
		})

		holdFirst := func(session *mcp.ClientSession) chan struct{} {
			finished := make(chan struct{})
			go func() {
				defer GinkgoRecover()
				defer close(finished)
				_, _ = call(session, "detect_platform", map[string]any{})
			}()
			Eventually(started).Should(BeClosed())
			DeferCleanup(func() {
				select {
				case <-release:
				default:
					close(release)
				}
				Eventually(finished).Should(BeClosed())
			})
			return finished
		}

		It("times out while queued instead of waiting for the slot", func() {
			options.RequestTimeout = 50 * time.Millisecond
			session := connect()
			holdFirst(session)

			queued := make(chan *mcp.CallToolResult, 1)
			go func() {
				defer GinkgoRecover()
				result, err := call(session, "detect_platform", map[string]any{})
				Expect(err).NotTo(HaveOccurred())
				queued <- result
			}()

			var result *mcp.CallToolResult
			Eventually(queued, 2*time.Second).Should(Receive(&result), "the queued request waited for the slow request")
			Expect(toolErrorText(result)).To(HavePrefix("timeout:"))
			Expect(calls.Load()).To(Equal(int32(1)), "the queued operation ran")
		})

		It("does not run a request the client cancelled while it was queued", func() {
			options.RequestTimeout = time.Minute
			session := connect()
			holdFirst(session)

			callCtx, cancel := context.WithCancel(ctx)
			queued := make(chan error, 1)
			go func() {
				_, err := session.CallTool(callCtx, &mcp.CallToolParams{Name: "detect_platform", Arguments: map[string]any{}})
				queued <- err
			}()
			// Let the queued request reach the server before cancelling it.
			time.Sleep(100 * time.Millisecond)
			cancel()
			Eventually(queued).Should(Receive(HaveOccurred()))
			// Give the server time to deliver the cancellation, then free the slot.
			time.Sleep(200 * time.Millisecond)
			close(release)

			Consistently(calls.Load, 300*time.Millisecond).Should(Equal(int32(1)), "the cancelled request ran after the slot was freed")
			result, err := call(session, "detect_platform", map[string]any{})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.IsError).To(BeFalse())
		})
	})

	It("serializes concurrent requests", func() {
		var active, peak atomic.Int32
		options.DetectPlatform = func(context.Context) (*platform.Platform, error) {
			now := active.Add(1)
			for {
				old := peak.Load()
				if now <= old || peak.CompareAndSwap(old, now) {
					break
				}
			}
			time.Sleep(10 * time.Millisecond)
			active.Add(-1)
			return &platform.Platform{OS: "linux"}, nil
		}
		session := connect()

		var wg sync.WaitGroup
		for range 5 {
			wg.Add(1)
			go func() {
				defer GinkgoRecover()
				defer wg.Done()
				result, err := call(session, "detect_platform", map[string]any{})
				Expect(err).NotTo(HaveOccurred())
				Expect(result.IsError).To(BeFalse())
			}()
		}
		wg.Wait()

		Expect(peak.Load()).To(Equal(int32(1)))
	})
})
