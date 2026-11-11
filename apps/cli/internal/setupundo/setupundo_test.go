package setupundo_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/jameswlane/devex/apps/cli/internal/setupundo"
	managed "github.com/jameswlane/devex/packages/plugin-sdk/shellrc"
)

// exitStatusError is a fake process exit, like *exec.ExitError.
type exitStatusError int

func (e exitStatusError) Error() string { return fmt.Sprintf("exit status %d", int(e)) }
func (e exitStatusError) ExitCode() int { return int(e) }

// fakeRunner answers reads from a table and records every command.
type fakeRunner struct {
	outputs map[string]string
	errs    map[string]error
	calls   [][]string
}

func (f *fakeRunner) run(_ context.Context, argv []string) (string, error) {
	f.calls = append(f.calls, argv)
	key := strings.Join(argv, " ")
	return f.outputs[key], f.errs[key]
}

var _ = Describe("Setup undo", func() {
	var (
		home   string
		ctx    context.Context
		runner *fakeRunner
	)

	BeforeEach(func() {
		home = GinkgoT().TempDir()
		ctx = context.Background()
		runner = &fakeRunner{outputs: map[string]string{}, errs: map[string]error{}}
	})

	read := func(path string) string {
		data, err := os.ReadFile(path)
		Expect(err).NotTo(HaveOccurred())
		return string(data)
	}

	Context("shell rc files", func() {
		It("restores the managed block, keeps later user edits and deletes files setup created", func() {
			bashrc := filepath.Join(home, ".bashrc")
			zshrc := filepath.Join(home, ".zshrc")
			Expect(os.WriteFile(bashrc, []byte("export EDITOR=vim\n"), 0o600)).To(Succeed())
			_, err := managed.Apply(bashrc, []string{"export OLD=1"})
			Expect(err).NotTo(HaveOccurred())

			snapshot := setupundo.SnapshotShellRC(setupundo.ShellRCPaths(home))
			_, err = managed.Merge(bashrc, []string{"eval \"$(zoxide init bash)\""})
			Expect(err).NotTo(HaveOccurred())
			_, err = managed.Apply(zshrc, []string{"eval \"$(zoxide init zsh)\""})
			Expect(err).NotTo(HaveOccurred())

			changes := snapshot.Changes()
			Expect(changes).To(HaveLen(2))
			Expect(changes[0]).To(Equal(setupundo.Change{Kind: setupundo.KindShellRC, Description: "DevEx block in " + bashrc, Path: bashrc, Existed: true, Entries: []string{"export OLD=1"}}))
			Expect(changes[1].Path).To(Equal(zshrc))
			Expect(changes[1].Existed).To(BeFalse())

			file, err := os.OpenFile(bashrc, os.O_APPEND|os.O_WRONLY, 0o600)
			Expect(err).NotTo(HaveOccurred())
			_, err = file.WriteString("alias gs='git status'\n")
			Expect(err).NotTo(HaveOccurred())
			Expect(file.Close()).To(Succeed())

			Expect(setupundo.Restore(ctx, runner.run, home, changes)).To(Succeed())
			_, entries, err := managed.Read(bashrc)
			Expect(err).NotTo(HaveOccurred())
			Expect(entries).To(Equal([]string{"export OLD=1"}))
			Expect(read(bashrc)).To(ContainSubstring("export EDITOR=vim\n"))
			Expect(read(bashrc)).To(ContainSubstring("alias gs='git status'"))
			Expect(zshrc).NotTo(BeAnExistingFile())
			leftovers, err := filepath.Glob(zshrc + "*")
			Expect(err).NotTo(HaveOccurred())
			Expect(leftovers).To(BeEmpty())

			// Undo is safe to repeat.
			Expect(setupundo.Restore(ctx, runner.run, home, changes)).To(Succeed())
		})

		It("keeps a file setup created once the user has added content to it", func() {
			zshrc := filepath.Join(home, ".zshrc")
			snapshot := setupundo.SnapshotShellRC([]string{zshrc})
			_, err := managed.Apply(zshrc, []string{"export A=1"})
			Expect(err).NotTo(HaveOccurred())
			Expect(os.WriteFile(zshrc, []byte(read(zshrc)+"export MINE=1\n"), 0o600)).To(Succeed())

			Expect(setupundo.Restore(ctx, runner.run, home, snapshot.Changes())).To(Succeed())
			Expect(read(zshrc)).To(Equal("export MINE=1\n"))
		})

		It("never deletes a created file whose block markers are damaged", func() {
			zshrc := filepath.Join(home, ".zshrc")
			content := managed.Start + "\nexport MINE=important\n"
			Expect(os.WriteFile(zshrc, []byte(content), 0o600)).To(Succeed())

			err := setupundo.Restore(ctx, runner.run, home, []setupundo.Change{{Kind: setupundo.KindShellRC, Description: "Created " + zshrc, Path: zshrc}})
			Expect(err).To(MatchError(ContainSubstring("fix the DevEx block markers first")))
			Expect(read(zshrc)).To(Equal(content))
		})

		It("refuses rc changes outside the shell rc files in home", func() {
			other := filepath.Join(home, "notes.txt")
			Expect(os.WriteFile(other, []byte("keep me\n"), 0o600)).To(Succeed())
			changes := []setupundo.Change{
				{Kind: setupundo.KindShellRC, Description: "a", Path: other, Existed: true, Entries: []string{"echo injected"}},
				{Kind: setupundo.KindShellRC, Description: "b", Path: filepath.Join(home, ".config", "..", "notes.txt")},
			}
			Expect(setupundo.Restore(ctx, runner.run, home, changes)).To(MatchError(ContainSubstring("not a shell rc file")))
			Expect(read(other)).To(Equal("keep me\n"))
		})

		It("reports nothing when no managed block changed", func() {
			bashrc := filepath.Join(home, ".bashrc")
			Expect(os.WriteFile(bashrc, []byte("export A=1\n"), 0o600)).To(Succeed())
			Expect(setupundo.SnapshotShellRC([]string{bashrc}).Changes()).To(BeEmpty())
		})
	})

	Context("Git identity", func() {
		It("records a previous value, or unset only when Git reports no value", func() {
			runner.outputs["git config --global --get user.name"] = "Ada Lovelace\n"
			runner.errs["git config --global --get user.email"] = exitStatusError(1)

			name, err := setupundo.GitConfigChange(ctx, runner.run, "user.name")
			Expect(err).NotTo(HaveOccurred())
			Expect(name).To(Equal(setupundo.Change{Kind: setupundo.KindGitConfig, Description: "Global Git user.name", Key: "user.name", Value: "Ada Lovelace"}))
			email, err := setupundo.GitConfigChange(ctx, runner.run, "user.email")
			Expect(err).NotTo(HaveOccurred())
			Expect(email.Unset).To(BeTrue())

			Expect(setupundo.Restore(ctx, runner.run, home, []setupundo.Change{name, email})).To(Succeed())
			Expect(runner.calls[2:]).To(Equal([][]string{
				{"git", "config", "--global", "--unset", "user.email"},
				{"git", "config", "--global", "--", "user.name", "Ada Lovelace"},
			}))
		})

		It("refuses to guess the earlier state when reading it fails", func() {
			runner.errs["git config --global --get user.name"] = exitStatusError(128)
			_, err := setupundo.GitConfigChange(ctx, runner.run, "user.name")
			Expect(err).To(MatchError(ContainSubstring("read global Git user.name")))

			runner.errs["git config --global --get user.name"] = fmt.Errorf("exec: git: not found")
			_, err = setupundo.GitConfigChange(ctx, runner.run, "user.name")
			Expect(err).To(HaveOccurred())

			cancelled, cancel := context.WithCancel(ctx)
			cancel()
			runner.errs["git config --global --get user.name"] = exitStatusError(1)
			_, err = setupundo.GitConfigChange(cancelled, runner.run, "user.name")
			Expect(err).To(HaveOccurred())
		})

		It("treats unsetting an already missing key as done, so undo can be retried", func() {
			runner.errs["git config --global --unset user.email"] = exitStatusError(5)
			Expect(setupundo.Restore(ctx, runner.run, home, []setupundo.Change{{Kind: setupundo.KindGitConfig, Description: "e", Key: "user.email", Unset: true}})).To(Succeed())
		})
	})

	Context("desktop settings", func() {
		It("restores each GNOME and KDE key the plan writes, deleting keys that were unset", func() {
			runner.outputs["gsettings get org.gnome.desktop.interface color-scheme"] = "'default'\n"
			runner.outputs["kreadconfig5 --file kdeglobals --group General --key ColorScheme --default __devex_unset__"] = "Breeze\n"
			runner.outputs["kreadconfig5 --file kwinrc --group Windows --key BorderlessMaximizedWindows --default __devex_unset__"] = "__devex_unset__\n"

			changes, err := setupundo.DesktopChanges(ctx, runner.run, [][]string{
				{"gsettings", "set", "org.gnome.desktop.interface", "color-scheme", "'prefer-dark'"},
				{"kwriteconfig5", "--file", "kdeglobals", "--group", "General", "--key", "ColorScheme", "BreezeDark"},
				{"kwriteconfig5", "--file", "kwinrc", "--group", "Windows", "--key", "BorderlessMaximizedWindows", "true"},
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(changes).To(HaveLen(3))

			runner.calls = nil
			Expect(setupundo.Restore(ctx, runner.run, home, changes)).To(Succeed())
			Expect(runner.calls).To(Equal([][]string{
				{"kwriteconfig5", "--file", "kwinrc", "--group", "Windows", "--key", "BorderlessMaximizedWindows", "--delete"},
				{"kwriteconfig5", "--file", "kdeglobals", "--group", "General", "--key", "ColorScheme", "--", "Breeze"},
				{"gsettings", "set", "org.gnome.desktop.interface", "color-scheme", "'default'"},
			}))
		})

		It("reports plan commands it cannot undo and keeps the rest", func() {
			runner.errs["gsettings get org.gnome.a b"] = exitStatusError(1)
			changes, err := setupundo.DesktopChanges(ctx, runner.run, [][]string{
				{"gsettings", "set", "org.gnome.a", "b", "1"},
				{"dconf", "write", "/x", "1"},
				{"kwriteconfig5", "--file", "../../.bashrc", "--group", "G", "--key", "K", "v"},
				{"gsettings", "set", "org.gnome.c", "d", "2"},
			})
			Expect(err).To(MatchError(And(
				ContainSubstring("read org.gnome.a b"),
				ContainSubstring(`no undo for desktop command ["dconf","write","/x","1"]`),
				ContainSubstring("invalid KDE setting"),
			)))
			Expect(changes).To(HaveLen(1))
			Expect(changes[0].Description).To(Equal("GNOME org.gnome.c d"))
		})
	})

	Context("tampered history", func() {
		It("builds restores only from validated fields and keeps going after a refusal", func() {
			err := setupundo.Restore(ctx, runner.run, home, []setupundo.Change{
				{Kind: setupundo.KindGSettings, Description: "ok", Schema: "org.gnome.desktop.interface", Key: "color-scheme", Value: "'default'"},
				{Kind: "command", Description: "old format"},
				{Kind: setupundo.KindGitConfig, Description: "alias", Key: "alias.review", Value: "!sh -c evil"},
				{Kind: setupundo.KindGSettings, Description: "option", Schema: "--schemadir=/tmp", Key: "x"},
				{Kind: setupundo.KindKConfig, Description: "path", File: "/etc/passwd", Group: "g", Key: "k"},
				{Kind: setupundo.KindKConfig, Description: "flag", File: "kdeglobals", Group: "--delete", Key: "k"},
				{Kind: setupundo.KindGitConfig, Description: "dash value", Key: "user.name", Value: "-c alias.x=!evil"},
			})
			Expect(err).To(MatchError(And(
				ContainSubstring(`unknown change kind "command"`),
				ContainSubstring(`git key "alias.review" is not undoable`),
				ContainSubstring("invalid GNOME setting"),
				ContainSubstring("invalid KDE setting"),
			)))
			Expect(runner.calls).To(Equal([][]string{
				{"git", "config", "--global", "--", "user.name", "-c alias.x=!evil"},
				{"gsettings", "set", "org.gnome.desktop.interface", "color-scheme", "'default'"},
			}))
		})

		It("round-trips changes through the undo history metadata", func() {
			changes := []setupundo.Change{{Kind: setupundo.KindShellRC, Description: "d", Path: "/p", Existed: true, Entries: []string{"x"}}}
			encoded, err := setupundo.Encode(changes)
			Expect(err).NotTo(HaveOccurred())
			Expect(setupundo.Decode(encoded)).To(Equal(changes))
			_, err = setupundo.Decode(map[string]any{})
			Expect(err).To(HaveOccurred())
		})
	})
})
