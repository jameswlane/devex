package commands

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"

	"github.com/jameswlane/devex/apps/cli/internal/setupundo"
	"github.com/jameswlane/devex/apps/cli/internal/undo"
	"github.com/jameswlane/devex/packages/plugin-sdk/sdktest"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Undo command partial failures", func() {
	It("reports a preserved clone even when another setup change cannot be restored", func() {
		home := GinkgoT().TempDir()
		GinkgoT().Setenv("HOME", home)
		clone := filepath.Join(home, ".oh-my-zsh")
		plugin := filepath.Join(clone, "custom", "plugins", "mine", "mine.plugin.zsh")
		Expect(os.MkdirAll(filepath.Dir(plugin), 0o700)).To(Succeed())
		Expect(os.WriteFile(plugin, []byte("user plugin\n"), 0o600)).To(Succeed())
		revision := "6010fb6260f359e71d99ba25f05f9215809b3ed2"
		Expect(os.WriteFile(filepath.Join(clone, setupundo.OhMyZshMarker), []byte(setupundo.OhMyZshMarkerContent(revision)), 0o600)).To(Succeed())

		aliases := filepath.Join(home, ".local", "share", "devex", "defaults", "zsh", "aliases")
		Expect(os.MkdirAll(filepath.Dir(aliases), 0o700)).To(Succeed())
		Expect(os.WriteFile(aliases, []byte("bundled aliases\n"), 0o600)).To(Succeed())
		target, err := filepath.EvalSymlinks(aliases)
		Expect(err).NotTo(HaveOccurred())
		changes := []setupundo.Change{
			{Kind: setupundo.KindCreatedFile, Description: "Created bundled Zsh aliases", Path: aliases, Root: filepath.Dir(aliases), Checksum: fmt.Sprintf("%x", sha256.Sum256([]byte("bundled aliases\n"))), Target: target},
			{Kind: setupundo.KindOhMyZsh, Description: "Oh My Zsh clone", Path: clone, Value: revision},
		}
		encoded, err := setupundo.Encode(changes)
		Expect(err).NotTo(HaveOccurred())
		manager := undo.NewUndoManager(filepath.Join(home, ".devex"))
		Expect(os.MkdirAll(filepath.Join(home, ".devex", "config"), 0o750)).To(Succeed())
		operation, err := manager.RecordOperation(setupundo.Operation, "devex setup", "", map[string]interface{}{setupundo.MetadataKey: encoded})
		Expect(err).NotTo(HaveOccurred())
		Expect(os.WriteFile(aliases, []byte("user customization\n"), 0o600)).To(Succeed())

		var undoErr error
		output := sdktest.CaptureStdout(GinkgoT(), func() {
			undoErr = undoSpecificOperation(manager, operation.ID, true)
		})

		Expect(undoErr).To(MatchError(ContainSubstring("changed after setup")))
		asides, err := filepath.Glob(clone + ".devex-undo-*")
		Expect(err).NotTo(HaveOccurred())
		Expect(asides).To(HaveLen(1))
		aside := filepath.Join(asides[0], ".oh-my-zsh")
		Expect(output).To(ContainSubstring(aside))
		Expect(filepath.Join(aside, "custom", "plugins", "mine", "mine.plugin.zsh")).To(BeAnExistingFile())
	})
})
