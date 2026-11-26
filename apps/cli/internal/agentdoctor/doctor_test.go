package agentdoctor_test

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/jameswlane/devex/apps/cli/internal/agentdoctor"
	"github.com/jameswlane/devex/apps/cli/internal/agentsandbox"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func writeFakeBinary(dir, name, script string) string {
	Expect(os.MkdirAll(dir, 0o755)).To(Succeed())
	fullPath := filepath.Join(dir, name)
	Expect(os.WriteFile(fullPath, []byte("#!/bin/sh\n"+script+"\n"), 0o755)).To(Succeed())
	return fullPath
}

func makeOSBackedProbe(home string, pathDirs []string, extraEnv map[string]string, sandboxProbe agentsandbox.Probe, readFileSpy func(string)) agentdoctor.Probe {
	env := map[string]string{
		"HOME": home,
		"PATH": strings.Join(pathDirs, ":"),
	}
	for k, v := range extraEnv {
		env[k] = v
	}
	fixedTime := time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC)
	return agentdoctor.Probe{
		GOOS: "linux",
		Now:  func() time.Time { return fixedTime },
		Getenv: func(key string) string {
			return env[key]
		},
		Stat: os.Stat,
		ReadFile: func(path string) ([]byte, error) {
			if readFileSpy != nil {
				readFileSpy(path)
			}
			return os.ReadFile(path)
		},
		EvalSymlinks: filepath.EvalSymlinks,
		Run: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			cmd := exec.CommandContext(ctx, name, args...)
			cmd.Env = make([]string, 0, len(env)+1)
			for k, v := range env {
				cmd.Env = append(cmd.Env, k+"="+v)
			}
			return cmd.Output()
		},
		SandboxProbe: sandboxProbe,
	}
}

func okSandboxProbe(runCount *int) agentsandbox.Probe {
	return agentsandbox.Probe{
		GOOS: "linux",
		Now:  func() time.Time { return time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC) },
		LookPath: func(name string) (string, error) {
			if name == "bwrap" || name == "socat" {
				return "/usr/bin/" + name, nil
			}
			return "", exec.ErrNotFound
		},
		Run: func(context.Context, string, ...string) ([]byte, error) {
			if runCount != nil {
				*runCount++
			}
			return nil, nil
		},
	}
}

func restrictedSandboxProbe(runCount *int) agentsandbox.Probe {
	return agentsandbox.Probe{
		GOOS: "linux",
		Now:  func() time.Time { return time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC) },
		LookPath: func(name string) (string, error) {
			if name == "bwrap" || name == "socat" {
				return "/usr/bin/" + name, nil
			}
			return "", exec.ErrNotFound
		},
		ReadFile: func(path string) ([]byte, error) {
			if path == "/proc/sys/kernel/apparmor_restrict_unprivileged_userns" {
				return []byte("1\n"), nil
			}
			return nil, os.ErrNotExist
		},
		Run: func(context.Context, string, ...string) ([]byte, error) {
			if runCount != nil {
				*runCount++
			}
			return []byte("bwrap: setting up uid map: Permission denied\n"), errors.New("exit status 1")
		},
	}
}

func findAgentReport(report agentdoctor.Report, id string) agentdoctor.AgentReport {
	for _, a := range report.Agents {
		if a.ID == id {
			return a
		}
	}
	Fail("agent report not found: " + id)
	return agentdoctor.AgentReport{}
}

var _ = Describe("Agent doctor diagnostics", func() {
	BeforeEach(func() {
		if runtime.GOOS == "windows" {
			Skip("POSIX fake binary scripts require Unix")
		}
	})

	It("reports all known agents as not installed with compact one-line table entries when PATH is empty", func() {
		home := GinkgoT().TempDir()
		probe := makeOSBackedProbe(home, nil, nil, okSandboxProbe(nil), nil)

		report := agentdoctor.Check(context.Background(), probe, nil)

		Expect(report.Agents).To(HaveLen(8))
		Expect(report.Problems).To(BeEmpty())
		for _, ag := range report.Agents {
			Expect(ag.Installed).To(BeFalse())
			Expect(ag.Sandbox).To(BeNil())
		}

		table := agentdoctor.FormatTable(report, nil)
		for _, id := range agentdoctor.ValidAgentIDs() {
			Expect(table).To(ContainSubstring(id))
			Expect(table).To(ContainSubstring("not installed"))
		}
		Expect(table).To(ContainSubstring("No problems detected."))
	})

	It("detects installed and logged-in agent via login-status command exit code 0", func() {
		home := GinkgoT().TempDir()
		binDir := filepath.Join(home, ".local", "bin")
		writeFakeBinary(binDir, "claude", `
if [ "$1" = "--version" ]; then
  echo "2.1.332 (Claude Code)"
  exit 0
fi
if [ "$1" = "auth" ] && [ "$2" = "status" ]; then
  echo '{"loggedIn":true}'
  exit 0
fi
exit 1`)

		sandboxRuns := 0
		probe := makeOSBackedProbe(home, []string{binDir}, nil, okSandboxProbe(&sandboxRuns), nil)
		specs, err := agentdoctor.FilterAgents([]string{"claude"})
		Expect(err).NotTo(HaveOccurred())

		report := agentdoctor.Check(context.Background(), probe, specs)
		Expect(sandboxRuns).To(Equal(1))
		Expect(report.Problems).To(BeEmpty())

		claude := findAgentReport(report, "claude")
		Expect(claude.Installed).To(BeTrue())
		Expect(claude.Version).To(Equal("2.1.332"))
		Expect(claude.ResolvedPath).To(Equal(filepath.Join(binDir, "claude")))
		Expect(claude.LoginState).To(Equal(agentdoctor.LoginStateLoggedIn))
		Expect(claude.LoginDetail).To(ContainSubstring("exited 0"))
		Expect(claude.Sandbox).NotTo(BeNil())
		Expect(claude.Sandbox.Status).To(Equal(agentsandbox.StatusOK))
	})

	It("detects installed but not-logged-in agent when status command exits non-zero", func() {
		home := GinkgoT().TempDir()
		binDir := filepath.Join(home, "bin")
		writeFakeBinary(binDir, "codex", `
if [ "$1" = "--version" ]; then
  echo "codex-cli 0.172.0"
  exit 0
fi
if [ "$1" = "login" ] && [ "$2" = "status" ]; then
  echo "Not logged in" >&2
  exit 1
fi
exit 1`)

		probe := makeOSBackedProbe(home, []string{binDir}, nil, okSandboxProbe(nil), nil)
		specs, err := agentdoctor.FilterAgents([]string{"codex"})
		Expect(err).NotTo(HaveOccurred())

		report := agentdoctor.Check(context.Background(), probe, specs)
		codex := findAgentReport(report, "codex")
		Expect(codex.Installed).To(BeTrue())
		Expect(codex.Version).To(Equal("0.172.0"))
		Expect(codex.LoginState).To(Equal(agentdoctor.LoginStateNotLoggedIn))
		Expect(codex.LoginDetail).To(ContainSubstring("exited non-zero"))
		Expect(codex.Problems).To(HaveLen(1))
		Expect(codex.Problems[0].Code).To(Equal("not_logged_in"))
		Expect(codex.Problems[0].Fix).To(Equal("devex agent login codex"))
	})

	It("detects credential file presence via Stat only without ever reading the credential file", func() {
		home := GinkgoT().TempDir()
		binDir := filepath.Join(home, ".opencode", "bin")
		writeFakeBinary(binDir, "opencode", `echo "opencode v2.0.26"`)

		credPath := filepath.Join(home, ".local", "share", "opencode", "auth.json")
		Expect(os.MkdirAll(filepath.Dir(credPath), 0o700)).To(Succeed())
		Expect(os.WriteFile(credPath, []byte(`{"token":"must-never-be-read"}`), 0o600)).To(Succeed())

		var readPaths []string
		probe := makeOSBackedProbe(home, []string{binDir}, nil, okSandboxProbe(nil), func(p string) {
			readPaths = append(readPaths, p)
		})
		specs, err := agentdoctor.FilterAgents([]string{"opencode"})
		Expect(err).NotTo(HaveOccurred())

		report := agentdoctor.Check(context.Background(), probe, specs)
		oc := findAgentReport(report, "opencode")
		Expect(oc.Installed).To(BeTrue())
		Expect(oc.LoginState).To(Equal(agentdoctor.LoginStateLoggedIn))
		Expect(oc.LoginDetail).To(ContainSubstring("credential file present"))
		Expect(readPaths).NotTo(ContainElement(credPath))
	})

	It("flags duplicate installs where an npm-global copy shadows the native binary", func() {
		home := GinkgoT().TempDir()
		npmBin := filepath.Join(home, "npm-global", "bin")
		npmPkgDir := filepath.Join(home, "npm-global", "lib", "node_modules", "@anthropic-ai", "claude-code")
		nativeBin := filepath.Join(home, ".local", "bin")

		realNPMCLI := writeFakeBinary(npmPkgDir, "cli.js", `
if [ "$1" = "--version" ]; then echo "1.0.0-npm"; exit 0; fi
if [ "$1" = "auth" ] && [ "$2" = "status" ]; then exit 0; fi
exit 0`)
		Expect(os.MkdirAll(npmBin, 0o755)).To(Succeed())
		Expect(os.Symlink(realNPMCLI, filepath.Join(npmBin, "claude"))).To(Succeed())

		writeFakeBinary(nativeBin, "claude", `
if [ "$1" = "--version" ]; then echo "2.1.332 (Claude Code)"; exit 0; fi
exit 0`)

		// Also provide a recent node so Node check passes for the npm copy.
		writeFakeBinary(nativeBin, "node", `echo "v22.14.0"`)

		probe := makeOSBackedProbe(home, []string{npmBin, nativeBin}, nil, okSandboxProbe(nil), nil)
		specs, err := agentdoctor.FilterAgents([]string{"claude"})
		Expect(err).NotTo(HaveOccurred())

		report := agentdoctor.Check(context.Background(), probe, specs)
		claude := findAgentReport(report, "claude")
		Expect(claude.Installed).To(BeTrue())
		Expect(claude.Shadowed).To(BeTrue())
		Expect(claude.ShadowedByNPM).To(BeTrue())
		Expect(claude.PathMatches).To(HaveLen(2))
		Expect(claude.PathMatches[0].IsNPMGlobal).To(BeTrue())
		Expect(claude.PathMatches[1].IsNPMGlobal).To(BeFalse())

		Expect(claude.Problems).To(ContainElement(SatisfyAll(
			HaveField("Code", "npm_shadowing"),
			HaveField("Fix", And(
				ContainSubstring("npm uninstall -g @anthropic-ai/claude-code"),
				ContainSubstring(nativeBin),
			)),
		)))
	})

	It("flags missing or too-old Node.js for Node-based agents with a mise fix", func() {
		home := GinkgoT().TempDir()
		binDir := filepath.Join(home, "bin")
		writeFakeBinary(binDir, "gemini", `echo "0.5.0"`)
		writeFakeBinary(binDir, "node", `echo "v18.20.4"`)

		credPath := filepath.Join(home, ".gemini", "oauth_creds.json")
		Expect(os.MkdirAll(filepath.Dir(credPath), 0o700)).To(Succeed())
		Expect(os.WriteFile(credPath, []byte(`{}`), 0o600)).To(Succeed())

		probe := makeOSBackedProbe(home, []string{binDir}, nil, okSandboxProbe(nil), nil)
		specs, err := agentdoctor.FilterAgents([]string{"gemini"})
		Expect(err).NotTo(HaveOccurred())

		report := agentdoctor.Check(context.Background(), probe, specs)
		gem := findAgentReport(report, "gemini")
		Expect(gem.Installed).To(BeTrue())
		Expect(gem.Node).NotTo(BeNil())
		Expect(gem.Node.OK).To(BeFalse())
		Expect(gem.Node.Required).To(Equal("20.0.0"))
		Expect(gem.Node.Detected).To(Equal("v18.20.4"))
		Expect(gem.Problems).To(ContainElement(SatisfyAll(
			HaveField("Code", "node_version"),
			HaveField("Fix", "mise use -g node@20"),
		)))
	})

	It("validates valid and invalid JSON, TOML, and YAML MCP configs without echoing file content", func() {
		home := GinkgoT().TempDir()
		binDir := filepath.Join(home, "bin")
		writeFakeBinary(binDir, "claude", `if [ "$1" = "--version" ]; then echo "2.0.0"; fi; exit 0`)
		writeFakeBinary(binDir, "codex", `if [ "$1" = "--version" ]; then echo "0.172.0"; fi; exit 0`)
		writeFakeBinary(binDir, "goose", `echo "1.0.0"`)

		// Valid Claude JSON MCP config.
		claudeMCP := filepath.Join(home, ".claude.json")
		Expect(os.WriteFile(claudeMCP, []byte(`{"mcpServers":{"devex":{"command":"devex","args":["mcp","serve"]}}}`), 0o600)).To(Succeed())

		// Invalid Codex TOML MCP config containing a sensitive token on the bad line.
		codexDir := filepath.Join(home, ".codex")
		Expect(os.MkdirAll(codexDir, 0o700)).To(Succeed())
		codexTOML := filepath.Join(codexDir, "config.toml")
		Expect(os.WriteFile(codexTOML, []byte("[mcp_servers.devex]\ncommand = \"devex\"\nbad_token = \"sk-never-echo-toml\" == broken\n"), 0o600)).To(Succeed())

		// Invalid Goose YAML MCP config containing a sensitive token.
		gooseDir := filepath.Join(home, ".config", "goose")
		Expect(os.MkdirAll(gooseDir, 0o700)).To(Succeed())
		gooseYAML := filepath.Join(gooseDir, "config.yaml")
		Expect(os.WriteFile(gooseYAML, []byte("extensions:\n  broken: [unclosed, \"sk-never-echo-yaml\"\n"), 0o600)).To(Succeed())

		probe := makeOSBackedProbe(home, []string{binDir}, nil, okSandboxProbe(nil), nil)
		specs, err := agentdoctor.FilterAgents([]string{"claude", "codex", "goose"})
		Expect(err).NotTo(HaveOccurred())

		report := agentdoctor.Check(context.Background(), probe, specs)

		claude := findAgentReport(report, "claude")
		Expect(claude.MCPConfigs).To(HaveLen(1))
		Expect(claude.MCPConfigs[0].Status).To(Equal(agentdoctor.MCPStatusOK))

		codex := findAgentReport(report, "codex")
		Expect(codex.MCPConfigs).To(HaveLen(1))
		Expect(codex.MCPConfigs[0].Status).To(Equal(agentdoctor.MCPStatusInvalid))
		Expect(codex.MCPConfigs[0].Error).To(ContainSubstring("line 3"))
		Expect(codex.MCPConfigs[0].Error).NotTo(ContainSubstring("sk-never-echo-toml"))

		goose := findAgentReport(report, "goose")
		Expect(goose.MCPConfigs).To(HaveLen(1))
		Expect(goose.MCPConfigs[0].Status).To(Equal(agentdoctor.MCPStatusInvalid))
		Expect(goose.MCPConfigs[0].Error).To(ContainSubstring("invalid:"))
		Expect(goose.MCPConfigs[0].Error).NotTo(ContainSubstring("sk-never-echo-yaml"))
	})

	It("reports unknown login state, not logged out, when the status command times out", func() {
		home := GinkgoT().TempDir()
		binDir := filepath.Join(home, "bin")
		sleepPath, err := exec.LookPath("sleep")
		Expect(err).NotTo(HaveOccurred())
		writeFakeBinary(binDir, "codex", `
if [ "$1" = "--version" ]; then echo "codex-cli 0.172.0"; exit 0; fi
exec '`+sleepPath+`' 5`)

		probe := makeOSBackedProbe(home, []string{binDir}, nil, okSandboxProbe(nil), nil)
		specs, err := agentdoctor.FilterAgents([]string{"codex"})
		Expect(err).NotTo(HaveOccurred())

		start := time.Now()
		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		defer cancel()
		report := agentdoctor.Check(ctx, probe, specs)

		codex := findAgentReport(report, "codex")
		Expect(codex.LoginState).To(Equal(agentdoctor.LoginStateUnknown))
		Expect(codex.LoginDetail).To(ContainSubstring("timed out"))
		Expect(codex.Problems).To(BeEmpty())
		Expect(time.Since(start)).To(BeNumerically("<", 3*time.Second))
	})

	It("accepts comments and trailing commas in OpenCode's opencode.jsonc and reports positions when it is broken", func() {
		home := GinkgoT().TempDir()
		binDir := filepath.Join(home, "bin")
		writeFakeBinary(binDir, "opencode", `echo "opencode v2.0.26"`)
		configDir := filepath.Join(home, ".config", "opencode")
		Expect(os.MkdirAll(configDir, 0o700)).To(Succeed())
		jsonc := filepath.Join(configDir, "opencode.jsonc")
		Expect(os.WriteFile(jsonc, []byte("{\n  // servers\n  \"mcp\": {\n    \"devex\": {\"type\": \"local\", \"url\": \"http://x//y\"}, /* inline */\n  },\n}\n"), 0o600)).To(Succeed())

		probe := makeOSBackedProbe(home, []string{binDir}, nil, okSandboxProbe(nil), nil)
		specs, err := agentdoctor.FilterAgents([]string{"opencode"})
		Expect(err).NotTo(HaveOccurred())

		opencode := findAgentReport(agentdoctor.Check(context.Background(), probe, specs), "opencode")
		Expect(opencode.MCPConfigs).To(HaveLen(1))
		Expect(opencode.MCPConfigs[0].Path).To(Equal(jsonc))
		Expect(opencode.MCPConfigs[0].Status).To(Equal(agentdoctor.MCPStatusOK))

		Expect(os.WriteFile(jsonc, []byte("{\n  // servers\n  \"mcp\": {\"sk-never-echo-jsonc\" }\n}\n"), 0o600)).To(Succeed())
		opencode = findAgentReport(agentdoctor.Check(context.Background(), probe, specs), "opencode")
		Expect(opencode.MCPConfigs[0].Status).To(Equal(agentdoctor.MCPStatusInvalid))
		Expect(opencode.MCPConfigs[0].Error).To(ContainSubstring("line 3"))
		Expect(opencode.MCPConfigs[0].Error).NotTo(ContainSubstring("sk-never-echo-jsonc"))
	})

	It("does not guess from status commands that die, special files, relative PATH entries, or non-login keys", func() {
		home := GinkgoT().TempDir()
		binDir := filepath.Join(home, "bin")
		writeFakeBinary(binDir, "codex", `
if [ "$1" = "--version" ]; then echo "codex-cli 0.172.0"; exit 0; fi
kill -9 $$`)
		writeFakeBinary(binDir, "gemini", `echo "0.5.0"`)
		Expect(os.MkdirAll(filepath.Join(home, ".gemini"), 0o700)).To(Succeed())
		Expect(exec.Command("mkfifo", filepath.Join(home, ".gemini", "oauth_creds.json")).Run()).To(Succeed())
		Expect(os.WriteFile(filepath.Join(home, ".gemini", "settings.json"), []byte(`{"mcpServers":!}`), 0o600)).To(Succeed())

		// A relative PATH entry that resolves from the working directory.
		writeFakeBinary(filepath.Join(home, "relbin"), "codex", `echo "0.1.0"`)
		wd, err := os.Getwd()
		Expect(err).NotTo(HaveOccurred())
		Expect(os.Chdir(home)).To(Succeed())
		DeferCleanup(os.Chdir, wd)

		probe := makeOSBackedProbe(home, []string{"relbin", binDir}, map[string]string{"GOOGLE_API_KEY": "google-key-value"}, okSandboxProbe(nil), nil)
		specs, err := agentdoctor.FilterAgents([]string{"codex", "gemini"})
		Expect(err).NotTo(HaveOccurred())
		report := agentdoctor.Check(context.Background(), probe, specs)

		codex := findAgentReport(report, "codex")
		Expect(codex.ResolvedPath).To(Equal(filepath.Join(binDir, "codex")))
		Expect(codex.LoginState).To(Equal(agentdoctor.LoginStateUnknown))
		Expect(codex.LoginDetail).To(ContainSubstring("could not run"))
		Expect(codex.PathMatches).To(HaveLen(1))

		gemini := findAgentReport(report, "gemini")
		Expect(gemini.LoginState).To(Equal(agentdoctor.LoginStateNotLoggedIn))
		Expect(gemini.EnvVarsSet).To(BeEmpty())
		Expect(gemini.MCPConfigs).To(ConsistOf(HaveField("Error", "invalid: syntax error at line 1, column 15")))
		Expect(agentdoctor.CollectSecretValues(probe.Getenv)).To(ContainElement("google-key-value"))
	})

	It("rejects an unterminated block comment in a JSONC config", func() {
		home := GinkgoT().TempDir()
		binDir := filepath.Join(home, "bin")
		writeFakeBinary(binDir, "opencode", `echo "opencode v2.0.26"`)
		configDir := filepath.Join(home, ".config", "opencode")
		Expect(os.MkdirAll(configDir, 0o700)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(configDir, "opencode.jsonc"), []byte("{\"mcp\": {}}\n/* never closed"), 0o600)).To(Succeed())

		probe := makeOSBackedProbe(home, []string{binDir}, nil, okSandboxProbe(nil), nil)
		specs, err := agentdoctor.FilterAgents([]string{"opencode"})
		Expect(err).NotTo(HaveOccurred())
		opencode := findAgentReport(agentdoctor.Check(context.Background(), probe, specs), "opencode")
		Expect(opencode.MCPConfigs).To(ConsistOf(HaveField("Error", "invalid: unterminated block comment")))
	})

	It("runs the sandbox probe at most once across multiple agents declaring sandbox prerequisites", func() {
		home := GinkgoT().TempDir()
		binDir := filepath.Join(home, "bin")
		writeFakeBinary(binDir, "claude", `if [ "$1" = "--version" ]; then echo "2.0.0"; fi; exit 0`)
		writeFakeBinary(binDir, "codex", `if [ "$1" = "--version" ]; then echo "0.172.0"; fi; exit 0`)
		writeFakeBinary(binDir, "copilot", `echo "1.0.0"`)
		writeFakeBinary(binDir, "aider", `echo "0.80.0"`)

		sandboxRuns := 0
		probe := makeOSBackedProbe(home, []string{binDir}, nil, restrictedSandboxProbe(&sandboxRuns), nil)
		specs, err := agentdoctor.FilterAgents([]string{"claude", "codex", "copilot", "aider"})
		Expect(err).NotTo(HaveOccurred())

		report := agentdoctor.Check(context.Background(), probe, specs)
		Expect(sandboxRuns).To(Equal(1))

		for _, id := range []string{"claude", "codex", "copilot"} {
			ag := findAgentReport(report, id)
			Expect(ag.Sandbox).NotTo(BeNil(), "expected sandbox summary for %s", id)
			Expect(ag.Sandbox.Status).To(Equal(agentsandbox.StatusRestricted))
			Expect(ag.Problems).To(ContainElement(SatisfyAll(
				HaveField("Code", "sandbox"),
				HaveField("Fix", ContainSubstring("devex agent sandbox")),
			)))
		}
		Expect(findAgentReport(report, "aider").Sandbox).To(BeNil())
	})

	It("keeps stderr, long secrets, and JSON-escaped secrets out of reports built with the real probe", func() {
		const (
			stderrSentinel = "sk-devex-stderr-sentinel-111"
			statusSentinel = "sk-devex-status-sentinel-222"
			pathSentinel   = `sk-devex-"path\sentinel-333`
		)
		envSentinel := "sk-devex-env-sentinel-" + strings.Repeat("x", 200)
		home := GinkgoT().TempDir()
		binDir := filepath.Join(home, pathSentinel, "bin")
		writeFakeBinary(binDir, "claude", `
echo "`+stderrSentinel+`" >&2
if [ "$1" = "--version" ]; then echo "sk-1.2.3-devex-sentinel `+envSentinel+` 2.1.0 (Claude Code)"; exit 0; fi
echo "`+statusSentinel+`"
exit 0`)
		writeFakeBinary(binDir, "node", `echo "sk-1.2.3-devex-sentinel v22.14.0 `+envSentinel+`"`)
		writeFakeBinary(binDir, "gemini", `echo "0.5.0"`)

		GinkgoT().Setenv("HOME", home)
		for _, name := range []string{"CLAUDE_CONFIG_DIR", "CODEX_HOME", "GEMINI_CLI_HOME", "COPILOT_HOME", "KIMI_CODE_HOME", "XDG_CONFIG_HOME", "XDG_DATA_HOME"} {
			GinkgoT().Setenv(name, "")
		}
		GinkgoT().Setenv("PATH", binDir)
		GinkgoT().Setenv("ANTHROPIC_API_KEY", envSentinel)
		GinkgoT().Setenv("OPENAI_API_KEY", pathSentinel)
		GinkgoT().Setenv("GEMINI_API_KEY", "sk-1.2.3-devex-sentinel")
		probe := agentdoctor.DefaultProbe()
		probe.SandboxProbe = okSandboxProbe(nil)
		specs, err := agentdoctor.FilterAgents([]string{"claude", "gemini"})
		Expect(err).NotTo(HaveOccurred())

		report := agentdoctor.Check(context.Background(), probe, specs)
		Expect(findAgentReport(report, "claude").Version).To(Equal("2.1.0"))
		gemini := findAgentReport(report, "gemini")
		Expect(gemini.Node).NotTo(BeNil())
		Expect(gemini.Node.Detected).To(Equal("v22.14.0"))

		secrets := agentdoctor.CollectSecretValues(os.Getenv)
		jsonOut, err := agentdoctor.FormatJSON(report, secrets)
		Expect(err).NotTo(HaveOccurred())
		var decoded map[string]any
		Expect(json.Unmarshal(jsonOut, &decoded)).To(Succeed())
		table := agentdoctor.FormatTable(report, secrets)
		for _, out := range []string{string(jsonOut), table} {
			Expect(out).NotTo(ContainSubstring("sentinel"))
			Expect(out).NotTo(ContainSubstring("xxxxxxxx"))
		}
		Expect(string(jsonOut)).To(ContainSubstring("[REDACTED]"))
	})

	It("does not open special files at MCP config paths", func() {
		home := GinkgoT().TempDir()
		binDir := filepath.Join(home, "bin")
		writeFakeBinary(binDir, "gemini", `echo "0.5.0"`)
		Expect(os.MkdirAll(filepath.Join(home, ".gemini"), 0o700)).To(Succeed())
		fifo := filepath.Join(home, ".gemini", "settings.json")
		Expect(exec.Command("mkfifo", fifo).Run()).To(Succeed())

		probe := makeOSBackedProbe(home, []string{binDir}, nil, okSandboxProbe(nil), nil)
		specs, err := agentdoctor.FilterAgents([]string{"gemini"})
		Expect(err).NotTo(HaveOccurred())

		done := make(chan agentdoctor.Report, 1)
		go func() { done <- agentdoctor.Check(context.Background(), probe, specs) }()
		var report agentdoctor.Report
		Eventually(done, 3*time.Second).Should(Receive(&report))
		gemini := findAgentReport(report, "gemini")
		Expect(gemini.MCPConfigs).To(HaveLen(1))
		Expect(gemini.MCPConfigs[0].Error).To(Equal("invalid: not a regular file"))
	})

	It("reports unknown login rather than a false state when the signal is not authoritative", func() {
		home := GinkgoT().TempDir()
		binDir := filepath.Join(home, "bin")
		writeFakeBinary(binDir, "opencode", `echo "opencode v2.0.26"`)
		writeFakeBinary(binDir, "kimi", `echo "2.6.0"`)
		writeFakeBinary(binDir, "gemini", `echo "0.5.0"`)
		geminiHome := filepath.Join(home, "gemini-home")
		Expect(os.MkdirAll(filepath.Join(geminiHome, ".gemini"), 0o700)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(geminiHome, ".gemini", "oauth_creds.json"), []byte("{}"), 0o600)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(geminiHome, ".gemini", "settings.json"), []byte("{\n  // comment\n  \"mcpServers\": {}\n}\n"), 0o600)).To(Succeed())

		probe := makeOSBackedProbe(home, []string{binDir}, map[string]string{
			"KIMI_API_KEY":    "kimi-value-not-read",
			"GEMINI_CLI_HOME": geminiHome,
		}, okSandboxProbe(nil), nil)
		specs, err := agentdoctor.FilterAgents([]string{"opencode", "kimi", "gemini"})
		Expect(err).NotTo(HaveOccurred())
		report := agentdoctor.Check(context.Background(), probe, specs)

		opencode := findAgentReport(report, "opencode")
		Expect(opencode.LoginState).To(Equal(agentdoctor.LoginStateUnknown))
		Expect(opencode.LoginDetail).To(ContainSubstring("OpenCode's database"))
		Expect(opencode.Problems).To(BeEmpty())

		kimi := findAgentReport(report, "kimi")
		Expect(kimi.LoginState).To(Equal(agentdoctor.LoginStateUnknown))
		Expect(kimi.EnvVarsSet).To(ConsistOf("KIMI_API_KEY"))

		gemini := findAgentReport(report, "gemini")
		Expect(gemini.LoginState).To(Equal(agentdoctor.LoginStateLoggedIn))
		Expect(gemini.LoginDetail).To(ContainSubstring(geminiHome))
		Expect(gemini.MCPConfigs).To(ConsistOf(HaveField("Status", agentdoctor.MCPStatusOK)))

		// Gemini's loader strips comments but rejects trailing commas.
		Expect(os.WriteFile(filepath.Join(geminiHome, ".gemini", "settings.json"), []byte(`{"mcpServers":{},}`), 0o600)).To(Succeed())
		gemini = findAgentReport(agentdoctor.Check(context.Background(), probe, specs), "gemini")
		Expect(gemini.MCPConfigs).To(ConsistOf(HaveField("Status", agentdoctor.MCPStatusInvalid)))
	})

	It("never leaks any secret sentinel from env vars, credential files, MCP configs, or status commands", func() {
		const sentinel = "sk-devex-sentinel-XYZ-987654321"
		home := GinkgoT().TempDir()
		binDir := filepath.Join(home, "bin")

		// Fake claude and codex print the sentinel on both --version and status stdout/stderr!
		writeFakeBinary(binDir, "claude", `
if [ "$1" = "--version" ]; then
  echo "2.1.0 token=`+sentinel+`"
  exit 0
fi
if [ "$1" = "auth" ] && [ "$2" = "status" ]; then
  echo "stdout-secret=`+sentinel+`"
  echo "stderr-secret=`+sentinel+`" >&2
  exit 0
fi`)
		writeFakeBinary(binDir, "codex", `
if [ "$1" = "--version" ]; then
  echo "codex-cli 0.172.0"
  exit 0
fi
if [ "$1" = "login" ] && [ "$2" = "status" ]; then
  echo "expired-token=`+sentinel+`" >&2
  exit 1
fi`)
		writeFakeBinary(binDir, "opencode", `echo "opencode v2.0.26"`)
		writeFakeBinary(binDir, "goose", `echo "1.0.0"`)
		writeFakeBinary(binDir, "gemini", `echo "0.5.0"`)
		writeFakeBinary(binDir, "copilot", `echo "1.0.0"`)
		writeFakeBinary(binDir, "aider", `echo "0.80.0"`)
		writeFakeBinary(binDir, "kimi", `echo "2.6.0"`)
		writeFakeBinary(binDir, "node", `echo "v22.14.0"`)

		// Populate every known API-key env var with the sentinel.
		extraEnv := map[string]string{}
		for _, envName := range agentdoctor.AllKnownSecretEnvVars() {
			extraEnv[envName] = sentinel
		}

		// Write credential files containing the sentinel (must never be read).
		credFiles := []string{
			filepath.Join(home, ".claude", ".credentials.json"),
			filepath.Join(home, ".codex", "auth.json"),
			filepath.Join(home, ".gemini", "oauth_creds.json"),
			filepath.Join(home, ".local", "share", "opencode", "auth.json"),
			filepath.Join(home, ".config", "goose", "secrets.yaml"),
		}
		for _, p := range credFiles {
			Expect(os.MkdirAll(filepath.Dir(p), 0o700)).To(Succeed())
			Expect(os.WriteFile(p, []byte(`{"api_key":"`+sentinel+`"}`), 0o600)).To(Succeed())
		}

		// Write valid and invalid MCP configs containing the sentinel in env values.
		Expect(os.WriteFile(filepath.Join(home, ".claude.json"), []byte(`{"mcpServers":{"s":{"command":"x","env":{"KEY":"`+sentinel+`"}}}}`), 0o600)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(home, ".gemini", "settings.json"), []byte(`{"mcpServers":{"s":{"env":{"KEY":"`+sentinel+`"}}`), 0o600)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(home, ".codex", "config.toml"), []byte("[mcp_servers.s]\nenv = { KEY = \""+sentinel+"\" } == invalid\n"), 0o600)).To(Succeed())

		var readPaths []string
		probe := makeOSBackedProbe(home, []string{binDir}, extraEnv, okSandboxProbe(nil), func(p string) {
			readPaths = append(readPaths, p)
		})

		report := agentdoctor.Check(context.Background(), probe, nil)
		secrets := agentdoctor.CollectSecretValues(probe.Getenv)

		// Verify no credential file was ever read.
		for _, cred := range credFiles {
			Expect(readPaths).NotTo(ContainElement(cred))
		}

		// Verify sentinel does not appear in table or JSON output.
		table := agentdoctor.FormatTable(report, secrets)
		Expect(table).NotTo(ContainSubstring(sentinel))

		jsonBytes, err := agentdoctor.FormatJSON(report, secrets)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(jsonBytes)).NotTo(ContainSubstring(sentinel))

		// Verify JSON is still valid and reports env var NAMES.
		var decoded agentdoctor.Report
		Expect(json.Unmarshal(jsonBytes, &decoded)).To(Succeed())
		Expect(findAgentReport(decoded, "aider").EnvVarsSet).To(ContainElement("ANTHROPIC_API_KEY"))
	})

	It("rejects unknown agent IDs with an error listing all valid IDs", func() {
		_, err := agentdoctor.FilterAgents([]string{"unknown-agent"})
		Expect(err).To(HaveOccurred())
		for _, id := range agentdoctor.ValidAgentIDs() {
			Expect(err.Error()).To(ContainSubstring(id))
		}
	})
})
