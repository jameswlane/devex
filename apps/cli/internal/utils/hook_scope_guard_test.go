//go:build unix

package utils_test

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/jameswlane/devex/apps/cli/internal/shellrc"

	"github.com/jameswlane/devex/apps/cli/internal/testhelper"
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
		testhelper.SetProfileEnv(GinkgoT(), home)
	})

	run := func(script string) (string, error) {
		return utils.ExecInstallHookWithContext(context.Background(), types.InstallCommand{Shell: script},
			utils.InstallHookPolicy{AppName: "demo"})
	}

	It("rejects and removes an rc file a hook creates through a computed path", func() {
		_, err := run(`name=zsh; echo 'eval "$(demo init)"' >> "$HOME/.${name}rc"`)
		var scopeErr *utils.HookScopeError
		Expect(errors.As(err, &scopeErr)).To(BeTrue())
		Expect(utils.IsHookRejection(err)).To(BeTrue())
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

	It("restores a file the hook only made read-only", func() {
		rc := filepath.Join(home, ".profile")
		Expect(os.WriteFile(rc, []byte("# mine\n"), 0o644)).To(Succeed())
		_, err := run(`p=.prof; chmod 0400 "$HOME/${p}ile"`)
		Expect(err).To(MatchError(And(ContainSubstring("~/.profile"), ContainSubstring("restored"))))
		info, statErr := os.Stat(rc)
		Expect(statErr).NotTo(HaveOccurred())
		Expect(info.Mode().Perm()).To(Equal(os.FileMode(0o644)))
	})

	It("recreates a symlink target the hook deleted", func() {
		target := filepath.Join(home, "dotfiles-zshrc")
		Expect(os.WriteFile(target, []byte("# dotfiles\n"), 0o644)).To(Succeed())
		Expect(os.Symlink(target, filepath.Join(home, ".zshrc"))).To(Succeed())

		_, err := run(`rm "$HOME/dotfiles-zshrc"`)
		Expect(err).To(MatchError(And(ContainSubstring("~/.zshrc"), ContainSubstring("restored"))))
		Expect(os.ReadFile(target)).To(Equal([]byte("# dotfiles\n")))
	})

	It("refuses to restore through a link chain the hook repointed", func() {
		dotfiles := filepath.Join(home, "dotfiles")
		other := filepath.Join(home, "other")
		Expect(os.MkdirAll(dotfiles, 0o755)).To(Succeed())
		Expect(os.MkdirAll(other, 0o755)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(dotfiles, "zshrc"), []byte("# dotfiles\n"), 0o644)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(other, "zshrc"), []byte("unrelated\n"), 0o644)).To(Succeed())
		Expect(os.Symlink(dotfiles, filepath.Join(home, "current"))).To(Succeed())
		Expect(os.Symlink(filepath.Join(home, "current", "zshrc"), filepath.Join(home, ".zshrc"))).To(Succeed())

		_, err := run(`ln -sfn "$HOME/other" "$HOME/current"`)
		Expect(err).To(MatchError(And(ContainSubstring("~/.zshrc"), ContainSubstring("resolves elsewhere"))))
		Expect(os.ReadFile(filepath.Join(other, "zshrc"))).To(Equal([]byte("unrelated\n")))
	})

	It("refuses to restore through a link the hook made dangling elsewhere", func() {
		real := filepath.Join(home, "real-zshrc")
		Expect(os.WriteFile(real, []byte("# dotfiles\n"), 0o644)).To(Succeed())
		Expect(os.Symlink(real, filepath.Join(home, ".zshrc"))).To(Succeed())

		_, err := run(`rm "$HOME/real-zshrc"; ln -s "$HOME/other-file" "$HOME/real-zshrc"`)
		Expect(err).To(MatchError(And(ContainSubstring("~/.zshrc"), ContainSubstring("resolves elsewhere"))))
		Expect(filepath.Join(home, "other-file")).NotTo(BeAnExistingFile())
	})

	It("refuses to restore through a parent directory the hook swapped", func() {
		elsewhere := GinkgoT().TempDir()
		Expect(os.WriteFile(filepath.Join(elsewhere, "config"), []byte("unrelated\n"), 0o644)).To(Succeed())

		_, err := run(`ln -s "` + elsewhere + `" "$HOME/.s""sh"`)
		Expect(err).To(MatchError(And(ContainSubstring("~/.ssh/config"), ContainSubstring("parent directory changed"))))
		Expect(os.ReadFile(filepath.Join(elsewhere, "config"))).To(Equal([]byte("unrelated\n")))
	})

	It("refuses to write through a hard link the hook introduced", func() {
		Expect(os.WriteFile(filepath.Join(home, ".bashrc"), []byte("# mine\n"), 0o644)).To(Succeed())
		other := filepath.Join(home, "notes.txt")
		Expect(os.WriteFile(other, []byte("unrelated\n"), 0o644)).To(Succeed())

		_, err := run(`b=.bash; ln -f "$HOME/notes.txt" "$HOME/${b}rc"`)
		Expect(err).To(MatchError(And(ContainSubstring("~/.bashrc"), ContainSubstring("hard link"))))
		Expect(os.ReadFile(other)).To(Equal([]byte("unrelated\n")))
	})

	It("restores a group the hook changed without root", func() {
		groups, idErr := exec.Command("id", "-G").Output()
		Expect(idErr).NotTo(HaveOccurred())
		fields := strings.Fields(string(groups))
		if len(fields) < 2 {
			Skip("needs a user in at least two groups")
		}
		rc := filepath.Join(home, ".zshenv")
		Expect(os.WriteFile(rc, []byte("# mine\n"), 0o644)).To(Succeed())
		info, statErr := os.Stat(rc)
		Expect(statErr).NotTo(HaveOccurred())
		original := info.Sys().(*syscall.Stat_t).Gid
		other := fields[0]
		if other == strconv.Itoa(int(original)) {
			other = fields[1]
		}

		_, err := run(`z=.zsh; chgrp ` + other + ` "$HOME/${z}env"`)
		Expect(err).To(MatchError(And(ContainSubstring("~/.zshenv"), ContainSubstring("restored"))))
		info, statErr = os.Stat(rc)
		Expect(statErr).NotTo(HaveOccurred())
		Expect(info.Sys().(*syscall.Stat_t).Gid).To(Equal(original))
	})

	It("lets a declarative rc write wait for a running guarded hook instead of being undone", func() {
		ctx := shellrc.WithShell(context.Background(), "bash")
		done := make(chan error, 1)
		started := filepath.Join(home, "hook-started")
		go func() { _, err := run(`touch "$HOME/hook-started"; sleep 0.5`); done <- err }()
		Eventually(started).Should(BeAnExistingFile())
		Expect(utils.ApplyShellActions(ctx, types.InstallCommand{ShellInit: map[string]string{"bash": `eval "$(other init bash)"`}})).To(Succeed())
		Expect(<-done).To(Succeed())
		Expect(os.ReadFile(filepath.Join(home, ".bashrc"))).To(ContainSubstring("other init bash"))
	})

	It("does not let a concurrent hook capture and put back another hook's violation", func() {
		bad := make(chan error, 1)
		go func() { _, err := run(`n=zsh; echo bad >> "$HOME/.${n}rc"; sleep 0.5`); bad <- err }()
		// Start the second hook only after the first one's write, while it still runs.
		Eventually(filepath.Join(home, ".zshrc")).Should(BeAnExistingFile())
		// The second hook outlasts the first guard's restore, which removes the file.
		_, err := run(`for i in $(seq 100); do [ -e "$HOME/.zs""hrc" ] || break; sleep 0.05; done`)
		Expect(err).NotTo(HaveOccurred())
		Expect(<-bad).To(MatchError(ContainSubstring("~/.zshrc")))
		Expect(filepath.Join(home, ".zshrc")).NotTo(BeAnExistingFile())
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
