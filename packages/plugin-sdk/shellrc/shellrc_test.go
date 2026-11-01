package shellrc_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	rc "github.com/jameswlane/devex/packages/plugin-sdk/shellrc"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestShellRC(t *testing.T) { RegisterFailHandler(Fail); RunSpecs(t, "Managed shell rc") }

var _ = Describe("Managed shell rc", func() {
	var path string
	BeforeEach(func() { path = filepath.Join(GinkgoT().TempDir(), ".bashrc") })
	read := func() string { data, err := os.ReadFile(path); Expect(err).NotTo(HaveOccurred()); return string(data) }
	It("is byte identical on repeat, replaces entries, preserves mode and backs up only once", func() {
		original := "# user content\nexport EDITOR=vim\n"
		Expect(os.WriteFile(path, []byte(original), 0600)).To(Succeed())
		info, err := os.Stat(path)
		Expect(err).NotTo(HaveOccurred())
		changed, err := rc.Apply(path, []string{"export CLICOLOR=1"})
		Expect(err).NotTo(HaveOccurred())
		Expect(changed).To(BeTrue())
		first := read()
		changed, err = rc.Apply(path, []string{"export CLICOLOR=1"})
		Expect(err).NotTo(HaveOccurred())
		Expect(changed).To(BeFalse())
		Expect(read()).To(Equal(first))
		Expect(strings.Count(first, rc.Start)).To(Equal(1))
		after, err := os.Stat(path)
		Expect(err).NotTo(HaveOccurred())
		Expect(after.Mode().Perm()).To(Equal(os.FileMode(0600)))
		Expect(os.SameFile(info, after)).To(BeFalse(), "atomic writes replace the inode")
		_, err = rc.Apply(path, []string{"export CLICOLOR=2"})
		Expect(err).NotTo(HaveOccurred())
		Expect(read()).NotTo(ContainSubstring("CLICOLOR=1"))
		backups, err := filepath.Glob(path + ".devex-bak-*")
		Expect(err).NotTo(HaveOccurred())
		Expect(backups).To(HaveLen(1))
		data, err := os.ReadFile(backups[0])
		Expect(err).NotTo(HaveOccurred())
		Expect(string(data)).To(Equal(original))
		changed, err = rc.Remove(path)
		Expect(err).NotTo(HaveOccurred())
		Expect(changed).To(BeTrue())
		Expect(read()).To(Equal(original))
	})
	It("skips equivalent whitespace and home syntax and preserves user definitions", func() {
		original := "  source   ~/tools/init.sh\nHISTCONTROL=ignoredups\nexport HISTSIZE=42\nalias ll='my-list'\n"
		Expect(os.WriteFile(path, []byte(original), 0640)).To(Succeed())
		changed, err := rc.Apply(path, []string{"source $HOME/tools/init.sh", "export HISTCONTROL=ignoreboth", "export HISTSIZE=10000", "alias ll='ls -la'"})
		Expect(err).NotTo(HaveOccurred())
		Expect(changed).To(BeFalse())
		Expect(read()).To(Equal(original))
	})
	It("merges components and collapses duplicate managed blocks", func() {
		block := rc.Start + "\n" + rc.Comment + "\nexport ONE=1\n" + rc.End + "\n"
		Expect(os.WriteFile(path, []byte(block+block), 0600)).To(Succeed())
		_, err := rc.Merge(path, []string{"export TWO=2"})
		Expect(err).NotTo(HaveOccurred())
		Expect(strings.Count(read(), rc.Start)).To(Equal(1))
		Expect(strings.Count(read(), "export ONE=1")).To(Equal(1))
		Expect(read()).To(ContainSubstring("export TWO=2"))
	})
	It("migrates only known legacy lines while keeping later user content", func() {
		known := []string{"# History", "export HISTSIZE=10000", "alias ll='ls -la'"}
		original := "HISTSIZE=50\n# DevEx Shell Configuration\n" + strings.Join(known, "\n") + "\n# my own settings\nexport KEEP=yes\n"
		Expect(os.WriteFile(path, []byte(original), 0600)).To(Succeed())
		_, err := rc.MergeLegacy(path, known, known)
		Expect(err).NotTo(HaveOccurred())
		Expect(read()).NotTo(ContainSubstring("# DevEx Shell Configuration"))
		Expect(read()).NotTo(ContainSubstring("HISTSIZE=10000"))
		Expect(read()).To(ContainSubstring("# my own settings\nexport KEEP=yes"))
		first := read()
		changed, err := rc.MergeLegacy(path, known, known)
		Expect(err).NotTo(HaveOccurred())
		Expect(changed).To(BeFalse())
		Expect(read()).To(Equal(first))
	})
	It("refuses malformed blocks without modifying the file", func() {
		original := rc.Start + "\nexport KEEP=1\n"
		Expect(os.WriteFile(path, []byte(original), 0600)).To(Succeed())
		_, err := rc.Apply(path, []string{"export NEW=1"})
		Expect(err).To(HaveOccurred())
		Expect(read()).To(Equal(original))
	})
	It("keeps symlinks to dotfiles", func() {
		target := filepath.Join(filepath.Dir(path), "actual")
		Expect(os.WriteFile(target, []byte("# original\n"), 0600)).To(Succeed())
		Expect(os.Symlink(target, path)).To(Succeed())
		_, err := rc.Apply(path, []string{"export NEW=1"})
		Expect(err).NotTo(HaveOccurred())
		info, err := os.Lstat(path)
		Expect(err).NotTo(HaveOccurred())
		Expect(info.Mode() & os.ModeSymlink).NotTo(BeZero())
		Expect(read()).To(ContainSubstring(rc.Start))
	})
	It("guards PATH even when sourced twice or inherited by a nested shell", func() {
		dir := "/opt/devex test/bin"
		entry, err := rc.PathEntry("bash", dir)
		Expect(err).NotTo(HaveOccurred())
		Expect(entry).To(ContainSubstring(`case ":$PATH:" in`))
		_, err = rc.Apply(path, []string{entry, entry})
		Expect(err).NotTo(HaveOccurred())
		Expect(strings.Count(read(), "export PATH=")).To(Equal(1))
		cmd := exec.CommandContext(context.Background(), "bash", "--noprofile", "--norc", "-c", `. "$1"; . "$1"; bash --noprofile --norc -c '. "$1"; printf "%s" "$PATH"' _ "$1"`, "_", path)
		cmd.Env = append(os.Environ(), "PATH=/usr/bin:/bin")
		output, err := cmd.CombinedOutput()
		Expect(err).NotTo(HaveOccurred(), string(output))
		Expect(strings.Count(string(output), dir)).To(Equal(1))
	})
	It("recognizes equivalent existing PATH entries", func() {
		Expect(rc.HasPath("export PATH=\"$PATH:$HOME/.local/bin\"\n", "~/.local/bin")).To(BeTrue())
		Expect(rc.HasPath("export PATH=\"$PATH:/opt/nvim-other/bin\"\n", "/opt/nvim/bin")).To(BeFalse())
		Expect(rc.HasPath("export PATH=\"$PATH:/opt/with spaces/bin\"\n", "/opt/with spaces/bin")).To(BeTrue())
		Expect(os.WriteFile(path, []byte("export PATH=\"$PATH:$HOME/.local/bin\"\n"), 0600)).To(Succeed())
		entry, err := rc.PathEntry("bash", "~/.local/bin")
		Expect(err).NotTo(HaveOccurred())
		changed, err := rc.Apply(path, []string{entry})
		Expect(err).NotTo(HaveOccurred())
		Expect(changed).To(BeFalse())
	})
})
