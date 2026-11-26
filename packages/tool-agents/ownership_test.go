package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"slices"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Managed entry ownership", func() {
	var (
		home    string
		specDir string
	)

	BeforeEach(func() {
		home = GinkgoT().TempDir()
		specDir = GinkgoT().TempDir()
	})

	apply := func(env Env, spec Spec) error {
		var out bytes.Buffer
		return NewAgentsPluginWithDeps(env, &out).ExecuteContext(context.Background(), "apply", []string{"--config", writeSpecFile(specDir, spec)})
	}

	It("keeps a Claude rule the user already had when DevEx stops declaring it", func() {
		env := newTestEnv(home, nil, "claude")
		settings := filepath.Join(home, ".claude", "settings.json")
		Expect(os.MkdirAll(filepath.Dir(settings), 0o700)).To(Succeed())
		Expect(os.WriteFile(settings, []byte("{\n  \"permissions\": {\n    \"allow\": [\n      \"Bash(git status *)\"\n    ]\n  }\n}\n"), 0o600)).To(Succeed())

		Expect(apply(env, Spec{Agents: []AgentSpec{{Name: AgentClaude, AllowedCommands: []string{"git status", "go test"}}}})).To(Succeed())
		Expect(apply(env, Spec{Agents: []AgentSpec{{Name: AgentClaude, AllowedCommands: []string{"make"}}}})).To(Succeed())

		data, err := os.ReadFile(settings)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(data)).To(ContainSubstring(`"Bash(git status *)"`))
		Expect(string(data)).NotTo(ContainSubstring(`"Bash(go test *)"`))
		Expect(string(data)).To(ContainSubstring(`"Bash(make *)"`))
	})

	It("keeps an OpenCode rule the user already had when DevEx stops declaring it", func() {
		env := newTestEnv(home, nil, "opencode")
		config := filepath.Join(home, ".config", "opencode", "opencode.json")
		Expect(os.MkdirAll(filepath.Dir(config), 0o700)).To(Succeed())
		Expect(os.WriteFile(config, []byte(`{"permissions": [{"action": "shell", "resource": "git status *", "effect": "allow"}]}`), 0o600)).To(Succeed())

		Expect(apply(env, Spec{Agents: []AgentSpec{{Name: AgentOpenCode, AllowedCommands: []string{"git status", "go test"}}}})).To(Succeed())
		Expect(apply(env, Spec{Agents: []AgentSpec{{Name: AgentOpenCode, AllowedCommands: []string{"make"}}}})).To(Succeed())

		data, err := os.ReadFile(config)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(data)).To(ContainSubstring(`"git status *"`))
		Expect(string(data)).NotTo(ContainSubstring(`"go test *"`))
		Expect(string(data)).To(ContainSubstring(`"make *"`))
	})

	It("still removes DevEx entries after an apply failed part-way through", func() {
		if runtime.GOOS == "windows" || os.Geteuid() == 0 {
			Skip("needs POSIX directory permissions enforced for the current user")
		}
		env := newTestEnv(home, nil, "claude", "gemini")
		geminiDir := filepath.Join(home, ".gemini")
		Expect(os.MkdirAll(geminiDir, 0o700)).To(Succeed())
		Expect(os.Chmod(geminiDir, 0o500)).To(Succeed())
		DeferCleanup(func() { _ = os.Chmod(geminiDir, 0o700) })

		withCommands := Spec{Agents: []AgentSpec{
			{Name: AgentClaude, AllowedCommands: []string{"go test"}},
			{Name: AgentGemini, AllowedCommands: []string{"go test"}},
		}}
		Expect(apply(env, withCommands)).To(HaveOccurred())
		Expect(os.Chmod(geminiDir, 0o700)).To(Succeed())

		Expect(apply(env, Spec{Agents: []AgentSpec{{Name: AgentClaude}, {Name: AgentGemini}}})).To(Succeed())
		data, err := os.ReadFile(filepath.Join(home, ".claude", "settings.json"))
		Expect(err).NotTo(HaveOccurred())
		Expect(string(data)).NotTo(ContainSubstring("go test"))
	})

	It("appends a declared OpenCode deny after user rules, keeps user rule objects verbatim, and restores them on withdrawal", func() {
		env := newTestEnv(home, nil, "opencode")
		config := filepath.Join(home, ".config", "opencode", "opencode.json")
		Expect(os.MkdirAll(filepath.Dir(config), 0o700)).To(Succeed())
		original := []OpenCodeRule{
			{Action: "shell", Resource: "git push *", Effect: "deny"},
			{Action: "shell", Resource: "git push *", Effect: "allow"},
			{Action: "shell", Resource: "ls *", Effect: "allow"},
			{Action: "shell", Resource: "ls *", Effect: "allow"},
		}
		Expect(os.WriteFile(config, []byte(`{"permissions": [
  {"action": "shell", "resource": "git push *", "effect": "deny", "note": "mine"},
  {"action": "shell", "resource": "git push *", "effect": "allow"},
  {"action": "shell", "resource": "ls *", "effect": "allow"},
  {"action": "shell", "resource": "ls *", "effect": "allow"}
]}`), 0o600)).To(Succeed())
		rules := func() []OpenCodeRule {
			doc, err := os.ReadFile(config)
			Expect(err).NotTo(HaveOccurred())
			var parsed struct {
				Permissions []OpenCodeRule `json:"permissions"`
			}
			Expect(json.Unmarshal(doc, &parsed)).To(Succeed())
			return parsed.Permissions
		}

		Expect(apply(env, Spec{Agents: []AgentSpec{{Name: AgentOpenCode, DeniedCommands: []string{"git push"}}}})).To(Succeed())
		Expect(rules()).To(Equal(append(slices.Clone(original), OpenCodeRule{Action: "shell", Resource: "git push *", Effect: "deny"})))
		Expect(os.ReadFile(config)).To(ContainSubstring(`"note": "mine"`))

		Expect(apply(env, Spec{Agents: []AgentSpec{{Name: AgentOpenCode, DeniedCommands: []string{"git push"}}}})).To(Succeed())
		Expect(rules()).To(HaveLen(5))

		Expect(apply(env, Spec{Agents: []AgentSpec{{Name: AgentOpenCode}}})).To(Succeed())
		Expect(rules()).To(Equal(original))
		Expect(os.ReadFile(config)).To(ContainSubstring(`"note": "mine"`))
	})

	It("still removes a withdrawn Claude rule after the removing write failed once", func() {
		if runtime.GOOS == "windows" || os.Geteuid() == 0 {
			Skip("needs POSIX directory permissions enforced for the current user")
		}
		env := newTestEnv(home, nil, "claude")
		claudeDir := filepath.Join(home, ".claude")
		Expect(apply(env, Spec{Agents: []AgentSpec{{Name: AgentClaude, AllowedCommands: []string{"go test"}}}})).To(Succeed())

		Expect(os.Chmod(claudeDir, 0o500)).To(Succeed())
		DeferCleanup(func() { _ = os.Chmod(claudeDir, 0o700) })
		Expect(apply(env, Spec{Agents: []AgentSpec{{Name: AgentClaude, AllowedCommands: []string{"make"}}}})).To(HaveOccurred())
		Expect(os.Chmod(claudeDir, 0o700)).To(Succeed())

		Expect(apply(env, Spec{Agents: []AgentSpec{{Name: AgentClaude, AllowedCommands: []string{"make"}}}})).To(Succeed())
		data, err := os.ReadFile(filepath.Join(claudeDir, "settings.json"))
		Expect(err).NotTo(HaveOccurred())
		Expect(string(data)).NotTo(ContainSubstring("go test"))
		Expect(string(data)).To(ContainSubstring("Bash(make *)"))
	})

	It("keeps a user OpenCode rule when a later agent fails after DevEx removed its own copy", func() {
		if runtime.GOOS == "windows" || os.Geteuid() == 0 {
			Skip("needs POSIX directory permissions enforced for the current user")
		}
		env := newTestEnv(home, nil, "opencode", "gemini")
		config := filepath.Join(home, ".config", "opencode", "opencode.json")
		Expect(os.MkdirAll(filepath.Dir(config), 0o700)).To(Succeed())
		Expect(os.WriteFile(config, []byte(`{"permissions": [{"action": "shell", "resource": "git push *", "effect": "deny"}]}`), 0o600)).To(Succeed())
		geminiDir := filepath.Join(home, ".gemini")
		Expect(os.MkdirAll(geminiDir, 0o700)).To(Succeed())

		Expect(apply(env, Spec{Agents: []AgentSpec{
			{Name: AgentOpenCode, DeniedCommands: []string{"git push"}},
			{Name: AgentGemini, DeniedCommands: []string{"git push"}},
		}})).To(Succeed())

		Expect(os.Chmod(geminiDir, 0o500)).To(Succeed())
		DeferCleanup(func() { _ = os.Chmod(geminiDir, 0o700) })
		withdrawn := Spec{Agents: []AgentSpec{{Name: AgentOpenCode}, {Name: AgentGemini}}}
		Expect(apply(env, withdrawn)).To(HaveOccurred())
		Expect(os.Chmod(geminiDir, 0o700)).To(Succeed())
		Expect(apply(env, withdrawn)).To(Succeed())

		doc, err := os.ReadFile(config)
		Expect(err).NotTo(HaveOccurred())
		var parsed struct {
			Permissions []OpenCodeRule `json:"permissions"`
		}
		Expect(json.Unmarshal(doc, &parsed)).To(Succeed())
		Expect(parsed.Permissions).To(Equal([]OpenCodeRule{{Action: "shell", Resource: "git push *", Effect: "deny"}}))
	})

	It("keeps ownership of a written rule when printing the status fails", func() {
		env := newTestEnv(home, nil, "claude")
		specPath := writeSpecFile(specDir, Spec{Agents: []AgentSpec{{Name: AgentClaude, AllowedCommands: []string{"go test"}}}})
		err := NewAgentsPluginWithDeps(env, failingWriter{}).ExecuteContext(context.Background(), "apply", []string{"--config", specPath})
		Expect(err).To(HaveOccurred())

		Expect(apply(env, Spec{Agents: []AgentSpec{{Name: AgentClaude}}})).To(Succeed())
		data, readErr := os.ReadFile(filepath.Join(home, ".claude", "settings.json"))
		Expect(readErr).NotTo(HaveOccurred())
		Expect(string(data)).NotTo(ContainSubstring("go test"))
	})

	It("refuses to replace or delete a Codex devex.rules file DevEx did not write", func() {
		env := newTestEnv(home, nil, "codex")
		rules := filepath.Join(home, ".codex", "rules", "devex.rules")
		Expect(os.MkdirAll(filepath.Dir(rules), 0o700)).To(Succeed())
		original := []byte("prefix_rule(pattern = [\"ls\"])\n")
		Expect(os.WriteFile(rules, original, 0o600)).To(Succeed())
		before := snapshotTree(home)

		err := apply(env, Spec{Agents: []AgentSpec{{Name: AgentCodex, AllowedCommands: []string{"git status"}}}})
		Expect(err).To(MatchError(ContainSubstring("was not written by DevEx")))
		err = apply(env, Spec{Agents: []AgentSpec{{Name: AgentCodex}}})
		Expect(err).To(MatchError(ContainSubstring("was not written by DevEx")))
		Expect(snapshotTree(home)).To(Equal(before))
	})
})

type failingWriter struct{}

func (failingWriter) Write([]byte) (int, error) { return 0, errors.New("write failed") }
