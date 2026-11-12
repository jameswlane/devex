package commands_test

import (
	"bytes"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/jameswlane/devex/apps/cli/internal/agentskills"
	"github.com/jameswlane/devex/apps/cli/internal/commands"
	"github.com/jameswlane/devex/apps/cli/internal/config"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/viper"
)

func runAgentCommand(args ...string) (string, string, error) {
	cmd := commands.NewAgentCmd()
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs(args)
	err := cmd.Execute()
	return stdout.String(), stderr.String(), err
}

func expectBundleMatches(destination string) {
	bundle, err := agentskills.Bundle()
	Expect(err).NotTo(HaveOccurred())
	expected := map[string][]byte{}
	Expect(fs.WalkDir(bundle, ".", func(path string, entry fs.DirEntry, err error) error {
		Expect(err).NotTo(HaveOccurred())
		if entry.IsDir() {
			return nil
		}
		data, err := fs.ReadFile(bundle, path)
		Expect(err).NotTo(HaveOccurred())
		expected[path] = data
		return nil
	})).To(Succeed())
	actual := map[string][]byte{}
	Expect(filepath.WalkDir(destination, func(path string, entry fs.DirEntry, err error) error {
		Expect(err).NotTo(HaveOccurred())
		if entry.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(destination, path)
		Expect(err).NotTo(HaveOccurred())
		data, err := os.ReadFile(path)
		Expect(err).NotTo(HaveOccurred())
		actual[filepath.ToSlash(rel)] = data
		return nil
	})).To(Succeed())
	Expect(actual).To(Equal(expected))
}

var _ = Describe("Agent skill installer", func() {
	var home string

	BeforeEach(func() {
		home = GinkgoT().TempDir()
		GinkgoT().Setenv("HOME", home)
		GinkgoT().Setenv("KIMI_CODE_HOME", "")
	})

	It("installs the embedded Claude skill at user scope", func() {
		stdout, stderr, err := runAgentCommand("install-skill", "--agent", "claude", "--scope", "user")
		Expect(err).NotTo(HaveOccurred())
		Expect(stderr).To(BeEmpty())
		destination := filepath.Join(home, ".claude", "skills", "devex")
		Expect(stdout).To(ContainSubstring("Destination: " + destination))
		installed, err := os.ReadFile(filepath.Join(destination, "SKILL.md"))
		Expect(err).NotTo(HaveOccurred())
		embedded, err := agentskills.Skill()
		Expect(err).NotTo(HaveOccurred())
		Expect(string(installed)).To(Equal(embedded))
	})

	DescribeTable("installs the complete bundle for each agent and scope",
		func(agent, scope string) {
			project := filepath.Join(home, "project with spaces")
			Expect(os.MkdirAll(project, 0o755)).To(Succeed())
			args := []string{"install-skill", "--agent", agent, "--scope", scope}
			if scope == "project" {
				args = append(args, "--project-dir", project)
			}
			stdout, _, err := runAgentCommand(args...)
			Expect(err).NotTo(HaveOccurred())
			root := home
			if scope == "project" {
				root = project
			}
			var directory string
			switch agent {
			case "claude":
				directory = ".claude"
			case "codex":
				directory = ".agents"
			case "kimi":
				directory = ".kimi-code"
			}
			destination := filepath.Join(root, directory, "skills", "devex")
			Expect(stdout).To(ContainSubstring("Destination: " + destination))
			expectBundleMatches(destination)
			info, err := os.Stat(filepath.Join(destination, "SKILL.md"))
			Expect(err).NotTo(HaveOccurred())
			Expect(info.Mode().Perm()).To(Equal(os.FileMode(0o644)))
		},
		Entry("Claude user", "claude", "user"),
		Entry("Claude project", "claude", "project"),
		Entry("Codex user", "codex", "user"),
		Entry("Codex project", "codex", "project"),
		Entry("Kimi user", "kimi", "user"),
		Entry("Kimi project", "kimi", "project"),
	)

	It("uses the current working directory when --project-dir is omitted", func() {
		project := GinkgoT().TempDir()
		previous, err := os.Getwd()
		Expect(err).NotTo(HaveOccurred())
		Expect(os.Chdir(project)).To(Succeed())
		DeferCleanup(func() { Expect(os.Chdir(previous)).To(Succeed()) })
		_, _, err = runAgentCommand("install-skill", "--agent", "claude", "--scope", "project")
		Expect(err).NotTo(HaveOccurred())
		expectBundleMatches(filepath.Join(project, ".claude", "skills", "devex"))
	})

	It("honors KIMI_CODE_HOME for a user installation", func() {
		custom := filepath.Join(home, "custom kimi home")
		GinkgoT().Setenv("KIMI_CODE_HOME", custom)
		_, _, err := runAgentCommand("install-skill", "--agent", "kimi", "--scope", "user")
		Expect(err).NotTo(HaveOccurred())
		expectBundleMatches(filepath.Join(custom, "skills", "devex"))
		Expect(filepath.Join(home, ".kimi-code")).NotTo(BeADirectory())
	})

	It("leaves an identical reinstall and unrelated skills untouched", func() {
		destination := filepath.Join(home, ".claude", "skills", "devex")
		unrelated := filepath.Join(home, ".claude", "skills", "other", "SKILL.md")
		Expect(os.MkdirAll(filepath.Dir(unrelated), 0o755)).To(Succeed())
		Expect(os.WriteFile(unrelated, []byte("other"), 0o644)).To(Succeed())
		_, _, err := runAgentCommand("install-skill", "--agent", "claude", "--scope", "user")
		Expect(err).NotTo(HaveOccurred())
		old := time.Now().Add(-24 * time.Hour)
		skill := filepath.Join(destination, "SKILL.md")
		Expect(os.Chtimes(skill, old, old)).To(Succeed())
		stdout, _, err := runAgentCommand("install-skill", "--agent", "claude", "--scope", "user")
		Expect(err).NotTo(HaveOccurred())
		Expect(stdout).To(ContainSubstring("Already installed: " + destination))
		info, err := os.Stat(skill)
		Expect(err).NotTo(HaveOccurred())
		Expect(info.ModTime().Equal(old)).To(BeTrue())
		Expect(os.ReadFile(unrelated)).To(Equal([]byte("other")))
	})

	It("refuses differing content without --overwrite", func() {
		destination := filepath.Join(home, ".claude", "skills", "devex")
		Expect(os.MkdirAll(destination, 0o755)).To(Succeed())
		customized := filepath.Join(destination, "SKILL.md")
		Expect(os.WriteFile(customized, []byte("local edits"), 0o644)).To(Succeed())
		_, _, err := runAgentCommand("install-skill", "--agent", "claude", "--scope", "user")
		Expect(err).To(MatchError(ContainSubstring("--overwrite")))
		Expect(os.ReadFile(customized)).To(Equal([]byte("local edits")))
		matches, globErr := filepath.Glob(filepath.Join(home, ".claude", "skills", ".devex-*"))
		Expect(globErr).NotTo(HaveOccurred())
		Expect(matches).To(BeEmpty())
	})

	It("atomically replaces customized content and retains a recoverable backup", func() {
		destination := filepath.Join(home, ".claude", "skills", "devex")
		Expect(os.MkdirAll(destination, 0o755)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(destination, "SKILL.md"), []byte("local edits"), 0o644)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(destination, "local.txt"), []byte("keep in backup"), 0o644)).To(Succeed())
		stdout, _, err := runAgentCommand("install-skill", "--agent", "claude", "--scope", "user", "--overwrite")
		Expect(err).NotTo(HaveOccurred())
		Expect(stdout).To(ContainSubstring("Backup: "))
		expectBundleMatches(destination)
		matches, err := filepath.Glob(filepath.Join(home, ".claude", "skills", ".devex-backup-*"))
		Expect(err).NotTo(HaveOccurred())
		Expect(matches).To(HaveLen(1))
		Expect(os.ReadFile(filepath.Join(matches[0], "SKILL.md"))).To(Equal([]byte("local edits")))
		Expect(os.ReadFile(filepath.Join(matches[0], "local.txt"))).To(Equal([]byte("keep in backup")))
	})

	It("dry-runs a fresh install with zero writes", func() {
		stdout, _, err := runAgentCommand("install-skill", "--agent", "claude", "--scope", "user", "--dry-run")
		Expect(err).NotTo(HaveOccurred())
		Expect(stdout).To(And(ContainSubstring("Destination: "), ContainSubstring("Would install")))
		entries, err := os.ReadDir(home)
		Expect(err).NotTo(HaveOccurred())
		Expect(entries).To(BeEmpty())
	})

	It("dry-runs an overwrite without changing customized content or creating a backup", func() {
		destination := filepath.Join(home, ".claude", "skills", "devex")
		Expect(os.MkdirAll(destination, 0o755)).To(Succeed())
		customized := filepath.Join(destination, "SKILL.md")
		Expect(os.WriteFile(customized, []byte("local edits"), 0o644)).To(Succeed())
		stdout, _, err := runAgentCommand("install-skill", "--agent", "claude", "--scope", "user", "--overwrite", "--dry-run")
		Expect(err).NotTo(HaveOccurred())
		Expect(stdout).To(ContainSubstring("Would replace"))
		Expect(os.ReadFile(customized)).To(Equal([]byte("local edits")))
		matches, err := filepath.Glob(filepath.Join(home, ".claude", "skills", ".devex-backup-*"))
		Expect(err).NotTo(HaveOccurred())
		Expect(matches).To(BeEmpty())
	})

	DescribeTable("rejects symlink escapes without writing outside the selected root",
		func(linkTarget bool) {
			outside := GinkgoT().TempDir()
			if linkTarget {
				Expect(os.MkdirAll(filepath.Join(home, ".claude", "skills"), 0o755)).To(Succeed())
				Expect(os.Symlink(outside, filepath.Join(home, ".claude", "skills", "devex"))).To(Succeed())
			} else {
				Expect(os.Symlink(outside, filepath.Join(home, ".claude"))).To(Succeed())
			}
			_, _, err := runAgentCommand("install-skill", "--agent", "claude", "--scope", "user", "--overwrite")
			Expect(err).To(HaveOccurred())
			entries, readErr := os.ReadDir(outside)
			Expect(readErr).NotTo(HaveOccurred())
			Expect(entries).To(BeEmpty())
		},
		Entry("skill directory", true),
		Entry("parent directory", false),
	)

	It("rejects a parent symlink even when its target remains inside the selected root", func() {
		linked := filepath.Join(home, "elsewhere")
		Expect(os.MkdirAll(linked, 0o755)).To(Succeed())
		Expect(os.Symlink("elsewhere", filepath.Join(home, ".claude"))).To(Succeed())
		_, _, err := runAgentCommand("install-skill", "--agent", "claude", "--scope", "user")
		Expect(err).To(MatchError(ContainSubstring("symlink")))
		entries, readErr := os.ReadDir(linked)
		Expect(readErr).NotTo(HaveOccurred())
		Expect(entries).To(BeEmpty())
	})

	DescribeTable("skips Kimi when a Codex installation already covers it",
		func(scope string) {
			project := filepath.Join(home, "project")
			Expect(os.MkdirAll(project, 0o755)).To(Succeed())
			root := home
			args := []string{"install-skill", "--agent", "kimi", "--scope", scope}
			if scope == "project" {
				root = project
				args = append(args, "--project-dir", project)
			}
			codex := filepath.Join(root, ".agents", "skills", "devex")
			Expect(os.MkdirAll(codex, 0o755)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(codex, "SKILL.md"), []byte("existing"), 0o644)).To(Succeed())
			destination := filepath.Join(root, ".kimi-code", "skills", "devex")
			stdout, stderr, err := runAgentCommand(args...)
			Expect(err).NotTo(HaveOccurred())
			Expect(stdout).To(And(ContainSubstring("Destination: "+destination), ContainSubstring("Skipped: "+destination)))
			Expect(stderr).To(And(ContainSubstring("Warning:"), ContainSubstring(codex), ContainSubstring("no duplicate was created")))
			Expect(destination).NotTo(BeADirectory())
			Expect(os.ReadFile(filepath.Join(codex, "SKILL.md"))).To(Equal([]byte("existing")))
		},
		Entry("user scope", "user"),
		Entry("project scope", "project"),
	)

	DescribeTable("installs for Codex and warns about an existing Kimi copy",
		func(scope string) {
			project := filepath.Join(home, "project")
			Expect(os.MkdirAll(project, 0o755)).To(Succeed())
			root := home
			args := []string{"install-skill", "--agent", "codex", "--scope", scope}
			if scope == "project" {
				root = project
				args = append(args, "--project-dir", project)
			}
			kimi := filepath.Join(root, ".kimi-code", "skills", "devex")
			Expect(os.MkdirAll(kimi, 0o755)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(kimi, "SKILL.md"), []byte("existing"), 0o644)).To(Succeed())
			destination := filepath.Join(root, ".agents", "skills", "devex")
			stdout, stderr, err := runAgentCommand(args...)
			Expect(err).NotTo(HaveOccurred())
			Expect(stdout).To(And(ContainSubstring("Destination: "+destination), ContainSubstring("Installed: "+destination)))
			Expect(stderr).To(And(ContainSubstring("Warning:"), ContainSubstring("load DevEx twice"), ContainSubstring(kimi), ContainSubstring("Remove")))
			expectBundleMatches(destination)
			Expect(os.ReadFile(filepath.Join(kimi, "SKILL.md"))).To(Equal([]byte("existing")))
		},
		Entry("user scope", "user"),
		Entry("project scope", "project"),
	)

	DescribeTable("rejects invalid selections before writing",
		func(args ...string) {
			_, _, err := runAgentCommand(args...)
			Expect(err).To(HaveOccurred())
			entries, readErr := os.ReadDir(home)
			Expect(readErr).NotTo(HaveOccurred())
			Expect(entries).To(BeEmpty())
		},
		Entry("unsupported agent", "install-skill", "--agent", "cursor", "--scope", "user"),
		Entry("unsupported scope", "install-skill", "--agent", "claude", "--scope", "system"),
		Entry("missing agent", "install-skill", "--scope", "user"),
		Entry("missing scope", "install-skill", "--agent", "claude"),
		Entry("project directory at user scope", "install-skill", "--agent", "claude", "--scope", "user", "--project-dir", "elsewhere"),
	)

	It("reports an unwritable skill parent without changing it", func() {
		if os.Geteuid() == 0 {
			Skip("permission checks do not restrict root")
		}
		parent := filepath.Join(home, ".claude", "skills")
		Expect(os.MkdirAll(parent, 0o755)).To(Succeed())
		Expect(os.Chmod(parent, 0o500)).To(Succeed())
		DeferCleanup(func() { Expect(os.Chmod(parent, 0o700)).To(Succeed()) })
		_, _, err := runAgentCommand("install-skill", "--agent", "claude", "--scope", "user")
		Expect(err).To(MatchError(ContainSubstring("staging")))
		Expect(filepath.Join(parent, "devex")).NotTo(BeADirectory())
	})

	It("rejects a file where the skill parent directory belongs", func() {
		Expect(os.WriteFile(filepath.Join(home, ".claude"), []byte("not a directory"), 0o644)).To(Succeed())
		_, _, err := runAgentCommand("install-skill", "--agent", "claude", "--scope", "user")
		Expect(err).To(HaveOccurred())
		Expect(os.ReadFile(filepath.Join(home, ".claude"))).To(Equal([]byte("not a directory")))
	})

	It("runs from the production root without initializing plugin state", func() {
		viper.Reset()
		DeferCleanup(viper.Reset)
		root := commands.NewRootCmd("test", nil, config.CrossPlatformSettings{HomeDir: home})
		var stdout bytes.Buffer
		root.SetOut(&stdout)
		root.SetErr(GinkgoWriter)
		root.SetArgs([]string{"agent", "install-skill", "--agent", "claude", "--scope", "user"})
		Expect(root.Execute()).To(Succeed())
		Expect(stdout.String()).To(ContainSubstring("Installed: "))
		Expect(filepath.Join(home, ".devex", "plugins")).NotTo(BeADirectory())
	})

	It("reports the resolved destination in command help", func() {
		stdout, _, err := runAgentCommand("install-skill", "--help")
		Expect(err).NotTo(HaveOccurred())
		for _, text := range []string{"~/.claude/skills/devex", "~/.agents/skills/devex", "KIMI_CODE_HOME", "current working directory", "--overwrite"} {
			Expect(stdout).To(ContainSubstring(text))
		}
		Expect(strings.Contains(stdout, "~/.codex/skills")).To(BeFalse())
	})
})
