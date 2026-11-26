package commands_test

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"runtime"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/viper"

	"github.com/jameswlane/devex/apps/cli/internal/commands"
	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/mocks"
	"github.com/jameswlane/devex/apps/cli/internal/testhelper"
	"github.com/jameswlane/devex/apps/cli/internal/types"
	devexpaths "github.com/jameswlane/devex/packages/plugin-sdk/paths"
)

func captureCmdStdout(fn func() error) (string, error) {
	orig := os.Stdout
	r, w, err := os.Pipe()
	Expect(err).NotTo(HaveOccurred())
	os.Stdout = w
	defer func() { os.Stdout = orig }()

	done := make(chan string, 1)
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, r)
		done <- buf.String()
	}()

	runErr := fn()
	Expect(w.Close()).To(Succeed())
	out := <-done
	_ = r.Close()
	return out, runErr
}

var _ = Describe("Setup AI agents precedence through root command", func() {
	var (
		homeDir  string
		settings config.CrossPlatformSettings
	)

	BeforeEach(func() {
		if runtime.GOOS != "linux" {
			Skip("automated setup plan fixtures target Linux")
		}
		viper.Reset()
		DeferCleanup(viper.Reset)
		DeferCleanup(types.PinCatalogOSForTesting("linux"))
		DeferCleanup(types.PinCatalogArchForTesting("amd64"))

		homeDir = GinkgoT().TempDir()
		testhelper.SetProfileEnv(GinkgoT(), homeDir)
		GinkgoT().Setenv("DEVEX_TEAM_CONFIG_DIR", "")
		GinkgoT().Setenv("DEVEX_ENV", "dev")
		GinkgoT().Setenv("DEVEX_DISABLE_UPDATE_CHECK", "true")
		GinkgoT().Setenv("DEVEX_OFFLINE", "true")
		GinkgoT().Setenv("DEVEX_SETUP_AI_AGENTS_APPS", "")
		GinkgoT().Setenv("DEVEX_SETUP_AI_AGENTS_PRESET", "")

		settings = config.CrossPlatformSettings{HomeDir: homeDir}
		Expect(config.WriteDefaultConfig(settings.GetConfigDir())).To(Succeed())
		loaded, err := config.LoadCrossPlatformSettings(homeDir)
		Expect(err).NotTo(HaveOccurred())
		settings = loaded
	})

	runRootSetup := func(args ...string) (string, error) {
		viper.Reset()
		root := commands.NewRootCmd("test", mocks.NewMockRepository(), settings)
		var buf bytes.Buffer
		root.SetOut(&buf)
		root.SetErr(&buf)
		root.SetArgs(append([]string{"setup", "--non-interactive", "--dry-run"}, args...))
		stdout, err := captureCmdStdout(root.Execute)
		return stdout + buf.String(), err
	}

	It("selects no AI coding agents when nothing is configured", func() {
		out, err := runRootSetup()
		Expect(err).NotTo(HaveOccurred())
		Expect(out).NotTo(ContainSubstring("Claude Code"))
		Expect(out).NotTo(ContainSubstring("Bubblewrap"))
		Expect(out).NotTo(ContainSubstring("agents.yaml"))
	})

	It("reads setup.ai_agents.* from config file, overridden by env, overridden by flags", func() {
		cfgDir := devexpaths.ForHome(homeDir).Config
		Expect(os.MkdirAll(cfgDir, 0o750)).To(Succeed())
		cfgPath := filepath.Join(cfgDir, "config.yaml")
		Expect(os.WriteFile(cfgPath, []byte("setup:\n  ai_agents:\n    apps:\n      - Aider\n    preset: balanced\n"), 0o600)).To(Succeed())

		// 1. Config file alone -> Aider
		out, err := runRootSetup()
		Expect(err).NotTo(HaveOccurred())
		Expect(out).To(ContainSubstring("Aider"))
		Expect(out).NotTo(ContainSubstring("Claude Code"))

		// 2. Environment variable overrides config file -> Claude Code + Bubblewrap + socat + strict preset
		GinkgoT().Setenv("DEVEX_SETUP_AI_AGENTS_APPS", "Claude Code")
		GinkgoT().Setenv("DEVEX_SETUP_AI_AGENTS_PRESET", "strict")
		out, err = runRootSetup()
		Expect(err).NotTo(HaveOccurred())
		Expect(out).To(ContainSubstring("Claude Code"))
		Expect(out).To(ContainSubstring("Bubblewrap"))
		Expect(out).To(ContainSubstring("socat"))
		Expect(out).To(ContainSubstring("preset: strict"))
		Expect(out).NotTo(ContainSubstring("Aider"))

		// 3. CLI flags override both env and config file -> Codex CLI + Bubblewrap + balanced preset
		out, err = runRootSetup("--ai-agents", "Codex CLI", "--ai-agents-preset", "balanced")
		Expect(err).NotTo(HaveOccurred())
		Expect(out).To(ContainSubstring("Codex CLI"))
		Expect(out).To(ContainSubstring("Bubblewrap"))
		Expect(out).To(ContainSubstring("preset: balanced"))
		Expect(out).NotTo(ContainSubstring("Claude Code"))
		Expect(out).NotTo(ContainSubstring("Aider"))
	})

	It("fails cleanly on invalid agent names or presets through the root command", func() {
		_, err := runRootSetup("--ai-agents", "BogusAgent")
		Expect(err).To(MatchError(And(
			ContainSubstring(`unknown AI coding agent "BogusAgent"`),
			ContainSubstring("Claude Code"),
		)))

		_, err = runRootSetup("--ai-agents", "Claude Code", "--ai-agents-preset", "yolo")
		Expect(err).To(MatchError(ContainSubstring(config.ErrAgentsSkipPermissions)))
	})
})
