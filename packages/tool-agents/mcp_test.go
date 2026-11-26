package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type recordedCommand struct {
	Name string
	Args []string
}

// newFakeClaudeEnv builds an Env whose fake RunCommand simulates
// `claude mcp add-json --scope user <name> <json>` and
// `claude mcp remove --scope user <name>` directly on the resolved .claude.json,
// while recording every invocation.
func newFakeClaudeEnv(home string, envVars map[string]string, installedBins ...string) (Env, *[]recordedCommand) {
	env := newTestEnv(home, envVars, installedBins...)
	var calls []recordedCommand
	env.RunCommand = func(_ context.Context, name string, args ...string) ([]byte, error) {
		calls = append(calls, recordedCommand{Name: name, Args: append([]string(nil), args...)})
		claudeJSONPath := resolveClaudeMCPPath(home, env.Getenv)
		raw := map[string]any{}
		if data, err := os.ReadFile(claudeJSONPath); err == nil && len(bytes.TrimSpace(data)) > 0 {
			if unmarshalErr := json.Unmarshal(data, &raw); unmarshalErr != nil {
				return nil, unmarshalErr
			}
		}
		mcpServers, _ := raw["mcpServers"].(map[string]any)
		if mcpServers == nil {
			mcpServers = map[string]any{}
		}
		if len(args) >= 5 && args[0] == "mcp" && args[1] == "add-json" && args[2] == "--scope" && args[3] == "user" {
			srvName := args[4]
			var payload map[string]any
			if err := json.Unmarshal([]byte(args[5]), &payload); err != nil {
				return nil, fmt.Errorf("invalid add-json payload: %w", err)
			}
			mcpServers[srvName] = payload
			raw["mcpServers"] = mcpServers
		} else if len(args) >= 5 && args[0] == "mcp" && args[1] == "remove" && args[2] == "--scope" && args[3] == "user" {
			delete(mcpServers, args[4])
			raw["mcpServers"] = mcpServers
		} else {
			return nil, fmt.Errorf("unexpected fake claude args: %v", args)
		}
		if err := os.MkdirAll(filepath.Dir(claudeJSONPath), 0o700); err != nil {
			return nil, err
		}
		encoded, err := json.MarshalIndent(raw, "", "  ")
		if err != nil {
			return nil, err
		}
		encoded = append(encoded, '\n')
		if err := os.WriteFile(claudeJSONPath, encoded, 0o600); err != nil {
			return nil, err
		}
		return []byte("ok\n"), nil
	}
	return env, &calls
}

var _ = Describe("MCP server configuration", func() {
	var (
		home    string
		specDir string
	)

	BeforeEach(func() {
		home = GinkgoT().TempDir()
		specDir = GinkgoT().TempDir()
	})

	runPlugin := func(env Env, cmd string, spec Spec, extraArgs ...string) (string, error) {
		var out bytes.Buffer
		args := append([]string{"--config", writeSpecFile(specDir, spec)}, extraArgs...)
		err := NewAgentsPluginWithDeps(env, &out).ExecuteContext(context.Background(), cmd, args)
		return out.String(), err
	}

	It("parses valid stdio and http mcp_servers in the plugin spec", func() {
		raw := []byte(`{
  "agents": [
    {
      "name": "codex",
      "mcp_servers": [
        {
          "name": "github",
          "transport": "http",
          "url": "https://api.githubcopilot.com/mcp/",
          "headers": {
            "Authorization": "Bearer ${env:GITHUB_TOKEN}"
          }
        },
        {
          "name": "filesystem",
          "transport": "stdio",
          "command": "npx",
          "args": ["-y", "@modelcontextprotocol/server-filesystem", "/tmp"],
          "env": {
            "GITHUB_TOKEN": "${env:GITHUB_TOKEN}"
          }
        }
      ]
    }
  ]
}`)
		spec, err := ParseSpec(raw)
		Expect(err).NotTo(HaveOccurred())
		Expect(spec.Agents).To(HaveLen(1))
		Expect(spec.Agents[0].MCPServers).To(HaveLen(2))
	})

	Describe("Claude Code MCP rendering", func() {
		It("diffs only mcpServers, runs zero commands on dry-run, adds/updates/removes via CLI, preserves unmanaged servers, and is idempotent", func() {
			env, calls := newFakeClaudeEnv(home, nil, "claude")
			claudeJSON := filepath.Join(home, ".claude.json")
			Expect(os.WriteFile(claudeJSON, []byte(`{
  "userID": "keep-this-huge-unrelated-field",
  "projects": {"/work": {"allowedTools": []}},
  "mcpServers": {
    "user-custom": {
      "type": "stdio",
      "command": "my-custom-mcp",
      "args": ["--flag"]
    }
  }
}
`), 0o600)).To(Succeed())

			spec1 := Spec{
				Agents: []AgentSpec{
					{
						Name: AgentClaude,
						MCPServers: []MCPServerSpec{
							{
								Name:      "github",
								Transport: MCPTransportHTTP,
								URL:       "https://api.githubcopilot.com/mcp/",
								Headers: map[string]string{
									"Authorization": "Bearer ${env:GITHUB_TOKEN}",
									"X-Org":         "acme",
								},
							},
							{
								Name:      "filesystem",
								Transport: MCPTransportStdio,
								Command:   "npx",
								Args:      []string{"-y", "@modelcontextprotocol/server-filesystem", "/tmp"},
								Env: map[string]string{
									"GITHUB_TOKEN": "${env:GITHUB_TOKEN}",
									"LOG_LEVEL":    "info",
								},
							},
						},
					},
				},
			}

			// 1. Diff and apply --dry-run perform zero writes and zero commands,
			// and diff only shows mcpServers (not userID / projects).
			before := snapshotTree(home)
			diffOut, err := runPlugin(env, "diff", spec1)
			Expect(err).NotTo(HaveOccurred())
			Expect(*calls).To(BeEmpty())
			Expect(snapshotTree(home)).To(Equal(before))
			Expect(diffOut).To(ContainSubstring(claudeJSON + " (mcpServers)"))
			Expect(diffOut).To(ContainSubstring(`+  "github":`))
			Expect(diffOut).To(ContainSubstring(`"Authorization": "Bearer ${GITHUB_TOKEN}"`))
			Expect(diffOut).To(ContainSubstring(`"GITHUB_TOKEN": "${GITHUB_TOKEN}"`))
			Expect(diffOut).NotTo(ContainSubstring("keep-this-huge-unrelated-field"))

			dryOut, err := runPlugin(env, "apply", spec1, "--dry-run")
			Expect(err).NotTo(HaveOccurred())
			Expect(dryOut).To(Equal(diffOut))
			Expect(*calls).To(BeEmpty())
			Expect(snapshotTree(home)).To(Equal(before))

			// 2. Apply creates one-time backup and runs `claude mcp add-json --scope user`.
			applyOut, err := runPlugin(env, "apply", spec1)
			Expect(err).NotTo(HaveOccurred())
			Expect(applyOut).To(ContainSubstring("updated: " + claudeJSON + " (mcpServers) (backup: "))
			Expect(*calls).To(HaveLen(2))
			Expect((*calls)[0].Args[:5]).To(Equal([]string{"mcp", "add-json", "--scope", "user", "github"}))
			Expect((*calls)[0].Args[5]).To(ContainSubstring(`"type":"http"`))
			Expect((*calls)[0].Args[5]).To(ContainSubstring(`"Authorization":"Bearer ${GITHUB_TOKEN}"`))
			Expect((*calls)[1].Args[:5]).To(Equal([]string{"mcp", "add-json", "--scope", "user", "filesystem"}))
			Expect((*calls)[1].Args[5]).To(ContainSubstring(`"type":"stdio"`))
			Expect((*calls)[1].Args[5]).To(ContainSubstring(`"GITHUB_TOKEN":"${GITHUB_TOKEN}"`))

			backups, err := filepath.Glob(claudeJSON + ".devex-bak-*")
			Expect(err).NotTo(HaveOccurred())
			Expect(backups).To(HaveLen(1))

			// 3. Re-apply with identical spec runs ZERO commands and creates no second backup.
			*calls = nil
			reapplyOut, err := runPlugin(env, "apply", spec1)
			Expect(err).NotTo(HaveOccurred())
			Expect(*calls).To(BeEmpty())
			Expect(reapplyOut).To(ContainSubstring("unchanged: " + claudeJSON + " (mcpServers)"))
			backups2, err := filepath.Glob(claudeJSON + ".devex-bak-*")
			Expect(err).NotTo(HaveOccurred())
			Expect(backups2).To(HaveLen(1))

			// 4. Update github URL and drop filesystem -> removes filesystem, removes+re-adds github, keeps user-custom.
			*calls = nil
			spec2 := Spec{
				Agents: []AgentSpec{
					{
						Name: AgentClaude,
						MCPServers: []MCPServerSpec{
							{
								Name:      "github",
								Transport: MCPTransportHTTP,
								URL:       "https://api.githubcopilot.com/mcp/v2/",
								Headers: map[string]string{
									"Authorization": "Bearer ${env:GITHUB_TOKEN}",
								},
							},
						},
					},
				},
			}
			_, err = runPlugin(env, "apply", spec2)
			Expect(err).NotTo(HaveOccurred())
			Expect(*calls).To(HaveLen(3))
			Expect((*calls)[0].Args).To(Equal([]string{"mcp", "remove", "--scope", "user", "filesystem"}))
			Expect((*calls)[1].Args).To(Equal([]string{"mcp", "remove", "--scope", "user", "github"}))
			Expect((*calls)[2].Args[:5]).To(Equal([]string{"mcp", "add-json", "--scope", "user", "github"}))

			rawData, err := os.ReadFile(claudeJSON)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(rawData)).To(ContainSubstring(`"user-custom"`))
			Expect(string(rawData)).To(ContainSubstring(`"https://api.githubcopilot.com/mcp/v2/"`))
			Expect(string(rawData)).NotTo(ContainSubstring(`"filesystem"`))
		})

		It("honours CLAUDE_CONFIG_DIR for .claude.json location", func() {
			customDir := filepath.Join(home, "custom-claude")
			Expect(os.MkdirAll(customDir, 0o700)).To(Succeed())
			env, calls := newFakeClaudeEnv(home, map[string]string{"CLAUDE_CONFIG_DIR": customDir}, "claude")

			spec := Spec{
				Agents: []AgentSpec{{
					Name: AgentClaude,
					MCPServers: []MCPServerSpec{{
						Name:      "devex",
						Transport: MCPTransportStdio,
						Command:   "devex",
						Args:      []string{"mcp", "serve"},
					}},
				}},
			}
			out, err := runPlugin(env, "apply", spec)
			Expect(err).NotTo(HaveOccurred())
			Expect(*calls).To(HaveLen(1))
			Expect(out).To(ContainSubstring(filepath.Join(customDir, ".claude.json") + " (mcpServers)"))
			Expect(filepath.Join(customDir, ".claude.json")).To(BeAnExistingFile())
		})

		It("skips Claude MCP with a hint when the claude binary is not on PATH", func() {
			Expect(os.MkdirAll(filepath.Join(home, ".claude"), 0o700)).To(Succeed())
			env, calls := newFakeClaudeEnv(home, nil) // claude dir exists, binary NOT on PATH

			spec := Spec{
				Agents: []AgentSpec{{
					Name:         AgentClaude,
					Instructions: "Hello",
					MCPServers: []MCPServerSpec{{
						Name:      "devex",
						Transport: MCPTransportStdio,
						Command:   "devex",
						Args:      []string{"mcp", "serve"},
					}},
				}},
			}
			out, err := runPlugin(env, "apply", spec)
			Expect(err).NotTo(HaveOccurred())
			Expect(*calls).To(BeEmpty())
			Expect(out).To(ContainSubstring("claude binary not found on PATH"))
			Expect(filepath.Join(home, ".claude.json")).NotTo(BeAnExistingFile())
			Expect(filepath.Join(home, ".claude", "CLAUDE.md")).To(BeAnExistingFile())
		})

		It("errors on unmanaged conflict and adopts identical unmanaged Claude MCP server", func() {
			env, calls := newFakeClaudeEnv(home, nil, "claude")
			claudeJSON := filepath.Join(home, ".claude.json")
			Expect(os.WriteFile(claudeJSON, []byte(`{
  "mcpServers": {
    "github": {
      "type": "http",
      "url": "https://other.example.com/mcp"
    },
    "devex": {
      "command": "devex",
      "args": ["mcp", "serve"]
    }
  }
}
`), 0o600)).To(Succeed())

			// Conflict on github: zero commands, error returned.
			conflictSpec := Spec{
				Agents: []AgentSpec{{
					Name: AgentClaude,
					MCPServers: []MCPServerSpec{{
						Name:      "github",
						Transport: MCPTransportHTTP,
						URL:       "https://api.githubcopilot.com/mcp/",
					}},
				}},
			}
			_, err := runPlugin(env, "apply", conflictSpec)
			Expect(err).To(MatchError(ContainSubstring("not managed by DevEx")))
			Expect(*calls).To(BeEmpty())

			// Identical devex entry (even without explicit type:"stdio"): adopted with zero commands.
			adoptSpec := Spec{
				Agents: []AgentSpec{{
					Name: AgentClaude,
					MCPServers: []MCPServerSpec{{
						Name:      "devex",
						Transport: MCPTransportStdio,
						Command:   "devex",
						Args:      []string{"mcp", "serve"},
					}},
				}},
			}
			out, err := runPlugin(env, "apply", adoptSpec)
			Expect(err).NotTo(HaveOccurred())
			Expect(*calls).To(BeEmpty())
			Expect(out).To(ContainSubstring("unchanged:"))

			// Dropping adopted devex removes it now that DevEx owns it.
			_, err = runPlugin(env, "apply", Spec{Agents: []AgentSpec{{Name: AgentClaude}}})
			Expect(err).NotTo(HaveOccurred())
			Expect(*calls).To(HaveLen(1))
			Expect((*calls)[0].Args).To(Equal([]string{"mcp", "remove", "--scope", "user", "devex"}))
		})
	})

	Describe("Codex MCP rendering and TOML preservation", func() {
		It("adds, updates (including subtables), removes, preserves comments/unmanaged tables byte-for-byte, and is idempotent", func() {
			env := newTestEnv(home, nil, "codex")
			configPath := filepath.Join(home, ".codex", "config.toml")
			Expect(os.MkdirAll(filepath.Dir(configPath), 0o700)).To(Succeed())
			initial := strings.Join([]string{
				"# Top file comment",
				`model = "gpt-5"`,
				"",
				"# Unmanaged server comment",
				"[mcp_servers.user_local]",
				`command = "my-local-mcp"`,
				`args = ["--verbose"]`,
				"",
				"# Another section comment",
				"[profiles.fast]",
				`model = "gpt-5-mini"`,
				"",
			}, "\n")
			Expect(os.WriteFile(configPath, []byte(initial), 0o600)).To(Succeed())

			spec1 := Spec{
				Agents: []AgentSpec{{
					Name: AgentCodex,
					MCPServers: []MCPServerSpec{
						{
							Name:      "github",
							Transport: MCPTransportHTTP,
							URL:       "https://api.githubcopilot.com/mcp/",
							Headers: map[string]string{
								"Authorization": "Bearer ${env:GITHUB_TOKEN}",
								"X-Tenant-Key":  "${env:TENANT_KEY}",
								"X-Region":      "us-east-1",
							},
						},
						{
							Name:      "filesystem",
							Transport: MCPTransportStdio,
							Command:   "npx",
							Args:      []string{"-y", "@modelcontextprotocol/server-filesystem", "/tmp"},
							Env: map[string]string{
								"GITHUB_TOKEN": "${env:GITHUB_TOKEN}",
								"LOG_LEVEL":    "debug",
							},
						},
					},
				}},
			}

			// Dry-run does not touch disk.
			before := snapshotTree(home)
			diffOut, err := runPlugin(env, "diff", spec1)
			Expect(err).NotTo(HaveOccurred())
			Expect(diffOut).To(ContainSubstring("[mcp_servers.github]"))
			Expect(diffOut).To(ContainSubstring(`bearer_token_env_var = "GITHUB_TOKEN"`))
			Expect(diffOut).To(ContainSubstring(`env_http_headers = { X-Tenant-Key = "TENANT_KEY" }`))
			Expect(diffOut).To(ContainSubstring(`http_headers = { X-Region = "us-east-1" }`))
			Expect(diffOut).To(ContainSubstring(`env_vars = ["GITHUB_TOKEN"]`))
			Expect(diffOut).To(ContainSubstring(`env = { LOG_LEVEL = "debug" }`))
			Expect(snapshotTree(home)).To(Equal(before))

			// Apply writes config.toml and preserves existing comments & tables byte-for-byte.
			_, err = runPlugin(env, "apply", spec1)
			Expect(err).NotTo(HaveOccurred())
			got1, err := os.ReadFile(configPath)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(got1)).To(HavePrefix(initial))

			var parsed map[string]any
			_, err = toml.Decode(string(got1), &parsed)
			Expect(err).NotTo(HaveOccurred())

			// Idempotent re-apply leaves mtime untouched.
			past := time.Now().Add(-10 * time.Second).Truncate(time.Second)
			Expect(os.Chtimes(configPath, past, past)).To(Succeed())
			stBefore, err := os.Stat(configPath)
			Expect(err).NotTo(HaveOccurred())
			outReapply, err := runPlugin(env, "apply", spec1)
			Expect(err).NotTo(HaveOccurred())
			Expect(outReapply).To(ContainSubstring("unchanged: " + configPath))
			stAfter, err := os.Stat(configPath)
			Expect(err).NotTo(HaveOccurred())
			Expect(stAfter.ModTime()).To(Equal(stBefore.ModTime()))

			// Manually introduce a [mcp_servers.filesystem.env] subtable and a trailing section comment
			// to verify replacement/removal cleans subtables and preserves comments for following sections.
			withSubtable := strings.Join([]string{
				"# Top file comment",
				`model = "gpt-5"`,
				"",
				"[mcp_servers.filesystem]",
				`command = "npx"`,
				`args = ["-y", "@modelcontextprotocol/server-filesystem", "/tmp"]`,
				"",
				"[mcp_servers.filesystem.env]",
				`LOG_LEVEL = "debug"`,
				"",
				"# Comment belonging to user_local",
				"[mcp_servers.user_local]",
				`command = "my-local-mcp"`,
				"",
				"[mcp_servers.github]",
				`url = "https://api.githubcopilot.com/mcp/"`,
				`bearer_token_env_var = "GITHUB_TOKEN"`,
				"",
			}, "\n")
			Expect(os.WriteFile(configPath, []byte(withSubtable), 0o600)).To(Succeed())

			// Update github and remove filesystem.
			spec2 := Spec{
				Agents: []AgentSpec{{
					Name: AgentCodex,
					MCPServers: []MCPServerSpec{{
						Name:      "github",
						Transport: MCPTransportHTTP,
						URL:       "https://api.githubcopilot.com/mcp/v2/",
						Headers: map[string]string{
							"Authorization": "Bearer ${env:GITHUB_TOKEN}",
						},
					}},
				}},
			}
			_, err = runPlugin(env, "apply", spec2)
			Expect(err).NotTo(HaveOccurred())
			got2, err := os.ReadFile(configPath)
			Expect(err).NotTo(HaveOccurred())
			text2 := string(got2)
			Expect(text2).NotTo(ContainSubstring("[mcp_servers.filesystem"))
			Expect(text2).To(ContainSubstring("# Comment belonging to user_local\n[mcp_servers.user_local]\ncommand = \"my-local-mcp\""))
			Expect(text2).To(ContainSubstring(`url = "https://api.githubcopilot.com/mcp/v2/"`))

			// Removing the last managed server restores cleanly while keeping unmanaged user_local.
			_, err = runPlugin(env, "apply", Spec{Agents: []AgentSpec{{Name: AgentCodex}}})
			Expect(err).NotTo(HaveOccurred())
			got3, err := os.ReadFile(configPath)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(got3)).NotTo(ContainSubstring("[mcp_servers.github]"))
			Expect(string(got3)).To(ContainSubstring("[mcp_servers.user_local]"))
		})

		It("returns a clear error when a Codex env reference key differs from the referenced variable name", func() {
			env := newTestEnv(home, nil, "codex")
			spec := Spec{
				Agents: []AgentSpec{{
					Name: AgentCodex,
					MCPServers: []MCPServerSpec{{
						Name:      "github",
						Transport: MCPTransportStdio,
						Command:   "github-mcp",
						Env: map[string]string{
							"GH_TOKEN": "${env:GITHUB_TOKEN}",
						},
					}},
				}},
			}
			_, err := runPlugin(env, "apply", spec)
			Expect(err).To(MatchError(And(
				ContainSubstring("codex"),
				ContainSubstring("github"),
				ContainSubstring("GH_TOKEN"),
				ContainSubstring("GITHUB_TOKEN"),
			)))
		})

		It("rejects unmanaged Codex server conflict and adopts identical unmanaged server", func() {
			env := newTestEnv(home, nil, "codex")
			configPath := filepath.Join(home, ".codex", "config.toml")
			Expect(os.MkdirAll(filepath.Dir(configPath), 0o700)).To(Succeed())
			Expect(os.WriteFile(configPath, []byte("[mcp_servers.devex]\ncommand = \"devex\"\nargs = [\"mcp\", \"serve\"]\n\n[mcp_servers.github]\nurl = \"https://other.example.com\"\n"), 0o600)).To(Succeed())

			_, err := runPlugin(env, "apply", Spec{
				Agents: []AgentSpec{{
					Name: AgentCodex,
					MCPServers: []MCPServerSpec{{
						Name:      "github",
						Transport: MCPTransportHTTP,
						URL:       "https://api.githubcopilot.com/mcp/",
					}},
				}},
			})
			Expect(err).To(MatchError(ContainSubstring("not managed by DevEx")))

			// Adopting identical devex entry is unchanged and tracks ownership.
			out, err := runPlugin(env, "apply", Spec{
				Agents: []AgentSpec{{
					Name: AgentCodex,
					MCPServers: []MCPServerSpec{{
						Name:      "devex",
						Transport: MCPTransportStdio,
						Command:   "devex",
						Args:      []string{"mcp", "serve"},
					}},
				}},
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(out).To(ContainSubstring("unchanged: " + configPath))

			// Removing adopted devex removes only devex, keeping github.
			_, err = runPlugin(env, "apply", Spec{Agents: []AgentSpec{{Name: AgentCodex}}})
			Expect(err).NotTo(HaveOccurred())
			after, err := os.ReadFile(configPath)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(after)).NotTo(ContainSubstring("[mcp_servers.devex]"))
			Expect(string(after)).To(ContainSubstring("[mcp_servers.github]"))
		})
	})

	Describe("Gemini CLI and OpenCode MCP rendering", func() {
		DescribeTable("adds, updates, removes, preserves unmanaged servers, rejects conflicts, adopts identical, and is idempotent",
			func(agentName, binaryName, settingsRel, rootKey, initialJSON, expectedHTTPField, expectedEnvField, expectedRef string) {
				env := newTestEnv(home, nil, binaryName)
				settingsPath := filepath.Join(home, filepath.FromSlash(settingsRel))
				Expect(os.MkdirAll(filepath.Dir(settingsPath), 0o700)).To(Succeed())
				Expect(os.WriteFile(settingsPath, []byte(initialJSON), 0o600)).To(Succeed())

				spec1 := Spec{
					Agents: []AgentSpec{{
						Name: agentName,
						MCPServers: []MCPServerSpec{
							{
								Name:      "github",
								Transport: MCPTransportHTTP,
								URL:       "https://api.githubcopilot.com/mcp/",
								Headers: map[string]string{
									"Authorization": "Bearer ${env:GITHUB_TOKEN}",
								},
							},
							{
								Name:      "filesystem",
								Transport: MCPTransportStdio,
								Command:   "npx",
								Args:      []string{"-y", "@modelcontextprotocol/server-filesystem", "/tmp"},
								Env: map[string]string{
									"GITHUB_TOKEN": "${env:GITHUB_TOKEN}",
								},
							},
						},
					}},
				}

				// Dry-run makes zero writes.
				before := snapshotTree(home)
				diffOut, err := runPlugin(env, "diff", spec1)
				Expect(err).NotTo(HaveOccurred())
				Expect(diffOut).To(ContainSubstring(expectedHTTPField))
				Expect(diffOut).To(ContainSubstring(expectedEnvField))
				Expect(diffOut).To(ContainSubstring(expectedRef))
				Expect(snapshotTree(home)).To(Equal(before))

				// Apply adds both servers and preserves unmanaged server + backup.
				applyOut, err := runPlugin(env, "apply", spec1)
				Expect(err).NotTo(HaveOccurred())
				Expect(applyOut).To(ContainSubstring("updated: " + settingsPath + " (backup: "))

				data1, err := os.ReadFile(settingsPath)
				Expect(err).NotTo(HaveOccurred())
				Expect(string(data1)).To(ContainSubstring(`"user-existing"`))
				Expect(string(data1)).To(ContainSubstring(`"github"`))
				Expect(string(data1)).To(ContainSubstring(`"filesystem"`))
				Expect(string(data1)).To(ContainSubstring(expectedHTTPField))
				Expect(string(data1)).To(ContainSubstring("Bearer " + expectedRef))
				if agentName == AgentOpenCode {
					Expect(string(data1)).To(ContainSubstring(`"type": "local"`))
					Expect(string(data1)).To(ContainSubstring(`"type": "remote"`))
					Expect(string(data1)).To(ContainSubstring(`"enabled": true`))
					Expect(string(data1)).To(ContainSubstring(`"npx"`))
				}

				// Idempotent re-apply leaves mtime untouched.
				past := time.Now().Add(-10 * time.Second).Truncate(time.Second)
				Expect(os.Chtimes(settingsPath, past, past)).To(Succeed())
				stBefore, err := os.Stat(settingsPath)
				Expect(err).NotTo(HaveOccurred())
				reapplyOut, err := runPlugin(env, "apply", spec1)
				Expect(err).NotTo(HaveOccurred())
				Expect(reapplyOut).To(ContainSubstring("unchanged: " + settingsPath))
				stAfter, err := os.Stat(settingsPath)
				Expect(err).NotTo(HaveOccurred())
				Expect(stAfter.ModTime()).To(Equal(stBefore.ModTime()))

				// Update github and remove filesystem.
				spec2 := Spec{
					Agents: []AgentSpec{{
						Name: agentName,
						MCPServers: []MCPServerSpec{{
							Name:      "github",
							Transport: MCPTransportHTTP,
							URL:       "https://api.githubcopilot.com/mcp/v2/",
							Headers: map[string]string{
								"Authorization": "Bearer ${env:GITHUB_TOKEN}",
							},
						}},
					}},
				}
				_, err = runPlugin(env, "apply", spec2)
				Expect(err).NotTo(HaveOccurred())
				data2, err := os.ReadFile(settingsPath)
				Expect(err).NotTo(HaveOccurred())
				Expect(string(data2)).To(ContainSubstring(`"user-existing"`))
				Expect(string(data2)).To(ContainSubstring(`"https://api.githubcopilot.com/mcp/v2/"`))
				Expect(string(data2)).NotTo(ContainSubstring(`"filesystem"`))

				// Conflict on unmanaged user-existing server with different config fails without writing.
				beforeConflict := snapshotTree(home)
				_, err = runPlugin(env, "apply", Spec{
					Agents: []AgentSpec{{
						Name: agentName,
						MCPServers: []MCPServerSpec{{
							Name:      "user-existing",
							Transport: MCPTransportStdio,
							Command:   "different-cmd",
						}},
					}},
				})
				Expect(err).To(MatchError(ContainSubstring("not managed by DevEx")))
				Expect(snapshotTree(home)).To(Equal(beforeConflict))

				// Remove github; unmanaged user-existing remains under rootKey.
				_, err = runPlugin(env, "apply", Spec{Agents: []AgentSpec{{Name: agentName}}})
				Expect(err).NotTo(HaveOccurred())
				data3, err := os.ReadFile(settingsPath)
				Expect(err).NotTo(HaveOccurred())
				Expect(string(data3)).To(ContainSubstring(`"` + rootKey + `"`))
				Expect(string(data3)).To(ContainSubstring(`"user-existing"`))
				Expect(string(data3)).NotTo(ContainSubstring(`"github"`))
			},
			Entry("gemini",
				AgentGemini, "gemini", ".gemini/settings.json", "mcpServers",
				"{\n  \"mcpServers\": {\n    \"user-existing\": {\n      \"command\": \"keep-me\"\n    }\n  }\n}\n",
				`"httpUrl": "https://api.githubcopilot.com/mcp/"`,
				`"env": {`,
				`${GITHUB_TOKEN}`,
			),
			Entry("opencode",
				AgentOpenCode, "opencode", ".config/opencode/opencode.json", "mcp",
				"{\n  \"mcp\": {\n    \"user-existing\": {\n      \"type\": \"local\",\n      \"command\": [\"keep-me\"],\n      \"enabled\": true\n    }\n  }\n}\n",
				`"url": "https://api.githubcopilot.com/mcp/"`,
				`"environment": {`,
				`{env:GITHUB_TOKEN}`,
			),
		)
	})

	Describe("Secret policy and existing literal-secret warnings", func() {
		DescribeTable("rejects literal secrets in spec without echoing the secret value",
			func(srv MCPServerSpec, secretVal, expectedFieldSubstr string) {
				_, err := ParseSpec(mustMarshalSpec(Spec{
					Agents: []AgentSpec{{
						Name:       AgentCodex,
						MCPServers: []MCPServerSpec{srv},
					}},
				}))
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring(srv.Name))
				Expect(err.Error()).To(ContainSubstring(expectedFieldSubstr))
				Expect(err.Error()).NotTo(ContainSubstring(secretVal))
			},
			Entry("secret env key with plain literal",
				MCPServerSpec{Name: "gh", Transport: MCPTransportStdio, Command: "mcp", Env: map[string]string{"GITHUB_TOKEN": "super-secret-value"}},
				"super-secret-value", "env.GITHUB_TOKEN",
			),
			Entry("PAT env key with plain literal",
				MCPServerSpec{Name: "az", Transport: MCPTransportStdio, Command: "mcp", Env: map[string]string{"AZURE_PAT": "my-pat-value"}},
				"my-pat-value", "env.AZURE_PAT",
			),
			Entry("Authorization header with literal Bearer token",
				MCPServerSpec{Name: "api", Transport: MCPTransportHTTP, URL: "https://api.example.com/mcp", Headers: map[string]string{"Authorization": "Bearer literal-bearer-token"}},
				"literal-bearer-token", "headers.Authorization",
			),
			Entry("non-secret env key containing ghp_ token",
				MCPServerSpec{Name: "gh", Transport: MCPTransportStdio, Command: "mcp", Env: map[string]string{"EXTRA": "ghp_1234567890abcdef"}},
				"ghp_1234567890abcdef", "env.EXTRA",
			),
			Entry("non-secret header containing sk-ant- key",
				MCPServerSpec{Name: "ai", Transport: MCPTransportHTTP, URL: "https://api.example.com/mcp", Headers: map[string]string{"X-Custom": "sk-ant-secret123456"}},
				"sk-ant-secret123456", "headers.X-Custom",
			),
			Entry("stdio arg containing AKIA AWS key",
				MCPServerSpec{Name: "aws", Transport: MCPTransportStdio, Command: "mcp", Args: []string{"--key=AKIAIOSFODNN7EXAMPLE"}},
				"AKIAIOSFODNN7EXAMPLE", "args[0]",
			),
			Entry("HTTP URL query parameter with token",
				MCPServerSpec{Name: "q", Transport: MCPTransportHTTP, URL: "https://api.example.com/mcp?api_key=raw-query-secret"},
				"raw-query-secret", "url",
			),
			Entry("HTTP URL with embedded userinfo credentials",
				MCPServerSpec{Name: "u", Transport: MCPTransportHTTP, URL: "https://user:raw-pass-secret@api.example.com/mcp"},
				"raw-pass-secret", "url",
			),
			Entry("keyring reference unsupported in v1",
				MCPServerSpec{Name: "kr", Transport: MCPTransportStdio, Command: "mcp", Env: map[string]string{"GITHUB_TOKEN": "${keyring:github}"}},
				"${keyring:github}", "keyring",
			),
		)

		It("allows non-secret literal env vars such as PATH and LOG_LEVEL", func() {
			_, err := ParseSpec(mustMarshalSpec(Spec{
				Agents: []AgentSpec{{
					Name: AgentCodex,
					MCPServers: []MCPServerSpec{{
						Name:      "fs",
						Transport: MCPTransportStdio,
						Command:   "npx",
						Env: map[string]string{
							"PATH":      "/usr/local/bin:/usr/bin",
							"LOG_LEVEL": "info",
						},
					}},
				}},
			}))
			Expect(err).NotTo(HaveOccurred())
		})

		It("warns without echoing values when existing native MCP configs contain literal secrets (and ignores safe native refs)", func() {
			env, _ := newFakeClaudeEnv(home, nil, "claude", "codex", "gemini", "opencode")

			claudeSecret := "ghp_claudeSecretToken12345"
			codexSecret := "sk-ant-codexSecret12345"
			geminiSecret := "AIzaSyGeminiLiteralSecret12345"
			opencodeSecret := "glpat-opencodeSecret12345"

			Expect(os.WriteFile(filepath.Join(home, ".claude.json"), []byte(fmt.Sprintf(`{
  "mcpServers": {
    "safe": {"type": "http", "url": "https://safe.example.com", "headers": {"Authorization": "Bearer ${SAFE_TOKEN}"}},
    "leaky-claude": {"type": "http", "url": "https://leaky.example.com", "headers": {"Authorization": "Bearer %s"}}
  }
}
`, claudeSecret)), 0o600)).To(Succeed())

			Expect(os.MkdirAll(filepath.Join(home, ".codex"), 0o700)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(home, ".codex", "config.toml"), []byte(fmt.Sprintf(`
[mcp_servers.safe_codex]
url = "https://safe.example.com"
bearer_token_env_var = "SAFE_TOKEN"
env_http_headers = { "X-API-Key" = "SAFE_KEY" }

[mcp_servers.leaky_codex]
command = "mcp"
env = { OPENAI_API_KEY = "%s" }
`, codexSecret)), 0o600)).To(Succeed())

			Expect(os.MkdirAll(filepath.Join(home, ".gemini"), 0o700)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(home, ".gemini", "settings.json"), []byte(fmt.Sprintf(`{
  "mcpServers": {
    "safe": {"httpUrl": "https://safe.example.com", "headers": {"Authorization": "Bearer $SAFE_TOKEN"}},
    "leaky-gemini": {"command": "mcp", "env": {"GOOGLE_KEY": "%s"}}
  }
}
`, geminiSecret)), 0o600)).To(Succeed())

			Expect(os.MkdirAll(filepath.Join(home, ".config", "opencode"), 0o700)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(home, ".config", "opencode", "opencode.json"), []byte(fmt.Sprintf(`{
  "mcp": {
    "safe": {"type": "remote", "url": "https://safe.example.com", "headers": {"Authorization": "Bearer {env:SAFE_TOKEN}"}, "enabled": true},
    "leaky-opencode": {"type": "local", "command": ["mcp"], "environment": {"GITLAB_TOKEN": "%s"}, "enabled": true}
  }
}
`, opencodeSecret)), 0o600)).To(Succeed())

			spec := Spec{
				Agents: []AgentSpec{
					{Name: AgentClaude},
					{Name: AgentCodex},
					{Name: AgentGemini},
					{Name: AgentOpenCode},
				},
			}

			for _, subcmd := range []string{"diff", "apply"} {
				out, err := runPlugin(env, subcmd, spec)
				Expect(err).NotTo(HaveOccurred())
				Expect(out).To(ContainSubstring(`warning: claude MCP server "leaky-claude" field headers.Authorization`))
				Expect(out).To(ContainSubstring(`warning: codex MCP server "leaky_codex" field env.OPENAI_API_KEY`))
				Expect(out).To(ContainSubstring(`warning: gemini MCP server "leaky-gemini" field env.GOOGLE_KEY`))
				Expect(out).To(ContainSubstring(`warning: opencode MCP server "leaky-opencode" field environment.GITLAB_TOKEN`))
				Expect(out).NotTo(ContainSubstring(`"safe"`))
				Expect(out).NotTo(ContainSubstring(`"safe_codex"`))
				Expect(out).NotTo(ContainSubstring(claudeSecret))
				Expect(out).NotTo(ContainSubstring(codexSecret))
				Expect(out).NotTo(ContainSubstring(geminiSecret))
				Expect(out).NotTo(ContainSubstring(opencodeSecret))
			}
		})

		It("enforces secret policy for Cookie, Proxy-Authorization, secret CLI flags in args, and allows AUTH_MODE", func() {
			for _, tc := range []struct {
				srv    MCPServerSpec
				secret string
			}{
				{
					srv:    MCPServerSpec{Name: "c1", Transport: MCPTransportHTTP, URL: "https://api.example.com/mcp", Headers: map[string]string{"Cookie": "session=raw-cookie-123"}},
					secret: "raw-cookie-123",
				},
				{
					srv:    MCPServerSpec{Name: "c2", Transport: MCPTransportHTTP, URL: "https://api.example.com/mcp", Headers: map[string]string{"Proxy-Authorization": "Basic dXNlcjpwYXNz"}},
					secret: "dXNlcjpwYXNz",
				},
				{
					srv:    MCPServerSpec{Name: "c3", Transport: MCPTransportStdio, Command: "mcp", Args: []string{"--token", "plain-arg-secret"}},
					secret: "plain-arg-secret",
				},
				{
					srv:    MCPServerSpec{Name: "c4", Transport: MCPTransportStdio, Command: "mcp", Args: []string{"--API-KEY=plain-eq-secret"}},
					secret: "plain-eq-secret",
				},
				{
					srv:    MCPServerSpec{Name: "c5", Transport: MCPTransportStdio, Command: "mcp", Args: []string{"--auth-token", "${env:MY_TOKEN}"}},
					secret: "${env:MY_TOKEN}",
				},
			} {
				_, err := ParseSpec(mustMarshalSpec(Spec{
					Agents: []AgentSpec{{Name: AgentCodex, MCPServers: []MCPServerSpec{tc.srv}}},
				}))
				Expect(err).To(HaveOccurred(), "expected %s to fail validation", tc.srv.Name)
				Expect(err.Error()).NotTo(ContainSubstring(tc.secret))
			}

			// AUTH_MODE=oauth must be allowed (AUTH alone is too broad).
			_, err := ParseSpec(mustMarshalSpec(Spec{
				Agents: []AgentSpec{{
					Name: AgentCodex,
					MCPServers: []MCPServerSpec{{
						Name:      "ok-auth-mode",
						Transport: MCPTransportStdio,
						Command:   "mcp",
						Env:       map[string]string{"AUTH_MODE": "oauth"},
					}},
				}},
			}))
			Expect(err).NotTo(HaveOccurred())

			// Existing-config scanners warn on Cookie, Proxy-Authorization, and secret args flags,
			// and do not warn on AUTH_MODE=oauth.
			env, _ := newFakeClaudeEnv(home, nil, "claude", "codex")
			Expect(os.WriteFile(filepath.Join(home, ".claude.json"), []byte(`{
  "mcpServers": {
    "ok": {"type": "stdio", "command": "mcp", "env": {"AUTH_MODE": "oauth"}},
    "bad-hdr": {"type": "http", "url": "https://api.example.com", "headers": {"Cookie": "sid=xyz", "Proxy-Authorization": "Basic abc"}},
    "bad-arg": {"type": "stdio", "command": "mcp", "args": ["--password", "lit-pass"]}
  }
}
`), 0o600)).To(Succeed())
			Expect(os.MkdirAll(filepath.Join(home, ".codex"), 0o700)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(home, ".codex", "config.toml"), []byte(`
[mcp_servers.ok_codex]
command = "mcp"
env = { AUTH_MODE = "oauth" }

[mcp_servers.bad_codex]
command = "mcp"
args = ["--access-token=lit-tok"]
http_headers = { Cookie = "sid=codex", "Proxy-Authorization" = "Basic codex" }
`), 0o600)).To(Succeed())

			out, err := runPlugin(env, "diff", Spec{Agents: []AgentSpec{{Name: AgentClaude}, {Name: AgentCodex}}})
			Expect(err).NotTo(HaveOccurred())
			Expect(out).To(ContainSubstring(`warning: claude MCP server "bad-hdr" field headers.Cookie`))
			Expect(out).To(ContainSubstring(`warning: claude MCP server "bad-hdr" field headers.Proxy-Authorization`))
			Expect(out).To(ContainSubstring(`warning: claude MCP server "bad-arg" field args[1]`))
			Expect(out).To(ContainSubstring(`warning: codex MCP server "bad_codex" field args[0]`))
			Expect(out).To(ContainSubstring(`warning: codex MCP server "bad_codex" field http_headers.Cookie`))
			Expect(out).To(ContainSubstring(`warning: codex MCP server "bad_codex" field http_headers.Proxy-Authorization`))
			Expect(out).NotTo(ContainSubstring(`"ok"`))
			Expect(out).NotTo(ContainSubstring(`"ok_codex"`))
		})

		It("rejects MCP server names starting with a hyphen or underscore", func() {
			for _, badName := range []string{"-flag", "--scope", "_leading"} {
				_, err := ParseSpec(mustMarshalSpec(Spec{
					Agents: []AgentSpec{{
						Name: AgentClaude,
						MCPServers: []MCPServerSpec{{
							Name:      badName,
							Transport: MCPTransportStdio,
							Command:   "mcp",
						}},
					}},
				}))
				Expect(err).To(HaveOccurred(), "expected name %q to be rejected", badName)
			}
		})

		It("redacts literal secrets in diff output (including removed entries and context lines) without mutating written files", func() {
			env, _ := newFakeClaudeEnv(home, nil, "claude", "codex")
			codexConfig := filepath.Join(home, ".codex", "config.toml")
			Expect(os.MkdirAll(filepath.Dir(codexConfig), 0o700)).To(Succeed())

			codexRemovedSecret := "codex-literal-bearer-999"
			codexContextSecret := "sk-ant-codexContextSecret888"
			claudeContextSecret := "ghp_claudeContextSecret7777"
			claudeCookieSecret := "session=claude-cookie-secret-666"

			// Seed managed state so Codex will remove "old_srv" which has a literal secret.
			statePath := filepath.Join(env.StateDir(home), "agents", "managed.json")
			Expect(SaveManagedState(statePath, ManagedState{
				Version: 1,
				Agents: map[string]AgentManagedState{
					AgentCodex: {MCPServers: []string{"old_srv"}},
				},
			})).To(Succeed())
			Expect(os.WriteFile(codexConfig, []byte(fmt.Sprintf(`
[mcp_servers.user_keep]
command = "mcp"
env = { OPENAI_API_KEY = "%s" }

[mcp_servers.old_srv]
url = "https://old.example.com/mcp"
http_headers = { Authorization = "Bearer %s" }
`, codexContextSecret, codexRemovedSecret)), 0o600)).To(Succeed())

			claudeJSON := filepath.Join(home, ".claude.json")
			Expect(os.WriteFile(claudeJSON, []byte(fmt.Sprintf(`{
  "mcpServers": {
    "user-keep": {
      "type": "http",
      "url": "https://keep.example.com",
      "headers": {
        "Cookie": "%s",
        "X-Note": "%s"
      }
    }
  }
}
`, claudeCookieSecret, claudeContextSecret)), 0o600)).To(Succeed())

			spec := Spec{
				Agents: []AgentSpec{
					{
						Name: AgentClaude,
						MCPServers: []MCPServerSpec{{
							Name:      "devex",
							Transport: MCPTransportStdio,
							Command:   "devex",
							Args:      []string{"mcp", "serve"},
						}},
					},
					{Name: AgentCodex},
				},
			}

			diffOut, err := runPlugin(env, "diff", spec)
			Expect(err).NotTo(HaveOccurred())
			Expect(diffOut).To(ContainSubstring("***"))
			Expect(diffOut).NotTo(ContainSubstring(codexRemovedSecret))
			Expect(diffOut).NotTo(ContainSubstring(codexContextSecret))
			Expect(diffOut).NotTo(ContainSubstring(claudeContextSecret))
			Expect(diffOut).NotTo(ContainSubstring(claudeCookieSecret))

			// Apply must still write the untouched unmanaged server's value verbatim (never "***").
			_, err = runPlugin(env, "apply", spec)
			Expect(err).NotTo(HaveOccurred())
			writtenCodex, err := os.ReadFile(codexConfig)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(writtenCodex)).To(ContainSubstring(codexContextSecret))
			Expect(string(writtenCodex)).NotTo(ContainSubstring("***"))
		})
	})

	Describe("Review hardening regressions", func() {
		It("preserves following unmanaged TOML table when removing a managed server whose multiline literal string contains triple double-quotes", func() {
			existing := strings.Join([]string{
				"[mcp_servers.managed]",
				`command = "node"`,
				"args = ['''",
				`fake = """`,
				"''']",
				"",
				"[profiles.fast]",
				`model = "gpt-5"`,
				"",
			}, "\n")

			after, changed, owned, err := EditTOMLMCPServers(existing, map[string]CodexMCPTable{}, nil, []string{"managed"})
			Expect(err).NotTo(HaveOccurred())
			Expect(changed).To(BeTrue())
			Expect(owned).To(BeEmpty())
			Expect(after).NotTo(ContainSubstring("[mcp_servers.managed]"))
			Expect(after).To(ContainSubstring("[profiles.fast]"))
			Expect(after).To(ContainSubstring(`model = "gpt-5"`))
		})

		It("rejects inline mcp_servers table and dotted/inline managed server definitions before adopting or writing", func() {
			// 1. Top-level inline table mcp_servers = { ... } when adding a new server.
			inlineRoot := "mcp_servers = { other = { command = \"y\" } }\n"
			_, _, _, err := EditTOMLMCPServers(inlineRoot, map[string]CodexMCPTable{
				"github": {URL: "https://api.githubcopilot.com/mcp/"},
			}, []string{"github"}, nil)
			Expect(err).To(MatchError(ContainSubstring("as standard tables so DevEx can manage them")))

			// 2. Top-level dotted keys defining a server identical to desired (must NOT adopt).
			dottedRoot := "mcp_servers.foo.command = \"x\"\n"
			_, _, _, err = EditTOMLMCPServers(dottedRoot, map[string]CodexMCPTable{
				"foo": {Command: "x"},
			}, []string{"foo"}, nil)
			Expect(err).To(MatchError(ContainSubstring("as standard tables so DevEx can manage them")))

			// 3. [mcp_servers] with inline table entry identical to desired (must NOT adopt).
			inlineUnderHeader := "[mcp_servers]\nfoo = { command = \"x\" }\n"
			_, _, _, err = EditTOMLMCPServers(inlineUnderHeader, map[string]CodexMCPTable{
				"foo": {Command: "x"},
			}, []string{"foo"}, nil)
			Expect(err).To(MatchError(ContainSubstring("as standard tables so DevEx can manage them")))
		})

		It("scopes MCP ownership by resolved config path so switching CODEX_HOME does not delete unrelated servers", func() {
			dirA := filepath.Join(home, "codex-a")
			dirB := filepath.Join(home, "codex-b")
			Expect(os.MkdirAll(dirA, 0o700)).To(Succeed())
			Expect(os.MkdirAll(dirB, 0o700)).To(Succeed())

			envA := newTestEnv(home, map[string]string{"CODEX_HOME": dirA}, "codex")
			envB := newTestEnv(home, map[string]string{"CODEX_HOME": dirB}, "codex")

			// Apply "shared" under CODEX_HOME=dirA.
			specWithShared := Spec{
				Agents: []AgentSpec{{
					Name: AgentCodex,
					MCPServers: []MCPServerSpec{{
						Name:      "shared",
						Transport: MCPTransportStdio,
						Command:   "cmd-a",
					}},
				}},
			}
			_, err := runPlugin(envA, "apply", specWithShared)
			Expect(err).NotTo(HaveOccurred())

			// Place an unrelated server also named "shared" in dirB/config.toml.
			dirBConfig := filepath.Join(dirB, "config.toml")
			Expect(os.WriteFile(dirBConfig, []byte("[mcp_servers.shared]\ncommand = \"unrelated-b\"\n"), 0o600)).To(Succeed())

			// Switching to CODEX_HOME=dirB with a conflicting spec must report an unmanaged conflict.
			_, err = runPlugin(envB, "apply", specWithShared)
			Expect(err).To(MatchError(ContainSubstring("not managed by DevEx")))

			// Switching to CODEX_HOME=dirB with empty MCP servers must NOT delete dirB's unrelated "shared".
			_, err = runPlugin(envB, "apply", Spec{Agents: []AgentSpec{{Name: AgentCodex}}})
			Expect(err).NotTo(HaveOccurred())
			dataB, err := os.ReadFile(dirBConfig)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(dataB)).To(ContainSubstring("[mcp_servers.shared]"))
			Expect(string(dataB)).To(ContainSubstring(`command = "unrelated-b"`))

			// Legacy state file with no mcp_config_path still matches the current path.
			statePath := filepath.Join(envB.StateDir(home), "agents", "managed.json")
			Expect(SaveManagedState(statePath, ManagedState{
				Version: 1,
				Agents: map[string]AgentManagedState{
					AgentCodex: {MCPServers: []string{"shared"}},
				},
			})).To(Succeed())
			_, err = runPlugin(envB, "apply", Spec{Agents: []AgentSpec{{Name: AgentCodex}}})
			Expect(err).NotTo(HaveOccurred())
			dataBAfter, err := os.ReadFile(dirBConfig)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(dataBAfter)).NotTo(ContainSubstring("[mcp_servers.shared]"))
		})

		It("persists partial ownership checkpoints when a later write or Claude MCP command fails within the same agent", func() {
			// Part A: Codex writes config.toml (MCP server) then fails writing rules/devex.rules
			// because the rules directory is read-only (0500).
			envCodex := newTestEnv(home, nil, "codex")
			codexDir := filepath.Join(home, ".codex")
			rulesDir := filepath.Join(codexDir, "rules")
			Expect(os.MkdirAll(rulesDir, 0o700)).To(Succeed())
			Expect(os.Chmod(rulesDir, 0o500)).To(Succeed())
			DeferCleanup(func() { _ = os.Chmod(rulesDir, 0o700) })

			codexSpec := Spec{
				Agents: []AgentSpec{{
					Name:            AgentCodex,
					AllowedCommands: []string{"git status"},
					MCPServers: []MCPServerSpec{{
						Name:      "github",
						Transport: MCPTransportHTTP,
						URL:       "https://api.githubcopilot.com/mcp/",
					}},
				}},
			}
			_, err := runPlugin(envCodex, "apply", codexSpec)
			Expect(err).To(HaveOccurred())
			// config.toml was written before rules/devex.rules failed.
			writtenCodex, err := os.ReadFile(filepath.Join(codexDir, "config.toml"))
			Expect(err).NotTo(HaveOccurred())
			Expect(string(writtenCodex)).To(ContainSubstring("[mcp_servers.github]"))

			// Restore rules/ permissions and remove github from spec: github must be removed because config.toml succeeded!
			Expect(os.Chmod(rulesDir, 0o700)).To(Succeed())
			_, err = runPlugin(envCodex, "apply", Spec{Agents: []AgentSpec{{Name: AgentCodex}}})
			Expect(err).NotTo(HaveOccurred())
			codexData, err := os.ReadFile(filepath.Join(codexDir, "config.toml"))
			Expect(err).NotTo(HaveOccurred())
			Expect(string(codexData)).NotTo(ContainSubstring("[mcp_servers.github]"))

			// Part B: Claude writes settings.json, succeeds adding "first", then fails adding "second".
			claudeEnv, calls := newFakeClaudeEnv(home, nil, "claude")
			origRun := claudeEnv.RunCommand
			failSecond := true
			claudeEnv.RunCommand = func(ctx context.Context, name string, args ...string) ([]byte, error) {
				if failSecond && len(args) >= 5 && args[1] == "add-json" && args[4] == "second" {
					return []byte("boom"), fmt.Errorf("exit status 1")
				}
				return origRun(ctx, name, args...)
			}

			claudeSpec := Spec{
				Agents: []AgentSpec{{
					Name:            AgentClaude,
					AllowedCommands: []string{"go test"},
					MCPServers: []MCPServerSpec{
						{Name: "first", Transport: MCPTransportStdio, Command: "first-mcp"},
						{Name: "second", Transport: MCPTransportStdio, Command: "second-mcp"},
					},
				}},
			}
			_, err = runPlugin(claudeEnv, "apply", claudeSpec)
			Expect(err).To(HaveOccurred())

			// Now drop both permission and MCP server: both settings.json allow rule and "first" must be removed.
			failSecond = false
			*calls = nil
			_, err = runPlugin(claudeEnv, "apply", Spec{Agents: []AgentSpec{{Name: AgentClaude}}})
			Expect(err).NotTo(HaveOccurred())
			settingsData, err := os.ReadFile(filepath.Join(home, ".claude", "settings.json"))
			Expect(err).NotTo(HaveOccurred())
			Expect(string(settingsData)).NotTo(ContainSubstring("go test"))
			Expect(*calls).To(HaveLen(1))
			Expect((*calls)[0].Args).To(Equal([]string{"mcp", "remove", "--scope", "user", "first"}))
		})

		It("supports OpenCode v2 mcp.servers layout with disabled field and rejects unmanaged conflicts in non-target opencode.json", func() {
			env := newTestEnv(home, nil, "opencode")
			openCodeDir := filepath.Join(home, ".config", "opencode")
			Expect(os.MkdirAll(openCodeDir, 0o700)).To(Succeed())
			jsonPath := filepath.Join(openCodeDir, "opencode.json")

			// Part A: v2 layout with mcp.servers object.
			Expect(os.WriteFile(jsonPath, []byte(`{
  "mcp": {
    "servers": {
      "existing": {
        "type": "local",
        "command": ["keep-me"],
        "disabled": false
      }
    }
  }
}
`), 0o600)).To(Succeed())

			v2Spec := Spec{
				Agents: []AgentSpec{{
					Name: AgentOpenCode,
					MCPServers: []MCPServerSpec{
						{
							Name:      "github",
							Transport: MCPTransportHTTP,
							URL:       "https://api.githubcopilot.com/mcp/",
							Headers:   map[string]string{"Authorization": "Bearer ${env:GITHUB_TOKEN}"},
						},
						{
							Name:      "fs",
							Transport: MCPTransportStdio,
							Command:   "npx",
							Args:      []string{"-y", "fs-server"},
							Env:       map[string]string{"TOKEN": "${env:TOKEN}"},
						},
					},
				}},
			}
			_, err := runPlugin(env, "apply", v2Spec)
			Expect(err).NotTo(HaveOccurred())

			rawV2, err := os.ReadFile(jsonPath)
			Expect(err).NotTo(HaveOccurred())
			var parsed map[string]any
			Expect(json.Unmarshal(rawV2, &parsed)).To(Succeed())
			mcpObj, ok := parsed["mcp"].(map[string]any)
			Expect(ok).To(BeTrue())
			Expect(mcpObj).NotTo(HaveKey("github"))
			serversObj, ok := mcpObj["servers"].(map[string]any)
			Expect(ok).To(BeTrue())
			Expect(serversObj).To(HaveKey("existing"))
			Expect(serversObj).To(HaveKey("github"))
			Expect(serversObj).To(HaveKey("fs"))
			ghObj := serversObj["github"].(map[string]any)
			Expect(ghObj).To(HaveKeyWithValue("disabled", false))
			Expect(ghObj).NotTo(HaveKey("enabled"))
			fsObj := serversObj["fs"].(map[string]any)
			Expect(fsObj).To(HaveKeyWithValue("disabled", false))
			Expect(fsObj).NotTo(HaveKey("enabled"))

			// Re-apply is idempotent.
			reapplyOut, err := runPlugin(env, "apply", v2Spec)
			Expect(err).NotTo(HaveOccurred())
			Expect(reapplyOut).To(ContainSubstring("unchanged: " + jsonPath))

			// Removing managed servers keeps "existing" under mcp.servers.
			_, err = runPlugin(env, "apply", Spec{Agents: []AgentSpec{{Name: AgentOpenCode}}})
			Expect(err).NotTo(HaveOccurred())
			rawAfterRemove, err := os.ReadFile(jsonPath)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(rawAfterRemove)).To(ContainSubstring(`"existing"`))
			Expect(string(rawAfterRemove)).NotTo(ContainSubstring(`"github"`))

			// Part B: both opencode.json and opencode.jsonc exist; desired server exists unmanaged in opencode.json.
			jsoncPath := filepath.Join(openCodeDir, "opencode.jsonc")
			Expect(os.WriteFile(jsoncPath, []byte("{\n  \"model\": \"anthropic/claude\"\n}\n"), 0o600)).To(Succeed())
			_, err = runPlugin(env, "apply", Spec{
				Agents: []AgentSpec{{
					Name: AgentOpenCode,
					MCPServers: []MCPServerSpec{{
						Name:      "existing",
						Transport: MCPTransportStdio,
						Command:   "other-cmd",
					}},
				}},
			})
			Expect(err).To(MatchError(And(
				ContainSubstring("existing"),
				ContainSubstring("opencode.json"),
			)))
		})
	})
})

func mustMarshalSpec(spec Spec) []byte {
	data, err := json.Marshal(spec)
	Expect(err).NotTo(HaveOccurred())
	return data
}
