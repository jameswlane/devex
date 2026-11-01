package tui

import (
	"context"
	"os"
	"path/filepath"

	"github.com/jameswlane/devex/apps/cli/internal/shellrc"
	"github.com/jameswlane/devex/apps/cli/internal/types"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Installer declarative post-install actions", func() {
	It("executes without launching a shell and writes only setup's selected rc", func() {
		home := GinkgoT().TempDir()
		GinkgoT().Setenv("HOME", home)
		GinkgoT().Setenv("SHELL", "/bin/bash")
		installer := &StreamingInstaller{}
		commands := []types.InstallCommand{{ShellInit: map[string]string{"bash": "source ~/.bash-init", "zsh": "source ~/.zsh-init"}}, {Path: []string{"/opt/nvim/bin"}}}
		ctx := shellrc.WithShell(context.Background(), "zsh")
		Expect(installer.executeCommands(ctx, commands)).To(Succeed())
		path := filepath.Join(home, ".zshrc")
		first, err := os.ReadFile(path)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(first)).To(ContainSubstring("source ~/.zsh-init"))
		Expect(string(first)).To(ContainSubstring("export PATH="))
		Expect(installer.executeCommands(ctx, commands)).To(Succeed())
		second, err := os.ReadFile(path)
		Expect(err).NotTo(HaveOccurred())
		Expect(second).To(Equal(first))
		Expect(filepath.Join(home, ".bashrc")).NotTo(BeAnExistingFile())
		Expect(filepath.Join(home, ".config/fish")).NotTo(BeADirectory())
	})
	It("rejects raw writes before command execution", func() {
		home := GinkgoT().TempDir()
		GinkgoT().Setenv("HOME", home)
		installer := &StreamingInstaller{}
		err := installer.executeCommands(context.Background(), []types.InstallCommand{{Shell: "echo bad >> ~/.bashrc"}})
		Expect(err).To(MatchError(ContainSubstring("raw shell rc appends")))
		Expect(filepath.Join(home, ".bashrc")).NotTo(BeAnExistingFile())
	})
})
