package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"runtime"

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

	It("records the entries of agents that finished when a later agent fails to write", func() {
		if runtime.GOOS == "windows" || os.Geteuid() == 0 {
			Skip("needs POSIX directory permissions enforced for the current user")
		}
		env := newTestEnv(home, nil, "claude", "gemini")
		geminiDir := filepath.Join(home, ".gemini")
		Expect(os.MkdirAll(geminiDir, 0o700)).To(Succeed())
		Expect(os.Chmod(geminiDir, 0o500)).To(Succeed())
		DeferCleanup(func() { _ = os.Chmod(geminiDir, 0o700) })

		spec := Spec{Agents: []AgentSpec{
			{Name: AgentClaude, AllowedCommands: []string{"go test"}},
			{Name: AgentGemini, AllowedCommands: []string{"go test"}},
		}}
		Expect(apply(env, spec)).To(HaveOccurred())

		state, err := LoadManagedState(filepath.Join(home, ".local", "state", "devex", "agents", "managed.json"))
		Expect(err).NotTo(HaveOccurred())
		Expect(state.Agents[AgentClaude].Allow).To(Equal([]string{"Bash(go test *)"}))
		Expect(state.Agents).NotTo(HaveKey(AgentGemini))
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
