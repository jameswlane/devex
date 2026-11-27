package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/jameswlane/devex/apps/cli/internal/agentdoctor"
	"github.com/jameswlane/devex/apps/cli/internal/agentsandbox"
	"github.com/jameswlane/devex/apps/cli/internal/localmodels"
	"github.com/jameswlane/devex/apps/cli/internal/log"
	"github.com/jameswlane/devex/apps/cli/internal/testhelper"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Agent doctor and login commands", func() {
	var home string

	BeforeEach(func() {
		home = GinkgoT().TempDir()
		testhelper.SetProfileEnv(GinkgoT(), home)
		GinkgoT().Setenv("XDG_STATE_HOME", "")
		GinkgoT().Setenv("DEVEX_STATE_DIR", filepath.Join(home, "state"))
	})

	makeFakeBinProbe := func(binDir string, extraEnv map[string]string) agentdoctor.Probe {
		env := map[string]string{
			"HOME": home,
			"PATH": binDir,
		}
		for k, v := range extraEnv {
			env[k] = v
		}
		return agentdoctor.Probe{
			GOOS: "linux",
			Now:  func() time.Time { return time.Date(2026, 4, 15, 12, 0, 0, 0, time.UTC) },
			Getenv: func(key string) string {
				return env[key]
			},
			Stat:         os.Stat,
			ReadFile:     os.ReadFile,
			EvalSymlinks: filepath.EvalSymlinks,
			Run: func(ctx context.Context, name string, args ...string) ([]byte, error) {
				cmd := exec.CommandContext(ctx, name, args...)
				for k, v := range env {
					cmd.Env = append(cmd.Env, k+"="+v)
				}
				return cmd.Output()
			},
			SandboxProbe: agentsandbox.Probe{
				GOOS: "linux",
				LookPath: func(name string) (string, error) {
					if name == "bwrap" || name == "socat" {
						return "/usr/bin/" + name, nil
					}
					return "", exec.ErrNotFound
				},
				Run: func(context.Context, string, ...string) ([]byte, error) { return nil, nil },
			},
		}
	}

	Context("devex agent doctor", func() {
		It("outputs table and JSON reports, filters by --agent, and exits 0 even when problems exist", func() {
			if runtime.GOOS == "windows" {
				Skip("POSIX shell script fake binaries require Unix")
			}
			binDir := filepath.Join(home, "bin")
			Expect(os.MkdirAll(binDir, 0o755)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(binDir, "codex"), []byte("#!/bin/sh\nif [ \"$1\" = \"--version\" ]; then echo 'codex-cli 0.172.0'; exit 0; fi\nexit 1\n"), 0o755)).To(Succeed())

			probe := makeFakeBinProbe(binDir, nil)

			// Table output with --agent filter.
			cmd := NewAgentCmdFromDeps(AgentCmdDeps{DoctorProbe: probe})
			var stdout, stderr bytes.Buffer
			cmd.SetOut(&stdout)
			cmd.SetErr(&stderr)
			cmd.SetArgs([]string{"doctor", "--agent", "codex", "--agent", "aider"})
			Expect(cmd.Execute()).To(Succeed())
			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).To(And(
				ContainSubstring("codex"),
				ContainSubstring("0.172.0"),
				ContainSubstring("not_logged_in"),
				ContainSubstring("devex agent login codex"),
				ContainSubstring("aider"),
				ContainSubstring("not installed"),
			))

			// JSON output exits 0 despite detected problems.
			cmdJSON := NewAgentCmdFromDeps(AgentCmdDeps{DoctorProbe: probe})
			stdout.Reset()
			stderr.Reset()
			cmdJSON.SetOut(&stdout)
			cmdJSON.SetErr(&stderr)
			cmdJSON.SetArgs([]string{"doctor", "--format", "json", "--agent", "codex"})
			Expect(cmdJSON.Execute()).To(Succeed())
			Expect(stderr.String()).To(BeEmpty())

			var report agentdoctor.Report
			Expect(json.Unmarshal(stdout.Bytes(), &report)).To(Succeed())
			Expect(report.Agents).To(HaveLen(1))
			Expect(report.Agents[0].ID).To(Equal("codex"))
			Expect(report.Problems).NotTo(BeEmpty())
		})

		It("writes the table to stdout, accepts --json, and never echoes rejected input", func() {
			probe := makeFakeBinProbe("", nil)

			readStdout := func(args ...string) (string, string, error) {
				r, w, err := os.Pipe()
				Expect(err).NotTo(HaveOccurred())
				original := os.Stdout
				os.Stdout = w
				defer func() { os.Stdout = original }()
				cmd := NewAgentCmdFromDeps(AgentCmdDeps{DoctorProbe: probe})
				var stderr bytes.Buffer
				cmd.SetErr(&stderr)
				cmd.SetArgs(args)
				runErr := cmd.Execute()
				Expect(w.Close()).To(Succeed())
				out, err := io.ReadAll(r)
				Expect(err).NotTo(HaveOccurred())
				return string(out), stderr.String(), runErr
			}

			out, errOut, err := readStdout("doctor")
			Expect(err).NotTo(HaveOccurred())
			Expect(out).To(ContainSubstring("Agent Doctor"))
			Expect(errOut).To(BeEmpty())

			out, _, err = readStdout("doctor", "--json")
			Expect(err).NotTo(HaveOccurred())
			var report agentdoctor.Report
			Expect(json.Unmarshal([]byte(out), &report)).To(Succeed())
			Expect(report.Agents).NotTo(BeEmpty())

			const pasted = "sk-pasted-into-the-wrong-flag"
			for _, args := range [][]string{
				{"doctor", "--agent", pasted},
				{"doctor", "--agent", `"` + pasted},
				{"doctor", "--format", pasted},
				{"doctor", "--json=" + pasted},
				{"doctor", "--" + pasted},
				{"doctor", pasted},
				{"login", pasted},
				{"login", "--" + pasted, "claude"},
			} {
				_, errOut, err = readStdout(args...)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).NotTo(ContainSubstring(pasted))
				Expect(errOut).NotTo(ContainSubstring(pasted))
			}
		})

		It("rejects unsupported --format values and unknown --agent IDs", func() {
			probe := makeFakeBinProbe("", nil)

			cmdBadFmt := NewAgentCmdFromDeps(AgentCmdDeps{DoctorProbe: probe})
			cmdBadFmt.SetOut(io.Discard)
			cmdBadFmt.SetErr(io.Discard)
			cmdBadFmt.SetArgs([]string{"doctor", "--format", "yaml"})
			Expect(cmdBadFmt.Execute()).To(MatchError(ContainSubstring(`unsupported format`)))

			cmdBadAgent := NewAgentCmdFromDeps(AgentCmdDeps{DoctorProbe: probe})
			cmdBadAgent.SetOut(io.Discard)
			cmdBadAgent.SetErr(io.Discard)
			cmdBadAgent.SetArgs([]string{"doctor", "--agent", "notanagent"})
			err := cmdBadAgent.Execute()
			Expect(err).To(HaveOccurred())
			for _, id := range agentdoctor.ValidAgentIDs() {
				Expect(err.Error()).To(ContainSubstring(id))
			}
		})

		It("never leaks secret sentinels to stdout, stderr, or the DevEx log file", func() {
			if runtime.GOOS == "windows" {
				Skip("POSIX shell script fake binaries require Unix")
			}
			const sentinel = "sk-devex-sentinel-XYZ"
			binDir := filepath.Join(home, "bin")
			Expect(os.MkdirAll(binDir, 0o755)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(binDir, "claude"), []byte("#!/bin/sh\nif [ \"$1\" = \"--version\" ]; then echo '2.1.0 "+sentinel+"'; exit 0; fi\necho '"+sentinel+"'\necho '"+sentinel+"' >&2\nexit 0\n"), 0o755)).To(Succeed())

			credFile := filepath.Join(home, ".claude", ".credentials.json")
			Expect(os.MkdirAll(filepath.Dir(credFile), 0o700)).To(Succeed())
			Expect(os.WriteFile(credFile, []byte(`{"token":"`+sentinel+`"}`), 0o600)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(home, ".claude.json"), []byte(`{"mcpServers":{"s":{"env":{"API_KEY":"`+sentinel+`"}}`), 0o600)).To(Succeed())

			envMap := map[string]string{}
			for _, name := range agentdoctor.AllKnownSecretEnvVars() {
				GinkgoT().Setenv(name, sentinel)
				envMap[name] = sentinel
			}

			// Initialize a real file logger to verify nothing sensitive hits the DevEx log file.
			Expect(log.InitFileLogger(true)).To(Succeed())
			DeferCleanup(func() {
				_ = log.Close()
				testhelper.SuppressLogs()
			})

			probe := makeFakeBinProbe(binDir, envMap)
			for _, format := range []string{"table", "json"} {
				cmd := NewAgentCmdFromDeps(AgentCmdDeps{DoctorProbe: probe})
				var stdout, stderr bytes.Buffer
				cmd.SetOut(&stdout)
				cmd.SetErr(&stderr)
				cmd.SetArgs([]string{"doctor", "--format", format})
				Expect(cmd.Execute()).To(Succeed())
				Expect(stdout.String()).NotTo(ContainSubstring(sentinel))
				Expect(stderr.String()).NotTo(ContainSubstring(sentinel))
			}
			Expect(log.Close()).To(Succeed())

			matches, err := filepath.Glob(filepath.Join(home, "state", "logs", "*.log"))
			Expect(err).NotTo(HaveOccurred())
			for _, logPath := range matches {
				data, readErr := os.ReadFile(logPath)
				Expect(readErr).NotTo(HaveOccurred())
				Expect(string(data)).NotTo(ContainSubstring(sentinel))
			}
		})
	})

	Context("devex agent login", func() {
		It("requires exactly one valid agent argument", func() {
			probe := makeFakeBinProbe("", nil)

			cmdNoArgs := NewAgentCmdFromDeps(AgentCmdDeps{DoctorProbe: probe})
			cmdNoArgs.SetOut(io.Discard)
			cmdNoArgs.SetErr(io.Discard)
			cmdNoArgs.SetArgs([]string{"login"})
			Expect(cmdNoArgs.Execute()).To(HaveOccurred())

			cmdUnknown := NewAgentCmdFromDeps(AgentCmdDeps{DoctorProbe: probe})
			cmdUnknown.SetOut(io.Discard)
			cmdUnknown.SetErr(io.Discard)
			cmdUnknown.SetArgs([]string{"login", "mystery"})
			err := cmdUnknown.Execute()
			Expect(err).To(HaveOccurred())
			for _, id := range agentdoctor.ValidAgentIDs() {
				Expect(err.Error()).To(ContainSubstring(id))
			}
		})

		It("returns install guidance when the requested agent is not installed", func() {
			called := false
			probe := makeFakeBinProbe("", nil)
			cmd := NewAgentCmdFromDeps(AgentCmdDeps{
				DoctorProbe: probe,
				LoginRunner: func(context.Context, string, []string, io.Reader, io.Writer, io.Writer) error {
					called = true
					return nil
				},
			})
			cmd.SetOut(io.Discard)
			cmd.SetErr(io.Discard)
			cmd.SetArgs([]string{"login", "claude"})

			err := cmd.Execute()
			Expect(err).To(MatchError(And(
				ContainSubstring("not installed"),
				ContainSubstring(`devex install "Claude Code"`),
			)))
			Expect(called).To(BeFalse())
		})

		DescribeTable("returns guidance without invoking runner for agents with no CLI login flow",
			func(agentID, binary, expectedHint string) {
				if runtime.GOOS == "windows" {
					Skip("POSIX fake binary requires Unix")
				}
				binDir := filepath.Join(home, "bin")
				Expect(os.MkdirAll(binDir, 0o755)).To(Succeed())
				Expect(os.WriteFile(filepath.Join(binDir, binary), []byte("#!/bin/sh\nexit 0\n"), 0o755)).To(Succeed())

				called := false
				cmd := NewAgentCmdFromDeps(AgentCmdDeps{
					DoctorProbe: makeFakeBinProbe(binDir, nil),
					LoginRunner: func(context.Context, string, []string, io.Reader, io.Writer, io.Writer) error {
						called = true
						return nil
					},
				})
				cmd.SetOut(io.Discard)
				cmd.SetErr(io.Discard)
				cmd.SetArgs([]string{"login", agentID})

				err := cmd.Execute()
				Expect(err).To(MatchError(ContainSubstring(expectedHint)))
				Expect(called).To(BeFalse())
			},
			Entry("aider", "aider", "aider", "ANTHROPIC_API_KEY"),
			Entry("gemini", "gemini", "gemini", "/auth"),
		)

		DescribeTable("hands off interactive login to the right binary and args with terminal streams attached",
			func(agentID, binary string, expectedArgs []string) {
				if runtime.GOOS == "windows" {
					Skip("POSIX fake binary requires Unix")
				}
				binDir := filepath.Join(home, "bin")
				Expect(os.MkdirAll(binDir, 0o755)).To(Succeed())
				binPath := filepath.Join(binDir, binary)
				Expect(os.WriteFile(binPath, []byte("#!/bin/sh\nexit 0\n"), 0o755)).To(Succeed())

				var (
					gotBin    string
					gotArgs   []string
					gotStdin  io.Reader
					gotStdout io.Writer
					gotStderr io.Writer
				)
				inBuf := strings.NewReader("user-input\n")
				var outBuf, errBuf bytes.Buffer

				cmd := NewAgentCmdFromDeps(AgentCmdDeps{
					DoctorProbe: makeFakeBinProbe(binDir, nil),
					LoginRunner: func(_ context.Context, path string, args []string, stdin io.Reader, stdout, stderr io.Writer) error {
						gotBin = path
						gotArgs = append([]string(nil), args...)
						gotStdin = stdin
						gotStdout = stdout
						gotStderr = stderr
						return nil
					},
				})
				cmd.SetIn(inBuf)
				cmd.SetOut(&outBuf)
				cmd.SetErr(&errBuf)
				cmd.SetArgs([]string{"login", agentID})

				Expect(cmd.Execute()).To(Succeed())
				Expect(gotBin).To(Equal(binPath))
				Expect(gotArgs).To(Equal(expectedArgs))
				Expect(gotStdin).To(Equal(inBuf))
				Expect(gotStdout).To(Equal(&outBuf))
				Expect(gotStderr).To(Equal(&errBuf))
			},
			Entry("claude", "claude", "claude", []string{"auth", "login"}),
			Entry("codex", "codex", "codex", []string{"login"}),
			Entry("copilot", "copilot", "copilot", []string{"login"}),
			Entry("opencode", "opencode", "opencode", []string{"auth", "login"}),
			Entry("goose", "goose", "goose", []string{"configure"}),
			Entry("kimi", "kimi", "kimi", []string{"login"}),
		)

		It("propagates the child login command exit error unchanged", func() {
			if runtime.GOOS == "windows" {
				Skip("POSIX fake binary requires Unix")
			}
			binDir := filepath.Join(home, "bin")
			Expect(os.MkdirAll(binDir, 0o755)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(binDir, "codex"), []byte("#!/bin/sh\nexit 0\n"), 0o755)).To(Succeed())

			sentinelErr := errors.New("exit status 2")
			cmd := NewAgentCmdFromDeps(AgentCmdDeps{
				DoctorProbe: makeFakeBinProbe(binDir, nil),
				LoginRunner: func(context.Context, string, []string, io.Reader, io.Writer, io.Writer) error {
					return sentinelErr
				},
			})
			cmd.SetOut(io.Discard)
			cmd.SetErr(io.Discard)
			cmd.SetArgs([]string{"login", "codex"})

			Expect(cmd.Execute()).To(MatchError(sentinelErr))
		})
	})

	Context("devex agent doctor local-model-runtime section", func() {
		It("reports installed ollama runtime, service status, bound address, GPU report, and hints to stdout with redaction intact", func() {
			const sentinel = "sk-localmodel-secret-999"
			ollamaSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/api/tags" {
					_, _ = w.Write([]byte(`{"models":[]}`))
					return
				}
				http.NotFound(w, r)
			}))
			DeferCleanup(ollamaSrv.Close)

			fsRoot := GinkgoT().TempDir()
			Expect(os.MkdirAll(filepath.Join(fsRoot, "run/systemd/system"), 0o755)).To(Succeed())
			Expect(os.MkdirAll(filepath.Join(fsRoot, "sys/bus/pci/devices/0000-01-00.0"), 0o755)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(fsRoot, "sys/bus/pci/devices/0000-01-00.0/class"), []byte("0x030000\n"), 0o644)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(fsRoot, "sys/bus/pci/devices/0000-01-00.0/vendor"), []byte("0x10de\n"), 0o644)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(fsRoot, "sys/bus/pci/devices/0000-01-00.0/device"), []byte("0x2684\n"), 0o644)).To(Succeed())

			GinkgoT().Setenv("OPENAI_API_KEY", sentinel)
			probe := makeFakeBinProbe("", map[string]string{"OPENAI_API_KEY": sentinel})
			probe.LocalModelsProbe = &localmodels.Probe{
				GOOS:       "linux",
				FSRoot:     fsRoot,
				HTTPClient: ollamaSrv.Client(),
				Getenv: func(key string) string {
					if key == "OLLAMA_HOST" {
						return ollamaSrv.URL
					}
					return ""
				},
				LookPath: func(name string) (string, error) {
					switch name {
					case "ollama":
						return "/usr/local/bin/ollama", nil
					case "systemctl":
						return "/usr/bin/systemctl", nil
					default:
						return "", exec.ErrNotFound
					}
				},
				Run: func(_ context.Context, name string, args ...string) ([]byte, error) {
					switch name {
					case "/usr/local/bin/ollama":
						return []byte("ollama version is 0.6.2 " + sentinel + "\n"), nil
					case "/usr/bin/systemctl":
						return []byte("active\n"), nil
					default:
						return nil, errors.New("unexpected")
					}
				},
			}

			cmd := NewAgentCmdFromDeps(AgentCmdDeps{DoctorProbe: probe})
			var stdout, stderr bytes.Buffer
			cmd.SetOut(&stdout)
			cmd.SetErr(&stderr)
			cmd.SetArgs([]string{"doctor", "--agent", "claude"})
			Expect(cmd.Execute()).To(Succeed())
			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).NotTo(ContainSubstring(sentinel))
			Expect(stdout.String()).To(And(
				ContainSubstring("Local Model Runtime:"),
				ContainSubstring("ollama"),
				ContainSubstring("installed (0.6.2)"),
				ContainSubstring("service:"),
				ContainSubstring("active"),
				ContainSubstring("address:"),
				ContainSubstring("reachable"),
				ContainSubstring("gpu:"),
				ContainSubstring("CPU only"),
				ContainSubstring("sudo ubuntu-drivers install"),
			))

			cmdJSON := NewAgentCmdFromDeps(AgentCmdDeps{DoctorProbe: probe})
			stdout.Reset()
			stderr.Reset()
			cmdJSON.SetOut(&stdout)
			cmdJSON.SetErr(&stderr)
			cmdJSON.SetArgs([]string{"doctor", "--json", "--agent", "claude"})
			Expect(cmdJSON.Execute()).To(Succeed())
			Expect(stderr.String()).To(BeEmpty())
			Expect(stdout.String()).NotTo(ContainSubstring(sentinel))

			var report agentdoctor.Report
			Expect(json.Unmarshal(stdout.Bytes(), &report)).To(Succeed())
			Expect(report.LocalModelRuntime).NotTo(BeNil())
			Expect(report.LocalModelRuntime.Installed).To(BeTrue())
			Expect(report.LocalModelRuntime.ServiceActive).To(BeTrue())
			Expect(report.LocalModelRuntime.APIReachable).To(BeTrue())
			Expect(report.LocalModelRuntime.GPU.Hints).NotTo(BeEmpty())
		})
	})
})
