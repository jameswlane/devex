package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"

	devexpaths "github.com/jameswlane/devex/packages/plugin-sdk/paths"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Startup migration preview policy", func() {
	It("honors environment and explicit true/false flags, stopping at the argument boundary", func() {
		GinkgoT().Setenv("DEVEX_DRY_RUN", "true")
		Expect(migrationDryRun(nil)).To(BeTrue())
		Expect(migrationDryRun([]string{"--dry-run=false", "paths"})).To(BeFalse())
		GinkgoT().Setenv("DEVEX_DRY_RUN", "")
		Expect(migrationDryRun([]string{"paths", "--dry-run"})).To(BeTrue())
		Expect(migrationDryRun([]string{"--", "--dry-run"})).To(BeFalse())
	})
})

var _ = Describe("Startup migration recovery", func() {
	It("recognizes inspection commands with global flag values and preserves normal setup migration", func() {
		Expect(informationalCommand([]string{"--log-level", "debug", "--config", "custom.yaml", "paths"})).To(BeTrue())
		Expect(informationalCommand(nil)).To(BeFalse())
		Expect(informationalCommand([]string{"install", "--app", "paths"})).To(BeFalse())
	})

	It("warns with repair guidance and continues after a refusal for every command", func(ctx SpecContext) {
		home := GinkgoT().TempDir()
		dirs := devexpaths.ForHome(home)
		Expect(os.MkdirAll(filepath.Join(home, ".devex"), 0700)).To(Succeed())
		Expect(os.Symlink("dotfiles.yaml", filepath.Join(home, ".devex", "config.yaml"))).To(Succeed())
		for _, args := range [][]string{{"--help"}, {"completion", "bash"}, {"paths"}, {"--version"}, {"config", "show"}, {"install", "editor"}} {
			var output bytes.Buffer
			Expect(migrateLegacy(ctx, dirs, false, args, &output)).To(Succeed())
			Expect(output.String()).To(And(ContainSubstring("Warning:"), ContainSubstring("devex paths"), ContainSubstring("regular files"), ContainSubstring("backing them up")))
		}
		Expect(dirs.Config).NotTo(BeADirectory())
	})

	It("allows inspection after I/O failures while operational commands return the error", func(ctx SpecContext) {
		home := GinkgoT().TempDir()
		// An overlong component produces an I/O error even when tests run as root.
		dirs := devexpaths.ForHome(filepath.Join(home, strings.Repeat("x", 300)))
		var output bytes.Buffer
		Expect(migrateLegacy(ctx, dirs, false, []string{"paths"}, &output)).To(Succeed())
		Expect(output.String()).To(ContainSubstring("Warning:"))
		Expect(migrateLegacy(ctx, dirs, false, []string{"install", "editor"}, &output)).To(HaveOccurred())
	})

	It("leaves safe legacy files in place during informational commands", func(ctx SpecContext) {
		home := GinkgoT().TempDir()
		legacy := filepath.Join(home, ".devex", "config.yaml")
		Expect(os.MkdirAll(filepath.Dir(legacy), 0700)).To(Succeed())
		Expect(os.WriteFile(legacy, []byte("config"), 0600)).To(Succeed())
		var output bytes.Buffer
		Expect(migrateLegacy(ctx, devexpaths.ForHome(home), false, []string{"paths"}, &output)).To(Succeed())
		Expect(legacy).To(BeAnExistingFile())
		Expect(devexpaths.ForHome(home).Config).NotTo(BeADirectory())
	})
})

var _ = Describe("Read-only startup classification", func() {
	DescribeTable("keeps JSON status inspection side-effect free while plain status bootstraps",
		func(args []string, readOnly bool) {
			Expect(readCommand(args)).To(Equal(readOnly))
		},
		Entry("plain status bootstraps plugins", []string{"status", "--all", "--no-tui"}, false),
		Entry("JSON status is read-only", []string{"status", "--format", "json"}, true),
		Entry("JSON status with equals form is read-only", []string{"status", "--format=json"}, true),
		Entry("plain list bootstraps plugins", []string{"list", "available"}, false),
		Entry("JSON list is read-only", []string{"list", "available", "--format", "json"}, true),
		Entry("platform is read-only", []string{"platform"}, true),
		Entry("agent skill installation is read-only", []string{"agent", "install-skill", "--agent", "claude", "--scope", "user"}, true),
		Entry("agent group alone is not read-only", []string{"agent"}, false),
		Entry("mcp serve is read-only", []string{"mcp", "serve"}, true),
		Entry("mcp serve with a flag is read-only", []string{"mcp", "serve", "--request-timeout", "5s"}, true),
		Entry("bare mcp is not a read route", []string{"mcp"}, false),
		Entry("inventory is read-only", []string{"inventory", "--format", "yaml"}, true),
		Entry("install is not read-only", []string{"install", "git"}, false),
		// Value-taking global flags before the command must not hide it.
		Entry("integer global flag before mcp serve", []string{"--install-concurrency", "1", "mcp", "serve"}, true),
		Entry("duration global flag before mcp serve", []string{"--apt-lists-max-age", "15m", "mcp", "serve"}, true),
		Entry("config file before mcp serve", []string{"--config", "/x/config.yaml", "mcp", "serve"}, true),
		Entry("log level before mcp serve", []string{"--log-level", "debug", "mcp", "serve"}, true),
		Entry("integer global flag before platform", []string{"--install-concurrency", "1", "platform"}, true),
		Entry("integer global flag before JSON status", []string{"--install-concurrency", "1", "status", "--format", "json"}, true),
		Entry("duration global flag before JSON list", []string{"--apt-lists-max-age", "15m", "list", "available", "-f", "json"}, true),
		Entry("integer global flag before config show", []string{"--install-concurrency", "1", "config", "show"}, true),
		Entry("integer global flag before plain status", []string{"--install-concurrency", "1", "status"}, false),
		Entry("integer global flag before install", []string{"--install-concurrency", "1", "install", "git"}, false),
		Entry("a flag value that names a read route", []string{"--config", "platform", "install", "git"}, false),
		Entry("unknown commands", []string{"--install-concurrency", "1", "not-a-command"}, false),
		Entry("integer global flag before agent install-skill", []string{"--install-concurrency", "4", "agent", "install-skill", "--agent", "claude"}, true),
		Entry("integer global flag before the agent group", []string{"--install-concurrency", "4", "agent"}, false),
		Entry("config show with --help=false is read-only", []string{"config", "show", "--help=false"}, true),
	)

	DescribeTable("skips installer dependency checks only for the read-only MCP server",
		func(args []string, skip bool) {
			Expect(readOnlyMCPServe(args)).To(Equal(skip))
		},
		Entry("mcp serve", []string{"mcp", "serve"}, true),
		Entry("mcp serve with a timeout", []string{"--log-level", "debug", "mcp", "serve", "--request-timeout", "5s"}, true),
		Entry("bare mcp", []string{"mcp"}, false),
		Entry("install", []string{"install", "git"}, false),
		Entry("platform", []string{"platform"}, false),
		Entry("integer global flag before mcp serve", []string{"--install-concurrency", "1", "mcp", "serve"}, true),
		Entry("duration global flag before mcp serve", []string{"--apt-lists-max-age", "15m", "mcp", "serve"}, true),
		Entry("config file before mcp serve", []string{"--config", "/x/config.yaml", "mcp", "serve"}, true),
		Entry("config value named mcp", []string{"--config", "mcp", "serve"}, false),
		Entry("after the argument terminator", []string{"install", "--", "mcp", "serve"}, false),
		Entry("mcp serve --help=false still serves read-only", []string{"mcp", "serve", "--help=false"}, true),
	)

	DescribeTable("recognizes other startup routes after value-taking global flags",
		func(classify func([]string) bool, args []string, want bool) {
			Expect(classify(args)).To(Equal(want))
		},
		Entry("config validate", configurationValidationCommand, []string{"--install-concurrency", "1", "config", "validate"}, true),
		Entry("config validate after a duration", configurationValidationCommand, []string{"--apt-lists-max-age", "15m", "config", "validate"}, true),
		Entry("config show is not validation", configurationValidationCommand, []string{"--install-concurrency", "1", "config", "show"}, false),
		Entry("help command", presentationCommand, []string{"--install-concurrency", "1", "help", "install"}, true),
		Entry("help flag", presentationCommand, []string{"--apt-lists-max-age", "15m", "install", "--help"}, true),
		Entry("completion", presentationCommand, []string{"--install-concurrency", "1", "completion", "bash"}, true),
		Entry("root version flag", presentationCommand, []string{"--install-concurrency", "1", "--version"}, true),
		Entry("install is not presentation", presentationCommand, []string{"--install-concurrency", "1", "install", "git"}, false),
		Entry("paths", informationalCommand, []string{"--install-concurrency", "1", "paths"}, true),
		Entry("config show", informationalCommand, []string{"--apt-lists-max-age", "15m", "config", "show"}, true),
		Entry("install is not informational", informationalCommand, []string{"--install-concurrency", "1", "install", "git"}, false),
		Entry("agent install-skill", agentSkillInstallCommand, []string{"agent", "install-skill", "--agent", "claude", "--scope", "user"}, true),
		Entry("agent install-skill after an integer flag", agentSkillInstallCommand, []string{"--install-concurrency", "4", "agent", "install-skill", "--agent", "claude"}, true),
		Entry("agent install-skill after a duration flag", agentSkillInstallCommand, []string{"--apt-lists-max-age", "15m", "agent", "install-skill", "--agent", "kimi"}, true),
		Entry("agent install-skill after a config file", agentSkillInstallCommand, []string{"--config", "/x/config.yaml", "agent", "install-skill"}, true),
		Entry("agent group alone", agentSkillInstallCommand, []string{"--install-concurrency", "4", "agent"}, false),
		Entry("a flag value named agent", agentSkillInstallCommand, []string{"--config", "agent", "install-skill"}, false),
		Entry("install-skill as an install argument", agentSkillInstallCommand, []string{"--install-concurrency", "4", "install", "agent", "install-skill"}, false),
		Entry("after the argument terminator", agentSkillInstallCommand, []string{"install", "--", "agent", "install-skill"}, false),
		Entry("agent install-skill --help=false still installs", agentSkillInstallCommand, []string{"agent", "install-skill", "--help=false"}, true),
		// Explicitly false booleans are set but false, as Cobra treats them:
		// these commands execute, so they must not take the nil-repository
		// early presentation path.
		Entry("install with --help=false", presentationCommand, []string{"install", "git", "--help=false", "--non-interactive"}, false),
		Entry("install with --help=true then --help=false", presentationCommand, []string{"install", "git", "--help=true", "--help=false"}, false),
		Entry("uninstall --forget with --help=false", presentationCommand, []string{"uninstall", "--forget", "git", "--force", "--help=false"}, false),
		Entry("root --version=false", presentationCommand, []string{"--version=false"}, false),
		Entry("install with --help=false is not informational", informationalCommand, []string{"install", "git", "--help=false"}, false),
		Entry("root --version=false is not informational", informationalCommand, []string{"--version=false"}, false),
		Entry("install with --help=true", presentationCommand, []string{"install", "git", "--help=true"}, true),
		Entry("root --version=true", presentationCommand, []string{"--version=true"}, true),
		Entry("-h", presentationCommand, []string{"install", "-h"}, true),
	)
})
