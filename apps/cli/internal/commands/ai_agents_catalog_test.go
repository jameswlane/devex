package commands_test

import (
	"bytes"
	"context"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	shipconfig "github.com/jameswlane/devex/apps/cli/config"
	"github.com/jameswlane/devex/apps/cli/internal/agentapi"
	"github.com/jameswlane/devex/apps/cli/internal/commands"
	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/installers"
	"github.com/jameswlane/devex/apps/cli/internal/mocks"
	"github.com/jameswlane/devex/apps/cli/internal/platform"
	"github.com/jameswlane/devex/apps/cli/internal/testhelper"
	"github.com/jameswlane/devex/apps/cli/internal/types"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/viper"
	"gopkg.in/yaml.v3"
)

var _ = Describe("AI Coding Agents catalog", func() {
	var (
		homeDir  string
		settings config.CrossPlatformSettings
		agents   map[string]types.CrossPlatformApp
		files    []string
	)

	BeforeEach(func() {
		viper.Reset()
		DeferCleanup(viper.Reset)
		DeferCleanup(types.PinCatalogOSForTesting("linux"))
		DeferCleanup(types.PinCatalogArchForTesting("amd64"))

		homeDir = GinkgoT().TempDir()
		testhelper.SetProfileEnv(GinkgoT(), homeDir)
		GinkgoT().Setenv("DEVEX_TEAM_CONFIG_DIR", "")
		GinkgoT().Setenv("DEVEX_ENV", "dev")

		settings = config.CrossPlatformSettings{HomeDir: homeDir}
		Expect(config.WriteDefaultConfig(settings.GetConfigDir())).To(Succeed())

		loaded, err := config.LoadCrossPlatformSettings(homeDir)
		Expect(err).NotTo(HaveOccurred())
		settings = loaded

		agents = map[string]types.CrossPlatformApp{}
		files = nil
		Expect(fs.WalkDir(shipconfig.Files, "applications/ai-agents", func(path string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil || entry.IsDir() || filepath.Ext(path) != ".yaml" {
				return walkErr
			}
			data, readErr := fs.ReadFile(shipconfig.Files, path)
			if readErr != nil {
				return readErr
			}
			var app types.CrossPlatformApp
			if unmarshalErr := yaml.Unmarshal(data, &app); unmarshalErr != nil {
				return unmarshalErr
			}
			if app.Category != "AI Coding Agents" && app.Category != "Local Models" {
				return nil
			}
			agents[app.Name] = app
			files = append(files, filepath.Base(path))
			return nil
		})).To(Succeed())
		sort.Strings(files)
	})

	It("leaves grok and agent links the Cursor CLI owns when uninstalling Grok Build", func() {
		if runtime.GOOS == "windows" {
			Skip("the Linux uninstall command runs under sh")
		}
		home := GinkgoT().TempDir()
		mk := func(dir string) string {
			path := filepath.Join(home, dir)
			Expect(os.MkdirAll(path, 0o755)).To(Succeed())
			return path
		}
		localBin, grokBin, downloads, cursor := mk(".local/bin"), mk(".grok/bin"), mk(".grok/downloads"), mk(".cursor/bin")
		Expect(os.WriteFile(filepath.Join(downloads, "grok-1.0.0"), []byte("#!/bin/sh\n"), 0o755)).To(Succeed())
		Expect(os.Symlink("../downloads/grok-1.0.0", filepath.Join(grokBin, "grok"))).To(Succeed())
		Expect(os.Symlink(filepath.Join(grokBin, "grok"), filepath.Join(localBin, "grok"))).To(Succeed())
		// The Cursor CLI switched agent to its own shim.
		Expect(os.Symlink(filepath.Join(cursor, "agent"), filepath.Join(grokBin, "agent"))).To(Succeed())
		Expect(os.Symlink(filepath.Join(cursor, "agent"), filepath.Join(localBin, "agent"))).To(Succeed())

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		DeferCleanup(cancel)
		cmd := exec.CommandContext(ctx, "sh", "-c", agents["Grok Build"].Linux.UninstallCommand)
		cmd.Env = []string{"HOME=" + home, "PATH=/usr/bin:/bin"}
		output, err := cmd.CombinedOutput()
		Expect(err).NotTo(HaveOccurred(), string(output))

		_, err = os.Lstat(filepath.Join(localBin, "grok"))
		Expect(os.IsNotExist(err)).To(BeTrue())
		_, err = os.Lstat(filepath.Join(grokBin, "grok"))
		Expect(os.IsNotExist(err)).To(BeTrue())
		target, err := os.Readlink(filepath.Join(localBin, "agent"))
		Expect(err).NotTo(HaveOccurred())
		Expect(target).To(Equal(filepath.Join(cursor, "agent")))
		target, err = os.Readlink(filepath.Join(grokBin, "agent"))
		Expect(err).NotTo(HaveOccurred())
		Expect(target).To(Equal(filepath.Join(cursor, "agent")))
	})

	It("fails Grok Build's uninstall when one of its own links cannot be removed", func() {
		if runtime.GOOS == "windows" || os.Geteuid() == 0 {
			Skip("needs a non-root sh to make a directory unwritable")
		}
		home := GinkgoT().TempDir()
		localBin, grokBin := filepath.Join(home, ".local", "bin"), filepath.Join(home, ".grok", "bin")
		Expect(os.MkdirAll(localBin, 0o755)).To(Succeed())
		Expect(os.MkdirAll(grokBin, 0o755)).To(Succeed())
		Expect(os.Symlink(filepath.Join(grokBin, "grok"), filepath.Join(localBin, "grok"))).To(Succeed())
		Expect(os.Chmod(localBin, 0o555)).To(Succeed())
		DeferCleanup(os.Chmod, localBin, os.FileMode(0o755))

		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		DeferCleanup(cancel)
		cmd := exec.CommandContext(ctx, "sh", "-c", agents["Grok Build"].Linux.UninstallCommand)
		cmd.Env = []string{"HOME=" + home, "PATH=/usr/bin:/bin"}
		Expect(cmd.Run()).To(HaveOccurred())
	})

	Describe("uninstall commands run against a temporary home", func() {
		var home string
		BeforeEach(func() {
			if runtime.GOOS == "windows" || os.Geteuid() == 0 {
				Skip("the Linux uninstall commands run under a non-root sh")
			}
			home = GinkgoT().TempDir()
		})
		run := func(name string) error {
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			DeferCleanup(cancel)
			cmd := exec.CommandContext(ctx, "sh", "-c", agents[name].Linux.UninstallCommand)
			cmd.Env = []string{"HOME=" + home, "PATH=/usr/bin:/bin", "DEVEX_LOCK_WAIT_SECONDS=1"}
			out, err := cmd.CombinedOutput()
			GinkgoWriter.Println(string(out))
			return err
		}
		touch := func(rel string) string {
			path := filepath.Join(home, rel)
			Expect(os.MkdirAll(filepath.Dir(path), 0o755)).To(Succeed())
			Expect(os.WriteFile(path, []byte("x"), 0o755)).To(Succeed())
			return path
		}
		gone := func(path string) bool {
			_, err := os.Lstat(path)
			return os.IsNotExist(err)
		}

		It("removes Muse Code's launcher state once it holds the update lock", func() {
			files := []string{touch(".local/bin/muse"), touch(".local/bin/muse-bin-1.0"), touch(".local/bin/.muse-version"), touch(".local/bin/.muse-release-info.json")}
			Expect(run("Muse Code")).To(Succeed())
			for _, f := range files {
				Expect(gone(f)).To(BeTrue(), f)
			}
			Expect(gone(filepath.Join(home, ".local/bin/.muse-update-lock"))).To(BeTrue())
		})

		It("fails Muse Code's uninstall and keeps its files while an update holds the lock", func() {
			muse := touch(".local/bin/muse")
			touch(".local/bin/.muse-update-lock/pid")
			Expect(run("Muse Code")).To(HaveOccurred())
			Expect(gone(muse)).To(BeFalse())
		})

		It("treats XDG_BIN_DIR naming ~/bin as one Harn directory", func() {
			harn := touch("bin/harn")
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			DeferCleanup(cancel)
			cmd := exec.CommandContext(ctx, "sh", "-c", agents["Harn"].Linux.UninstallCommand)
			cmd.Env = []string{"HOME=" + home, "PATH=/usr/bin:/bin", "XDG_BIN_DIR=" + filepath.Join(home, "bin")}
			Expect(cmd.Run()).To(Succeed())
			Expect(gone(harn)).To(BeTrue())
		})

		It("tells Harn copies apart behind a symlink followed by ..", func() {
			one := touch("bin/harn")
			Expect(os.MkdirAll(filepath.Join(home, "real", "inner"), 0o755)).To(Succeed())
			Expect(os.Symlink(filepath.Join(home, "real", "inner"), filepath.Join(home, "bin", "link"))).To(Succeed())
			two := touch("real/harn")
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			DeferCleanup(cancel)
			cmd := exec.CommandContext(ctx, "sh", "-c", agents["Harn"].Linux.UninstallCommand)
			// $HOME/bin/link/.. is $HOME/bin logically but $HOME/real physically.
			cmd.Env = []string{"HOME=" + home, "PATH=/usr/bin:/bin", "XDG_BIN_DIR=" + filepath.Join(home, "bin", "link") + "/.."}
			output, err := cmd.CombinedOutput()
			Expect(err).To(HaveOccurred())
			Expect(string(output)).To(ContainSubstring("Harn is in both"))
			Expect(gone(one)).To(BeFalse())
			Expect(gone(two)).To(BeFalse())
		})

		It("succeeds for Muse Code when ~/.local/bin does not exist", func() {
			Expect(run("Muse Code")).To(Succeed())
		})

		It("leaves a lock held by a live updater untouched", func() {
			muse := touch(".local/bin/muse")
			lock := filepath.Join(home, ".local/bin/.muse-update-lock")
			Expect(os.MkdirAll(lock, 0o755)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(lock, "pid"), []byte("1\n"), 0o644)).To(Succeed())
			Expect(run("Muse Code")).To(HaveOccurred())
			Expect(gone(muse)).To(BeFalse())
			pid, err := os.ReadFile(filepath.Join(lock, "pid"))
			Expect(err).NotTo(HaveOccurred())
			Expect(string(pid)).To(Equal("1\n"))
			entries, err := os.ReadDir(lock)
			Expect(err).NotTo(HaveOccurred())
			Expect(entries).To(HaveLen(1))
		})

		It("removes an unrecorded Harn install from the one directory holding it", func() {
			harn := touch("bin/harn")
			Expect(run("Harn")).To(Succeed())
			Expect(gone(harn)).To(BeTrue())
		})

		It("stops when an unrecorded Harn is in two directories", func() {
			one, two := touch("bin/harn"), touch(".local/bin/harn")
			Expect(run("Harn")).To(HaveOccurred())
			Expect(gone(one)).To(BeFalse())
			Expect(gone(two)).To(BeFalse())
		})

		It("removes a recorded Harn install only from its directory", func() {
			other, mine := touch("bin/harn"), touch(".local/bin/harn")
			Expect(os.MkdirAll(filepath.Join(home, ".harn"), 0o755)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(home, ".harn/devex-install-dir"), []byte(filepath.Join(home, ".local/bin")+"\n"), 0o644)).To(Succeed())
			Expect(run("Harn")).To(Succeed())
			Expect(gone(mine)).To(BeTrue())
			Expect(gone(other)).To(BeFalse())
		})
	})

	It("keeps Codex CLI's Bubblewrap sandbox prerequisite on its mise channel", func() {
		DeferCleanup(types.PinCatalogPlatformForTesting(platform.DetectionResult{
			OS: "linux", Distribution: "ubuntu", DistributionLike: []string{"debian"}, Version: "24.04", Architecture: "amd64",
		}))
		apps, _, err := agentapi.SelectApplications(context.Background(), settings, agentapi.PlanRequest{Names: []string{"Codex CLI"}, Channel: "mise"})
		Expect(err).NotTo(HaveOccurred())
		names := make([]string, 0, len(apps))
		for _, app := range apps {
			names = append(names, app.Name)
		}
		Expect(names).To(Equal([]string{"Bubblewrap", "Codex CLI"}))
		Expect(apps[1].SelectedChannel()).To(Equal("mise"))
	})

	It("ships and strictly validates every AI coding agent definition", func() {
		Expect(files).To(Equal([]string{
			"aider.yaml",
			"amp.yaml",
			"auggie-cli.yaml",
			"autohand-code.yaml",
			"bob.yaml",
			"claude-code.yaml",
			"cline.yaml",
			"codebuddy-code.yaml",
			"codex.yaml",
			"copilot-cli.yaml",
			"cortex-code.yaml",
			"crow-cli.yaml",
			"cursor-cli.yaml",
			"deepagents.yaml",
			"devin.yaml",
			"dimcode.yaml",
			"dirac.yaml",
			"factory-droid.yaml",
			"fast-agent.yaml",
			"fx.yaml",
			"gemini-cli.yaml",
			"goose.yaml",
			"grok-build.yaml",
			"harn.yaml",
			"junie.yaml",
			"kilo.yaml",
			"kimchi.yaml",
			"kimi-code.yaml",
			"kiro-cli.yaml",
			"lm-studio.yaml",
			"minimax-code.yaml",
			"minion-code.yaml",
			"mistral-vibe.yaml",
			"muse-code.yaml",
			"nova.yaml",
			"ollama.yaml",
			"opencode.yaml",
			"pi.yaml",
			"poolside.yaml",
			"qoder-cli.yaml",
			"qwen-code.yaml",
			"sigit-code.yaml",
			"stakpak.yaml",
			"vt-code.yaml",
		}))

		result, err := config.ValidateConfig(context.Background(), settings, true)
		Expect(err).NotTo(HaveOccurred())
		Expect(result.Valid).To(BeTrue(), "diagnostics: %+v", result.Diagnostics)
		Expect(result.Warnings).To(BeZero())

		for name, app := range agents {
			Expect(app.Default).To(BeFalse(), "%s must opt in with default: false", name)
			wantCategory := "AI Coding Agents"
			if name == "Ollama" || name == "LM Studio" {
				// Grouped under setup's "Local models" heading, not agent CLIs.
				wantCategory = "Local Models"
			}
			Expect(app.Category).To(Equal(wantCategory), name)
			Expect(app.Linux.InstallMethod).NotTo(BeEmpty(), "%s must define a Linux install method", name)
			Expect(app.Linux.OfficialSupport).To(BeTrue(), "%s Linux primary must have official_support: true", name)
			Expect(app.Linux.CleanupFiles).To(BeEmpty(), "%s must not list user data in cleanup_files", name)
			Expect(app.Linux.ConfigFiles).To(BeEmpty(), "%s must not list user config in config_files", name)

			checkCurlpipe := func(label string, cfg types.OSConfig) {
				if cfg.InstallMethod != "curlpipe" {
					return
				}
				if cfg.DownloadURL == "" && (len(cfg.PreInstall) > 0 || len(cfg.PostInstall) > 0) {
					Expect(cfg.InstallScope).To(BeEmpty(), "%s (%s) hook-driven system curlpipe must not set install_scope", name, label)
				} else {
					Expect(cfg.InstallScope).To(Equal("user"), "%s (%s) curlpipe must use install_scope: user", name, label)
					Expect(cfg.DownloadURL).To(HavePrefix("https://"), "%s (%s) curlpipe must use HTTPS download_url", name, label)
				}
				Expect(cfg.InstalledBinary).NotTo(BeEmpty(), "%s (%s) curlpipe must declare installed_binary", name, label)
				Expect(cfg.UninstallCommand).NotTo(BeEmpty(), "%s (%s) curlpipe must declare uninstall_command", name, label)
			}
			checkUniqueMethods := func(label string, cfg types.OSConfig) {
				if cfg.InstallMethod == "" {
					return
				}
				seen := map[string]bool{cfg.InstallMethod: true}
				for _, alt := range cfg.Alternatives {
					Expect(seen[alt.InstallMethod]).To(BeFalse(), "%s (%s) has duplicate install_method %q across primary and alternatives", name, label, alt.InstallMethod)
					seen[alt.InstallMethod] = true
				}
			}
			checkCurlpipe("linux", app.Linux)
			checkCurlpipe("macos", app.MacOS)
			checkCurlpipe("windows", app.Windows)
			checkUniqueMethods("linux", app.Linux)
			checkUniqueMethods("macos", app.MacOS)
			checkUniqueMethods("windows", app.Windows)

			for label, cfg := range map[string]types.OSConfig{
				"linux":   app.Linux,
				"macos":   app.MacOS,
				"windows": app.Windows,
			} {
				if cfg.InstallMethod == "" {
					continue
				}
				seen := map[string]bool{cfg.InstallMethod: true}
				for _, alt := range cfg.Alternatives {
					checkCurlpipe(label+"-alt", alt)
					if alt.InstallMethod == "curlpipe" && installers.IsReleaseArchive(alt.DownloadURL) && len(alt.PlatformRequirements) > 0 {
						continue
					}
					Expect(seen[alt.InstallMethod]).To(BeFalse(), "%s (%s) duplicate channel %q", name, label, alt.InstallMethod)
					seen[alt.InstallMethod] = true
				}
			}

			legacy := app.ToLegacyAppConfig()
			Expect(legacy.SelfUpdating).To(Equal(app.Linux.SelfUpdating), name)
			Expect(legacy.InstallScope).To(Equal(app.Linux.InstallScope), name)
		}
	})

	DescribeTable("resolves each AI coding agent install plan on Ubuntu 24.04",
		func(name, wantMethod, wantTarget, wantScope string, wantSelfUpdating bool, wantDeps, wantSandboxPrereqs []string, wantUninstall string) {
			app, ok := agents[name]
			Expect(ok).To(BeTrue(), "expected %s in ai-agents catalog", name)

			ubuntu := platform.DetectionResult{
				OS:               "linux",
				Distribution:     "ubuntu",
				DistributionLike: []string{"debian"},
				Version:          "24.04",
				Architecture:     "amd64",
			}
			resolved := app.LinuxConfigForPlatform(ubuntu)
			Expect(resolved.InstallMethod).To(Equal(wantMethod))
			Expect(resolved.InstallScope).To(Equal(wantScope))
			Expect(resolved.SelfUpdating).To(Equal(wantSelfUpdating))
			Expect(resolved.UninstallCommand).To(Equal(wantUninstall))
			Expect(resolved.SandboxPrerequisites).To(Equal(wantSandboxPrereqs))

			switch wantMethod {
			case "curlpipe":
				Expect(resolved.DownloadURL).To(Equal(wantTarget))
				if wantTarget != "" {
					Expect(resolved.InstallScope).To(Equal("user"))
				}
			case "deb":
				Expect(resolved.DownloadURL).To(Equal(wantTarget))
			case "mise":
				Expect(resolved.InstallCommand).To(Equal(wantTarget))
				packages, err := installers.CatalogPackageArguments("mise", resolved.InstallCommand)
				Expect(err).NotTo(HaveOccurred())
				Expect(packages).To(Equal(strings.Fields(wantTarget)))
				uninstallArgs, err := installers.CatalogPackageArguments("mise", resolved.UninstallCommand)
				Expect(err).NotTo(HaveOccurred())
				Expect(uninstallArgs).To(Equal([]string{wantUninstall}))
			}

			plan, err := agentapi.PlanInstall(context.Background(), settings, agentapi.PlanRequest{Names: []string{name}})
			Expect(err).NotTo(HaveOccurred())
			Expect(plan.DependencyOrder).To(ContainElement(name))
			for _, prereq := range wantSandboxPrereqs {
				Expect(plan.DependencyOrder).To(ContainElement(prereq))
			}
			for _, dep := range wantDeps {
				if strings.EqualFold(dep, "Node.js") {
					Expect(plan.DependencyOrder).To(Equal([]string{dep, name}))
				}
			}

			var planned *agentapi.PlannedApp
			for i := range plan.Apps {
				if plan.Apps[i].Name == name {
					planned = &plan.Apps[i]
					break
				}
			}
			Expect(planned).NotTo(BeNil())
			Expect(planned.Method).To(Equal(wantMethod))
			Expect(planned.Plugin).To(Equal("package-manager-" + wantMethod))
			if len(wantDeps) > 0 {
				Expect(planned.Dependencies).To(Equal(wantDeps))
			}

			var out bytes.Buffer
			cmd := commands.NewRootCmd("test", mocks.NewMockRepository(), settings)
			cmd.SetOut(&out)
			cmd.SetErr(&out)
			cmd.SetArgs([]string{"install", "--dry-run", name})
			Expect(cmd.Execute()).To(Succeed())
			for _, prereq := range wantSandboxPrereqs {
				Expect(out.String()).To(ContainSubstring("Would install " + prereq))
			}
			Expect(out.String()).To(ContainSubstring("Would install " + name))
		},
		Entry("Claude Code", "Claude Code", "curlpipe", "https://claude.ai/install.sh", "user", true, nil, []string{"Bubblewrap", "socat"}, `rm -f "$HOME/.local/bin/claude" && rm -rf "$HOME/.local/share/claude"`),
		Entry("Codex CLI", "Codex CLI", "curlpipe", "https://chatgpt.com/codex/install.sh", "user", true, nil, []string{"Bubblewrap"}, `rm -f "$HOME/.local/bin/codex" && rm -rf "$HOME/.codex/packages/standalone"`),
		Entry("Gemini CLI", "Gemini CLI", "mise", "node@24 npm:@google/gemini-cli@latest", "", true, []string{"Node.js"}, nil, "npm:@google/gemini-cli"),
		Entry("GitHub Copilot CLI", "GitHub Copilot CLI", "curlpipe", "https://raw.githubusercontent.com/github/copilot-cli/main/install.sh", "user", true, nil, []string{"Bubblewrap"}, `rm -f "$HOME/.local/bin/copilot"`),
		Entry("OpenCode", "OpenCode", "curlpipe", "https://opencode.ai/install", "user", true, nil, nil, `rm -f "$HOME/.opencode/bin/opencode"`),
		Entry("Kimi Code", "Kimi Code", "curlpipe", "https://code.kimi.com/kimi-code/install.sh", "user", false, nil, nil, `rm -f "$HOME/.kimi-code/bin/kimi" "$HOME/.kimi-code/bin/kimi.bak"`),
		Entry("Aider", "Aider", "mise", "uv@latest pipx:aider-chat[python=3.12]@latest", "", false, []string{"mise"}, nil, "pipx:aider-chat"),
		Entry("fast-agent", "fast-agent", "mise", "uv@latest pipx:fast-agent-mcp[python=3.12]@latest", "", false, []string{"mise"}, nil, "pipx:fast-agent-mcp"),
		Entry("Minion Code", "Minion Code", "mise", "uv@latest pipx:minion-code[python=3.12]@latest", "", false, []string{"mise"}, nil, "pipx:minion-code"),
		Entry("Mistral Vibe", "Mistral Vibe", "mise", "uv@latest pipx:mistral-vibe[python=3.14]@latest", "", false, []string{"mise"}, nil, "pipx:mistral-vibe"),
		Entry("Goose", "Goose", "mise", "aqua:aaif-goose/goose@latest", "", false, []string{"mise"}, nil, "aqua:aaif-goose/goose"),
		Entry("Auggie CLI", "Auggie CLI", "mise", "node@24 npm:@augmentcode/auggie@latest", "", false, []string{"Node.js"}, nil, "npm:@augmentcode/auggie"),
		Entry("Bob", "Bob", "mise", "node@24 npm:@ibm/bob@latest", "", false, []string{"Node.js"}, nil, "npm:@ibm/bob"),
		Entry("Cline", "Cline", "mise", "node@24 npm:cline@latest", "", false, []string{"Node.js"}, nil, "npm:cline"),
		Entry("Codebuddy Code", "Codebuddy Code", "curlpipe", "https://www.codebuddy.cn/cli/install.sh", "user", true, nil, nil, `rm -f "$HOME/.local/bin/codebuddy" && rm -rf "$HOME/.local/share/codebuddy"`),
		Entry("DeepAgents", "DeepAgents", "mise", "uv@latest pipx:deepagents-code[python=3.14]@latest", "", false, []string{"mise"}, nil, "pipx:deepagents-code"),
		Entry("DimCode", "DimCode", "mise", "node@24 npm:dimcode@latest", "", false, []string{"Node.js"}, nil, "npm:dimcode"),
		Entry("Dirac", "Dirac", "mise", "node@24 npm:dirac-cli@latest", "", false, []string{"Node.js"}, nil, "npm:dirac-cli"),
		Entry("Factory Droid", "Factory Droid", "curlpipe", "https://app.factory.ai/cli", "user", true, nil, nil, `rm -f "$HOME/.local/bin/droid"`),
		Entry("Grok Build", "Grok Build", "curlpipe", "https://x.ai/cli/install.sh", "user", true, nil, nil, `for n in grok agent; do if [ "$(readlink "$HOME/.local/bin/$n" 2>/dev/null || true)" = "$HOME/.grok/bin/$n" ]; then rm -f "$HOME/.local/bin/$n" || exit 1; fi; case "$(readlink "$HOME/.grok/bin/$n" 2>/dev/null || true)" in ../downloads/* | "$HOME"/.grok/downloads/*) rm -f "$HOME/.grok/bin/$n" || exit 1 ;; esac; done; rm -f "$HOME/.config/fish/completions/grok.fish" && rm -rf "$HOME/.grok/downloads" "$HOME/.grok/completions" "$HOME/.grok/docs"`),
		Entry("MiniMax Code", "MiniMax Code", "mise", "node@24 npm:@minimax-ai/code@latest", "", false, []string{"Node.js"}, nil, "npm:@minimax-ai/code"),
		Entry("Nova", "Nova", "mise", "node@24 npm:@compass-ai/nova@latest", "", false, []string{"Node.js"}, nil, "npm:@compass-ai/nova"),
		Entry("Pi", "Pi", "mise", "node@24 npm:@earendil-works/pi-coding-agent@latest", "", false, []string{"Node.js"}, nil, "npm:@earendil-works/pi-coding-agent"),
		Entry("Qoder CLI", "Qoder CLI", "curlpipe", "https://qoder.com/install", "user", true, nil, nil, `rm -f "$HOME/.local/bin/qodercli" && rm -rf "$HOME/.qoder/bin" "$HOME/.qoder/entry"`),
		Entry("Qwen Code", "Qwen Code", "curlpipe", "https://qwen-code-assets.oss-cn-hangzhou.aliyuncs.com/installation/install-qwen-standalone.sh", "user", true, nil, nil, `rm -f "$HOME/.local/bin/qwen" && rm -rf "$HOME/.local/lib/qwen-code"`),
		Entry("siGit Code", "siGit Code", "mise", "node@24 npm:@getsigit/sigit[allow_low_downloads=true]@latest", "", false, []string{"Node.js"}, nil, "npm:@getsigit/sigit"),
		Entry("Amp", "Amp", "curlpipe", "https://ampcode.com/install.sh", "user", true, nil, nil, `rm -f "$HOME/.local/bin/amp" "$HOME/.amp/bin/amp"`),
		Entry("Autohand Code", "Autohand Code", "mise", "node@24 npm:autohand-cli@latest", "", false, []string{"Node.js"}, nil, "npm:autohand-cli"),
		Entry("Cortex Code", "Cortex Code", "curlpipe", "https://ai.snowflake.com/static/cc-scripts/install.sh", "user", true, nil, nil, `rm -f "$HOME/.local/bin/cortex" && rm -rf "$HOME/.local/share/cortex"`),
		Entry("crow-cli", "crow-cli", "mise", "uv@latest pipx:crow-cli[python=3.13]@latest", "", false, []string{"mise"}, nil, "pipx:crow-cli"),
		Entry("Cursor CLI", "Cursor CLI", "curlpipe", "https://cursor.com/install", "user", true, nil, nil, `case "$(readlink "$HOME/.local/bin/agent" 2>/dev/null || true)" in "$HOME"/.local/share/cursor-agent/*) rm -f "$HOME/.local/bin/agent" || exit 1 ;; esac; rm -f "$HOME/.local/bin/cursor-agent" && rm -rf "$HOME/.local/share/cursor-agent"`),
		Entry("Devin", "Devin", "mise", "node@24 npm:devin@latest", "", false, []string{"Node.js"}, nil, "npm:devin"),
		Entry("fx", "fx", "curlpipe", "https://fx.sh/setup.sh", "user", true, nil, nil, `rm -f "$HOME/.local/bin/fx"`),
		Entry("Harn", "Harn", "curlpipe", "https://harnlang.com/install.sh", "user", false, nil, nil, `d="$(cat "$HOME/.harn/devex-install-dir" 2>/dev/null || true)"; if [ -z "$d" ]; then for c in ${XDG_BIN_DIR:+"$XDG_BIN_DIR"} "$HOME/bin" "$HOME/.local/bin" "$HOME/.harn/bin"; do if [ -e "$c/harn" ]; then p="$(CDPATH= cd -P -- "$c" && pwd -P)" || exit 1; if [ -n "$d" ] && [ "$p" != "$dp" ]; then echo "Harn is in both $d and $c, and DevEx did not record which it installed; remove that copy by hand." >&2; exit 1; fi; d="$c"; dp="$p"; fi; done; fi; if [ -n "$d" ]; then rm -f "$d/harn" "$d/harn-dap" "$d/harn-lsp" && rm -rf "$d/harn-licenses" || exit 1; fi; rm -f "$HOME/.harn/devex-install-dir"`),
		Entry("Junie", "Junie", "curlpipe", "https://junie.jetbrains.com/install.sh", "user", true, nil, nil, `rm -f "$HOME/.local/bin/junie" && rm -rf "$HOME/.local/share/junie"`),
		Entry("Kilo", "Kilo", "curlpipe", "https://kilo.ai/cli/install", "user", false, nil, nil, `rm -f "$HOME/.kilo/bin/kilo" && rm -rf "$HOME/.kilo/bin/tree-sitter" && case "$(readlink "$HOME/.local/bin/kilo" 2>/dev/null || true)" in */.kilo/bin/kilo) rm -f "$HOME/.local/bin/kilo" ;; esac`),
		Entry("Kimchi", "Kimchi", "mise", "github:getkimchi/kimchi@latest", "", false, []string{"mise"}, nil, "github:getkimchi/kimchi"),
		Entry("Kiro CLI", "Kiro CLI", "curlpipe", "https://cli.kiro.dev/install", "user", true, nil, nil, `rm -f "$HOME/.local/bin/kiro-cli" "$HOME/.local/bin/kiro-cli-chat"`),
		Entry("Muse Code", "Muse Code", "curlpipe", "https://dev.meta.ai/install.sh", "user", true, nil, nil, `B="$HOME/.local/bin"; [ -d "$B" ] || exit 0; L="$B/.muse-update-lock"; T="$B/.muse-uninstall.$$"; i=0; while :; do rm -rf "$T"; mkdir "$T" && echo "$$" > "$T/pid" || exit 1; if mv "$T" "$L" 2>/dev/null && [ ! -e "$L/.muse-uninstall.$$" ] && [ "$(cat "$L/pid" 2>/dev/null)" = "$$" ]; then break; fi; rm -rf "$L/.muse-uninstall.$$" "$T"; i=$((i + 1)); if [ "$i" -ge "${DEVEX_LOCK_WAIT_SECONDS:-60}" ]; then echo "Muse Code is updating itself; run the uninstall again shortly (if no update is running, remove $L first)." >&2; exit 1; fi; sleep 1; done; rc=0; rm -f "$B/muse" "$B/muse-bin" "$B/muse-bin-"* "$B/.muse-launcher.previous" "$B/.muse-launcher."* "$B/.muse-version" "$B/.muse-release-info.json" "$B/.muse-update-checked-at" "$B/.muse-update-notice" "$B/.muse-update."* "$B/.muse-state" || rc=1; rm -rf "$B/.muse-updater" "$B/.muse-install-lock" || rc=1; rm -rf "$L" || rc=1; exit "$rc"`),
		Entry("Poolside", "Poolside", "mise", "github:poolsideai/pool@latest", "", false, []string{"mise"}, nil, "github:poolsideai/pool"),
		Entry("Stakpak", "Stakpak", "mise", "github:stakpak/agent@latest", "", true, []string{"mise"}, nil, "github:stakpak/agent"),
		Entry("VT Code", "VT Code", "curlpipe", "https://raw.githubusercontent.com/vinhnx/vtcode/main/scripts/install.sh", "user", false, nil, nil, `rm -f "$HOME/.local/bin/vtcode"`),
	)

	DescribeTable("plans each AI coding agent upgrade on Ubuntu 24.04",
		func(name, want string) {
			var out bytes.Buffer
			cmd := commands.NewRootCmd("test", mocks.NewMockRepository(), settings)
			cmd.SetOut(&out)
			cmd.SetErr(&out)
			cmd.SetArgs([]string{"upgrade", "--dry-run", "--non-interactive", name})
			Expect(cmd.Execute()).To(Succeed(), out.String())
			Expect(out.String()).To(ContainSubstring(want))
		},
		Entry("Claude Code updates itself", "Claude Code", "Claude Code manages its own updates; skipped"),
		Entry("Codex CLI updates itself", "Codex CLI", "Codex CLI manages its own updates; skipped"),
		Entry("Gemini CLI updates itself", "Gemini CLI", "Gemini CLI manages its own updates; skipped"),
		Entry("GitHub Copilot CLI updates itself", "GitHub Copilot CLI", "GitHub Copilot CLI manages its own updates; skipped"),
		Entry("OpenCode updates itself", "OpenCode", "OpenCode manages its own updates; skipped"),
		Entry("Kimi Code reruns its installer", "Kimi Code", `Plugin invocation: devex-plugin-package-manager-curlpipe "install" "--user" "https://code.kimi.com/kimi-code/install.sh"`),
		Entry("Aider upgrades through mise", "Aider", `Plugin invocation: devex-plugin-package-manager-mise "update" "pipx:aider-chat"`),
		Entry("Goose upgrades through mise", "Goose", `Plugin invocation: devex-plugin-package-manager-mise "update" "aqua:aaif-goose/goose"`),
		Entry("Auggie CLI upgrades through mise", "Auggie CLI", `Plugin invocation: devex-plugin-package-manager-mise "update" "npm:@augmentcode/auggie"`),
		Entry("Bob upgrades through mise", "Bob", `Plugin invocation: devex-plugin-package-manager-mise "update" "npm:@ibm/bob"`),
		Entry("Cline upgrades through mise", "Cline", `Plugin invocation: devex-plugin-package-manager-mise "update" "npm:cline"`),
		Entry("Codebuddy Code updates itself", "Codebuddy Code", "Codebuddy Code manages its own updates; skipped"),
		Entry("DeepAgents upgrades through mise", "DeepAgents", `Plugin invocation: devex-plugin-package-manager-mise "update" "pipx:deepagents-code"`),
		Entry("DimCode upgrades through mise", "DimCode", `Plugin invocation: devex-plugin-package-manager-mise "update" "npm:dimcode"`),
		Entry("Dirac upgrades through mise", "Dirac", `Plugin invocation: devex-plugin-package-manager-mise "update" "npm:dirac-cli"`),
		Entry("Factory Droid updates itself", "Factory Droid", "Factory Droid manages its own updates; skipped"),
		Entry("Grok Build updates itself", "Grok Build", "Grok Build manages its own updates; skipped"),
		Entry("MiniMax Code upgrades through mise", "MiniMax Code", `Plugin invocation: devex-plugin-package-manager-mise "update" "npm:@minimax-ai/code"`),
		Entry("Nova upgrades through mise", "Nova", `Plugin invocation: devex-plugin-package-manager-mise "update" "npm:@compass-ai/nova"`),
		Entry("Pi upgrades through mise", "Pi", `Plugin invocation: devex-plugin-package-manager-mise "update" "npm:@earendil-works/pi-coding-agent"`),
		Entry("Qoder CLI updates itself", "Qoder CLI", "Qoder CLI manages its own updates; skipped"),
		Entry("Qwen Code updates itself", "Qwen Code", "Qwen Code manages its own updates; skipped"),
		Entry("siGit Code upgrades through mise", "siGit Code", `Plugin invocation: devex-plugin-package-manager-mise "update" "npm:@getsigit/sigit"`),
		Entry("Amp updates itself", "Amp", "Amp manages its own updates; skipped"),
		Entry("Autohand Code upgrades through mise", "Autohand Code", `Plugin invocation: devex-plugin-package-manager-mise "update" "npm:autohand-cli"`),
		Entry("Cortex Code updates itself", "Cortex Code", "Cortex Code manages its own updates; skipped"),
		Entry("crow-cli upgrades through mise", "crow-cli", `Plugin invocation: devex-plugin-package-manager-mise "update" "pipx:crow-cli"`),
		Entry("Cursor CLI updates itself", "Cursor CLI", "Cursor CLI manages its own updates; skipped"),
		Entry("Devin upgrades through mise", "Devin", `Plugin invocation: devex-plugin-package-manager-mise "update" "npm:devin"`),
		Entry("fx updates itself", "fx", "fx manages its own updates; skipped"),
		Entry("Harn reruns its installer", "Harn", `Plugin invocation: devex-plugin-package-manager-curlpipe "install" "--user" "https://harnlang.com/install.sh"`),
		Entry("Junie updates itself", "Junie", "Junie manages its own updates; skipped"),
		Entry("Kilo reruns its installer", "Kilo", `Plugin invocation: devex-plugin-package-manager-curlpipe "install" "--user" "https://kilo.ai/cli/install"`),
		Entry("Kimchi upgrades through mise", "Kimchi", `Plugin invocation: devex-plugin-package-manager-mise "update" "github:getkimchi/kimchi"`),
		Entry("Kiro CLI updates itself", "Kiro CLI", "Kiro CLI manages its own updates; skipped"),
		Entry("Muse Code updates itself", "Muse Code", "Muse Code manages its own updates; skipped"),
		Entry("Poolside upgrades through mise", "Poolside", `Plugin invocation: devex-plugin-package-manager-mise "update" "github:poolsideai/pool"`),
		Entry("Stakpak updates itself", "Stakpak", "Stakpak manages its own updates; skipped"),
		Entry("VT Code reruns its installer", "VT Code", `Plugin invocation: devex-plugin-package-manager-curlpipe "install" "--user" "https://raw.githubusercontent.com/vinhnx/vtcode/main/scripts/install.sh"`),
	)

	DescribeTable("plans each AI coding agent uninstall on Ubuntu 24.04, keeping config",
		func(name, want string) {
			repo := mocks.NewMockRepository()
			Expect(repo.AddApp(name)).To(Succeed())
			var out bytes.Buffer
			cmd := commands.NewRootCmd("test", repo, settings)
			cmd.SetOut(&out)
			cmd.SetErr(&out)
			cmd.SetArgs([]string{"uninstall", "--app", name, "--dry-run"})
			Expect(cmd.Execute()).To(Succeed(), out.String())
			Expect(out.String()).To(ContainSubstring("Would uninstall " + name + " via " + want))
			Expect(out.String()).NotTo(ContainSubstring("Would remove configuration"))
			Expect(out.String()).NotTo(ContainSubstring("Would remove data"))
		},
		Entry("Claude Code", "Claude Code", `curlpipe: rm -f "$HOME/.local/bin/claude" && rm -rf "$HOME/.local/share/claude"`),
		Entry("Codex CLI", "Codex CLI", `curlpipe: rm -f "$HOME/.local/bin/codex" && rm -rf "$HOME/.codex/packages/standalone"`),
		Entry("Gemini CLI", "Gemini CLI", "mise: npm:@google/gemini-cli"),
		Entry("GitHub Copilot CLI", "GitHub Copilot CLI", `curlpipe: rm -f "$HOME/.local/bin/copilot"`),
		Entry("OpenCode", "OpenCode", `curlpipe: rm -f "$HOME/.opencode/bin/opencode"`),
		Entry("Kimi Code", "Kimi Code", `curlpipe: rm -f "$HOME/.kimi-code/bin/kimi" "$HOME/.kimi-code/bin/kimi.bak"`),
		Entry("Aider", "Aider", "mise: pipx:aider-chat"),
		Entry("Goose", "Goose", "mise: aqua:aaif-goose/goose"),
		Entry("Auggie CLI", "Auggie CLI", "mise: npm:@augmentcode/auggie"),
		Entry("Bob", "Bob", "mise: npm:@ibm/bob"),
		Entry("Cline", "Cline", "mise: npm:cline"),
		Entry("Codebuddy Code", "Codebuddy Code", `curlpipe: rm -f "$HOME/.local/bin/codebuddy" && rm -rf "$HOME/.local/share/codebuddy"`),
		Entry("DeepAgents", "DeepAgents", "mise: pipx:deepagents-code"),
		Entry("DimCode", "DimCode", "mise: npm:dimcode"),
		Entry("Dirac", "Dirac", "mise: npm:dirac-cli"),
		Entry("Factory Droid", "Factory Droid", `curlpipe: rm -f "$HOME/.local/bin/droid"`),
		Entry("Grok Build", "Grok Build", `curlpipe: for n in grok agent; do if [ "$(readlink "$HOME/.local/bin/$n" 2>/dev/null || true)" = "$HOME/.grok/bin/$n" ]; then rm -f "$HOME/.local/bin/$n" || exit 1; fi; case "$(readlink "$HOME/.grok/bin/$n" 2>/dev/null || true)" in ../downloads/* | "$HOME"/.grok/downloads/*) rm -f "$HOME/.grok/bin/$n" || exit 1 ;; esac; done; rm -f "$HOME/.config/fish/completions/grok.fish" && rm -rf "$HOME/.grok/downloads" "$HOME/.grok/completions" "$HOME/.grok/docs"`),
		Entry("MiniMax Code", "MiniMax Code", "mise: npm:@minimax-ai/code"),
		Entry("Nova", "Nova", "mise: npm:@compass-ai/nova"),
		Entry("Pi", "Pi", "mise: npm:@earendil-works/pi-coding-agent"),
		Entry("Qoder CLI", "Qoder CLI", `curlpipe: rm -f "$HOME/.local/bin/qodercli" && rm -rf "$HOME/.qoder/bin" "$HOME/.qoder/entry"`),
		Entry("Qwen Code", "Qwen Code", `curlpipe: rm -f "$HOME/.local/bin/qwen" && rm -rf "$HOME/.local/lib/qwen-code"`),
		Entry("siGit Code", "siGit Code", "mise: npm:@getsigit/sigit"),
		Entry("Amp", "Amp", `curlpipe: rm -f "$HOME/.local/bin/amp" "$HOME/.amp/bin/amp"`),
		Entry("Autohand Code", "Autohand Code", "mise: npm:autohand-cli"),
		Entry("Cortex Code", "Cortex Code", `curlpipe: rm -f "$HOME/.local/bin/cortex" && rm -rf "$HOME/.local/share/cortex"`),
		Entry("crow-cli", "crow-cli", "mise: pipx:crow-cli"),
		Entry("Cursor CLI", "Cursor CLI", `curlpipe: case "$(readlink "$HOME/.local/bin/agent" 2>/dev/null || true)" in "$HOME"/.local/share/cursor-agent/*) rm -f "$HOME/.local/bin/agent" || exit 1 ;; esac; rm -f "$HOME/.local/bin/cursor-agent" && rm -rf "$HOME/.local/share/cursor-agent"`),
		Entry("Devin", "Devin", "mise: npm:devin"),
		Entry("fx", "fx", `curlpipe: rm -f "$HOME/.local/bin/fx"`),
		Entry("Harn", "Harn", `curlpipe: d="$(cat "$HOME/.harn/devex-install-dir" 2>/dev/null || true)"; if [ -z "$d" ]; then for c in ${XDG_BIN_DIR:+"$XDG_BIN_DIR"} "$HOME/bin" "$HOME/.local/bin" "$HOME/.harn/bin"; do if [ -e "$c/harn" ]; then p="$(CDPATH= cd -P -- "$c" && pwd -P)" || exit 1; if [ -n "$d" ] && [ "$p" != "$dp" ]; then echo "Harn is in both $d and $c, and DevEx did not record which it installed; remove that copy by hand." >&2; exit 1; fi; d="$c"; dp="$p"; fi; done; fi; if [ -n "$d" ]; then rm -f "$d/harn" "$d/harn-dap" "$d/harn-lsp" && rm -rf "$d/harn-licenses" || exit 1; fi; rm -f "$HOME/.harn/devex-install-dir"`),
		Entry("Junie", "Junie", `curlpipe: rm -f "$HOME/.local/bin/junie" && rm -rf "$HOME/.local/share/junie"`),
		Entry("Kilo", "Kilo", `curlpipe: rm -f "$HOME/.kilo/bin/kilo" && rm -rf "$HOME/.kilo/bin/tree-sitter" && case "$(readlink "$HOME/.local/bin/kilo" 2>/dev/null || true)" in */.kilo/bin/kilo) rm -f "$HOME/.local/bin/kilo" ;; esac`),
		Entry("Kimchi", "Kimchi", "mise: github:getkimchi/kimchi"),
		Entry("Kiro CLI", "Kiro CLI", `curlpipe: rm -f "$HOME/.local/bin/kiro-cli" "$HOME/.local/bin/kiro-cli-chat"`),
		Entry("Muse Code", "Muse Code", `curlpipe: B="$HOME/.local/bin"; [ -d "$B" ] || exit 0; L="$B/.muse-update-lock"; T="$B/.muse-uninstall.$$"; i=0; while :; do rm -rf "$T"; mkdir "$T" && echo "$$" > "$T/pid" || exit 1; if mv "$T" "$L" 2>/dev/null && [ ! -e "$L/.muse-uninstall.$$" ] && [ "$(cat "$L/pid" 2>/dev/null)" = "$$" ]; then break; fi; rm -rf "$L/.muse-uninstall.$$" "$T"; i=$((i + 1)); if [ "$i" -ge "${DEVEX_LOCK_WAIT_SECONDS:-60}" ]; then echo "Muse Code is updating itself; run the uninstall again shortly (if no update is running, remove $L first)." >&2; exit 1; fi; sleep 1; done; rc=0; rm -f "$B/muse" "$B/muse-bin" "$B/muse-bin-"* "$B/.muse-launcher.previous" "$B/.muse-launcher."* "$B/.muse-version" "$B/.muse-release-info.json" "$B/.muse-update-checked-at" "$B/.muse-update-notice" "$B/.muse-update."* "$B/.muse-state" || rc=1; rm -rf "$B/.muse-updater" "$B/.muse-install-lock" || rc=1; rm -rf "$L" || rc=1; exit "$rc"`),
		Entry("Poolside", "Poolside", "mise: github:poolsideai/pool"),
		Entry("Stakpak", "Stakpak", "mise: github:stakpak/agent"),
		Entry("VT Code", "VT Code", `curlpipe: rm -f "$HOME/.local/bin/vtcode"`),
	)

	It("refuses to rerun an install script for an offline upgrade", func() {
		var out bytes.Buffer
		cmd := commands.NewRootCmd("test", mocks.NewMockRepository(), settings)
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		cmd.SetArgs([]string{"upgrade", "--dry-run", "--non-interactive", "--offline", "Kimi Code"})
		Expect(cmd.Execute()).To(MatchError(ContainSubstring("rerunning the install script needs the network")))
	})

	DescribeTable("plans dry-run uninstall and cross-platform channels for Python AI coding agents",
		func(name, wantInstall, wantUninstall string, wantBrewAlt bool) {
			app, ok := agents[name]
			Expect(ok).To(BeTrue(), "expected %s in ai-agents catalog", name)

			for label, cfg := range map[string]types.OSConfig{
				"linux":   app.Linux,
				"macos":   app.MacOS,
				"windows": app.Windows,
			} {
				Expect(cfg.InstallMethod).To(Equal("mise"), "%s (%s)", name, label)
				Expect(cfg.InstallCommand).To(Equal(wantInstall), "%s (%s)", name, label)
				Expect(cfg.UninstallCommand).To(Equal(wantUninstall), "%s (%s)", name, label)
				Expect(cfg.SelfUpdating).To(BeFalse(), "%s (%s)", name, label)
				Expect(cfg.OfficialSupport).To(BeTrue(), "%s (%s)", name, label)
				Expect(cfg.Dependencies).To(Equal([]string{"mise"}), "%s (%s)", name, label)

				// Every channel within a platform block must have a unique install_method.
				seen := map[string]bool{cfg.InstallMethod: true}
				for _, alt := range cfg.Alternatives {
					Expect(seen[alt.InstallMethod]).To(BeFalse(), "%s (%s) duplicate channel %q", name, label, alt.InstallMethod)
					seen[alt.InstallMethod] = true
				}
			}

			if wantBrewAlt {
				for label, cfg := range map[string]types.OSConfig{"linux": app.Linux, "macos": app.MacOS} {
					Expect(cfg.Alternatives).To(HaveLen(1), "%s (%s)", name, label)
					Expect(cfg.Alternatives[0].InstallMethod).To(Equal("brew"), "%s (%s)", name, label)
					Expect(cfg.Alternatives[0].OfficialSupport).To(BeFalse(), "%s (%s)", name, label)
					Expect(cfg.Alternatives[0].InstalledBinary).To(Equal("vibe"), "%s (%s)", name, label)
				}
			}

			// Dry-run uninstall removes only the mise pipx package and leaves user config intact.
			repo := mocks.NewMockRepository()
			Expect(repo.AddApp(name)).To(Succeed())

			var uninstallOut bytes.Buffer
			uninstallCmd := commands.NewRootCmd("test", repo, settings)
			uninstallCmd.SetOut(&uninstallOut)
			uninstallCmd.SetErr(&uninstallOut)
			uninstallCmd.SetArgs([]string{"uninstall", "--app", name, "--dry-run"})
			Expect(uninstallCmd.Execute()).To(Succeed())
			Expect(uninstallOut.String()).To(ContainSubstring("Would uninstall " + name + " via mise: " + wantUninstall))
			Expect(uninstallOut.String()).NotTo(ContainSubstring("Would remove configuration"))
			Expect(uninstallOut.String()).NotTo(ContainSubstring("Would remove data"))

			// Upgrade runs `mise upgrade` on the uninstalled tool; mise has no
			// metadata refresh, so update reports that explicitly.
			var upgradeOut bytes.Buffer
			upgradeCmd := commands.NewRootCmd("test", mocks.NewMockRepository(), settings)
			upgradeCmd.SetOut(&upgradeOut)
			upgradeCmd.SetErr(&upgradeOut)
			upgradeCmd.SetArgs([]string{"upgrade", name, "--dry-run"})
			Expect(upgradeCmd.Execute()).To(Succeed(), upgradeOut.String())
			Expect(upgradeOut.String()).To(ContainSubstring(`Plugin invocation: devex-plugin-package-manager-mise "update" "` + wantUninstall + `"`))

			var updateOut bytes.Buffer
			updateCmd := commands.NewRootCmd("test", mocks.NewMockRepository(), settings)
			updateCmd.SetOut(&updateOut)
			updateCmd.SetErr(&updateOut)
			updateCmd.SetArgs([]string{"update", name, "--dry-run"})
			Expect(updateCmd.Execute()).To(MatchError(ContainSubstring(`manager "mise" does not support safe update`)))
		},
		Entry("Aider", "Aider", "uv@latest pipx:aider-chat[python=3.12]@latest", "pipx:aider-chat", false),
		Entry("fast-agent", "fast-agent", "uv@latest pipx:fast-agent-mcp[python=3.12]@latest", "pipx:fast-agent-mcp", false),
		Entry("Minion Code", "Minion Code", "uv@latest pipx:minion-code[python=3.12]@latest", "pipx:minion-code", false),
		Entry("Mistral Vibe", "Mistral Vibe", "uv@latest pipx:mistral-vibe[python=3.14]@latest", "pipx:mistral-vibe", true),
	)

	It("resolves Ollama with system hook-driven install, curl/zstd dependencies, and full cleanup uninstall", func() {
		app, ok := agents["Ollama"]
		Expect(ok).To(BeTrue(), "expected Ollama in ai-agents catalog")

		ubuntu := platform.DetectionResult{
			OS:               "linux",
			Distribution:     "ubuntu",
			DistributionLike: []string{"debian"},
			Version:          "24.04",
			Architecture:     "amd64",
		}
		resolved := app.LinuxConfigForPlatform(ubuntu)
		Expect(resolved.InstallMethod).To(Equal("curlpipe"))
		Expect(resolved.InstallScope).To(BeEmpty())
		Expect(resolved.InstalledBinary).To(Equal("/usr/local/bin/ollama"), "absolute so a mise ollama on PATH is not mistaken for the system install")
		Expect(resolved.DownloadURL).To(BeEmpty())
		Expect(resolved.Dependencies).To(Equal([]string{"curl", "zstd"}))
		Expect(resolved.PostInstall).To(HaveLen(1))
		Expect(resolved.PostInstall[0].Scope).To(Equal("system"))
		Expect(resolved.PostInstall[0].Shell).NotTo(ContainSubstring("install.sh"))
		Expect(resolved.UninstallCommand).To(And(
			ContainSubstring("systemctl stop ollama"),
			ContainSubstring("systemctl disable ollama"),
			ContainSubstring("/etc/systemd/system/ollama.service"),
			ContainSubstring("/usr/local/bin/ollama"),
			ContainSubstring("/usr/local/lib/ollama"),
			ContainSubstring("userdel ollama"),
			ContainSubstring("groupdel ollama"),
			ContainSubstring("/usr/share/ollama"),
		))
		Expect(app.MacOS.InstallMethod).To(Equal("brew"))
		Expect(app.MacOS.InstallCommand).To(Equal("ollama-app"))
		Expect(app.MacOS.BrewCask).To(BeTrue())
		Expect(app.Windows.InstallMethod).To(Equal("winget"))
		Expect(app.Windows.InstallCommand).To(Equal("Ollama.Ollama"))

		plan, err := agentapi.PlanInstall(context.Background(), settings, agentapi.PlanRequest{Names: []string{"Ollama"}})
		Expect(err).NotTo(HaveOccurred())
		Expect(plan.DependencyOrder).To(ContainElements("curl", "zstd", "Ollama"))

		var out bytes.Buffer
		cmd := commands.NewRootCmd("test", mocks.NewMockRepository(), settings)
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		cmd.SetArgs([]string{"install", "--dry-run", "Ollama"})
		Expect(cmd.Execute()).To(Succeed())
		Expect(out.String()).To(ContainSubstring("Would install Ollama"))
	})

	It("resolves LM Studio to its .deb on Ubuntu and the AppImage on other Linux", func() {
		app, ok := agents["LM Studio"]
		Expect(ok).To(BeTrue(), "expected LM Studio in ai-agents catalog")

		fedora := platform.DetectionResult{
			OS:           "linux",
			Distribution: "fedora",
			Version:      "41",
			Architecture: "amd64",
		}
		// Not in the table above: its .deb is limited to Debian and Ubuntu, so the
		// host-detecting install plan would pick the AppImage on macOS CI.
		ubuntu := platform.DetectionResult{OS: "linux", Distribution: "ubuntu", DistributionLike: []string{"debian"}, Version: "24.04", Architecture: "amd64"}
		deb := app.LinuxConfigForPlatform(ubuntu)
		Expect(deb.InstallMethod).To(Equal("deb"))
		Expect(deb.DownloadURL).To(Equal("https://lmstudio.ai/download/latest/linux/x64?format=deb"))
		Expect(deb.UninstallCommand).To(Equal("lm-studio"))

		resolved := app.LinuxConfigForPlatform(fedora)
		Expect(resolved.InstallMethod).To(Equal("appimage"))
		Expect(resolved.DownloadURL).To(Equal("https://lmstudio.ai/download/latest/linux/x64"))
		Expect(resolved.InstalledBinary).To(Equal("LM-Studio.AppImage"))
		Expect(app.MacOS.InstallMethod).To(Equal("brew"))
		Expect(app.MacOS.InstallCommand).To(Equal("lm-studio"))
		Expect(app.MacOS.BrewCask).To(BeTrue())
		Expect(app.Windows.InstallMethod).To(Equal("winget"))
		Expect(app.Windows.InstallCommand).To(Equal("ElementLabs.LMStudio"))
	})
})
