package configfiles_test

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/jameswlane/devex/apps/cli/internal/configfiles"
	"github.com/jameswlane/devex/apps/cli/internal/types"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Recorded copies", func() {
	const app = "revert-test"
	var home, src string

	write := func(path, content string) {
		Expect(os.MkdirAll(filepath.Dir(path), 0o700)).To(Succeed())
		Expect(os.WriteFile(path, []byte(content), 0o600)).To(Succeed())
	}
	read := func(path string) string {
		data, err := os.ReadFile(path)
		Expect(err).NotTo(HaveOccurred())
		return string(data)
	}
	destinations := func() []string {
		result, err := configfiles.ReadResult(app)
		Expect(err).NotTo(HaveOccurred())
		out := make([]string, 0, len(result.Files))
		for _, file := range result.Files {
			out = append(out, file.Destination)
		}
		return out
	}
	recordPath := func() string {
		return filepath.Join(home, ".local", "state", "devex", "config-files", fmt.Sprintf("%x.json", sha256.Sum256([]byte(app))))
	}
	editRecord := func(edit func(*configfiles.Result)) {
		data, err := os.ReadFile(recordPath())
		Expect(err).NotTo(HaveOccurred())
		var result configfiles.Result
		Expect(json.Unmarshal(data, &result)).To(Succeed())
		edit(&result)
		data, err = json.Marshal(result)
		Expect(err).NotTo(HaveOccurred())
		Expect(os.WriteFile(recordPath(), data, 0o600)).To(Succeed())
	}
	backupOf := func(destination string) string {
		result, err := configfiles.ReadResult(app)
		Expect(err).NotTo(HaveOccurred())
		for _, file := range result.Files {
			if file.Destination == destination {
				return file.Backup
			}
		}
		Fail("no record for " + destination)
		return ""
	}
	skipAsRoot := func() {
		if os.Geteuid() == 0 {
			Skip("permission checks do not apply to root")
		}
	}
	// lock makes dir read-only (or inaccessible) until the spec ends.
	lock := func(dir string, mode os.FileMode) {
		Expect(os.Chmod(dir, mode)).To(Succeed())
		DeferCleanup(os.Chmod, dir, os.FileMode(0o700))
	}

	BeforeEach(func() {
		home = GinkgoT().TempDir()
		GinkgoT().Setenv("HOME", home)
		src = GinkgoT().TempDir()
		write(filepath.Join(src, "a"), "managed a\n")
		write(filepath.Join(src, "b"), "managed b\n")
	})

	Describe("a destination that already matches the source", func() {
		It("is not taken over, so a revert keeps the user's file", func(ctx SpecContext) {
			same, created := filepath.Join(home, ".same"), filepath.Join(home, "new", "b")
			write(same, "managed a\n")
			result := configfiles.Apply(ctx, app, []types.ConfigFile{
				{Source: filepath.Join(src, "a"), Destination: same},
				{Source: filepath.Join(src, "b"), Destination: created},
			}, false, nil)
			Expect(result.Warnings).To(BeEmpty())
			Expect(destinations()).To(ConsistOf(created))

			Expect(configfiles.Revert(app, false, nil)).To(Succeed())
			Expect(read(same)).To(Equal("managed a\n"))
			Expect(created).NotTo(BeAnExistingFile())
		})

		It("keeps an earlier ownership record of the destination", func(ctx SpecContext) {
			destination := filepath.Join(home, ".rc")
			write(destination, "original\n")
			mapping := []types.ConfigFile{{Source: filepath.Join(src, "a"), Destination: destination}}
			Expect(configfiles.Apply(ctx, app, mapping, false, nil).Warnings).To(BeEmpty())
			backup := backupOf(destination)
			Expect(configfiles.Apply(ctx, app, mapping, false, nil).Warnings).To(BeEmpty())
			Expect(backupOf(destination)).To(Equal(backup))

			Expect(configfiles.Revert(app, false, nil)).To(Succeed())
			Expect(read(destination)).To(Equal("original\n"))
		})
	})

	Describe("destinations reached through symlinked parent directories", func() {
		It("refuses a destination that resolves into the managed-file records", func(ctx SpecContext) {
			records := filepath.Join(home, ".local", "state", "devex", "config-files")
			Expect(os.MkdirAll(records, 0o700)).To(Succeed())
			Expect(os.Symlink(filepath.Join(".local", "state", "devex", "config-files"), filepath.Join(home, "alias"))).To(Succeed())
			name := fmt.Sprintf("%x.json", sha256.Sum256([]byte(app)))
			result := configfiles.Apply(ctx, app, []types.ConfigFile{{Source: filepath.Join(src, "a"), Destination: filepath.Join(home, "alias", name)}}, false, nil)
			Expect(result.Warnings).To(ConsistOf(ContainSubstring("DevEx managed-file records")))
			data, err := os.ReadFile(filepath.Join(records, name))
			Expect(err).NotTo(HaveOccurred())
			Expect(string(data)).NotTo(ContainSubstring("managed a"))
		})

		It("refuses a destination that resolves into a protected directory", func(ctx SpecContext) {
			protected := filepath.Join(home, ".config", "devex")
			write(filepath.Join(protected, "config.yaml"), "mine\n")
			Expect(os.MkdirAll(filepath.Join(home, "deep"), 0o700)).To(Succeed())
			Expect(os.Symlink(filepath.Join("..", ".config"), filepath.Join(home, "deep", "cfg"))).To(Succeed())
			result := configfiles.ApplyWithOptions(ctx, app, []types.ConfigFile{{Source: filepath.Join(src, "a"), Destination: filepath.Join(home, "deep", "cfg", "devex", "config.yaml")}}, false, nil, configfiles.Options{Protected: []string{protected}})
			Expect(result.Warnings).To(ConsistOf(ContainSubstring("inside DevEx's own directory")))
			Expect(read(filepath.Join(protected, "config.yaml"))).To(Equal("mine\n"))
			matches, err := filepath.Glob(filepath.Join(protected, "*.devex-bak"))
			Expect(err).NotTo(HaveOccurred())
			Expect(matches).To(BeEmpty())
		})

		It("still deploys through a symlinked parent that stays outside protected directories", func(ctx SpecContext) {
			Expect(os.MkdirAll(filepath.Join(home, "dotfiles", "config"), 0o700)).To(Succeed())
			Expect(os.Symlink(filepath.Join("dotfiles", "config"), filepath.Join(home, ".config"))).To(Succeed())
			destination := filepath.Join(home, ".config", "tool", "a")
			result := configfiles.ApplyWithOptions(ctx, app, []types.ConfigFile{{Source: filepath.Join(src, "a"), Destination: destination}}, false, nil, configfiles.Options{Protected: []string{filepath.Join(home, ".config", "devex")}})
			Expect(result.Warnings).To(BeEmpty())
			Expect(read(filepath.Join(home, "dotfiles", "config", "tool", "a"))).To(Equal("managed a\n"))
		})
	})

	Describe("reverting", func() {
		var nested, rc string

		BeforeEach(func(ctx SpecContext) {
			nested, rc = filepath.Join(home, "dir", "b"), filepath.Join(home, ".rc")
			write(rc, "original\n")
			result := configfiles.Apply(ctx, app, []types.ConfigFile{
				{Source: filepath.Join(src, "a"), Destination: rc},
				{Source: filepath.Join(src, "b"), Destination: nested},
			}, false, nil)
			Expect(result.Warnings).To(BeEmpty())
		})

		It("keeps the record of a copy it could not remove, and a retry completes", func() {
			skipAsRoot()
			lock(filepath.Join(home, "dir"), 0o500)
			Expect(configfiles.Revert(app, false, nil)).To(MatchError(ContainSubstring(nested)))
			Expect(read(rc)).To(Equal("original\n"))
			Expect(destinations()).To(ConsistOf(nested))

			Expect(os.Chmod(filepath.Join(home, "dir"), 0o700)).To(Succeed())
			Expect(configfiles.Revert(app, false, nil)).To(Succeed())
			Expect(nested).NotTo(BeAnExistingFile())
			Expect(recordPath()).NotTo(BeAnExistingFile())
		})

		It("keeps the deployed copy and its record when the backup is missing", func() {
			backup := backupOf(rc)
			Expect(os.Remove(backup)).To(Succeed())
			Expect(configfiles.Revert(app, true, nil)).To(MatchError(ContainSubstring(backup)))
			Expect(configfiles.Revert(app, false, nil)).To(MatchError(ContainSubstring(backup)))
			Expect(read(rc)).To(Equal("managed a\n"))
			Expect(destinations()).To(ConsistOf(rc))
			Expect(nested).NotTo(BeAnExistingFile())
		})

		It("keeps the deployed copy and its record when the backup is not a regular file", func() {
			backup := backupOf(rc)
			Expect(os.Remove(backup)).To(Succeed())
			Expect(os.Mkdir(backup, 0o700)).To(Succeed())
			Expect(configfiles.Revert(app, false, nil)).To(MatchError(ContainSubstring(backup)))
			Expect(read(rc)).To(Equal("managed a\n"))
			Expect(destinations()).To(ConsistOf(rc))
		})

		It("keeps the deployed copy and its record when the backup cannot be accessed", func() {
			skipAsRoot()
			locked := filepath.Join(home, "locked")
			moved := filepath.Join(locked, "rc.devex-bak")
			Expect(os.Mkdir(locked, 0o700)).To(Succeed())
			Expect(os.Rename(backupOf(rc), moved)).To(Succeed())
			editRecord(func(result *configfiles.Result) {
				for i := range result.Files {
					if result.Files[i].Destination == rc {
						result.Files[i].Backup = moved
					}
				}
			})
			lock(locked, 0o000)
			Expect(configfiles.Revert(app, false, nil)).To(MatchError(ContainSubstring(moved)))
			Expect(read(rc)).To(Equal("managed a\n"))
			Expect(destinations()).To(ConsistOf(rc))
		})

		It("keeps the deployed copy and its record when restoring the backup fails", func() {
			skipAsRoot()
			backup := backupOf(rc)
			lock(home, 0o500)
			Expect(configfiles.Revert(app, false, nil)).To(MatchError(ContainSubstring("restore " + rc)))
			Expect(read(rc)).To(Equal("managed a\n"))
			Expect(read(backup)).To(Equal("original\n"))

			Expect(os.Chmod(home, 0o700)).To(Succeed())
			Expect(destinations()).To(ContainElement(rc))
			Expect(configfiles.Revert(app, false, nil)).To(Succeed())
			Expect(read(rc)).To(Equal("original\n"))
		})

		It("forgets a copy when neither it nor its backup exists", func() {
			Expect(os.Remove(backupOf(rc))).To(Succeed())
			Expect(os.Remove(rc)).To(Succeed())
			var plan []string
			Expect(configfiles.Revert(app, true, func(line string) { plan = append(plan, line) })).To(Succeed())
			Expect(plan).To(ContainElement(ContainSubstring("Forget " + rc)))
			Expect(configfiles.Revert(app, false, nil)).To(Succeed())
			Expect(recordPath()).NotTo(BeAnExistingFile())
		})
	})
})
