package utils_test

import (
	"context"
	"os"
	"path/filepath"

	"github.com/jameswlane/devex/apps/cli/internal/types"
	"github.com/jameswlane/devex/apps/cli/internal/utils"
	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Install hook scope guard", func() {
	var home string

	BeforeEach(func() {
		originalExec := utils.CommandExec
		originalResolver := sdk.NewPrivilegeResolver
		DeferCleanup(func() {
			utils.CommandExec = originalExec
			sdk.NewPrivilegeResolver = originalResolver
		})
		utils.CommandExec = utils.OSCommandExecutor{}
		home = GinkgoT().TempDir()
		GinkgoT().Setenv("HOME", home)
	})

	run := func(script string) (string, error) {
		return utils.ExecInstallHookWithContext(context.Background(), types.InstallCommand{Shell: script},
			utils.InstallHookPolicy{AppName: "demo"})
	}

	It("rejects and removes an rc file a hook creates through a computed path", func() {
		_, err := run(`name=zsh; echo 'eval "$(demo init)"' >> "$HOME/.${name}rc"`)
		var scopeErr *utils.HookScopeError
		Expect(err).To(BeAssignableToTypeOf(scopeErr))
		Expect(err.Error()).To(And(ContainSubstring(`app "demo"`), ContainSubstring("~/.zshrc"), ContainSubstring("restored")))
		Expect(filepath.Join(home, ".zshrc")).NotTo(BeAnExistingFile())
	})

	It("restores an existing file an external script edits", func() {
		rc := filepath.Join(home, ".bashrc")
		Expect(os.WriteFile(rc, []byte("# mine\n"), 0o640)).To(Succeed())
		script := filepath.Join(home, "installer.sh")
		Expect(os.WriteFile(script, []byte("#!/bin/sh\necho 'export X=1' >> \"$1\"\n"), 0o700)).To(Succeed())

		_, err := run(`sh "$HOME/installer.sh" "$(printf '%s/.%src' "$HOME" bash)"`)
		Expect(err).To(MatchError(ContainSubstring("~/.bashrc")))
		Expect(os.ReadFile(rc)).To(Equal([]byte("# mine\n")))
		info, statErr := os.Stat(rc)
		Expect(statErr).NotTo(HaveOccurred())
		Expect(info.Mode().Perm()).To(Equal(os.FileMode(0o640)))
	})

	It("restores content written through a symlinked rc file", func() {
		target := filepath.Join(home, "dotfiles-zshrc")
		Expect(os.WriteFile(target, []byte("# dotfiles\n"), 0o644)).To(Succeed())
		Expect(os.Symlink(target, filepath.Join(home, ".zshrc"))).To(Succeed())

		_, err := run(`f=.zsh; echo extra >> "$HOME/${f}rc"`)
		Expect(err).To(MatchError(ContainSubstring("~/.zshrc")))
		Expect(os.Readlink(filepath.Join(home, ".zshrc"))).To(Equal(target))
		Expect(os.ReadFile(target)).To(Equal([]byte("# dotfiles\n")))
	})

	It("reports the scope violation when the hook also fails", func() {
		_, err := run(`d=.ssh; mkdir -p "$HOME/$d"; echo 'Host *' > "$HOME/$d/config"; exit 3`)
		Expect(err).To(MatchError(ContainSubstring("~/.ssh/config")))
		Expect(err).To(MatchError(ContainSubstring("exit status 3")))
		Expect(filepath.Join(home, ".ssh", "config")).NotTo(BeAnExistingFile())
	})

	It("allows hooks that stay inside their own files", func() {
		Expect(os.WriteFile(filepath.Join(home, ".bashrc"), []byte("# mine\n"), 0o644)).To(Succeed())
		_, err := run(`mkdir -p "$HOME/.local/share/demo" && echo ok > "$HOME/.local/share/demo/state"`)
		Expect(err).NotTo(HaveOccurred())
		Expect(filepath.Join(home, ".local", "share", "demo", "state")).To(BeAnExistingFile())
		Expect(os.ReadFile(filepath.Join(home, ".bashrc"))).To(Equal([]byte("# mine\n")))
	})
})
