package setupundo_test

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"

	"github.com/jameswlane/devex/apps/cli/internal/setupundo"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Oh My Zsh undo records", func() {
	var home string

	BeforeEach(func() {
		home = GinkgoT().TempDir()
	})

	It("removes an unchanged file DevEx created and preserves one the user changed", func() {
		path := filepath.Join(home, ".local", "share", "devex", "defaults", "zsh", "rc")
		Expect(os.MkdirAll(filepath.Dir(path), 0o700)).To(Succeed())
		data := []byte("bundled defaults\n")
		Expect(os.WriteFile(path, data, 0o600)).To(Succeed())
		target, err := filepath.EvalSymlinks(path)
		Expect(err).NotTo(HaveOccurred())
		change := setupundo.Change{
			Kind: setupundo.KindCreatedFile, Description: "bundled rc", Path: path,
			Checksum: fmt.Sprintf("%x", sha256.Sum256(data)), Root: filepath.Dir(path), Target: target,
		}
		Expect(setupundo.Restore(context.Background(), nil, home, []setupundo.Change{change})).To(Succeed())
		Expect(path).NotTo(BeAnExistingFile())
		Expect(setupundo.Restore(context.Background(), nil, home, []setupundo.Change{change})).To(Succeed())

		Expect(os.WriteFile(path, []byte("user customization\n"), 0o600)).To(Succeed())
		err = setupundo.Restore(context.Background(), nil, home, []setupundo.Change{change})
		Expect(err).To(MatchError(ContainSubstring("changed after setup")))
		content, readErr := os.ReadFile(path)
		Expect(readErr).NotTo(HaveOccurred())
		Expect(string(content)).To(Equal("user customization\n"))
	})

	It("preserves an unchanged legacy file whose setup-time target is unknown", func() {
		path := filepath.Join(home, ".local", "share", "devex", "defaults", "zsh", "rc")
		Expect(os.MkdirAll(filepath.Dir(path), 0o700)).To(Succeed())
		data := []byte("bundled defaults\n")
		Expect(os.WriteFile(path, data, 0o600)).To(Succeed())
		change := setupundo.Change{
			Kind: setupundo.KindCreatedFile, Description: "legacy bundled rc", Path: path,
			Checksum: fmt.Sprintf("%x", sha256.Sum256(data)), Root: filepath.Dir(path),
		}

		err := setupundo.Restore(context.Background(), nil, home, []setupundo.Change{change})
		Expect(err).To(MatchError(ContainSubstring("legacy undo record")))
		Expect(err).To(MatchError(ContainSubstring("not removed")))
		Expect(err).To(MatchError(ContainSubstring("manually")))
		content, readErr := os.ReadFile(path)
		Expect(readErr).NotTo(HaveOccurred())
		Expect(content).To(Equal(data))

		Expect(os.Remove(path)).To(Succeed())
		Expect(setupundo.Restore(context.Background(), nil, home, []setupundo.Change{change})).To(Succeed())
	})

	It("preserves a legacy file behind a repointed parent", func() {
		root := filepath.Join(home, "data", "defaults", "zsh")
		path := filepath.Join(root, "rc")
		Expect(os.MkdirAll(root, 0o700)).To(Succeed())
		data := []byte("identical bundled content\n")
		Expect(os.WriteFile(path, data, 0o600)).To(Succeed())
		change := setupundo.Change{
			Kind: setupundo.KindCreatedFile, Description: "legacy bundled rc", Path: path,
			Checksum: fmt.Sprintf("%x", sha256.Sum256(data)), Root: root,
		}

		movedRoot := filepath.Join(filepath.Dir(root), "zsh-created")
		Expect(os.Rename(root, movedRoot)).To(Succeed())
		replacement := filepath.Join(home, "user-zsh")
		Expect(os.MkdirAll(replacement, 0o700)).To(Succeed())
		replacementPath := filepath.Join(replacement, "rc")
		Expect(os.WriteFile(replacementPath, data, 0o600)).To(Succeed())
		Expect(os.Symlink(replacement, root)).To(Succeed())

		err := setupundo.Restore(context.Background(), nil, home, []setupundo.Change{change})
		Expect(err).To(MatchError(ContainSubstring("legacy undo record")))
		Expect(err).To(MatchError(ContainSubstring("not removed")))
		Expect(replacementPath).To(BeAnExistingFile())
		Expect(filepath.Join(movedRoot, "rc")).To(BeAnExistingFile())
	})

	It("refuses paths outside the recorded root and symlinked defaults", func() {
		root := GinkgoT().TempDir()
		outside := filepath.Join(home, "outside")
		Expect(os.MkdirAll(outside, 0o700)).To(Succeed())
		data := []byte("keep user content\n")
		outsidePath := filepath.Join(outside, "rc")
		Expect(os.WriteFile(outsidePath, data, 0o600)).To(Succeed())
		checksum := fmt.Sprintf("%x", sha256.Sum256(data))
		outsideChange := setupundo.Change{Kind: setupundo.KindCreatedFile, Description: "outside", Path: outsidePath, Checksum: checksum, Root: root}
		Expect(setupundo.Restore(context.Background(), nil, home, []setupundo.Change{outsideChange})).To(MatchError(ContainSubstring("not a recorded DevEx Zsh default")))
		Expect(outsidePath).To(BeAnExistingFile())

		target := filepath.Join(outside, "target")
		Expect(os.WriteFile(target, data, 0o600)).To(Succeed())
		link := filepath.Join(root, "aliases")
		Expect(os.Symlink(target, link)).To(Succeed())
		linkChange := setupundo.Change{Kind: setupundo.KindCreatedFile, Description: "link", Path: link, Checksum: checksum, Root: root}
		Expect(setupundo.Restore(context.Background(), nil, home, []setupundo.Change{linkChange})).To(MatchError(ContainSubstring("not a regular file")))
		Expect(target).To(BeAnExistingFile())
		Expect(link).To(BeAnExistingFile())
	})

	It("refuses removal when a parent directory has been repointed", func() {
		root := filepath.Join(home, "data", "defaults", "zsh")
		path := filepath.Join(root, "rc")
		Expect(os.MkdirAll(root, 0o700)).To(Succeed())
		data := []byte("identical bundled content\n")
		Expect(os.WriteFile(path, data, 0o600)).To(Succeed())
		target, err := filepath.EvalSymlinks(path)
		Expect(err).NotTo(HaveOccurred())
		change := setupundo.Change{
			Kind: setupundo.KindCreatedFile, Description: "bundled rc", Path: path,
			Checksum: fmt.Sprintf("%x", sha256.Sum256(data)), Root: root, Target: target,
		}

		movedRoot := filepath.Join(filepath.Dir(root), "zsh-created")
		Expect(os.Rename(root, movedRoot)).To(Succeed())
		replacement := filepath.Join(home, "user-zsh")
		Expect(os.MkdirAll(replacement, 0o700)).To(Succeed())
		replacementPath := filepath.Join(replacement, "rc")
		Expect(os.WriteFile(replacementPath, data, 0o600)).To(Succeed())
		Expect(os.Symlink(replacement, root)).To(Succeed())

		err = setupundo.Restore(context.Background(), nil, home, []setupundo.Change{change})
		Expect(err).To(MatchError(ContainSubstring("now points to")))
		Expect(err).To(MatchError(ContainSubstring("not removed")))
		Expect(replacementPath).To(BeAnExistingFile())
		Expect(filepath.Join(movedRoot, "rc")).To(BeAnExistingFile())
	})

	It("moves aside only a marked clone and preserves content the user added later", func() {
		path := filepath.Join(home, ".oh-my-zsh")
		custom := filepath.Join(path, "custom", "plugins", "mine", "mine.plugin.zsh")
		Expect(os.MkdirAll(filepath.Dir(custom), 0o700)).To(Succeed())
		Expect(os.WriteFile(custom, []byte("user plugin\n"), 0o600)).To(Succeed())
		revision := "6010fb6260f359e71d99ba25f05f9215809b3ed2"
		change := setupundo.Change{Kind: setupundo.KindOhMyZsh, Description: "clone", Path: path, Value: revision}
		Expect(os.WriteFile(filepath.Join(path, setupundo.OhMyZshMarker), []byte("not DevEx\n"), 0o600)).To(Succeed())
		err := setupundo.Restore(context.Background(), nil, home, []setupundo.Change{change})
		Expect(err).To(MatchError(ContainSubstring("ownership marker does not match")))
		Expect(path).To(BeADirectory())

		Expect(os.WriteFile(filepath.Join(path, setupundo.OhMyZshMarker), []byte(setupundo.OhMyZshMarkerContent(revision)), 0o600)).To(Succeed())
		messages, err := setupundo.RestoreWithMessages(context.Background(), nil, home, []setupundo.Change{change})
		Expect(err).NotTo(HaveOccurred())
		Expect(path).NotTo(BeADirectory())
		asides, globErr := filepath.Glob(path + ".devex-undo-*")
		Expect(globErr).NotTo(HaveOccurred())
		Expect(asides).To(HaveLen(1))
		aside := filepath.Join(asides[0], ".oh-my-zsh")
		Expect(messages).To(ConsistOf(ContainSubstring(aside)))
		Expect(filepath.Join(aside, "custom", "plugins", "mine", "mine.plugin.zsh")).To(BeAnExistingFile())
		Expect(setupundo.Restore(context.Background(), nil, home, []setupundo.Change{change})).To(Succeed())
	})

	It("never removes an existing installation without a marker or another directory", func() {
		path := filepath.Join(home, ".oh-my-zsh")
		Expect(os.MkdirAll(path, 0o700)).To(Succeed())
		framework := filepath.Join(path, "oh-my-zsh.sh")
		Expect(os.WriteFile(framework, []byte("existing\n"), 0o600)).To(Succeed())
		revision := "6010fb6260f359e71d99ba25f05f9215809b3ed2"
		changes := []setupundo.Change{
			{Kind: setupundo.KindOhMyZsh, Description: "existing", Path: path, Value: revision},
			{Kind: setupundo.KindOhMyZsh, Description: "other", Path: filepath.Join(home, "other"), Value: revision},
		}
		Expect(setupundo.Restore(context.Background(), nil, home, changes)).To(HaveOccurred())
		content, err := os.ReadFile(framework)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(content)).To(Equal("existing\n"))
	})
})
