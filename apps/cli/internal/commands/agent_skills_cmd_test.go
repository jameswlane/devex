package commands_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/jameswlane/devex/apps/cli/internal/commands"
	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/skillpacks"
	"github.com/jameswlane/devex/apps/cli/internal/testhelper"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/viper"
)

func runFixtureGitCmd(dir string, args ...string) string {
	cmd := exec.CommandContext(context.Background(), "git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=DevEx Test",
		"GIT_AUTHOR_EMAIL=test@devex.sh",
		"GIT_COMMITTER_NAME=DevEx Test",
		"GIT_COMMITTER_EMAIL=test@devex.sh",
		"GIT_CONFIG_NOSYSTEM=1",
		"GIT_CONFIG_GLOBAL="+os.DevNull,
	)
	out, err := cmd.CombinedOutput()
	ExpectWithOffset(1, err).NotTo(HaveOccurred(), "git %v failed: %s", args, string(out))
	return strings.TrimSpace(string(out))
}

func buildCommandFixtureRepo() (string, string) {
	fixtureSrc := filepath.Join("..", "skillpacks", "testdata", "tstack")
	repoDir := GinkgoT().TempDir()
	Expect(filepath.WalkDir(fixtureSrc, func(p string, d fs.DirEntry, err error) error {
		Expect(err).NotTo(HaveOccurred())
		rel, err := filepath.Rel(fixtureSrc, p)
		Expect(err).NotTo(HaveOccurred())
		target := filepath.Join(repoDir, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		info, err := d.Info()
		Expect(err).NotTo(HaveOccurred())
		data, err := os.ReadFile(p)
		Expect(err).NotTo(HaveOccurred())
		return os.WriteFile(target, data, info.Mode().Perm())
	})).To(Succeed())
	runFixtureGitCmd(repoDir, "init", "-b", "main")
	runFixtureGitCmd(repoDir, "config", "uploadpack.allowAnySHA1InWant", "true")
	runFixtureGitCmd(repoDir, "add", "-A")
	runFixtureGitCmd(repoDir, "commit", "-m", "initial snapshot")
	return repoDir, runFixtureGitCmd(repoDir, "rev-parse", "HEAD")
}

func newFixtureSkillPackManager(repoDir, sha string, reqs skillpacks.RequirementChecker) *skillpacks.Manager {
	embedded, err := skillpacks.LoadEmbeddedCatalog()
	Expect(err).NotTo(HaveOccurred())
	pack, ok := embedded.Lookup("tstack")
	Expect(ok).To(BeTrue())
	pack.SHA = sha
	cat, err := skillpacks.NewCatalog(pack)
	Expect(err).NotTo(HaveOccurred())
	return &skillpacks.Manager{
		Catalog: cat,
		Fetcher: &skillpacks.GitFetcher{
			RewriteSource: func(source string) string {
				if source == pack.Source || source == "https://example.test/raw/pack.git" {
					return "file://" + repoDir
				}
				return source
			},
		},
		Requirements: reqs,
		Now:          func() time.Time { return time.Date(2026, 4, 20, 12, 0, 0, 0, time.UTC) },
	}
}

func runAgentSkillsCmd(mgr *skillpacks.Manager, args ...string) (string, string, error) {
	cmd := commands.NewAgentCmdFromDeps(commands.AgentCmdDeps{SkillPackManager: mgr})
	var stdout, stderr bytes.Buffer
	cmd.SetOut(&stdout)
	cmd.SetErr(&stderr)
	cmd.SetArgs(append([]string{"skills"}, args...))
	err := cmd.Execute()
	return stdout.String(), stderr.String(), err
}

var _ = Describe("Agent skills CLI commands", func() {
	var (
		home    string
		repoDir string
		sha1    string
	)

	BeforeEach(func() {
		home = GinkgoT().TempDir()
		testhelper.SetProfileEnv(GinkgoT(), home)
		GinkgoT().Setenv("KIMI_CODE_HOME", "")
		GinkgoT().Setenv("DEVEX_STATE_DIR", filepath.Join(home, "state"))
		repoDir, sha1 = buildCommandFixtureRepo()
	})

	It("prints dry-run plan with council/sol-loop notes and missing requirements without writing", func() {
		mgr := newFixtureSkillPackManager(repoDir, sha1, skillpacks.RequirementChecker{
			LookPath: func(string) (string, error) { return "", exec.ErrNotFound },
			RunCheck: func(context.Context, string, ...string) error { return errors.New("missing") },
		})

		stdout, _, err := runAgentSkillsCmd(mgr, "add", "tstack", "--agent", "claude", "--scope", "user", "--dry-run")
		Expect(err).NotTo(HaveOccurred())
		Expect(stdout).To(And(
			ContainSubstring("Dry run"),
			ContainSubstring("Would install"),
			ContainSubstring("delegate_task"),
			ContainSubstring("Missing requirements"),
			ContainSubstring("gh"),
			ContainSubstring("python3"),
		))
		Expect(filepath.Join(home, ".claude")).NotTo(BeADirectory())
		Expect(filepath.Join(home, "state", skillpacks.LockFileName)).NotTo(BeAnExistingFile())
	})

	It("supports add, list (table and json), update with script diff, and remove", func() {
		allOK := skillpacks.RequirementChecker{
			LookPath: func(name string) (string, error) { return "/usr/bin/" + name, nil },
			RunCheck: func(context.Context, string, ...string) error { return nil },
		}
		mgr1 := newFixtureSkillPackManager(repoDir, sha1, allOK)

		stdout, _, err := runAgentSkillsCmd(mgr1, "add", "tstack", "--skill", "babysit-pr", "--agent", "claude", "--scope", "user")
		Expect(err).NotTo(HaveOccurred())
		Expect(stdout).To(And(ContainSubstring("Installed"), ContainSubstring("babysit-pr"), ContainSubstring("leaving-pr-comments")))

		// List table.
		tableOut, _, err := runAgentSkillsCmd(mgr1, "list", "--agent", "claude", "--scope", "user")
		Expect(err).NotTo(HaveOccurred())
		Expect(tableOut).To(And(ContainSubstring("PACK"), ContainSubstring("tstack"), ContainSubstring("babysit-pr"), ContainSubstring(skillpacks.ShortSHA(sha1))))

		// Edit leaving-pr-comments locally so list reports edited=yes.
		editedFile := filepath.Join(home, ".claude", "skills", "leaving-pr-comments", "SKILL.md")
		Expect(os.WriteFile(editedFile, []byte("custom edit\n"), 0o644)).To(Succeed())

		jsonOut, _, err := runAgentSkillsCmd(mgr1, "list", "--format", "json", "--scope", "user")
		Expect(err).NotTo(HaveOccurred())
		var entries []skillpacks.ListEntry
		Expect(json.Unmarshal([]byte(jsonOut), &entries)).To(Succeed())
		Expect(entries).To(HaveLen(2))
		editedBySkill := map[string]bool{}
		for _, e := range entries {
			editedBySkill[e.Skill] = e.Edited
		}
		Expect(editedBySkill["babysit-pr"]).To(BeFalse())
		Expect(editedBySkill["leaving-pr-comments"]).To(BeTrue())

		// Commit a script update in the fixture repo and run update.
		prWaitPath := filepath.Join(repoDir, "skills", "babysit-pr", "scripts", "pr-wait")
		origScript, err := os.ReadFile(prWaitPath)
		Expect(err).NotTo(HaveOccurred())
		Expect(os.WriteFile(prWaitPath, append(origScript, []byte("\n# cli update diff test\n")...), 0o755)).To(Succeed())
		runFixtureGitCmd(repoDir, "add", "-A")
		runFixtureGitCmd(repoDir, "commit", "-m", "update pr-wait")
		sha2 := runFixtureGitCmd(repoDir, "rev-parse", "HEAD")

		mgr2 := newFixtureSkillPackManager(repoDir, sha2, allOK)
		updOut, _, err := runAgentSkillsCmd(mgr2, "update", "tstack", "--agent", "claude", "--scope", "user")
		Expect(err).NotTo(HaveOccurred())
		Expect(updOut).To(And(
			ContainSubstring("M scripts/pr-wait"),
			ContainSubstring("+# cli update diff test"),
			ContainSubstring("Skipped (edited)"),
		))

		// Remove pack with --force backs up edited skill and cleans lock.
		rmOut, _, err := runAgentSkillsCmd(mgr2, "remove", "tstack", "--agent", "claude", "--scope", "user", "--force")
		Expect(err).NotTo(HaveOccurred())
		Expect(rmOut).To(And(ContainSubstring("Backed up+removed"), ContainSubstring("Backup: ")))

		emptyOut, _, err := runAgentSkillsCmd(mgr2, "list", "--scope", "user")
		Expect(err).NotTo(HaveOccurred())
		Expect(emptyOut).To(ContainSubstring("No agent skill packs installed."))
	})

	It("warns loudly on stderr when installing from a raw source with --allow-unreviewed", func() {
		allOK := skillpacks.RequirementChecker{
			LookPath: func(name string) (string, error) { return "/usr/bin/" + name, nil },
			RunCheck: func(context.Context, string, ...string) error { return nil },
		}
		mgr := newFixtureSkillPackManager(repoDir, sha1, allOK)

		_, _, err := runAgentSkillsCmd(mgr, "add", "git+https://example.test/raw/pack.git@main", "--skill", "council", "--agent", "claude", "--scope", "user")
		Expect(err).To(MatchError(ContainSubstring("--allow-unreviewed")))

		stdout, stderr, err := runAgentSkillsCmd(mgr, "add", "git+https://example.test/raw/pack.git@main", "--skill", "council", "--agent", "claude", "--scope", "user", "--allow-unreviewed")
		Expect(err).NotTo(HaveOccurred())
		Expect(stderr).To(ContainSubstring("unreviewed source"))
		Expect(stdout).To(ContainSubstring("[unreviewed]"))
	})

	It("runs list from the production root without initializing plugin state", func() {
		viper.Reset()
		DeferCleanup(viper.Reset)
		root := commands.NewRootCmd("test", nil, config.CrossPlatformSettings{HomeDir: home})
		var stdout bytes.Buffer
		root.SetOut(&stdout)
		root.SetErr(GinkgoWriter)
		root.SetArgs([]string{"agent", "skills", "list", "--scope", "user"})
		Expect(root.Execute()).To(Succeed())
		Expect(stdout.String()).To(ContainSubstring("No agent skill packs installed."))
		Expect(filepath.Join(home, ".devex", "plugins")).NotTo(BeADirectory())
	})
})
