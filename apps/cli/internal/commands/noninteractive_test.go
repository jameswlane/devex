package commands

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/interaction"
	"github.com/jameswlane/devex/apps/cli/internal/mocks"
	"github.com/jameswlane/devex/apps/cli/internal/types"
)

var _ = Describe("Non-interactive application commands", func() {
	var settings config.CrossPlatformSettings
	var repo *mocks.MockRepository
	var out bytes.Buffer
	var installed, removed []string
	var prepared int
	app := func(name string, defaultApp bool) types.CrossPlatformApp {
		return types.CrossPlatformApp{Name: name, Default: defaultApp, Category: "Utilities", AllPlatforms: types.OSConfig{InstallMethod: "apt", InstallCommand: name, UninstallCommand: name}}
	}
	setEnv := func(key, value string) {
		old, present := os.LookupEnv(key)
		Expect(os.Setenv(key, value)).To(Succeed())
		DeferCleanup(func() {
			if present {
				_ = os.Setenv(key, old)
			} else {
				_ = os.Unsetenv(key)
			}
		})
	}
	run := func(cmd *cobra.Command, args ...string) error {
		cmd.SetOut(&out)
		cmd.SetErr(&out)
		cmd.SetArgs(append([]string{}, args...))
		return cmd.Execute()
	}
	BeforeEach(func() {
		viper.Reset()
		viper.SetConfigType("yaml")
		DeferCleanup(viper.Reset)
		for _, key := range []string{"CI", "GITHUB_ACTIONS", "GITLAB_CI", "DEVEX_NONINTERACTIVE", "DEVEX_NON_INTERACTIVE", "TERM"} {
			setEnv(key, "")
		}
		repo = mocks.NewMockRepository()
		settings = config.CrossPlatformSettings{Terminal: config.TerminalApplicationsConfig{Utilities: []types.CrossPlatformApp{app("git", true), app("ripgrep", false)}}}
		out.Reset()
		installed = nil
		removed = nil
		prepared = 0
		oldInstall, oldPrepare, oldRemove := installApplication, prepareApplicationInstallers, uninstallApplication
		DeferCleanup(func() {
			installApplication = oldInstall
			prepareApplicationInstallers = oldPrepare
			uninstallApplication = oldRemove
		})
		prepareApplicationInstallers = func(context.Context, pluginBootstrapProvider) error { prepared++; return nil }
		installApplication = func(_ context.Context, a types.CrossPlatformApp, _ config.CrossPlatformSettings, _ types.Repository) error {
			installed = append(installed, a.Name)
			return nil
		}
		uninstallApplication = func(_ *cobra.Command, a types.CrossPlatformApp, _ types.Repository) error {
			removed = append(removed, a.Name)
			return nil
		}
	})
	It("installs defaults in plain mode and records them, safely repeating", func() {
		cmd := NewInstallCmd(repo, settings)
		Expect(run(cmd, "--non-interactive")).To(Succeed())
		Expect(installed).To(Equal([]string{"git"}))
		Expect(prepared).To(Equal(1))
		Expect(out.String()).To(Equal("installing git...\ninstalled git\ninstalled 1 of 1 applications\n"))
		Expect(repo.GetApp("git")).NotTo(BeNil())
		Expect(run(NewInstallCmd(repo, settings), "--non-interactive")).To(Succeed())
	})
	It("continues in request order after failures and names failures", func() {
		installApplication = func(_ context.Context, a types.CrossPlatformApp, _ config.CrossPlatformSettings, _ types.Repository) error {
			installed = append(installed, a.Name)
			if a.Name == "ripgrep" {
				return fmt.Errorf("package unavailable")
			}
			return nil
		}
		err := run(NewInstallCmd(repo, settings), "ripgrep", "git", "--non-interactive")
		Expect(err).To(MatchError(ContainSubstring("ripgrep")))
		Expect(installed).To(Equal([]string{"ripgrep", "git"}))
		Expect(out.String()).To(ContainSubstring("failed ripgrep: package unavailable\n"))
		Expect(out.String()).To(ContainSubstring("installed 1 of 2 applications"))
		Expect(repo.GetApp("git")).NotTo(BeNil())
		_, err = repo.GetApp("ripgrep")
		Expect(err).To(HaveOccurred())
	})
	It("selects categories without opening a TUI", func() {
		Expect(run(NewInstallCmd(repo, settings), "--categories", "Utilities", "--non-interactive")).To(Succeed())
		Expect(installed).To(Equal([]string{"git", "ripgrep"}))
	})
	It("keeps dry runs free of installer and bootstrap calls", func() {
		viper.Set("dry-run", true)
		Expect(run(NewInstallCmd(repo, settings), "--non-interactive")).To(Succeed())
		Expect(installed).To(BeEmpty())
		Expect(prepared).To(BeZero())
	})
	It("replaces defaults with names and inline definitions, while explicit args win", func() {
		Expect(viper.ReadConfig(strings.NewReader(`applications:
 - ripgrep
 - name: jq
   category: Utilities
   all_platforms:
     install_method: apt
     install_command: jq
     uninstall_command: jq
non-interactive: true
`))).To(Succeed())
		Expect(run(NewInstallCmd(repo, settings))).To(Succeed())
		Expect(installed).To(Equal([]string{"ripgrep", "jq"}))
		installed = nil
		Expect(run(NewInstallCmd(repo, settings), "jq", "git")).To(Succeed())
		Expect(installed).To(Equal([]string{"jq", "git"}))
	})
	It("rejects unknown names before initializing or installing anything", func() {
		viper.Set("applications", []any{"git", "missing-app", "another-missing-app"})
		Expect(run(NewInstallCmd(repo, settings), "--non-interactive")).To(MatchError(ContainSubstring("missing-app")))
		Expect(installed).To(BeEmpty())
		Expect(prepared).To(BeZero())
	})
	It("reports bootstrap failures for every resolved application", func() {
		prepareApplicationInstallers = func(context.Context, pluginBootstrapProvider) error { return fmt.Errorf("plugin download failed") }
		err := run(NewInstallCmd(repo, settings), "git", "ripgrep", "--non-interactive")
		Expect(err).To(MatchError(ContainSubstring("git, ripgrep")))
		Expect(installed).To(BeEmpty())
		Expect(out.String()).To(ContainSubstring("installing git...\nfailed git: plugin download failed"))
		Expect(out.String()).To(ContainSubstring("installing ripgrep...\nfailed ripgrep: plugin download failed"))
		Expect(out.String()).To(ContainSubstring("installed 0 of 2 applications"))
	})
	It("lists successfully installed inline applications", func() {
		viper.Set("applications", []any{map[string]any{"name": "jq", "all_platforms": map[string]any{"install_method": "apt", "install_command": "jq"}}})
		Expect(run(NewInstallCmd(repo, settings), "--non-interactive")).To(Succeed())
		out.Reset()
		Expect(run(NewListCmd(repo, settings), "installed")).To(Succeed())
		apps, err := repo.ListApps()
		Expect(err).NotTo(HaveOccurred())
		Expect(apps).To(HaveLen(1))
		Expect(apps[0].Name).To(Equal("jq"))
		Expect(out.String()).To(ContainSubstring("jq"))
		viper.Reset()
		out.Reset()
		Expect(run(NewListCmd(repo, settings), "installed")).To(Succeed())
		Expect(out.String()).To(ContainSubstring("jq"))
	})
	It("honors an empty replacement list", func() {
		viper.Set("applications", []any{})
		Expect(run(NewInstallCmd(repo, settings), "--non-interactive")).To(Succeed())
		Expect(installed).To(BeEmpty())
		Expect(out.String()).To(ContainSubstring("installed 0 of 0 applications"))
	})
	It("uses configured applications during automated setup", func() {
		viper.Set("applications", []any{"ripgrep"})
		Expect(run(NewSetupCmd(repo, settings), "--non-interactive")).To(Succeed())
		Expect(installed).To(Equal([]string{"ripgrep"}))
	})
	It("removes inline applications through the installer without prompting", func() {
		viper.Set("applications", []any{map[string]any{"name": "jq", "all_platforms": map[string]any{"install_method": "apt", "install_command": "jq", "uninstall_command": "jq"}}})
		Expect(run(NewInstallCmd(repo, settings), "--non-interactive")).To(Succeed())
		Expect(run(NewRemoveCmd(repo, settings), "jq", "--force")).To(Succeed())
		Expect(removed).To(Equal([]string{"jq"}))
		_, err := repo.GetApp("jq")
		Expect(err).To(HaveOccurred())
	})
	It("removes installed inline apps after the original config is gone", func() {
		viper.Set("applications", []any{map[string]any{"name": "jq", "all_platforms": map[string]any{"install_method": "apt", "install_command": "jq"}}})
		Expect(run(NewInstallCmd(repo, settings), "--non-interactive")).To(Succeed())
		viper.Reset()
		Expect(run(NewRemoveCmd(repo, settings), "jq", "--force")).To(Succeed())
		Expect(removed).To(Equal([]string{"jq"}))
	})
	It("leaves bundled apps to the regular removal flow", func() {
		viper.Set("applications", []any{"ripgrep"})
		Expect(run(NewInstallCmd(repo, settings), "--non-interactive")).To(Succeed())
		handled, err := removeInstalledApplication(NewRemoveCmd(repo, settings), "ripgrep", settings, repo, false, false, nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(handled).To(BeFalse())
		Expect(removed).To(BeEmpty())
	})
	DescribeTable("detects automation", func(key string) {
		setEnv(key, "1")
		viper.SetDefault("non-interactive", false)
		Expect(interaction.IsInteractiveMode()).To(BeFalse())
		Expect(run(NewInstallCmd(repo, settings))).To(Succeed())
		Expect(installed).To(Equal([]string{"git"}))
	}, Entry("legacy environment variable", "DEVEX_NONINTERACTIVE"), Entry("CI", "CI"), Entry("GitHub Actions", "GITHUB_ACTIONS"))
	It("applies flag then environment then config precedence", func() {
		viper.SetConfigType("yaml")
		Expect(viper.ReadConfig(strings.NewReader("non-interactive: true\n"))).To(Succeed())
		setEnv("DEVEX_NONINTERACTIVE", "0")
		Expect(interaction.IsInteractiveMode()).To(BeTrue())
		cmd := NewInstallCmd(repo, settings)
		Expect(cmd.Flags().Set("non-interactive", "true")).To(Succeed())
		Expect(interaction.IsInteractiveMode()).To(BeFalse())
	})
})
