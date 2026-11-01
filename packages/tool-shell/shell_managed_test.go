package main_test

import (
	"os"
	"path/filepath"
	"strings"

	main "github.com/jameswlane/devex/packages/tool-shell"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Managed shell setup", func() {
	It("migrates the old block without overriding user settings or app hooks", func() {
		home := GinkgoT().TempDir()
		GinkgoT().Setenv("HOME", home)
		GinkgoT().Setenv("SHELL", "/bin/bash")
		stdin, err := os.Open(os.DevNull)
		Expect(err).NotTo(HaveOccurred())
		oldStdin := os.Stdin
		os.Stdin = stdin
		DeferCleanup(func() { os.Stdin = oldStdin; Expect(stdin.Close()).To(Succeed()) })
		plugin := main.NewShellPlugin()
		path := filepath.Join(home, ".bashrc")
		original := "export HISTSIZE=42\nHISTCONTROL=ignoredups\nalias ll='custom-list'\n# DevEx Shell Configuration\n" + strings.Join(plugin.GetShellConfigs("bash"), "\n") + "\n# user init\neval \"$(zoxide init bash)\"\n"
		Expect(os.WriteFile(path, []byte(original), 0600)).To(Succeed())
		Expect(plugin.Execute("setup", nil)).To(Succeed())
		first, err := os.ReadFile(path)
		Expect(err).NotTo(HaveOccurred())
		text := string(first)
		Expect(strings.Count(text, "# >>> devex >>>")).To(Equal(1))
		Expect(text).To(ContainSubstring("# <<< devex <<<"))
		Expect(text).NotTo(ContainSubstring("HISTSIZE=10000"))
		Expect(text).NotTo(ContainSubstring("HISTCONTROL=ignoreboth"))
		Expect(text).NotTo(ContainSubstring("alias ll='ls -la'"))
		Expect(text).To(ContainSubstring("# user init\neval"))
		info, err := os.Stat(path)
		Expect(err).NotTo(HaveOccurred())
		Expect(info.Mode().Perm()).To(Equal(os.FileMode(0600)))
		Expect(plugin.Execute("setup", nil)).To(Succeed())
		second, err := os.ReadFile(path)
		Expect(err).NotTo(HaveOccurred())
		Expect(second).To(Equal(first))
		backups, err := filepath.Glob(path + ".devex-bak-*")
		Expect(err).NotTo(HaveOccurred())
		Expect(backups).To(HaveLen(1))
		backup, err := os.ReadFile(backups[0])
		Expect(err).NotTo(HaveOccurred())
		Expect(string(backup)).To(Equal(original))
		Expect(filepath.Join(home, ".zshrc")).NotTo(BeAnExistingFile())
	})
})
