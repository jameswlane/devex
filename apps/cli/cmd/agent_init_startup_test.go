package main_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"time"

	"github.com/jameswlane/devex/apps/cli/internal/testhelper"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Agent init startup", func() {
	It("honors --dry-run and DEVEX_DRY_RUN=1 with zero writes and runs offline without a usable datastore", func(ctx SpecContext) {
		if runtime.GOOS == "windows" {
			Skip("Unix dependency-free startup")
		}
		boundedCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()

		home := GinkgoT().TempDir()
		goBin, buildEnv := isolatedGoRunEnv(home)
		binary := testhelper.Executable(GinkgoT().TempDir(), "devex")
		build := exec.CommandContext(boundedCtx, goBin, "build", "-o", binary, "main.go")
		build.Env = goBuildEnv(buildEnv)
		output, err := build.CombinedOutput()
		Expect(err).NotTo(HaveOccurred(), string(output))

		state := filepath.Join(home, "state")
		Expect(os.MkdirAll(state, 0o700)).To(Succeed())
		datastore := filepath.Join(state, "datastore.db")
		Expect(os.WriteFile(datastore, []byte("not a sqlite database"), 0o600)).To(Succeed())

		project := filepath.Join(GinkgoT().TempDir(), "real-binary-project")
		Expect(os.MkdirAll(project, 0o755)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(project, "go.mod"), []byte("module example.com/real\n\ngo 1.26\n"), 0o644)).To(Succeed())

		runEnv := []string{
			"HOME=" + home,
			"PATH=",
			"DEVEX_CONFIG_DIR=" + filepath.Join(home, "config"),
			"DEVEX_STATE_DIR=" + state,
			"DEVEX_CACHE_DIR=" + filepath.Join(home, "cache"),
			"DEVEX_DATA_DIR=" + filepath.Join(home, "data"),
			"DEVEX_DISABLE_UPDATE_CHECK=true",
			"DEVEX_OFFLINE=true",
		}

		homeBefore := startupSnapshot(home)
		projectBefore := startupSnapshot(project)

		// 1. Explicit --dry-run flag: zero writes to project or DevEx state.
		dryFlagCmd := exec.CommandContext(boundedCtx, binary, "agent", "init", "--project-dir", project, "--dry-run")
		dryFlagCmd.Env = runEnv
		output, err = dryFlagCmd.CombinedOutput()
		Expect(err).NotTo(HaveOccurred(), string(output))
		Expect(string(output)).To(And(
			ContainSubstring("Would create: "+filepath.Join(project, "AGENTS.md")),
			ContainSubstring("Would create: "+filepath.Join(project, "CLAUDE.md")),
			ContainSubstring("Would create: "+filepath.Join(project, "GEMINI.md")),
			ContainSubstring("Would create: "+filepath.Join(project, ".gitignore")),
		))
		Expect(startupSnapshot(home)).To(Equal(homeBefore))
		Expect(startupSnapshot(project)).To(Equal(projectBefore))

		// 2. DEVEX_DRY_RUN=1 env var: zero writes to project or DevEx state.
		dryEnvCmd := exec.CommandContext(boundedCtx, binary, "agent", "init", "--project-dir", project)
		dryEnvCmd.Env = append(append([]string(nil), runEnv...), "DEVEX_DRY_RUN=1")
		output, err = dryEnvCmd.CombinedOutput()
		Expect(err).NotTo(HaveOccurred(), string(output))
		Expect(string(output)).To(ContainSubstring("Would create: " + filepath.Join(project, "AGENTS.md")))
		Expect(startupSnapshot(home)).To(Equal(homeBefore))
		Expect(startupSnapshot(project)).To(Equal(projectBefore))

		// 3. Real run writes project files without touching DevEx home/state or datastore.
		realCmd := exec.CommandContext(boundedCtx, binary, "agent", "init", "--project-dir", project)
		realCmd.Env = runEnv
		output, err = realCmd.CombinedOutput()
		Expect(err).NotTo(HaveOccurred(), string(output))
		Expect(string(output)).To(ContainSubstring("Created: " + filepath.Join(project, "AGENTS.md")))
		Expect(filepath.Join(project, "AGENTS.md")).To(BeAnExistingFile())
		Expect(filepath.Join(project, "CLAUDE.md")).To(BeAnExistingFile())
		Expect(filepath.Join(project, "GEMINI.md")).To(BeAnExistingFile())
		Expect(filepath.Join(project, ".gitignore")).To(BeAnExistingFile())
		Expect(startupSnapshot(home)).To(Equal(homeBefore))
		Expect(os.ReadFile(datastore)).To(Equal([]byte("not a sqlite database")))

		// 4. --mcp loads agents.yaml from user config dir ($DEVEX_CONFIG_DIR/config) and writes .mcp.json without touching DevEx state.
		userConfigDir := filepath.Join(home, "config", "config")
		Expect(os.MkdirAll(userConfigDir, 0o700)).To(Succeed())
		agentsYAML := `agents:
  enabled:
    - claude
  mcp_servers:
    - name: github
      transport: stdio
      command: npx
      args:
        - -y
        - "@modelcontextprotocol/server-github"
      env:
        GITHUB_TOKEN: "${env:GITHUB_TOKEN}"
`
		Expect(os.WriteFile(filepath.Join(userConfigDir, "agents.yaml"), []byte(agentsYAML), 0o600)).To(Succeed())
		homeWithConfig := startupSnapshot(home)

		mcpProject := filepath.Join(GinkgoT().TempDir(), "mcp-project")
		Expect(os.MkdirAll(mcpProject, 0o755)).To(Succeed())
		mcpCmd := exec.CommandContext(boundedCtx, binary, "agent", "init", "--project-dir", mcpProject, "--mcp")
		mcpCmd.Env = runEnv
		output, err = mcpCmd.CombinedOutput()
		Expect(err).NotTo(HaveOccurred(), string(output))
		Expect(string(output)).To(ContainSubstring("Created: " + filepath.Join(mcpProject, ".mcp.json")))
		mcpRaw, err := os.ReadFile(filepath.Join(mcpProject, ".mcp.json"))
		Expect(err).NotTo(HaveOccurred())
		Expect(string(mcpRaw)).To(ContainSubstring(`"GITHUB_TOKEN": "${GITHUB_TOKEN}"`))
		Expect(startupSnapshot(home)).To(Equal(homeWithConfig))
	})
})
