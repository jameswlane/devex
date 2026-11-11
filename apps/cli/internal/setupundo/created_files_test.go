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
		change := setupundo.Change{
			Kind: setupundo.KindCreatedFile, Description: "bundled rc", Path: path,
			Checksum: fmt.Sprintf("%x", sha256.Sum256(data)),
		}
		Expect(setupundo.Restore(context.Background(), nil, home, []setupundo.Change{change})).To(Succeed())
		Expect(path).NotTo(BeAnExistingFile())
		Expect(setupundo.Restore(context.Background(), nil, home, []setupundo.Change{change})).To(Succeed())

		Expect(os.WriteFile(path, []byte("user customization\n"), 0o600)).To(Succeed())
		err := setupundo.Restore(context.Background(), nil, home, []setupundo.Change{change})
		Expect(err).To(MatchError(ContainSubstring("changed after setup")))
		content, readErr := os.ReadFile(path)
		Expect(readErr).NotTo(HaveOccurred())
		Expect(string(content)).To(Equal("user customization\n"))
	})

	It("removes only a clone with the matching DevEx ownership marker", func() {
		path := filepath.Join(home, ".oh-my-zsh")
		Expect(os.MkdirAll(path, 0o700)).To(Succeed())
		revision := "6010fb6260f359e71d99ba25f05f9215809b3ed2"
		change := setupundo.Change{Kind: setupundo.KindOhMyZsh, Description: "clone", Path: path, Value: revision}
		Expect(os.WriteFile(filepath.Join(path, setupundo.OhMyZshMarker), []byte("not DevEx\n"), 0o600)).To(Succeed())
		err := setupundo.Restore(context.Background(), nil, home, []setupundo.Change{change})
		Expect(err).To(MatchError(ContainSubstring("ownership marker does not match")))
		Expect(path).To(BeADirectory())

		Expect(os.WriteFile(filepath.Join(path, setupundo.OhMyZshMarker), []byte(setupundo.OhMyZshMarkerContent(revision)), 0o600)).To(Succeed())
		Expect(setupundo.Restore(context.Background(), nil, home, []setupundo.Change{change})).To(Succeed())
		Expect(path).NotTo(BeADirectory())
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
