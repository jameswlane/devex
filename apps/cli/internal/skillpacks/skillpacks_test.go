package skillpacks_test

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/jameswlane/devex/apps/cli/internal/skillpacks"
	"github.com/jameswlane/devex/apps/cli/internal/testhelper"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const testSourceURL = "https://github.com/t3dotgg/tstack.git"

func runFixtureGit(dir string, args ...string) string {
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

func copyTree(src, dst string) {
	ExpectWithOffset(1, filepath.WalkDir(src, func(p string, d fs.DirEntry, err error) error {
		Expect(err).NotTo(HaveOccurred())
		rel, err := filepath.Rel(src, p)
		Expect(err).NotTo(HaveOccurred())
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		info, err := d.Info()
		Expect(err).NotTo(HaveOccurred())
		data, err := os.ReadFile(p)
		Expect(err).NotTo(HaveOccurred())
		return os.WriteFile(target, data, info.Mode().Perm())
	})).To(Succeed())
}

// buildTstackFixtureRepo creates a local git repository from the vendored
// testdata/tstack snapshot and returns its directory and initial commit SHA.
func buildTstackFixtureRepo() (string, string) {
	repoDir := GinkgoT().TempDir()
	copyTree(filepath.Join("testdata", "tstack"), repoDir)
	runFixtureGit(repoDir, "init", "-b", "main")
	runFixtureGit(repoDir, "config", "uploadpack.allowAnySHA1InWant", "true")
	runFixtureGit(repoDir, "add", "-A")
	runFixtureGit(repoDir, "commit", "-m", "initial tstack snapshot")
	sha := runFixtureGit(repoDir, "rev-parse", "HEAD")
	return repoDir, sha
}

func tstackCatalogWithSHA(sha string) *skillpacks.Catalog {
	embedded, err := skillpacks.LoadEmbeddedCatalog()
	Expect(err).NotTo(HaveOccurred())
	pack, ok := embedded.Lookup("tstack")
	Expect(ok).To(BeTrue())
	pack.SHA = sha
	cat, err := skillpacks.NewCatalog(pack)
	Expect(err).NotTo(HaveOccurred())
	return cat
}

func fixtureManager(repoDir, sha string, reqChecker skillpacks.RequirementChecker) *skillpacks.Manager {
	return &skillpacks.Manager{
		Catalog: tstackCatalogWithSHA(sha),
		Fetcher: &skillpacks.GitFetcher{
			RewriteSource: func(source string) string {
				if source == testSourceURL {
					return "file://" + repoDir
				}
				return source
			},
		},
		Requirements: reqChecker,
		Now:          func() time.Time { return time.Date(2026, 4, 20, 12, 0, 0, 0, time.UTC) },
	}
}

func allPresentReqs() skillpacks.RequirementChecker {
	return skillpacks.RequirementChecker{
		LookPath: func(name string) (string, error) { return "/usr/bin/" + name, nil },
		RunCheck: func(context.Context, string, ...string) error { return nil },
	}
}

func treeSnapshot(root string) []string {
	var out []string
	if _, err := os.Stat(root); errors.Is(err, os.ErrNotExist) {
		return out
	}
	ExpectWithOffset(1, filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, relErr := filepath.Rel(root, p)
		if relErr != nil {
			return relErr
		}
		if rel == "." {
			return nil
		}
		entry := filepath.ToSlash(rel)
		if d.IsDir() {
			entry += "/"
		}
		out = append(out, entry)
		return nil
	})).To(Succeed())
	sort.Strings(out)
	return out
}

var _ = Describe("Skill packs catalog", func() {
	It("loads the real embedded tstack.yaml with the exact #413 schema and dependency map", func() {
		cat, err := skillpacks.LoadEmbeddedCatalog()
		Expect(err).NotTo(HaveOccurred())
		Expect(cat.Names()).To(ContainElement("tstack"))

		pack, ok := cat.Lookup("tstack")
		Expect(ok).To(BeTrue())
		Expect(pack.Name).To(Equal("tstack"))
		Expect(pack.Source).To(Equal("https://github.com/t3dotgg/tstack.git"))
		Expect(pack.Path).To(Equal("skills"))
		Expect(pack.License).To(Equal("MIT"))
		Expect(pack.Ref).To(Equal("main"))
		Expect(pack.SHA).To(Equal("11e37a74d0a6868c121cd0910ab2b3bd9873b92a"))

		Expect(pack.SkillNames()).To(ConsistOf(
			"babysit-pr",
			"council",
			"file-pr",
			"full-send",
			"full-stop",
			"leaving-pr-comments",
			"modernize",
			"sol-loop",
			"takeover",
		))
		Expect(pack.Skills["full-send"].Depends).To(Equal([]string{"sol-loop", "file-pr", "babysit-pr"}))
		Expect(pack.Skills["full-stop"].Depends).To(Equal([]string{"full-send"}))
		Expect(pack.Skills["takeover"].Depends).To(Equal([]string{"modernize", "file-pr", "babysit-pr", "leaving-pr-comments"}))
		Expect(pack.Skills["file-pr"].Depends).To(Equal([]string{"leaving-pr-comments"}))
		Expect(pack.Skills["babysit-pr"].Depends).To(Equal([]string{"leaving-pr-comments"}))
		Expect(pack.Skills["modernize"].Depends).To(Equal([]string{"leaving-pr-comments"}))
		Expect(pack.Skills["leaving-pr-comments"].Depends).To(BeEmpty())

		Expect(pack.Skills["council"].Notes).NotTo(BeEmpty())
		Expect(pack.Skills["council"].Notes[0]).To(And(ContainSubstring("subagents"), ContainSubstring("delegate_task")))
		Expect(pack.Skills["sol-loop"].Notes).NotTo(BeEmpty())
		Expect(pack.Skills["sol-loop"].Notes[0]).To(And(ContainSubstring("subagents"), ContainSubstring("delegate_task")))

		Expect(pack.Requirements).To(HaveLen(2))
		Expect(pack.Requirements[0].Command).To(Equal("gh"))
		Expect(pack.Requirements[0].Check).To(Equal([]string{"gh", "auth", "status"}))
		Expect(pack.Requirements[1].Command).To(Equal("python3"))
		Expect(pack.Requirements[1].Skills).To(Equal([]string{"babysit-pr"}))

		closure, err := pack.ResolveClosure([]string{"full-send"})
		Expect(err).NotTo(HaveOccurred())
		Expect(closure).To(ConsistOf("full-send", "sol-loop", "file-pr", "babysit-pr", "leaving-pr-comments"))

		_, err = pack.ResolveClosure([]string{"does-not-exist"})
		Expect(err).To(MatchError(And(ContainSubstring("unknown skill"), ContainSubstring("full-send"))))
	})

	DescribeTable("rejects invalid catalog definitions",
		func(pack skillpacks.Pack, substr string) {
			_, err := skillpacks.NewCatalog(pack)
			Expect(err).To(MatchError(ContainSubstring(substr)))
		},
		Entry("non-https source", skillpacks.Pack{
			Name:   "bad",
			Source: "http://example.com/repo.git",
			Path:   "skills",
			SHA:    "11e37a74d0a6868c121cd0910ab2b3bd9873b92a",
			Skills: map[string]skillpacks.SkillEntry{"a": {}},
		}, "https"),
		Entry("non-40-hex SHA", skillpacks.Pack{
			Name:   "bad",
			Source: "https://example.com/repo.git",
			Path:   "skills",
			SHA:    "not-a-sha",
			Skills: map[string]skillpacks.SkillEntry{"a": {}},
		}, "40-character"),
		Entry("unknown dependency", skillpacks.Pack{
			Name:   "bad",
			Source: "https://example.com/repo.git",
			Path:   "skills",
			SHA:    "11e37a74d0a6868c121cd0910ab2b3bd9873b92a",
			Skills: map[string]skillpacks.SkillEntry{"a": {Depends: []string{"missing"}}},
		}, "unknown skill"),
		Entry("dependency cycle", skillpacks.Pack{
			Name:   "bad",
			Source: "https://example.com/repo.git",
			Path:   "skills",
			SHA:    "11e37a74d0a6868c121cd0910ab2b3bd9873b92a",
			Skills: map[string]skillpacks.SkillEntry{
				"a": {Depends: []string{"b"}},
				"b": {Depends: []string{"a"}},
			},
		}, "cycle"),
		Entry("escaping skills path", skillpacks.Pack{
			Name:   "bad",
			Source: "https://example.com/repo.git",
			Path:   "../outside",
			SHA:    "11e37a74d0a6868c121cd0910ab2b3bd9873b92a",
			Skills: map[string]skillpacks.SkillEntry{"a": {}},
		}, "escape"),
	)
})

var _ = Describe("Skill packs installer (local fixture git repo)", func() {
	var (
		home       string
		projectDir string
		repoDir    string
		sha1       string
	)

	BeforeEach(func() {
		home = GinkgoT().TempDir()
		projectDir = GinkgoT().TempDir()
		testhelper.SetProfileEnv(GinkgoT(), home)
		GinkgoT().Setenv("KIMI_CODE_HOME", "")
		GinkgoT().Setenv("DEVEX_STATE_DIR", filepath.Join(home, "state"))
		repoDir, sha1 = buildTstackFixtureRepo()
	})

	DescribeTable("installs tstack across all three agents and both scopes",
		func(agent, scope string) {
			mgr := fixtureManager(repoDir, sha1, allPresentReqs())
			opts := skillpacks.AddOptions{
				PackSpec: "tstack",
				Skills:   []string{"leaving-pr-comments"},
				Agent:    agent,
				Scope:    scope,
			}
			if scope == "project" {
				opts.ProjectDir = projectDir
			}
			res, err := mgr.Add(context.Background(), opts)
			Expect(err).NotTo(HaveOccurred())
			Expect(res.Items).To(HaveLen(1))
			Expect(res.Items[0].Action).To(Equal(skillpacks.ActionInstall))

			root := home
			if scope == "project" {
				root = projectDir
			}
			var dir string
			switch agent {
			case "claude":
				dir = ".claude"
			case "codex":
				dir = ".agents"
			case "kimi":
				dir = ".kimi-code"
			}
			dest := filepath.Join(root, dir, "skills", "leaving-pr-comments")
			Expect(res.Items[0].Destination).To(Equal(dest))
			Expect(filepath.Join(dest, "SKILL.md")).To(BeARegularFile())
			Expect(filepath.Join(dest, "agents", "openai.yaml")).To(BeARegularFile())

			lockPath := filepath.Join(home, "state", skillpacks.LockFileName)
			if scope == "project" {
				lockPath = filepath.Join(projectDir, ".devex", skillpacks.LockFileName)
			}
			lockInfo, err := os.Stat(lockPath)
			Expect(err).NotTo(HaveOccurred())
			if runtime.GOOS != "windows" {
				Expect(lockInfo.Mode().Perm()).To(Equal(os.FileMode(0o600)))
			}
			lock, err := skillpacks.LoadLock(lockPath)
			Expect(err).NotTo(HaveOccurred())
			entry, ok := lock.Skills[skillpacks.DestinationKey(dest)]
			Expect(ok).To(BeTrue())
			Expect(entry.SHA).To(Equal(sha1))
			Expect(entry.Files).NotTo(BeEmpty())
		},
		Entry("Claude user", "claude", "user"),
		Entry("Claude project", "claude", "project"),
		Entry("Codex user", "codex", "user"),
		Entry("Codex project", "codex", "project"),
		Entry("Kimi user", "kimi", "user"),
		Entry("Kimi project", "kimi", "project"),
	)

	It("installs all nine tstack skills for claude and codex, preserving pr-wait exec bit and lock entries per destination", func() {
		mgr := fixtureManager(repoDir, sha1, allPresentReqs())

		for _, agent := range []string{"claude", "codex"} {
			res, err := mgr.Add(context.Background(), skillpacks.AddOptions{
				PackSpec: "tstack",
				Agent:    agent,
				Scope:    "user",
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(res.Items).To(HaveLen(9))
			for _, item := range res.Items {
				Expect(item.Action).To(Equal(skillpacks.ActionInstall))
			}
		}

		// Verify pr-wait executable bit and openai.yaml in both locations.
		for _, base := range []string{
			filepath.Join(home, ".claude", "skills"),
			filepath.Join(home, ".agents", "skills"),
		} {
			prWait := filepath.Join(base, "babysit-pr", "scripts", "pr-wait")
			info, err := os.Stat(prWait)
			Expect(err).NotTo(HaveOccurred())
			if runtime.GOOS != "windows" {
				Expect(info.Mode().Perm()).To(Equal(os.FileMode(0o755)))
			}
			skillInfo, err := os.Stat(filepath.Join(base, "babysit-pr", "SKILL.md"))
			Expect(err).NotTo(HaveOccurred())
			if runtime.GOOS != "windows" {
				Expect(skillInfo.Mode().Perm()).To(Equal(os.FileMode(0o644)))
			}
			Expect(filepath.Join(base, "full-send", "agents", "openai.yaml")).To(BeARegularFile())
		}

		lock, err := skillpacks.LoadLock(filepath.Join(home, "state", skillpacks.LockFileName))
		Expect(err).NotTo(HaveOccurred())
		Expect(lock.Skills).To(HaveLen(18))

		prEntry := lock.Skills[skillpacks.DestinationKey(filepath.Join(home, ".claude", "skills", "babysit-pr"))]
		Expect(prEntry.Files).To(ContainElement(HaveField("Path", "scripts/pr-wait")))
		for _, f := range prEntry.Files {
			if f.Path == "scripts/pr-wait" {
				Expect(f.Mode).To(Equal("0755"))
				Expect(f.SHA256).To(HaveLen(64))
			}
		}
	})

	It("installs only the dependency closure when --skill full-send is passed", func() {
		mgr := fixtureManager(repoDir, sha1, allPresentReqs())
		res, err := mgr.Add(context.Background(), skillpacks.AddOptions{
			PackSpec: "tstack",
			Skills:   []string{"full-send"},
			Agent:    "claude",
			Scope:    "user",
		})
		Expect(err).NotTo(HaveOccurred())
		names := make([]string, 0, len(res.Items))
		for _, item := range res.Items {
			names = append(names, item.Skill)
		}
		Expect(names).To(ConsistOf("full-send", "sol-loop", "file-pr", "babysit-pr", "leaving-pr-comments"))
		Expect(filepath.Join(home, ".claude", "skills", "council")).NotTo(BeADirectory())
		Expect(filepath.Join(home, ".claude", "skills", "full-stop")).NotTo(BeADirectory())
		Expect(filepath.Join(home, ".claude", "skills", "takeover")).NotTo(BeADirectory())
		Expect(filepath.Join(home, ".claude", "skills", "modernize")).NotTo(BeADirectory())
	})

	It("detects matching unmanaged copies in ~/.claude/skills, adopts them with --adopt without rewriting, and leaves differing manual copies alone even with --overwrite", func() {
		mgr := fixtureManager(repoDir, sha1, allPresentReqs())

		// Matching manual copy of file-pr.
		manualMatch := filepath.Join(home, ".claude", "skills", "file-pr")
		copyTree(filepath.Join("testdata", "tstack", "skills", "file-pr"), manualMatch)
		oldTime := time.Now().Add(-24 * time.Hour).Truncate(time.Second)
		skillMD := filepath.Join(manualMatch, "SKILL.md")
		Expect(os.Chtimes(skillMD, oldTime, oldTime)).To(Succeed())

		// Differing manual copy of leaving-pr-comments.
		manualDiff := filepath.Join(home, ".claude", "skills", "leaving-pr-comments")
		copyTree(filepath.Join("testdata", "tstack", "skills", "leaving-pr-comments"), manualDiff)
		Expect(os.WriteFile(filepath.Join(manualDiff, "SKILL.md"), []byte("my custom comments\n"), 0o644)).To(Succeed())

		// First run without --adopt (even with --overwrite!).
		res, err := mgr.Add(context.Background(), skillpacks.AddOptions{
			PackSpec:  "tstack",
			Skills:    []string{"file-pr"},
			Agent:     "claude",
			Scope:     "user",
			Overwrite: true,
		})
		Expect(err).NotTo(HaveOccurred())
		bySkill := map[string]skillpacks.SkillPlanItem{}
		for _, item := range res.Items {
			bySkill[item.Skill] = item
		}
		Expect(bySkill["file-pr"].Action).To(Equal(skillpacks.ActionSkipAdoptable))
		Expect(bySkill["file-pr"].Reason).To(And(ContainSubstring("matches tstack@"+skillpacks.ShortSHA(sha1)), ContainSubstring("--adopt")))
		Expect(bySkill["leaving-pr-comments"].Action).To(Equal(skillpacks.ActionSkipLocalCopy))
		Expect(os.ReadFile(filepath.Join(manualDiff, "SKILL.md"))).To(Equal([]byte("my custom comments\n")))

		// Run with --adopt: file-pr is adopted without rewriting; leaving-pr-comments stays untouched.
		resAdopt, err := mgr.Add(context.Background(), skillpacks.AddOptions{
			PackSpec:  "tstack",
			Skills:    []string{"file-pr"},
			Agent:     "claude",
			Scope:     "user",
			Adopt:     true,
			Overwrite: true,
		})
		Expect(err).NotTo(HaveOccurred())
		bySkill = map[string]skillpacks.SkillPlanItem{}
		for _, item := range resAdopt.Items {
			bySkill[item.Skill] = item
		}
		Expect(bySkill["file-pr"].Action).To(Equal(skillpacks.ActionAdopt))
		Expect(bySkill["leaving-pr-comments"].Action).To(Equal(skillpacks.ActionSkipLocalCopy))

		info, err := os.Stat(skillMD)
		Expect(err).NotTo(HaveOccurred())
		Expect(info.ModTime().Truncate(time.Second)).To(Equal(oldTime))

		lock, err := skillpacks.LoadLock(filepath.Join(home, "state", skillpacks.LockFileName))
		Expect(err).NotTo(HaveOccurred())
		Expect(lock.Skills).To(HaveKey(skillpacks.DestinationKey(manualMatch)))
		Expect(lock.Skills).NotTo(HaveKey(skillpacks.DestinationKey(manualDiff)))
		Expect(os.ReadFile(filepath.Join(manualDiff, "SKILL.md"))).To(Equal([]byte("my custom comments\n")))
	})

	It("reports legacy ~/.codex/skills copies without modifying them while installing Codex skills to ~/.agents/skills", func() {
		mgr := fixtureManager(repoDir, sha1, allPresentReqs())

		legacyMatch := filepath.Join(home, ".codex", "skills", "file-pr")
		copyTree(filepath.Join("testdata", "tstack", "skills", "file-pr"), legacyMatch)

		legacyDiff := filepath.Join(home, ".codex", "skills", "leaving-pr-comments")
		copyTree(filepath.Join("testdata", "tstack", "skills", "leaving-pr-comments"), legacyDiff)
		Expect(os.WriteFile(filepath.Join(legacyDiff, "SKILL.md"), []byte("custom codex copy\n"), 0o644)).To(Succeed())

		res, err := mgr.Add(context.Background(), skillpacks.AddOptions{
			PackSpec: "tstack",
			Skills:   []string{"file-pr"},
			Agent:    "codex",
			Scope:    "user",
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(filepath.Join(home, ".agents", "skills", "file-pr", "SKILL.md")).To(BeARegularFile())
		Expect(filepath.Join(home, ".agents", "skills", "leaving-pr-comments", "SKILL.md")).To(BeARegularFile())

		Expect(res.LegacyCodexCopies).To(HaveLen(2))
		notices := map[string]skillpacks.LegacyCopyNotice{}
		for _, n := range res.LegacyCodexCopies {
			notices[n.Skill] = n
		}
		Expect(notices["file-pr"].MatchesPin).To(BeTrue())
		Expect(notices["file-pr"].Path).To(Equal(legacyMatch))
		Expect(notices["leaving-pr-comments"].MatchesPin).To(BeFalse())
		Expect(os.ReadFile(filepath.Join(legacyDiff, "SKILL.md"))).To(Equal([]byte("custom codex copy\n")))
	})

	It("updates with file-level and scripts/ line-level diffs and protects locally edited skills unless --overwrite is set", func() {
		mgr1 := fixtureManager(repoDir, sha1, allPresentReqs())
		_, err := mgr1.Add(context.Background(), skillpacks.AddOptions{
			PackSpec: "tstack",
			Skills:   []string{"babysit-pr"},
			Agent:    "claude",
			Scope:    "user",
		})
		Expect(err).NotTo(HaveOccurred())

		// Make a second commit modifying babysit-pr/SKILL.md and babysit-pr/scripts/pr-wait.
		skillPath := filepath.Join(repoDir, "skills", "babysit-pr", "SKILL.md")
		origSkill, err := os.ReadFile(skillPath)
		Expect(err).NotTo(HaveOccurred())
		Expect(os.WriteFile(skillPath, append(origSkill, []byte("\n## Updated guidance\n")...), 0o644)).To(Succeed())

		prWaitPath := filepath.Join(repoDir, "skills", "babysit-pr", "scripts", "pr-wait")
		origScript, err := os.ReadFile(prWaitPath)
		Expect(err).NotTo(HaveOccurred())
		updatedScript := strings.Replace(string(origScript), "#!/usr/bin/env python3", "#!/usr/bin/env python3\n# updated in fixture v2", 1)
		Expect(os.WriteFile(prWaitPath, []byte(updatedScript), 0o755)).To(Succeed())

		runFixtureGit(repoDir, "add", "-A")
		runFixtureGit(repoDir, "commit", "-m", "update babysit-pr")
		sha2 := runFixtureGit(repoDir, "rev-parse", "HEAD")

		// Locally edit leaving-pr-comments on disk.
		editedSkillPath := filepath.Join(home, ".claude", "skills", "leaving-pr-comments", "SKILL.md")
		Expect(os.WriteFile(editedSkillPath, []byte("user local edit\n"), 0o644)).To(Succeed())

		mgr2 := fixtureManager(repoDir, sha2, allPresentReqs())
		upd, err := mgr2.Update(context.Background(), skillpacks.UpdateOptions{
			PackSpec: "tstack",
			Agent:    "claude",
			Scope:    "user",
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(upd.Packs).To(HaveLen(1))

		items := map[string]skillpacks.SkillPlanItem{}
		for _, item := range upd.Packs[0].Items {
			items[item.Skill] = item
		}
		Expect(items["babysit-pr"].Action).To(Equal(skillpacks.ActionReplace))
		Expect(items["babysit-pr"].Diff).NotTo(BeNil())
		var sawSkillMD, sawScriptDiff bool
		for _, ch := range items["babysit-pr"].Diff.Changes {
			if ch.Path == "SKILL.md" && ch.Kind == skillpacks.FileModified {
				sawSkillMD = true
			}
			if ch.Path == "scripts/pr-wait" && ch.Kind == skillpacks.FileModified {
				Expect(ch.UnifiedDiff).To(ContainSubstring("+# updated in fixture v2"))
				sawScriptDiff = true
			}
		}
		Expect(sawSkillMD).To(BeTrue())
		Expect(sawScriptDiff).To(BeTrue())

		// Locally edited skill was skipped and preserved.
		Expect(items["leaving-pr-comments"].Action).To(Equal(skillpacks.ActionSkipEdited))
		Expect(os.ReadFile(editedSkillPath)).To(Equal([]byte("user local edit\n")))

		// Re-run with --overwrite: replaces edited skill and retains a recoverable backup.
		updOverwrite, err := mgr2.Update(context.Background(), skillpacks.UpdateOptions{
			PackSpec:  "tstack",
			Agent:     "claude",
			Scope:     "user",
			Overwrite: true,
		})
		Expect(err).NotTo(HaveOccurred())
		items = map[string]skillpacks.SkillPlanItem{}
		for _, item := range updOverwrite.Packs[0].Items {
			items[item.Skill] = item
		}
		Expect(items["leaving-pr-comments"].Action).To(Equal(skillpacks.ActionReplace))
		Expect(items["leaving-pr-comments"].BackupPath).NotTo(BeEmpty())
		Expect(os.ReadFile(filepath.Join(items["leaving-pr-comments"].BackupPath, "SKILL.md"))).To(Equal([]byte("user local edit\n")))
		Expect(os.ReadFile(editedSkillPath)).NotTo(Equal([]byte("user local edit\n")))
	})

	It("guards removal against dependents and local edits, and backs up edited skills on --force", func() {
		mgr := fixtureManager(repoDir, sha1, allPresentReqs())
		_, err := mgr.Add(context.Background(), skillpacks.AddOptions{
			PackSpec: "tstack",
			Skills:   []string{"file-pr"},
			Agent:    "claude",
			Scope:    "user",
		})
		Expect(err).NotTo(HaveOccurred())

		// Removing leaving-pr-comments while file-pr depends on it fails without --force.
		_, err = mgr.Remove(skillpacks.RemoveOptions{
			Pack:   "tstack",
			Skills: []string{"leaving-pr-comments"},
			Agent:  "claude",
			Scope:  "user",
		})
		Expect(err).To(MatchError(And(ContainSubstring("file-pr"), ContainSubstring("--force"))))

		// Locally edit file-pr and try to remove it without --force.
		filePRMD := filepath.Join(home, ".claude", "skills", "file-pr", "SKILL.md")
		Expect(os.WriteFile(filePRMD, []byte("custom file-pr\n"), 0o644)).To(Succeed())

		_, err = mgr.Remove(skillpacks.RemoveOptions{
			Pack:   "tstack",
			Skills: []string{"file-pr"},
			Agent:  "claude",
			Scope:  "user",
		})
		Expect(err).To(MatchError(And(ContainSubstring("locally edited"), ContainSubstring("file-pr"), ContainSubstring("--force"))))

		// With --force, locally edited file-pr is moved to a backup (never destroyed!) and removed from lock.
		rmRes, err := mgr.Remove(skillpacks.RemoveOptions{
			Pack:   "tstack",
			Skills: []string{"file-pr"},
			Agent:  "claude",
			Scope:  "user",
			Force:  true,
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(rmRes.Items).To(HaveLen(1))
		Expect(rmRes.Items[0].Action).To(Equal(skillpacks.ActionBackupAndRemove))
		Expect(rmRes.Items[0].BackupPath).NotTo(BeEmpty())
		Expect(filepath.Join(home, ".claude", "skills", "file-pr")).NotTo(BeADirectory())
		Expect(os.ReadFile(filepath.Join(rmRes.Items[0].BackupPath, "SKILL.md"))).To(Equal([]byte("custom file-pr\n")))

		// Now leaving-pr-comments has no dependents and is unedited, so it removes cleanly.
		rmDep, err := mgr.Remove(skillpacks.RemoveOptions{
			Pack:   "tstack",
			Skills: []string{"leaving-pr-comments"},
			Agent:  "claude",
			Scope:  "user",
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(rmDep.Items[0].Action).To(Equal(skillpacks.ActionRemove))
		Expect(filepath.Join(home, ".claude", "skills", "leaving-pr-comments")).NotTo(BeADirectory())
	})

	It("rejects symlinks in snapshots and symlink escapes at destinations", func() {
		if runtime.GOOS == "windows" {
			Skip("POSIX symlinks require Unix")
		}
		// Add a symlink inside a skill in the fixture repo.
		Expect(os.Symlink("/etc/hosts", filepath.Join(repoDir, "skills", "council", "escape-link"))).To(Succeed())
		runFixtureGit(repoDir, "add", "-A")
		runFixtureGit(repoDir, "commit", "-m", "add symlink")
		badSHA := runFixtureGit(repoDir, "rev-parse", "HEAD")

		mgrBad := fixtureManager(repoDir, badSHA, allPresentReqs())
		_, err := mgrBad.Add(context.Background(), skillpacks.AddOptions{
			PackSpec: "tstack",
			Skills:   []string{"council"},
			Agent:    "claude",
			Scope:    "user",
		})
		Expect(err).To(MatchError(ContainSubstring("symlink")))
		Expect(filepath.Join(home, ".claude", "skills", "council")).NotTo(BeADirectory())

		// Destination parent symlink escape rejected.
		outside := GinkgoT().TempDir()
		Expect(os.Symlink(outside, filepath.Join(home, ".claude"))).To(Succeed())
		mgrGood := fixtureManager(repoDir, sha1, allPresentReqs())
		_, err = mgrGood.Add(context.Background(), skillpacks.AddOptions{
			PackSpec: "tstack",
			Skills:   []string{"council"},
			Agent:    "claude",
			Scope:    "user",
		})
		Expect(err).To(MatchError(ContainSubstring("symlink")))
		entries, readErr := os.ReadDir(outside)
		Expect(readErr).NotTo(HaveOccurred())
		Expect(entries).To(BeEmpty())

		// Direct path traversal rejected in ReadSkillSnapshot.
		_, err = skillpacks.ReadSkillSnapshot(repoDir, "../escape", "council")
		Expect(err).To(HaveOccurred())
		_, err = skillpacks.ReadSkillSnapshot(repoDir, "skills", "../council")
		Expect(err).To(HaveOccurred())
	})

	It("writes nothing during --dry-run and reports subagent notes and scoped missing requirements", func() {
		missingReqs := skillpacks.RequirementChecker{
			LookPath: func(name string) (string, error) {
				if name == "gh" {
					return "/usr/bin/gh", nil
				}
				return "", exec.ErrNotFound
			},
			RunCheck: func(context.Context, string, ...string) error {
				return errors.New("not logged in")
			},
		}
		mgr := fixtureManager(repoDir, sha1, missingReqs)

		beforeHome := treeSnapshot(home)
		beforeProject := treeSnapshot(projectDir)

		res, err := mgr.Add(context.Background(), skillpacks.AddOptions{
			PackSpec:   "tstack",
			Skills:     []string{"full-send", "council"},
			Agent:      "claude",
			Scope:      "project",
			ProjectDir: projectDir,
			DryRun:     true,
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(treeSnapshot(home)).To(Equal(beforeHome))
		Expect(treeSnapshot(projectDir)).To(Equal(beforeProject))

		// Subagent notes for council & sol-loop present.
		Expect(res.Notes).To(HaveKey("council"))
		Expect(res.Notes).To(HaveKey("sol-loop"))
		Expect(res.Notes["council"][0]).To(ContainSubstring("delegate_task"))

		// Both gh (auth check failed) and python3 (babysit-pr is in closure) reported.
		Expect(res.MissingRequirements).To(HaveLen(2))

		// When installing council only, python3 (scoped to babysit-pr) is NOT reported.
		resCouncil, err := mgr.Add(context.Background(), skillpacks.AddOptions{
			PackSpec: "tstack",
			Skills:   []string{"council"},
			Agent:    "claude",
			Scope:    "user",
			DryRun:   true,
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(resCouncil.MissingRequirements).To(HaveLen(1))
		Expect(resCouncil.MissingRequirements[0].Command).To(Equal("gh"))
		Expect(treeSnapshot(home)).To(Equal(beforeHome))
	})

	It("gates raw git+https:// sources behind --allow-unreviewed and pins the resolved SHA", func() {
		rawURL := "https://example.test/owner/custom-pack.git"
		mgr := &skillpacks.Manager{
			Catalog: tstackCatalogWithSHA(sha1),
			Fetcher: &skillpacks.GitFetcher{
				RewriteSource: func(source string) string {
					if source == rawURL {
						return "file://" + repoDir
					}
					return source
				},
			},
			Requirements: allPresentReqs(),
		}

		// Without --allow-unreviewed: rejected immediately.
		_, err := mgr.Add(context.Background(), skillpacks.AddOptions{
			PackSpec: "git+" + rawURL + "@main",
			Skills:   []string{"full-send"},
			Agent:    "claude",
			Scope:    "user",
		})
		Expect(err).To(MatchError(ContainSubstring("--allow-unreviewed")))

		// With --allow-unreviewed: warns loudly, installs ONLY requested skill (no dependency map), pins SHA.
		res, err := mgr.Add(context.Background(), skillpacks.AddOptions{
			PackSpec:        "git+" + rawURL + "@main",
			Skills:          []string{"full-send"},
			SkillsPath:      "skills",
			Agent:           "claude",
			Scope:           "user",
			AllowUnreviewed: true,
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(res.Unreviewed).To(BeTrue())
		Expect(res.SHA).To(Equal(sha1))
		Expect(res.Warnings).To(ContainElement(ContainSubstring("unreviewed source")))
		Expect(res.Items).To(HaveLen(1))
		Expect(res.Items[0].Skill).To(Equal("full-send"))

		lock, err := skillpacks.LoadLock(filepath.Join(home, "state", skillpacks.LockFileName))
		Expect(err).NotTo(HaveOccurred())
		entry := lock.Skills[skillpacks.DestinationKey(filepath.Join(home, ".claude", "skills", "full-send"))]
		Expect(entry.SHA).To(Equal(sha1))
		Expect(entry.Unreviewed).To(BeTrue())
	})

	It("returns an actionable error when git is missing from PATH", func() {
		f := &skillpacks.GitFetcher{
			LookPath: func(string) (string, error) { return "", exec.ErrNotFound },
		}
		_, err := f.ResolveRef(context.Background(), testSourceURL, "main")
		Expect(err).To(MatchError(ContainSubstring("git is required")))
	})

	It("enforces Codex/Kimi shared .agents/skills conflict warnings and zero-write dry-runs on update and remove", func() {
		mgr := fixtureManager(repoDir, sha1, allPresentReqs())

		// Install for Codex first; Kimi skips with warning because Kimi reads .agents/skills.
		_, err := mgr.Add(context.Background(), skillpacks.AddOptions{
			PackSpec: "tstack",
			Skills:   []string{"council"},
			Agent:    "codex",
			Scope:    "user",
		})
		Expect(err).NotTo(HaveOccurred())

		kimiRes, err := mgr.Add(context.Background(), skillpacks.AddOptions{
			PackSpec: "tstack",
			Skills:   []string{"council"},
			Agent:    "kimi",
			Scope:    "user",
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(kimiRes.Items).To(HaveLen(1))
		Expect(kimiRes.Items[0].Action).To(Equal(skillpacks.ActionSkipSharedCodex))
		Expect(kimiRes.Warnings).To(ContainElement(ContainSubstring("no duplicate was created")))
		Expect(filepath.Join(home, ".kimi-code", "skills", "council")).NotTo(BeADirectory())

		// Dry-run update and dry-run remove write nothing.
		beforeHome := treeSnapshot(home)
		_, err = mgr.Update(context.Background(), skillpacks.UpdateOptions{
			PackSpec: "tstack",
			Agent:    "codex",
			Scope:    "user",
			DryRun:   true,
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(treeSnapshot(home)).To(Equal(beforeHome))

		_, err = mgr.Remove(skillpacks.RemoveOptions{
			Pack:   "tstack",
			Skills: []string{"council"},
			Agent:  "codex",
			Scope:  "user",
			DryRun: true,
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(treeSnapshot(home)).To(Equal(beforeHome))
	})
})
