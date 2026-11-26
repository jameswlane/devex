package agentapi_test

import (
	"context"
	"sort"

	"github.com/jameswlane/devex/apps/cli/internal/agentapi"
	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/types"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/viper"
)

var _ = Describe("Sandbox prerequisite selection expansion", func() {
	var settings config.CrossPlatformSettings

	appNames := func(apps []types.CrossPlatformApp) []string {
		names := make([]string, 0, len(apps))
		for _, app := range apps {
			names = append(names, app.Name)
		}
		return names
	}

	BeforeEach(func() {
		viper.Reset()
		DeferCleanup(viper.Reset)
		DeferCleanup(types.PinCatalogOSForTesting("linux"))

		settings = config.CrossPlatformSettings{
			HomeDir: GinkgoT().TempDir(),
			ApplicationOverrides: []types.CrossPlatformApp{
				{
					Name:     "Bubblewrap",
					Category: "Dependency",
					Linux: types.OSConfig{
						InstallMethod:    "apt",
						InstallCommand:   "bubblewrap",
						UninstallCommand: "bubblewrap",
					},
				},
				{
					Name:     "socat",
					Category: "Dependency",
					Linux: types.OSConfig{
						InstallMethod:    "apt",
						InstallCommand:   "socat",
						UninstallCommand: "socat",
					},
				},
				{
					Name:     "Claude Code",
					Category: "AI Coding Agents",
					Linux: types.OSConfig{
						InstallMethod:        "curlpipe",
						DownloadURL:          "https://claude.ai/install.sh",
						InstallScope:         "user",
						InstalledBinary:      "claude",
						UninstallCommand:     `rm -f "$HOME/.local/bin/claude"`,
						SandboxPrerequisites: []string{"Bubblewrap", "socat"},
					},
					MacOS: types.OSConfig{
						InstallMethod:    "brew",
						InstallCommand:   "claude-code",
						UninstallCommand: "claude-code",
					},
				},
				{
					Name:     "Codex CLI",
					Category: "AI Coding Agents",
					Linux: types.OSConfig{
						InstallMethod:        "mise",
						InstallCommand:       "codex@latest",
						UninstallCommand:     "codex",
						SandboxPrerequisites: []string{"Bubblewrap"},
					},
					MacOS: types.OSConfig{
						InstallMethod:    "brew",
						InstallCommand:   "codex",
						UninstallCommand: "codex",
						// Declared on macOS to verify unsupported prerequisites are skipped.
						SandboxPrerequisites: []string{"Bubblewrap"},
					},
				},
				{
					Name:     "Gemini CLI",
					Category: "AI Coding Agents",
					Linux: types.OSConfig{
						InstallMethod:    "mise",
						InstallCommand:   "node@lts npm:@google/gemini-cli@latest",
						UninstallCommand: "npm:@google/gemini-cli",
					},
				},
			},
		}
	})

	It("prepends sandbox prerequisites before a named app and includes them in JSON plans", func() {
		apps, _, err := agentapi.SelectApplications(context.Background(), settings, agentapi.PlanRequest{
			Names: []string{"Claude Code"},
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(appNames(apps)).To(Equal([]string{"Bubblewrap", "socat", "Claude Code"}))

		plan, err := agentapi.PlanInstall(context.Background(), settings, agentapi.PlanRequest{
			Names: []string{"Claude Code"},
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(plan.DependencyOrder).To(Equal([]string{"Bubblewrap", "socat", "Claude Code"}))

		snapshot, err := agentapi.ResolveExecution(context.Background(), settings, agentapi.PlanRequest{
			Names: []string{"Claude Code"},
		}, agentapi.ExecutionPolicy{}, agentapi.ExecutionTrust{})
		Expect(err).NotTo(HaveOccurred())
		Expect(snapshot.RootNames()).To(Equal([]string{"Bubblewrap", "socat", "Claude Code"}))
	})

	It("deduplicates prerequisites when the user also selected Bubblewrap", func() {
		appsBefore, _, err := agentapi.SelectApplications(context.Background(), settings, agentapi.PlanRequest{
			Names: []string{"Bubblewrap", "Claude Code"},
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(appNames(appsBefore)).To(Equal([]string{"Bubblewrap", "socat", "Claude Code"}))

		appsAfter, _, err := agentapi.SelectApplications(context.Background(), settings, agentapi.PlanRequest{
			Names: []string{"Claude Code", "Bubblewrap"},
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(appNames(appsAfter)).To(Equal([]string{"Bubblewrap", "socat", "Claude Code"}))
	})

	It("expands and deduplicates prerequisites across an AI Coding Agents category selection", func() {
		apps, _, err := agentapi.SelectApplications(context.Background(), settings, agentapi.PlanRequest{
			Categories: []string{"AI Coding Agents"},
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(appNames(apps)).To(Equal([]string{"Bubblewrap", "socat", "Claude Code", "Codex CLI", "Gemini CLI"}))
	})

	It("skips prerequisites not supported on the current platform", func() {
		DeferCleanup(types.PinCatalogOSForTesting("darwin"))

		apps, _, err := agentapi.SelectApplications(context.Background(), settings, agentapi.PlanRequest{
			Names: []string{"Claude Code", "Codex CLI"},
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(appNames(apps)).To(Equal([]string{"Claude Code", "Codex CLI"}))
	})

	It("keeps the same roots and order when the sorted expanded names are resolved again", func() {
		// MCP confirmation sorts the expanded root names and resolves them again.
		settings.ApplicationOverrides = append(settings.ApplicationOverrides, types.CrossPlatformApp{
			Name:     "Build Essential",
			Category: "Dependency",
			Linux:    types.OSConfig{InstallMethod: "apt", InstallCommand: "build-essential", UninstallCommand: "build-essential"},
		})
		first, _, err := agentapi.SelectApplications(context.Background(), settings, agentapi.PlanRequest{
			Names: []string{"Build Essential", "Claude Code"},
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(appNames(first)).To(Equal([]string{"Bubblewrap", "socat", "Build Essential", "Claude Code"}))

		replayed := append([]string(nil), appNames(first)...)
		sort.Strings(replayed)
		again, _, err := agentapi.SelectApplications(context.Background(), settings, agentapi.PlanRequest{Names: replayed})
		Expect(err).NotTo(HaveOccurred())
		Expect(appNames(again)).To(Equal(appNames(first)))
	})

	It("does not add prerequisites for requested-only selections such as update", func() {
		apps, _, err := agentapi.SelectRequestedApplications(context.Background(), settings, agentapi.PlanRequest{
			Names: []string{"Claude Code"},
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(appNames(apps)).To(Equal([]string{"Claude Code"}))
	})
})
