package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"

	"github.com/jameswlane/devex/apps/cli/internal/testhelper"
	devexpaths "github.com/jameswlane/devex/packages/plugin-sdk/paths"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Startup migration preview policy", func() {
	It("honors environment and explicit true/false flags, stopping at the argument boundary", func() {
		explicit := func(args []string) bool { dry, _ := explicitDryRun(args); return dry }
		GinkgoT().Setenv("DEVEX_DRY_RUN", "true")
		Expect(explicit(nil)).To(BeTrue())
		Expect(explicit([]string{"--dry-run=false", "paths"})).To(BeFalse())
		GinkgoT().Setenv("DEVEX_DRY_RUN", "")
		Expect(explicit([]string{"paths", "--dry-run"})).To(BeTrue())
		Expect(explicit([]string{"--", "--dry-run"})).To(BeFalse())
	})
})

var _ = Describe("Startup preview from the config file", func() {
	var home, preview string

	BeforeEach(func() {
		home = GinkgoT().TempDir()
		testhelper.SetProfileEnv(GinkgoT(), home)
		GinkgoT().Setenv("DEVEX_DRY_RUN", "")
		GinkgoT().Setenv("DEVEX_CONFIG", "")
		GinkgoT().Chdir(home)
		preview = filepath.Join(home, "preview.yaml")
		Expect(os.WriteFile(preview, []byte("dry-run: true\n"), 0600)).To(Succeed())
	})

	It("treats dry-run in a --config file as a preview", func() {
		Expect(startupDryRun([]string{"--config", preview, "install", "editor"})).To(BeTrue())
		Expect(startupDryRun([]string{"--config=" + preview, "install", "editor"})).To(BeTrue())
		Expect(startupDryRun([]string{"install", "editor"})).To(BeFalse())
	})

	It("reads the file named by DEVEX_CONFIG", func() {
		GinkgoT().Setenv("DEVEX_CONFIG", preview)
		Expect(startupDryRun([]string{"install", "editor"})).To(BeTrue())
	})

	It("reads config.yaml from the standard locations", func() {
		Expect(os.WriteFile(filepath.Join(home, "config.yaml"), []byte("dry-run: true\n"), 0600)).To(Succeed())
		Expect(startupDryRun([]string{"install", "editor"})).To(BeTrue())
	})

	It("lets the flag and the environment override the file", func() {
		Expect(startupDryRun([]string{"--config", preview, "--dry-run=false", "install", "editor"})).To(BeFalse())
		GinkgoT().Setenv("DEVEX_DRY_RUN", "false")
		Expect(startupDryRun([]string{"--config", preview, "install", "editor"})).To(BeFalse())
	})

	It("does not take a flag value that looks like --dry-run for the flag", func() {
		Expect(startupDryRun([]string{"--config", preview, "install", "--template", "--dry-run=false"})).To(BeTrue())
	})

	It("writes nothing first when the config file cannot be read", func() {
		Expect(startupDryRun([]string{"--config", filepath.Join(home, "missing.yaml"), "install", "editor"})).To(BeTrue())
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
		testhelper.SetProfileEnv(GinkgoT(), home)
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
		testhelper.SetProfileEnv(GinkgoT(), home)
		// An overlong component produces an I/O error even when tests run as root.
		dirs := devexpaths.ForHome(filepath.Join(home, strings.Repeat("x", 300)))
		var output bytes.Buffer
		Expect(migrateLegacy(ctx, dirs, false, []string{"paths"}, &output)).To(Succeed())
		Expect(output.String()).To(ContainSubstring("Warning:"))
		Expect(migrateLegacy(ctx, dirs, false, []string{"install", "editor"}, &output)).To(HaveOccurred())
	})

	It("leaves safe legacy files in place during informational commands", func(ctx SpecContext) {
		home := GinkgoT().TempDir()
		testhelper.SetProfileEnv(GinkgoT(), home)
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
		Entry("agent init is read-only for DevEx state", []string{"agent", "init"}, true),
		Entry("agent init with flags is read-only for DevEx state", []string{"agent", "init", "--project-dir", "/tmp/demo", "--mcp"}, true),
		Entry("agent skills list is read-only", []string{"agent", "skills", "list"}, true),
		Entry("agent skills add is read-only", []string{"agent", "skills", "add", "tstack", "--agent", "claude", "--scope", "user"}, true),
		Entry("agent sandbox check is read-only", []string{"agent", "sandbox"}, true),
		Entry("agent sandbox json check is read-only", []string{"agent", "sandbox", "--format", "json"}, true),
		Entry("agent diff is read-only", []string{"agent", "diff"}, true),
		Entry("agent diff with flags is read-only", []string{"agent", "diff", "--agents", "claude", "--preset", "strict"}, true),
		Entry("agent apply bootstraps plugins", []string{"agent", "apply"}, false),
		Entry("agent doctor check is read-only", []string{"agent", "doctor"}, true),
		Entry("agent doctor json check is read-only", []string{"agent", "doctor", "--format", "json", "--agent", "claude"}, true),
		Entry("agent login handoff is read-only", []string{"agent", "login", "claude"}, true),
		Entry("agent group alone is not read-only", []string{"agent"}, false),
		Entry("mcp serve is read-only", []string{"mcp", "serve"}, true),
		Entry("mcp serve with a flag is read-only", []string{"mcp", "serve", "--request-timeout", "5s"}, true),
		Entry("bare mcp is not a read route", []string{"mcp"}, false),
		Entry("mcp serve with mutations records state", []string{"mcp", "serve", "--allow-mutations"}, false),
		Entry("mcp serve with mutations disabled is read-only", []string{"mcp", "serve", "--allow-mutations=false"}, true),
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
		Entry("mcp serve with mutations", []string{"mcp", "serve", "--allow-mutations"}, false),
		Entry("mcp serve with mutations disabled", []string{"mcp", "serve", "--allow-mutations=false"}, true),
		Entry("valued global flag before mcp serve with mutations", []string{"--install-concurrency", "4", "mcp", "serve", "--allow-mutations"}, false),
		Entry("mutation timeout without mutations", []string{"--apt-lists-max-age", "15m", "mcp", "serve", "--mutation-timeout", "5m"}, true),
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
		Entry("agent init", agentInitCommand, []string{"agent", "init"}, true),
		Entry("agent init after a global flag", agentInitCommand, []string{"--install-concurrency", "4", "agent", "init", "--project-dir", "/tmp/demo"}, true),
		Entry("agent init --help=false still runs", agentInitCommand, []string{"agent", "init", "--help=false"}, true),
		Entry("agent init --mcp requests MCP config", agentInitRequestsMCP, []string{"agent", "init", "--mcp"}, true),
		Entry("agent init without --mcp does not request MCP config", agentInitRequestsMCP, []string{"agent", "init"}, false),
		Entry("agent skills add", agentSkillsCommand, []string{"agent", "skills", "add", "tstack", "--agent", "claude", "--scope", "user"}, true),
		Entry("agent skills list after a global flag", agentSkillsCommand, []string{"--install-concurrency", "4", "agent", "skills", "list", "--format", "json"}, true),
		Entry("agent skills update", agentSkillsCommand, []string{"agent", "skills", "update", "tstack", "--dry-run"}, true),
		Entry("agent skills remove", agentSkillsCommand, []string{"agent", "skills", "remove", "tstack", "--agent", "claude", "--scope", "user"}, true),
		Entry("agent skills --help=false still runs", agentSkillsCommand, []string{"agent", "skills", "list", "--help=false"}, true),
		Entry("agent sandbox", agentSandboxCommand, []string{"agent", "sandbox"}, true),
		Entry("agent sandbox with format", agentSandboxCommand, []string{"--install-concurrency", "4", "agent", "sandbox", "--format", "json"}, true),
		Entry("agent sandbox --help=false still checks", agentSandboxCommand, []string{"agent", "sandbox", "--help=false"}, true),
		Entry("agent doctor", agentDoctorCommand, []string{"agent", "doctor"}, true),
		Entry("agent doctor with flags", agentDoctorCommand, []string{"--install-concurrency", "4", "agent", "doctor", "--format", "json", "--agent", "claude"}, true),
		Entry("agent doctor --help=false still checks", agentDoctorCommand, []string{"agent", "doctor", "--help=false"}, true),
		Entry("agent login", agentLoginCommand, []string{"agent", "login", "claude"}, true),
		Entry("agent login after a global flag", agentLoginCommand, []string{"--install-concurrency", "4", "agent", "login", "codex"}, true),
		Entry("agent login --help=false still runs", agentLoginCommand, []string{"agent", "login", "claude", "--help=false"}, true),
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
		Entry("root --version before an unknown command", presentationCommand, []string{"--version", "notacommand"}, true),
		Entry("root --help before an unknown command", presentationCommand, []string{"--help", "notacommand"}, true),
		Entry("unknown command", presentationCommand, []string{"notacommand"}, false),
		Entry("-h", presentationCommand, []string{"install", "-h"}, true),
	)
})
