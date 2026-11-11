package setupundo_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/jameswlane/devex/apps/cli/internal/setupundo"
	managed "github.com/jameswlane/devex/packages/plugin-sdk/shellrc"
)

// fakeRunner answers reads from a table and records every command.
type fakeRunner struct {
	outputs map[string]string
	fail    map[string]bool
	calls   [][]string
}

func (f *fakeRunner) run(_ context.Context, argv []string) (string, error) {
	f.calls = append(f.calls, argv)
	key := strings.Join(argv, " ")
	if f.fail[key] {
		return "", errors.New("exit status 1")
	}
	return f.outputs[key], nil
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
		runner = &fakeRunner{outputs: map[string]string{}, fail: map[string]bool{}}
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

			// A user edit after setup survives the undo.
			file, err := os.OpenFile(bashrc, os.O_APPEND|os.O_WRONLY, 0o600)
			Expect(err).NotTo(HaveOccurred())
			_, err = file.WriteString("alias gs='git status'\n")
			Expect(err).NotTo(HaveOccurred())
			Expect(file.Close()).To(Succeed())

			Expect(setupundo.Restore(ctx, runner.run, changes)).To(Succeed())
			_, entries, err := managed.Read(bashrc)
			Expect(err).NotTo(HaveOccurred())
			Expect(entries).To(Equal([]string{"export OLD=1"}))
			Expect(read(bashrc)).To(ContainSubstring("export EDITOR=vim\n"))
			Expect(read(bashrc)).To(ContainSubstring("alias gs='git status'"))
			Expect(zshrc).NotTo(BeAnExistingFile())
			leftovers, err := filepath.Glob(zshrc + "*")
			Expect(err).NotTo(HaveOccurred())
			Expect(leftovers).To(BeEmpty())
		})

		It("keeps a file setup created once the user has added content to it", func() {
			zshrc := filepath.Join(home, ".zshrc")
			snapshot := setupundo.SnapshotShellRC([]string{zshrc})
			_, err := managed.Apply(zshrc, []string{"export A=1"})
			Expect(err).NotTo(HaveOccurred())
			Expect(os.WriteFile(zshrc, []byte(read(zshrc)+"export MINE=1\n"), 0o600)).To(Succeed())

			Expect(setupundo.Restore(ctx, runner.run, snapshot.Changes())).To(Succeed())
			Expect(read(zshrc)).To(Equal("export MINE=1\n"))
		})

		It("reports nothing when no managed block changed", func() {
			bashrc := filepath.Join(home, ".bashrc")
			Expect(os.WriteFile(bashrc, []byte("export A=1\n"), 0o600)).To(Succeed())
			snapshot := setupundo.SnapshotShellRC([]string{bashrc})
			Expect(snapshot.Changes()).To(BeEmpty())
		})
	})

	Context("Git identity", func() {
		It("restores a previous value or unsets a key that had none", func() {
			runner.outputs["git config --global --get user.name"] = "Ada Lovelace\n"
			runner.fail["git config --global --get user.email"] = true

			name := setupundo.GitConfigChange(ctx, runner.run, "user.name")
			email := setupundo.GitConfigChange(ctx, runner.run, "user.email")
			Expect(name.Restore).To(Equal([][]string{{"git", "config", "--global", "user.name", "Ada Lovelace"}}))
			Expect(email.Restore).To(Equal([][]string{{"git", "config", "--global", "--unset", "user.email"}}))
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
			Expect(changes[0].Restore).To(Equal([][]string{{"gsettings", "set", "org.gnome.desktop.interface", "color-scheme", "'default'"}}))
			Expect(changes[1].Restore).To(Equal([][]string{{"kwriteconfig5", "--file", "kdeglobals", "--group", "General", "--key", "ColorScheme", "Breeze"}}))
			Expect(changes[2].Restore).To(Equal([][]string{{"kwriteconfig5", "--file", "kwinrc", "--group", "Windows", "--key", "BorderlessMaximizedWindows", "--delete"}}))
		})

		It("reports plan commands it cannot undo and keeps the rest", func() {
			runner.fail["gsettings get org.gnome.a b"] = true
			changes, err := setupundo.DesktopChanges(ctx, runner.run, [][]string{
				{"gsettings", "set", "org.gnome.a", "b", "1"},
				{"dconf", "write", "/x", "1"},
				{"gsettings", "set", "org.gnome.c", "d", "2"},
			})
			Expect(err).To(MatchError(And(ContainSubstring("read org.gnome.a b"), ContainSubstring(`no undo for desktop command ["dconf","write","/x","1"]`))))
			Expect(changes).To(HaveLen(1))
			Expect(changes[0].Description).To(Equal("GNOME org.gnome.c d"))
		})
	})

	Context("restoring", func() {
		It("runs restores newest first, keeps going after a failure and refuses other executables", func() {
			runner.fail["git config --global user.name Old"] = true
			err := setupundo.Restore(ctx, runner.run, []setupundo.Change{
				{Kind: setupundo.KindCommand, Description: "first", Restore: [][]string{{"gsettings", "set", "a", "b", "1"}}},
				{Kind: setupundo.KindCommand, Description: "evil", Restore: [][]string{{"rm", "-rf", "/"}}},
				{Kind: setupundo.KindCommand, Description: "last", Restore: [][]string{{"git", "config", "--global", "user.name", "Old"}}},
			})
			Expect(err).To(MatchError(And(ContainSubstring("restore last"), ContainSubstring("refusing to run restore command"))))
			Expect(runner.calls).To(Equal([][]string{
				{"git", "config", "--global", "user.name", "Old"},
				{"gsettings", "set", "a", "b", "1"},
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
