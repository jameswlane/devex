package setup

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/log"
	"github.com/jameswlane/devex/apps/cli/internal/types"
	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v3"
)

func setupRepoRoot() string {
	_, source, _, ok := runtime.Caller(0)
	Expect(ok).To(BeTrue())
	return filepath.Clean(filepath.Join(filepath.Dir(source), "../../../../.."))
}

var _ = Describe("Setup failure regressions", func() {
	It("calls commands declared by Git and every desktop plugin", func(ctx SpecContext) {
		pluginDirs, err := filepath.Glob(filepath.Join(setupRepoRoot(), "packages/desktop-*"))
		Expect(err).NotTo(HaveOccurred())
		Expect(pluginDirs).NotTo(BeEmpty())
		pluginDirs = append(pluginDirs, filepath.Join(setupRepoRoot(), "packages/tool-git"))
		for _, dir := range pluginDirs {
			path := filepath.Join(GinkgoT().TempDir(), filepath.Base(dir))
			cmd := exec.CommandContext(ctx, "go", "build", "-o", path, ".")
			cmd.Dir = dir
			output, err := cmd.CombinedOutput()
			Expect(err).NotTo(HaveOccurred(), "%s: %s", dir, output)
			output, err = exec.CommandContext(ctx, path, "--plugin-info").Output()
			Expect(err).NotTo(HaveOccurred())
			var info sdk.PluginInfo
			Expect(json.Unmarshal(output, &info)).To(Succeed())
			commands := make([]string, 0, len(info.Commands))
			for _, command := range info.Commands {
				commands = append(commands, command.Name)
			}
			calls := desktopConfigurationArgs("dark")
			if info.Name == "tool-git" {
				calls = [][]string{gitConfigurationArgs("Setup User", "setup@example.com")}
				GinkgoT().Setenv("GIT_CONFIG_GLOBAL", filepath.Join(GinkgoT().TempDir(), "gitconfig"))
				Expect(runConfigurationPlugin(ctx, path, calls[0])).To(Succeed())
				for key, value := range map[string]string{"user.name": "Setup User", "user.email": "setup@example.com"} {
					output, err := exec.CommandContext(ctx, "git", "config", "--global", "--get", key).Output()
					Expect(err).NotTo(HaveOccurred())
					Expect(string(output)).To(Equal(value + "\n"))
				}
			}
			for _, args := range calls {
				Expect(commands).To(ContainElement(args[0]), "%s does not declare %s", info.Name, args[0])
			}
		}
		Expect(desktopConfigurationArgs("dark")).To(Equal([][]string{{"configure"}, {"apply-theme", "dark"}}))
	}, SpecTimeout(2*time.Minute))

	It("installs Erlang before Elixir even when Erlang was not selected", func() {
		model := &SetupModel{
			system:     SystemInfo{languages: []string{"Elixir", "Erlang"}},
			selections: UISelections{selectedLangs: map[int]bool{0: true}},
		}
		apps := model.getLanguageApps()
		Expect(apps).To(HaveLen(2))
		Expect(apps[0].GetOSConfig().InstallCommand).To(Equal("erlang@latest"))
		Expect(apps[1].GetOSConfig().InstallCommand).To(Equal("elixir@latest"))
		model.selections.selectedLangs[1] = true
		Expect(model.getLanguageApps()).To(HaveLen(2))
		model.selections.selectedLangs = map[int]bool{1: true}
		Expect(model.getLanguageApps()).To(HaveLen(1))
	})

	It("uses tealdeer only for APT and gives Elixir a description and Erlang dependency", func() {
		content, err := os.ReadFile(filepath.Join(setupRepoRoot(), "apps/cli/config/applications/development/tldr.yaml"))
		Expect(err).NotTo(HaveOccurred())
		var app types.CrossPlatformApp
		Expect(yaml.Unmarshal(content, &app)).To(Succeed())
		Expect(app.Linux.InstallCommand).To(Equal("tealdeer"))
		Expect(app.Linux.UninstallCommand).To(Equal("tealdeer"))
		Expect(app.MacOS.InstallCommand).To(Equal("tldr"))
		Expect(app.Windows.InstallCommand).To(Equal("tldr-pages.tlrc"))
		content, err = os.ReadFile(filepath.Join(setupRepoRoot(), "apps/cli/config/environments/programming-languages/elixir.yaml"))
		Expect(err).NotTo(HaveOccurred())
		var language types.AppConfig
		Expect(yaml.Unmarshal(content, &language)).To(Succeed())
		Expect(language.Description).NotTo(BeEmpty())
		Expect(language.Dependencies).To(ContainElement("erlang"))
	})

	It("lists every app and configuration failure in both summaries and the log", func() {
		GinkgoT().Setenv("HOME", GinkgoT().TempDir())
		GinkgoT().Setenv("XDG_STATE_HOME", GinkgoT().TempDir())
		Expect(log.InitFileLogger(false)).To(Succeed())
		DeferCleanup(func() { Expect(log.Close()).To(Succeed()); log.InitTestLogger() })
		model := &SetupModel{step: StepComplete, settings: config.CrossPlatformSettings{}}
		model.recordInstallationErrors(errors.Join(errors.New("tldr: package unavailable"), errors.New("Elixir: erl not found")))
		model.installation.addError("Git configuration: plugin failed")
		model.logCompletion()
		for _, summary := range []string{model.View(), captureAutomatedCompletion(model), model.CompletionError().Error()} {
			Expect(summary).To(ContainSubstring("completed with 3 errors"))
			Expect(summary).To(ContainSubstring("tldr: package unavailable"))
			Expect(summary).To(ContainSubstring("Elixir: erl not found"))
			Expect(summary).To(ContainSubstring("Git configuration: plugin failed"))
			Expect(summary).To(ContainSubstring(log.GetLogFile()))
			Expect(summary).NotTo(ContainSubstring("successfully"))
		}
		content, err := os.ReadFile(log.GetLogFile())
		Expect(err).NotTo(HaveOccurred())
		Expect(string(content)).To(ContainSubstring("Setup completed with 3 errors"))
		Expect(string(content)).To(ContainSubstring("Git configuration: plugin failed"))
	})

	It("captures plugin stdout and stderr in the log and preserves failure details", func(ctx SpecContext) {
		GinkgoT().Setenv("HOME", GinkgoT().TempDir())
		GinkgoT().Setenv("XDG_STATE_HOME", GinkgoT().TempDir())
		Expect(log.InitFileLogger(false)).To(Succeed())
		DeferCleanup(func() { Expect(log.Close()).To(Succeed()); log.InitTestLogger() })
		path := filepath.Join(GinkgoT().TempDir(), "plugin")
		Expect(os.WriteFile(path, []byte("#!/bin/sh\necho 'plugin stdout'\necho 'plugin stderr' >&2\nexit 1\n"), 0755)).To(Succeed())
		Expect(runConfigurationPlugin(ctx, path, []string{"configure"})).To(MatchError(ContainSubstring("plugin stderr")))
		content, err := os.ReadFile(log.GetLogFile())
		Expect(err).NotTo(HaveOccurred())
		Expect(string(content)).To(ContainSubstring("plugin stdout"))
		Expect(string(content)).To(ContainSubstring("plugin stderr"))
		cancelled, cancel := context.WithCancel(ctx)
		cancel()
		Expect(runConfigurationPlugin(cancelled, path, []string{"configure"})).To(MatchError(ContainSubstring("context canceled")))
	})
})
