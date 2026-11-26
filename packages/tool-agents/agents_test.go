package main

import (
	"bytes"
	"context"
	"encoding/json"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type fileSnap struct {
	isDir bool
	mode  os.FileMode
	mtime time.Time
	data  []byte
}

func snapshotTree(root string) map[string]fileSnap {
	snap := map[string]fileSnap{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		info, err := os.Lstat(path)
		if err != nil {
			return err
		}
		entry := fileSnap{
			isDir: info.IsDir(),
			mode:  info.Mode(),
			mtime: info.ModTime(),
		}
		if info.Mode().IsRegular() {
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			entry.data = data
		}
		snap[filepath.ToSlash(rel)] = entry
		return nil
	})
	Expect(err).NotTo(HaveOccurred())
	return snap
}

func writeSpecFile(dir string, spec Spec) string {
	data, err := json.Marshal(spec)
	Expect(err).NotTo(HaveOccurred())
	path := filepath.Join(dir, "spec.json")
	Expect(os.WriteFile(path, data, 0o600)).To(Succeed())
	return path
}

func newTestEnv(home string, envVars map[string]string, installedBins ...string) Env {
	bins := map[string]bool{}
	for _, b := range installedBins {
		bins[b] = true
	}
	return Env{
		HomeDir: func() (string, error) { return home, nil },
		Getenv: func(key string) string {
			if envVars != nil {
				return envVars[key]
			}
			return ""
		},
		LookPath: func(file string) (string, error) {
			if bins[file] {
				return "/usr/local/bin/" + file, nil
			}
			return "", exec.ErrNotFound
		},
		StateDir: func(h string) string {
			return filepath.Join(h, ".local", "state", "devex")
		},
	}
}

var _ = Describe("tool-agents plugin", func() {
	var (
		home    string
		specDir string
	)

	BeforeEach(func() {
		home = GinkgoT().TempDir()
		specDir = GinkgoT().TempDir()
	})

	Describe("Per-agent end-to-end rendering", func() {
		DescribeTable("creates, re-applies idempotently, preserves user edits and unknown keys, updates commands, backs up once, and supports zero-write diff",
			func(agentName, binaryName, instructionsRel, settingsRel, initialSettings, expectedKeyOrderSubstr, userCommand, expectedPresetSubstr string) {
				env := newTestEnv(home, nil, binaryName)
				spec := Spec{
					Agents: []AgentSpec{
						{
							Name:            agentName,
							Preset:          PresetBalanced,
							Instructions:    "Prefer small diffs.\nRun tests before committing.",
							Model:           "provider/test-model",
							AllowedCommands: []string{"git status", "go test ./..."},
							DeniedCommands:  []string{"git push --force"},
						},
					},
				}
				specPath := writeSpecFile(specDir, spec)

				// 1. diff on clean HOME performs zero writes.
				beforeDiff := snapshotTree(home)
				var diffOut bytes.Buffer
				plugin := NewAgentsPluginWithDeps(env, &diffOut)
				Expect(plugin.ExecuteContext(context.Background(), "diff", []string{"--config", specPath})).To(Succeed())
				Expect(diffOut.String()).To(ContainSubstring("created:"))
				Expect(diffOut.String()).To(ContainSubstring("+++ b/"))
				Expect(snapshotTree(home)).To(Equal(beforeDiff))

				// apply --dry-run behaves identically to diff with zero writes.
				var dryOut bytes.Buffer
				pluginDry := NewAgentsPluginWithDeps(env, &dryOut)
				Expect(pluginDry.ExecuteContext(context.Background(), "apply", []string{"--config", specPath, "--dry-run"})).To(Succeed())
				Expect(dryOut.String()).To(Equal(diffOut.String()))
				Expect(snapshotTree(home)).To(Equal(beforeDiff))

				// 2. First apply creates native files with no backups (new files).
				var applyOut bytes.Buffer
				pluginApply := NewAgentsPluginWithDeps(env, &applyOut)
				Expect(pluginApply.ExecuteContext(context.Background(), "apply", []string{"--config", specPath})).To(Succeed())
				Expect(applyOut.String()).To(ContainSubstring("created:"))
				Expect(applyOut.String()).NotTo(ContainSubstring("backup:"))

				instPath := filepath.Join(home, filepath.FromSlash(instructionsRel))
				setPath := filepath.Join(home, filepath.FromSlash(settingsRel))

				instBytes, err := os.ReadFile(instPath)
				Expect(err).NotTo(HaveOccurred())
				Expect(string(instBytes)).To(And(
					ContainSubstring(BlockStart),
					ContainSubstring("Prefer small diffs."),
					ContainSubstring(BlockEnd),
				))

				setBytes, err := os.ReadFile(setPath)
				Expect(err).NotTo(HaveOccurred())
				Expect(string(setBytes)).To(ContainSubstring("provider/test-model"))
				Expect(string(setBytes)).To(ContainSubstring(expectedPresetSubstr))

				// 3. Re-applying unchanged config is a byte-identical no-op (mtime unchanged, no new backup).
				past := time.Now().Add(-10 * time.Second).Truncate(time.Second)
				Expect(os.Chtimes(instPath, past, past)).To(Succeed())
				Expect(os.Chtimes(setPath, past, past)).To(Succeed())
				instStatBefore, err := os.Stat(instPath)
				Expect(err).NotTo(HaveOccurred())
				setStatBefore, err := os.Stat(setPath)
				Expect(err).NotTo(HaveOccurred())

				var reapplyOut bytes.Buffer
				pluginReapply := NewAgentsPluginWithDeps(env, &reapplyOut)
				Expect(pluginReapply.ExecuteContext(context.Background(), "apply", []string{"--config", specPath})).To(Succeed())
				Expect(reapplyOut.String()).To(ContainSubstring("unchanged: " + instPath))
				Expect(reapplyOut.String()).To(ContainSubstring("unchanged: " + setPath))
				Expect(reapplyOut.String()).NotTo(ContainSubstring("updated:"))

				instStatAfter, err := os.Stat(instPath)
				Expect(err).NotTo(HaveOccurred())
				setStatAfter, err := os.Stat(setPath)
				Expect(err).NotTo(HaveOccurred())
				Expect(instStatAfter.ModTime()).To(Equal(instStatBefore.ModTime()))
				Expect(setStatAfter.ModTime()).To(Equal(setStatBefore.ModTime()))

				// 4. Seed pre-existing user content outside managed block + unknown keys/comments + user command entry.
				home2 := GinkgoT().TempDir()
				env2 := newTestEnv(home2, nil, binaryName)
				instPath2 := filepath.Join(home2, filepath.FromSlash(instructionsRel))
				setPath2 := filepath.Join(home2, filepath.FromSlash(settingsRel))
				Expect(os.MkdirAll(filepath.Dir(instPath2), 0o700)).To(Succeed())
				Expect(os.MkdirAll(filepath.Dir(setPath2), 0o700)).To(Succeed())
				Expect(os.WriteFile(instPath2, []byte("# User Notes\n\nKeep this untouched.\n"), 0o640)).To(Succeed())
				Expect(os.WriteFile(setPath2, []byte(initialSettings), 0o600)).To(Succeed())

				var mergeOut bytes.Buffer
				Expect(NewAgentsPluginWithDeps(env2, &mergeOut).ExecuteContext(context.Background(), "apply", []string{"--config", specPath})).To(Succeed())
				Expect(mergeOut.String()).To(ContainSubstring("updated: " + instPath2 + " (backup: "))
				Expect(mergeOut.String()).To(ContainSubstring("updated: " + setPath2 + " (backup: "))

				// Verify one backup created per existing file and mode preserved.
				instBackups, err := filepath.Glob(instPath2 + ".devex-bak-*")
				Expect(err).NotTo(HaveOccurred())
				Expect(instBackups).To(HaveLen(1))
				setBackups, err := filepath.Glob(setPath2 + ".devex-bak-*")
				Expect(err).NotTo(HaveOccurred())
				Expect(setBackups).To(HaveLen(1))
				if runtime.GOOS != "windows" {
					info, statErr := os.Stat(instPath2)
					Expect(statErr).NotTo(HaveOccurred())
					Expect(info.Mode().Perm()).To(Equal(os.FileMode(0o640)))
				}

				mergedInst, err := os.ReadFile(instPath2)
				Expect(err).NotTo(HaveOccurred())
				Expect(string(mergedInst)).To(HavePrefix("# User Notes\n\nKeep this untouched.\n\n" + BlockStart))

				mergedSet, err := os.ReadFile(setPath2)
				Expect(err).NotTo(HaveOccurred())
				Expect(string(mergedSet)).To(ContainSubstring(expectedKeyOrderSubstr))
				if userCommand != "" {
					Expect(string(mergedSet)).To(ContainSubstring(userCommand))
				}

				// 5. Drop one DevEx command and update instructions: second modify must NOT create a second backup,
				// must drop only the removed DevEx command, and must keep the user command and user instructions.
				spec2 := Spec{
					Agents: []AgentSpec{
						{
							Name:            agentName,
							Preset:          PresetStrict,
							Instructions:    "Updated instruction line.",
							Model:           "provider/test-model-v2",
							AllowedCommands: []string{"git status"},
							DeniedCommands:  nil,
						},
					},
				}
				specPath2 := writeSpecFile(specDir, spec2)
				var updateOut bytes.Buffer
				Expect(NewAgentsPluginWithDeps(env2, &updateOut).ExecuteContext(context.Background(), "apply", []string{"--config", specPath2})).To(Succeed())
				Expect(updateOut.String()).NotTo(ContainSubstring(instPath2 + " (backup:"))
				Expect(updateOut.String()).NotTo(ContainSubstring(setPath2 + " (backup:"))

				instBackups2, err := filepath.Glob(instPath2 + ".devex-bak-*")
				Expect(err).NotTo(HaveOccurred())
				Expect(instBackups2).To(HaveLen(1))
				setBackups2, err := filepath.Glob(setPath2 + ".devex-bak-*")
				Expect(err).NotTo(HaveOccurred())
				Expect(setBackups2).To(HaveLen(1))

				updatedInst, err := os.ReadFile(instPath2)
				Expect(err).NotTo(HaveOccurred())
				Expect(string(updatedInst)).To(ContainSubstring("Keep this untouched."))
				Expect(string(updatedInst)).To(ContainSubstring("Updated instruction line."))
				Expect(string(updatedInst)).NotTo(ContainSubstring("Prefer small diffs."))

				updatedSet, err := os.ReadFile(setPath2)
				Expect(err).NotTo(HaveOccurred())
				Expect(string(updatedSet)).To(ContainSubstring("provider/test-model-v2"))
				if agentName != AgentCodex {
					Expect(string(updatedSet)).To(ContainSubstring("git status"))
					Expect(string(updatedSet)).NotTo(ContainSubstring("go test ./..."))
					Expect(string(updatedSet)).NotTo(ContainSubstring("git push --force"))
					if userCommand != "" {
						Expect(string(updatedSet)).To(ContainSubstring(userCommand))
					}
				} else {
					rulesBytes, readErr := os.ReadFile(filepath.Join(home2, ".codex", "rules", "devex.rules"))
					Expect(readErr).NotTo(HaveOccurred())
					Expect(string(rulesBytes)).To(ContainSubstring(`pattern = ["git", "status"]`))
					Expect(string(rulesBytes)).NotTo(ContainSubstring("go"))
					Expect(string(rulesBytes)).NotTo(ContainSubstring("forbidden"))
				}

				// 6. Empty instructions removes the managed block while preserving user content.
				spec3 := Spec{
					Agents: []AgentSpec{
						{
							Name:  agentName,
							Model: "provider/test-model-v2",
						},
					},
				}
				specPath3 := writeSpecFile(specDir, spec3)
				var clearOut bytes.Buffer
				Expect(NewAgentsPluginWithDeps(env2, &clearOut).ExecuteContext(context.Background(), "apply", []string{"--config", specPath3})).To(Succeed())
				clearedInst, err := os.ReadFile(instPath2)
				Expect(err).NotTo(HaveOccurred())
				Expect(string(clearedInst)).To(Equal("# User Notes\n\nKeep this untouched.\n"))
			},
			Entry("claude",
				AgentClaude, "claude", ".claude/CLAUDE.md", ".claude/settings.json",
				"{\n  \"zebra\": true,\n  \"alpha\": 1,\n  \"permissions\": {\n    \"allow\": [\n      \"Bash(user-tool *)\"\n    ]\n  }\n}\n",
				"\"zebra\": true,\n  \"alpha\": 1,",
				"Bash(user-tool *)",
				"\"defaultMode\": \"acceptEdits\"",
			),
			Entry("codex",
				AgentCodex, "codex", ".codex/AGENTS.md", ".codex/config.toml",
				"# Top comment\n# Keep me\nnotify = [\"notify-send\"]\n\n[mcp_servers.demo]\ncommand = \"demo\"\n",
				"# Top comment\n# Keep me\nnotify = [\"notify-send\"]\nmodel = \"provider/test-model\"\napproval_policy = \"on-request\"\nsandbox_mode = \"workspace-write\"\n\n[mcp_servers.demo]",
				"",
				`sandbox_mode = "workspace-write"`,
			),
			Entry("gemini",
				AgentGemini, "gemini", ".gemini/GEMINI.md", ".gemini/settings.json",
				"{\n  \"zeta\": \"keep\",\n  \"tools\": {\n    \"allowed\": [\n      \"run_shell_command(user-cmd)\"\n    ]\n  },\n  \"alpha\": 42\n}\n",
				"\"zeta\": \"keep\",",
				"run_shell_command(user-cmd)",
				"\"defaultApprovalMode\": \"auto_edit\"",
			),
			Entry("opencode",
				AgentOpenCode, "opencode", ".config/opencode/AGENTS.md", ".config/opencode/opencode.json",
				"{\n  \"$schema\": \"https://opencode.ai/config.json\",\n  \"zebra\": 1,\n  \"permissions\": [\n    {\n      \"action\": \"shell\",\n      \"resource\": \"user-cmd *\",\n      \"effect\": \"allow\"\n    }\n  ]\n}\n",
				"\"$schema\": \"https://opencode.ai/config.json\",\n  \"zebra\": 1,",
				"user-cmd *",
				"\"effect\": \"allow\"",
			),
		)
	})

	Describe("Preset and permission safety", func() {
		It("leaves permission-mode keys untouched when preset is omitted", func() {
			env := newTestEnv(home, nil, "claude", "codex", "gemini", "opencode")
			Expect(os.MkdirAll(filepath.Join(home, ".claude"), 0o700)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(home, ".claude", "settings.json"), []byte("{\n  \"permissions\": {\n    \"defaultMode\": \"plan\"\n  }\n}\n"), 0o600)).To(Succeed())
			Expect(os.MkdirAll(filepath.Join(home, ".codex"), 0o700)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(home, ".codex", "config.toml"), []byte("approval_policy = \"on-request\"\nsandbox_mode = \"read-only\"\n"), 0o600)).To(Succeed())

			specPath := writeSpecFile(specDir, Spec{
				Agents: []AgentSpec{
					{Name: AgentClaude, Model: "claude-sonnet-5"},
					{Name: AgentCodex, Model: "gpt-5"},
				},
			})
			var out bytes.Buffer
			Expect(NewAgentsPluginWithDeps(env, &out).ExecuteContext(context.Background(), "apply", []string{"--config", specPath})).To(Succeed())

			claudeBytes, err := os.ReadFile(filepath.Join(home, ".claude", "settings.json"))
			Expect(err).NotTo(HaveOccurred())
			Expect(string(claudeBytes)).To(ContainSubstring(`"defaultMode": "plan"`))
			Expect(string(claudeBytes)).To(ContainSubstring(`"model": "claude-sonnet-5"`))

			codexBytes, err := os.ReadFile(filepath.Join(home, ".codex", "config.toml"))
			Expect(err).NotTo(HaveOccurred())
			Expect(string(codexBytes)).To(ContainSubstring(`sandbox_mode = "read-only"`))
			Expect(string(codexBytes)).To(ContainSubstring(`model = "gpt-5"`))
		})

		DescribeTable("rejects skip-permission and unsandboxed presets with explicit guidance",
			func(forbidden string) {
				env := newTestEnv(home, nil, "claude")
				specPath := writeSpecFile(specDir, Spec{
					Agents: []AgentSpec{{Name: AgentClaude, Preset: forbidden}},
				})
				var out bytes.Buffer
				err := NewAgentsPluginWithDeps(env, &out).ExecuteContext(context.Background(), "apply", []string{"--config", specPath})
				Expect(err).To(MatchError(ContainSubstring(ErrSkipPermissionsText)))
			},
			Entry("bypassPermissions", "bypassPermissions"),
			Entry("danger-full-access", "danger-full-access"),
			Entry("never", "never"),
			Entry("yolo", "yolo"),
			Entry("allow", "allow"),
			Entry("auto", "auto"),
			Entry("dontAsk", "dontAsk"),
		)
	})

	Describe("Installed detection and vendor home env overrides", func() {
		It("skips agents that are neither on PATH nor have an existing config directory", func() {
			env := newTestEnv(home, nil) // nothing on PATH, no dirs created
			specPath := writeSpecFile(specDir, Spec{
				Agents: []AgentSpec{
					{Name: AgentClaude, Instructions: "hello"},
					{Name: AgentCodex, Instructions: "hello"},
					{Name: AgentGemini, Instructions: "hello"},
					{Name: AgentOpenCode, Instructions: "hello"},
				},
			})
			before := snapshotTree(home)
			var out bytes.Buffer
			Expect(NewAgentsPluginWithDeps(env, &out).ExecuteContext(context.Background(), "apply", []string{"--config", specPath})).To(Succeed())
			Expect(out.String()).To(And(
				ContainSubstring("skipped (not installed): claude"),
				ContainSubstring("skipped (not installed): codex"),
				ContainSubstring("skipped (not installed): gemini"),
				ContainSubstring("skipped (not installed): opencode"),
			))
			Expect(snapshotTree(home)).To(Equal(before))
		})

		It("honours CLAUDE_CONFIG_DIR, CODEX_HOME, GEMINI_CLI_HOME, OPENCODE_CONFIG_DIR, and XDG_CONFIG_HOME", func() {
			claudeDir := filepath.Join(home, "custom-claude")
			codexDir := filepath.Join(home, "custom-codex")
			geminiRoot := filepath.Join(home, "custom-gemini-home")
			opencodeDir := filepath.Join(home, "custom-opencode")
			Expect(os.MkdirAll(claudeDir, 0o700)).To(Succeed())
			Expect(os.MkdirAll(codexDir, 0o700)).To(Succeed())
			Expect(os.MkdirAll(filepath.Join(geminiRoot, ".gemini"), 0o700)).To(Succeed())
			Expect(os.MkdirAll(opencodeDir, 0o700)).To(Succeed())

			env := newTestEnv(home, map[string]string{
				"CLAUDE_CONFIG_DIR":   claudeDir,
				"CODEX_HOME":          codexDir,
				"GEMINI_CLI_HOME":     geminiRoot,
				"OPENCODE_CONFIG_DIR": opencodeDir,
			})
			specPath := writeSpecFile(specDir, Spec{
				Agents: []AgentSpec{
					{Name: AgentClaude, Instructions: "hi"},
					{Name: AgentCodex, Instructions: "hi"},
					{Name: AgentGemini, Instructions: "hi"},
					{Name: AgentOpenCode, Instructions: "hi"},
				},
			})
			var out bytes.Buffer
			Expect(NewAgentsPluginWithDeps(env, &out).ExecuteContext(context.Background(), "apply", []string{"--config", specPath})).To(Succeed())
			Expect(filepath.Join(claudeDir, "CLAUDE.md")).To(BeAnExistingFile())
			Expect(filepath.Join(codexDir, "AGENTS.md")).To(BeAnExistingFile())
			Expect(filepath.Join(geminiRoot, ".gemini", "GEMINI.md")).To(BeAnExistingFile())
			Expect(filepath.Join(opencodeDir, "AGENTS.md")).To(BeAnExistingFile())

			// Also verify XDG_CONFIG_HOME fallback for OpenCode when OPENCODE_CONFIG_DIR is unset.
			xdgDir := filepath.Join(home, "xdg-cfg")
			Expect(ResolveAgentConfigDir(AgentOpenCode, home, func(k string) string {
				if k == "XDG_CONFIG_HOME" {
					return xdgDir
				}
				return ""
			})).To(Equal(filepath.Join(xdgDir, "opencode")))
		})
	})

	Describe("Error handling without writes", func() {
		DescribeTable("fails with no writes on malformed instruction block markers",
			func(badContent string) {
				env := newTestEnv(home, nil, "claude")
				claudeMD := filepath.Join(home, ".claude", "CLAUDE.md")
				Expect(os.MkdirAll(filepath.Dir(claudeMD), 0o700)).To(Succeed())
				Expect(os.WriteFile(claudeMD, []byte(badContent), 0o644)).To(Succeed())
				before := snapshotTree(home)

				specPath := writeSpecFile(specDir, Spec{
					Agents: []AgentSpec{{Name: AgentClaude, Instructions: "new text"}},
				})
				var out bytes.Buffer
				err := NewAgentsPluginWithDeps(env, &out).ExecuteContext(context.Background(), "apply", []string{"--config", specPath})
				Expect(err).To(HaveOccurred())
				Expect(snapshotTree(home)).To(Equal(before))
			},
			Entry("unterminated start marker", BlockStart+"\nhello\n"),
			Entry("end marker before start", BlockEnd+"\n"+BlockStart+"\n"),
			Entry("duplicate blocks", BlockStart+"\na\n"+BlockEnd+"\n"+BlockStart+"\nb\n"+BlockEnd+"\n"),
			Entry("nested start marker", BlockStart+"\n"+BlockStart+"\n"+BlockEnd+"\n"),
		)

		It("fails with no writes on invalid JSON in Claude settings.json", func() {
			env := newTestEnv(home, nil, "claude")
			path := filepath.Join(home, ".claude", "settings.json")
			Expect(os.MkdirAll(filepath.Dir(path), 0o700)).To(Succeed())
			Expect(os.WriteFile(path, []byte(`{"model": `), 0o600)).To(Succeed())
			before := snapshotTree(home)

			specPath := writeSpecFile(specDir, Spec{
				Agents: []AgentSpec{{Name: AgentClaude, Model: "sonnet"}},
			})
			var out bytes.Buffer
			err := NewAgentsPluginWithDeps(env, &out).ExecuteContext(context.Background(), "apply", []string{"--config", specPath})
			Expect(err).To(HaveOccurred())
			Expect(snapshotTree(home)).To(Equal(before))
		})

		It("fails clearly instead of stripping JSON comments in Gemini settings.json", func() {
			env := newTestEnv(home, nil, "gemini")
			path := filepath.Join(home, ".gemini", "settings.json")
			Expect(os.MkdirAll(filepath.Dir(path), 0o700)).To(Succeed())
			Expect(os.WriteFile(path, []byte("{\n  // user comment\n  \"model\": {\"name\": \"gemini-2.5-pro\"}\n}\n"), 0o600)).To(Succeed())
			before := snapshotTree(home)

			specPath := writeSpecFile(specDir, Spec{
				Agents: []AgentSpec{{Name: AgentGemini, Model: "gemini-2.5-flash"}},
			})
			var out bytes.Buffer
			err := NewAgentsPluginWithDeps(env, &out).ExecuteContext(context.Background(), "apply", []string{"--config", specPath})
			Expect(err).To(MatchError(ContainSubstring("comments")))
			Expect(snapshotTree(home)).To(Equal(before))
		})

		It("fails clearly instead of stripping comments in existing OpenCode opencode.jsonc", func() {
			env := newTestEnv(home, nil, "opencode")
			path := filepath.Join(home, ".config", "opencode", "opencode.jsonc")
			Expect(os.MkdirAll(filepath.Dir(path), 0o700)).To(Succeed())
			Expect(os.WriteFile(path, []byte("{\n  /* comment */\n  \"model\": \"anthropic/claude-sonnet-4-5\"\n}\n"), 0o600)).To(Succeed())
			before := snapshotTree(home)

			specPath := writeSpecFile(specDir, Spec{
				Agents: []AgentSpec{{Name: AgentOpenCode, Model: "anthropic/claude-opus-4-6"}},
			})
			var out bytes.Buffer
			err := NewAgentsPluginWithDeps(env, &out).ExecuteContext(context.Background(), "apply", []string{"--config", specPath})
			Expect(err).To(MatchError(ContainSubstring("comments")))
			Expect(snapshotTree(home)).To(Equal(before))
		})

		It("fails with no writes when a Codex owned key is defined in non-scalar table form", func() {
			env := newTestEnv(home, nil, "codex")
			path := filepath.Join(home, ".codex", "config.toml")
			Expect(os.MkdirAll(filepath.Dir(path), 0o700)).To(Succeed())
			Expect(os.WriteFile(path, []byte("approval_policy = { granular = { sandbox_approval = true } }\n"), 0o600)).To(Succeed())
			before := snapshotTree(home)

			specPath := writeSpecFile(specDir, Spec{
				Agents: []AgentSpec{{Name: AgentCodex, Preset: PresetBalanced}},
			})
			var out bytes.Buffer
			err := NewAgentsPluginWithDeps(env, &out).ExecuteContext(context.Background(), "apply", []string{"--config", specPath})
			Expect(err).To(MatchError(ContainSubstring("non-scalar/table")))
			Expect(snapshotTree(home)).To(Equal(before))
		})

		It("writes through symlinks without replacing the symlink", func() {
			if runtime.GOOS == "windows" {
				Skip("Unix symlink test")
			}
			env := newTestEnv(home, nil, "claude")
			dotfilesDir := filepath.Join(home, "dotfiles")
			Expect(os.MkdirAll(dotfilesDir, 0o700)).To(Succeed())
			target := filepath.Join(dotfilesDir, "CLAUDE.md")
			Expect(os.WriteFile(target, []byte("Original user line\n"), 0o644)).To(Succeed())

			claudeDir := filepath.Join(home, ".claude")
			Expect(os.MkdirAll(claudeDir, 0o700)).To(Succeed())
			link := filepath.Join(claudeDir, "CLAUDE.md")
			Expect(os.Symlink(target, link)).To(Succeed())

			specPath := writeSpecFile(specDir, Spec{
				Agents: []AgentSpec{{Name: AgentClaude, Instructions: "Managed line"}},
			})
			var out bytes.Buffer
			Expect(NewAgentsPluginWithDeps(env, &out).ExecuteContext(context.Background(), "apply", []string{"--config", specPath})).To(Succeed())

			info, err := os.Lstat(link)
			Expect(err).NotTo(HaveOccurred())
			Expect(info.Mode() & os.ModeSymlink).NotTo(BeZero())
			data, err := os.ReadFile(target)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(data)).To(And(
				ContainSubstring("Original user line"),
				ContainSubstring("Managed line"),
			))
		})

		It("deletes codex rules/devex.rules when all commands are removed", func() {
			env := newTestEnv(home, nil, "codex")
			specPath1 := writeSpecFile(specDir, Spec{
				Agents: []AgentSpec{{Name: AgentCodex, AllowedCommands: []string{"git status"}}},
			})
			var out1 bytes.Buffer
			Expect(NewAgentsPluginWithDeps(env, &out1).ExecuteContext(context.Background(), "apply", []string{"--config", specPath1})).To(Succeed())
			rulesPath := filepath.Join(home, ".codex", "rules", "devex.rules")
			Expect(rulesPath).To(BeAnExistingFile())

			specPath2 := writeSpecFile(specDir, Spec{
				Agents: []AgentSpec{{Name: AgentCodex, Model: "gpt-5"}},
			})
			var out2 bytes.Buffer
			Expect(NewAgentsPluginWithDeps(env, &out2).ExecuteContext(context.Background(), "apply", []string{"--config", specPath2})).To(Succeed())
			Expect(out2.String()).To(ContainSubstring("deleted: " + rulesPath))
			Expect(rulesPath).NotTo(BeAnExistingFile())
		})

		It("rejects unknown JSON fields, unsupported agents, duplicate agents, and multiline commands", func() {
			_, err := ParseSpec([]byte(`{"agents": [], "unknown": true}`))
			Expect(err).To(HaveOccurred())

			_, err = ParseSpec([]byte(`{"agents": [{"name": "unknown-agent"}]}`))
			Expect(err).To(MatchError(ContainSubstring("unsupported agent")))

			_, err = ParseSpec([]byte(`{"agents": [{"name": "claude"}, {"name": "claude"}]}`))
			Expect(err).To(MatchError(ContainSubstring("duplicate agent")))

			_, err = ParseSpec([]byte(`{"agents": [{"name": "claude", "allowed_commands": ["git status\nrm -rf /"]}]}`))
			Expect(err).To(MatchError(ContainSubstring("single line")))

			_, err = ParseSpec([]byte(`{"agents": [{"name": "claude", "allowed_commands": ["   "]}]}`))
			Expect(err).To(MatchError(ContainSubstring("must not be empty")))
		})
	})

	Describe("Preserves JSON key order across edits", func() {
		It("updates nested keys without re-sorting top-level or nested keys", func() {
			env := newTestEnv(home, nil, "claude")
			path := filepath.Join(home, ".claude", "settings.json")
			Expect(os.MkdirAll(filepath.Dir(path), 0o700)).To(Succeed())
			Expect(os.WriteFile(path, []byte("{\n  \"z_first\": 1,\n  \"model\": \"old\",\n  \"a_last\": 2\n}\n"), 0o600)).To(Succeed())

			specPath := writeSpecFile(specDir, Spec{
				Agents: []AgentSpec{{Name: AgentClaude, Model: "new"}},
			})
			var out bytes.Buffer
			Expect(NewAgentsPluginWithDeps(env, &out).ExecuteContext(context.Background(), "apply", []string{"--config", specPath})).To(Succeed())

			got, err := os.ReadFile(path)
			Expect(err).NotTo(HaveOccurred())
			text := string(got)
			Expect(strings.Index(text, `"z_first"`)).To(BeNumerically("<", strings.Index(text, `"model"`)))
			Expect(strings.Index(text, `"model"`)).To(BeNumerically("<", strings.Index(text, `"a_last"`)))
		})
	})
})
