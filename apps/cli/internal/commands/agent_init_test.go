package commands

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/testhelper"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/viper"
)

func dirSnapshot(root string) map[string][]byte {
	snap := map[string][]byte{}
	Expect(filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		Expect(err).NotTo(HaveOccurred())
		if path == root {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		Expect(err).NotTo(HaveOccurred())
		info, err := os.Lstat(path)
		Expect(err).NotTo(HaveOccurred())
		if info.Mode()&os.ModeSymlink != 0 {
			target, err := os.Readlink(path)
			Expect(err).NotTo(HaveOccurred())
			snap[filepath.ToSlash(rel)] = []byte("symlink:" + target)
			return nil
		}
		if entry.IsDir() {
			snap[filepath.ToSlash(rel)+"/"] = nil
			return nil
		}
		data, err := os.ReadFile(path)
		Expect(err).NotTo(HaveOccurred())
		snap[filepath.ToSlash(rel)] = data
		return nil
	})).To(Succeed())
	return snap
}

func runAgentInitRoot(settings config.CrossPlatformSettings, args ...string) (string, string, error) {
	root := NewRootCmd("test", nil, settings)
	var stdout, stderr bytes.Buffer
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	root.SetArgs(append([]string{"agent", "init"}, args...))
	err := root.Execute()
	return stdout.String(), stderr.String(), err
}

var _ = Describe("Agent init command", func() {
	var (
		home     string
		project  string
		settings config.CrossPlatformSettings
	)

	BeforeEach(func() {
		viper.Reset()
		home = GinkgoT().TempDir()
		testhelper.SetProfileEnv(GinkgoT(), home)
		GinkgoT().Setenv("DEVEX_DRY_RUN", "")
		GinkgoT().Setenv("DEVEX_DISABLE_UPDATE_CHECK", "true")
		GinkgoT().Setenv("DEVEX_OFFLINE", "true")

		project = filepath.Join(GinkgoT().TempDir(), "sample-go-app")
		Expect(os.MkdirAll(project, 0o755)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(project, "go.mod"), []byte("module example.com/sample\n\ngo 1.26\n"), 0o644)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(project, "Taskfile.yml"), []byte("version: '3'\ntasks:\n  test:\n    cmds: [go test ./...]\n  lint:\n    cmds: [golangci-lint run]\n"), 0o644)).To(Succeed())

		settings = config.CrossPlatformSettings{HomeDir: home}
	})

	AfterEach(func() {
		viper.Reset()
	})

	It("scaffolds AGENTS.md, CLAUDE.md, GEMINI.md, and .gitignore in a fresh repo", func() {
		stdout, stderr, err := runAgentInitRoot(settings, "--project-dir", project)
		Expect(err).NotTo(HaveOccurred())
		Expect(stderr).To(BeEmpty())

		Expect(stdout).To(And(
			ContainSubstring("Created: "+filepath.Join(project, "AGENTS.md")),
			ContainSubstring("Created: "+filepath.Join(project, "CLAUDE.md")),
			ContainSubstring("Created: "+filepath.Join(project, "GEMINI.md")),
			ContainSubstring("Created: "+filepath.Join(project, ".gitignore")),
		))

		agentsMD, err := os.ReadFile(filepath.Join(project, "AGENTS.md"))
		Expect(err).NotTo(HaveOccurred())
		Expect(string(agentsMD)).To(And(
			ContainSubstring("# sample-go-app"),
			ContainSubstring("Go"),
			ContainSubstring("`task`"),
			ContainSubstring("task test"),
			ContainSubstring("task lint"),
			ContainSubstring("## Conventions"),
			ContainSubstring("## Do Not Touch"),
		))

		claudeMD, err := os.ReadFile(filepath.Join(project, "CLAUDE.md"))
		Expect(err).NotTo(HaveOccurred())
		Expect(string(claudeMD)).To(ContainSubstring("@AGENTS.md"))
		Expect(string(claudeMD)).NotTo(ContainSubstring("## Conventions"))

		geminiMD, err := os.ReadFile(filepath.Join(project, "GEMINI.md"))
		Expect(err).NotTo(HaveOccurred())
		Expect(string(geminiMD)).To(ContainSubstring("@./AGENTS.md"))
		Expect(string(geminiMD)).NotTo(ContainSubstring("## Conventions"))

		gitignore, err := os.ReadFile(filepath.Join(project, ".gitignore"))
		Expect(err).NotTo(HaveOccurred())
		Expect(string(gitignore)).To(Equal("# DevEx agent-local files\n.claude/settings.local.json\nCLAUDE.local.md\n"))

		Expect(filepath.Join(project, ".mcp.json")).NotTo(BeAnExistingFile())
		Expect(filepath.Join(home, ".devex", "plugins")).NotTo(BeADirectory())
	})

	It("uses the current working directory when --project-dir is omitted", func() {
		GinkgoT().Chdir(project)
		_, _, err := runAgentInitRoot(settings)
		Expect(err).NotTo(HaveOccurred())
		Expect(filepath.Join(project, "AGENTS.md")).To(BeAnExistingFile())
		Expect(filepath.Join(project, "CLAUDE.md")).To(BeAnExistingFile())
		Expect(filepath.Join(project, "GEMINI.md")).To(BeAnExistingFile())
	})

	Describe("existing files handling", func() {
		It("reports identical files unchanged, skips differing files without --overwrite, and backs up under --overwrite", func() {
			customAgents := []byte("# Custom team instructions\nKeep this intact.\n")
			Expect(os.WriteFile(filepath.Join(project, "AGENTS.md"), customAgents, 0o644)).To(Succeed())

			// First run without --overwrite: AGENTS.md skipped, others created, exit 0.
			stdout, _, err := runAgentInitRoot(settings, "--project-dir", project)
			Expect(err).NotTo(HaveOccurred())
			Expect(stdout).To(ContainSubstring("Skipped (exists): " + filepath.Join(project, "AGENTS.md") + " (use --overwrite to replace)"))
			Expect(stdout).To(ContainSubstring("Created: " + filepath.Join(project, "CLAUDE.md")))
			Expect(stdout).To(ContainSubstring("Created: " + filepath.Join(project, "GEMINI.md")))
			Expect(os.ReadFile(filepath.Join(project, "AGENTS.md"))).To(Equal(customAgents))

			// Second run without --overwrite: CLAUDE.md, GEMINI.md, .gitignore are unchanged.
			stdout, _, err = runAgentInitRoot(settings, "--project-dir", project)
			Expect(err).NotTo(HaveOccurred())
			Expect(stdout).To(And(
				ContainSubstring("Skipped (exists): "+filepath.Join(project, "AGENTS.md")),
				ContainSubstring("Unchanged: "+filepath.Join(project, "CLAUDE.md")),
				ContainSubstring("Unchanged: "+filepath.Join(project, "GEMINI.md")),
				ContainSubstring("Unchanged: "+filepath.Join(project, ".gitignore")),
			))

			// Third run with --overwrite: AGENTS.md backed up and replaced; identical files untouched.
			stdout, _, err = runAgentInitRoot(settings, "--project-dir", project, "--overwrite")
			Expect(err).NotTo(HaveOccurred())
			Expect(stdout).To(And(
				ContainSubstring("Replaced: "+filepath.Join(project, "AGENTS.md")),
				ContainSubstring("Backup: "+filepath.Join(project, "AGENTS.md.devex-backup-")),
				ContainSubstring("Unchanged: "+filepath.Join(project, "CLAUDE.md")),
				ContainSubstring("Unchanged: "+filepath.Join(project, "GEMINI.md")),
			))

			backups, globErr := filepath.Glob(filepath.Join(project, "AGENTS.md.devex-backup-*"))
			Expect(globErr).NotTo(HaveOccurred())
			Expect(backups).To(HaveLen(1))
			Expect(os.ReadFile(backups[0])).To(Equal(customAgents))

			replaced, readErr := os.ReadFile(filepath.Join(project, "AGENTS.md"))
			Expect(readErr).NotTo(HaveOccurred())
			Expect(string(replaced)).To(ContainSubstring("# sample-go-app"))
		})
	})

	Describe("dry-run mode", func() {
		It("leaves the directory tree byte-identical on fresh and existing repos", func() {
			before := dirSnapshot(project)
			stdout, _, err := runAgentInitRoot(settings, "--project-dir", project, "--dry-run")
			Expect(err).NotTo(HaveOccurred())
			Expect(stdout).To(And(
				ContainSubstring("Would create: "+filepath.Join(project, "AGENTS.md")),
				ContainSubstring("Would create: "+filepath.Join(project, "CLAUDE.md")),
				ContainSubstring("Would create: "+filepath.Join(project, "GEMINI.md")),
				ContainSubstring("Would create: "+filepath.Join(project, ".gitignore")),
			))
			Expect(dirSnapshot(project)).To(Equal(before))

			// Also test existing differing file + existing .gitignore under --overwrite --dry-run.
			Expect(os.WriteFile(filepath.Join(project, "AGENTS.md"), []byte("custom"), 0o644)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(project, ".gitignore"), []byte("bin/\n"), 0o644)).To(Succeed())
			beforeExisting := dirSnapshot(project)

			stdout, _, err = runAgentInitRoot(settings, "--project-dir", project, "--overwrite", "--dry-run")
			Expect(err).NotTo(HaveOccurred())
			Expect(stdout).To(And(
				ContainSubstring("Would replace: "+filepath.Join(project, "AGENTS.md")),
				ContainSubstring("Would append: "+filepath.Join(project, ".gitignore")),
			))
			Expect(dirSnapshot(project)).To(Equal(beforeExisting))
		})

		It("honors DEVEX_DRY_RUN=1 through the root command", func() {
			GinkgoT().Setenv("DEVEX_DRY_RUN", "1")
			before := dirSnapshot(project)
			stdout, _, err := runAgentInitRoot(settings, "--project-dir", project)
			Expect(err).NotTo(HaveOccurred())
			Expect(stdout).To(ContainSubstring("Would create: " + filepath.Join(project, "AGENTS.md")))
			Expect(dirSnapshot(project)).To(Equal(before))
		})
	})

	Describe("path traversal and directory validation", func() {
		It("refuses when CLAUDE.md is a symlink outside the project and writes nothing", func() {
			outside := GinkgoT().TempDir()
			outsideFile := filepath.Join(outside, "victim.md")
			Expect(os.WriteFile(outsideFile, []byte("untouched"), 0o644)).To(Succeed())
			Expect(os.Symlink(outsideFile, filepath.Join(project, "CLAUDE.md"))).To(Succeed())

			before := dirSnapshot(project)
			_, _, err := runAgentInitRoot(settings, "--project-dir", project, "--overwrite")
			Expect(err).To(MatchError(ContainSubstring("symlink")))
			Expect(dirSnapshot(project)).To(Equal(before))
			Expect(os.ReadFile(outsideFile)).To(Equal([]byte("untouched")))
			Expect(filepath.Join(project, "AGENTS.md")).NotTo(BeAnExistingFile())
		})

		It("refuses when a destination is a directory and writes nothing", func() {
			Expect(os.MkdirAll(filepath.Join(project, "GEMINI.md"), 0o755)).To(Succeed())
			before := dirSnapshot(project)
			_, _, err := runAgentInitRoot(settings, "--project-dir", project, "--overwrite")
			Expect(err).To(MatchError(ContainSubstring("not a regular file")))
			Expect(dirSnapshot(project)).To(Equal(before))
		})

		It("fails without creating a nonexistent --project-dir", func() {
			missing := filepath.Join(GinkgoT().TempDir(), "does-not-exist")
			_, _, err := runAgentInitRoot(settings, "--project-dir", missing)
			Expect(err).To(MatchError(ContainSubstring("does not exist")))
			Expect(missing).NotTo(BeADirectory())
		})

		It("fails when --project-dir points at a regular file", func() {
			filePath := filepath.Join(GinkgoT().TempDir(), "not-a-dir")
			Expect(os.WriteFile(filePath, []byte("hi"), 0o644)).To(Succeed())
			_, _, err := runAgentInitRoot(settings, "--project-dir", filePath)
			Expect(err).To(MatchError(ContainSubstring("not a directory")))
		})
	})

	Describe(".gitignore handling", func() {
		It("preserves existing bytes, handles missing trailing newline, and is idempotent on rerun", func() {
			original := "bin/\n*.tmp"
			Expect(os.WriteFile(filepath.Join(project, ".gitignore"), []byte(original), 0o644)).To(Succeed())

			stdout, _, err := runAgentInitRoot(settings, "--project-dir", project)
			Expect(err).NotTo(HaveOccurred())
			Expect(stdout).To(ContainSubstring("Appended: " + filepath.Join(project, ".gitignore")))

			afterFirst, err := os.ReadFile(filepath.Join(project, ".gitignore"))
			Expect(err).NotTo(HaveOccurred())
			Expect(string(afterFirst)).To(Equal("bin/\n*.tmp\n# DevEx agent-local files\n.claude/settings.local.json\nCLAUDE.local.md\n"))

			stdout2, _, err := runAgentInitRoot(settings, "--project-dir", project)
			Expect(err).NotTo(HaveOccurred())
			Expect(stdout2).To(ContainSubstring("Unchanged: " + filepath.Join(project, ".gitignore")))
			Expect(os.ReadFile(filepath.Join(project, ".gitignore"))).To(Equal(afterFirst))
		})
	})

	Describe(".mcp.json generation with --mcp", func() {
		It("writes Claude-compatible stdio, http, and a portable devex server while excluding codex-only servers", func() {
			origLookPath := agentLookPath
			origExecutable := agentExecutable
			DeferCleanup(func() {
				agentLookPath = origLookPath
				agentExecutable = origExecutable
			})
			// The running binary is not the one on PATH: a committed .mcp.json
			// must still use the bare "devex" command, not this machine's path.
			agentExecutable = func() (string, error) { return filepath.Join(home, "build", "devex"), nil }
			agentLookPath = func(string) (string, error) { return "", exec.ErrNotFound }

			settings.Agents = config.AgentsConfig{
				Enabled:  []string{"claude", "codex"},
				DevExMCP: true,
				MCPServers: []config.AgentMCPServerConfig{
					{
						Name:      "github",
						Transport: config.MCPTransportStdio,
						Command:   "npx",
						Args:      []string{"-y", "@modelcontextprotocol/server-github"},
						Env: map[string]string{
							"GITHUB_TOKEN": "${env:GITHUB_TOKEN}",
						},
					},
					{
						Name:      "remote-api",
						Transport: config.MCPTransportHTTP,
						URL:       "https://mcp.example.com/sse",
						Headers: map[string]string{
							"Authorization": "Bearer ${env:API_TOKEN}",
						},
						Agents: []string{"claude"},
					},
					{
						Name:      "codex-only",
						Transport: config.MCPTransportStdio,
						Command:   "codex-server",
						Agents:    []string{"codex"},
					},
				},
			}

			stdout, _, err := runAgentInitRoot(settings, "--project-dir", project, "--mcp")
			Expect(err).NotTo(HaveOccurred())
			Expect(stdout).To(ContainSubstring("Created: " + filepath.Join(project, ".mcp.json")))

			raw, err := os.ReadFile(filepath.Join(project, ".mcp.json"))
			Expect(err).NotTo(HaveOccurred())
			Expect(strings.HasSuffix(string(raw), "\n")).To(BeTrue())

			var doc map[string]any
			Expect(json.Unmarshal(raw, &doc)).To(Succeed())
			servers := doc["mcpServers"].(map[string]any)
			Expect(servers).To(HaveLen(3))
			Expect(servers).NotTo(HaveKey("codex-only"))
			Expect(servers["github"].(map[string]any)["env"]).To(Equal(map[string]any{
				"GITHUB_TOKEN": "${GITHUB_TOKEN}",
			}))
			Expect(servers["remote-api"].(map[string]any)["headers"]).To(Equal(map[string]any{
				"Authorization": "Bearer ${API_TOKEN}",
			}))
			Expect(servers["devex"]).To(Equal(map[string]any{
				"type":    "stdio",
				"command": "devex",
				"args":    []any{"mcp", "serve"},
			}))
		})

		It("prints a skip notice and writes no .mcp.json when no servers are configured", func() {
			stdout, _, err := runAgentInitRoot(settings, "--project-dir", project, "--mcp")
			Expect(err).NotTo(HaveOccurred())
			Expect(stdout).To(ContainSubstring("Skipped (.mcp.json)"))
			Expect(filepath.Join(project, ".mcp.json")).NotTo(BeAnExistingFile())
			Expect(filepath.Join(project, "AGENTS.md")).To(BeAnExistingFile())
		})

		It("fails with nothing written when agents config contains a literal secret", func() {
			settings.Agents = config.AgentsConfig{
				Enabled: []string{"claude"},
				MCPServers: []config.AgentMCPServerConfig{
					{
						Name:      "github",
						Transport: config.MCPTransportStdio,
						Command:   "npx",
						Env: map[string]string{
							"GITHUB_TOKEN": "ghp_1234567890abcdef",
						},
					},
				},
			}
			before := dirSnapshot(project)
			_, _, err := runAgentInitRoot(settings, "--project-dir", project, "--mcp")
			Expect(err).To(HaveOccurred())
			Expect(dirSnapshot(project)).To(Equal(before))
		})
	})
})
