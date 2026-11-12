package dotfiles_test

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/jameswlane/devex/apps/cli/internal/dotfiles"
)

var _ = Describe("Dotfiles", func() {
	DescribeTable("validates repositories",
		func(repository string, valid bool) {
			err := dotfiles.ValidateRepository(repository)
			if valid {
				Expect(err).NotTo(HaveOccurred())
			} else {
				Expect(err).To(HaveOccurred())
			}
		},
		Entry("https", "https://github.com/me/dotfiles", true),
		Entry("ssh URL with a user", "ssh://git@github.com/me/dotfiles.git", true),
		Entry("scp-like", "git@github.com:me/dotfiles.git", true),
		Entry("absolute path", "/srv/dotfiles", true),
		Entry("empty", "", false),
		Entry("option injection", "--upload-pack=touch /tmp/x", false),
		Entry("https token", "https://me:ghp_secret@github.com/me/dotfiles", false),
		Entry("https user", "https://me@github.com/me/dotfiles", false),
		Entry("plain http", "http://example.com/dotfiles", false),
		Entry("relative path", "dotfiles", false),
	)

	It("refuses refs that could be options or ranges", func() {
		Expect(dotfiles.ValidateRef("")).To(Succeed())
		Expect(dotfiles.ValidateRef("v1.2.0")).To(Succeed())
		Expect(dotfiles.ValidateRef("feature/zsh")).To(Succeed())
		Expect(dotfiles.ValidateRef("-x")).To(HaveOccurred())
		Expect(dotfiles.ValidateRef("main..dev")).To(HaveOccurred())
		Expect(dotfiles.ValidateRef("main;rm")).To(HaveOccurred())
	})

	It("redacts user information from URLs", func() {
		Expect(dotfiles.Redact("ssh://git@github.com/me/d.git")).To(Equal("ssh://github.com/me/d.git"))
		Expect(dotfiles.Redact("git@github.com:me/d.git")).To(Equal("git@github.com:me/d.git"))
	})

	Describe("LoadManifest", func() {
		var home, repo string
		write := func(path, content string) {
			Expect(os.MkdirAll(filepath.Dir(path), 0o700)).To(Succeed())
			Expect(os.WriteFile(path, []byte(content), 0o600)).To(Succeed())
		}
		load := func(manifest string) error {
			write(filepath.Join(repo, dotfiles.ManifestFile), manifest)
			_, err := dotfiles.LoadManifest(dotfiles.Checkout{Dir: repo}, home)
			return err
		}

		BeforeEach(func() {
			home = GinkgoT().TempDir()
			GinkgoT().Setenv("HOME", home)
			for _, key := range []string{"XDG_CONFIG_HOME", "XDG_DATA_HOME", "XDG_STATE_HOME", "XDG_CACHE_HOME", "DEVEX_CONFIG_DIR", "DEVEX_DATA_DIR", "DEVEX_STATE_DIR", "DEVEX_CACHE_DIR"} {
				GinkgoT().Setenv(key, "")
			}
			repo = GinkgoT().TempDir()
			write(filepath.Join(repo, "zsh", ".zshrc"), "export EDITOR=vim\n")
		})

		It("maps repository files to home destinations", func() {
			write(filepath.Join(repo, dotfiles.ManifestFile), "files:\n  - source: zsh/.zshrc\n    destination: ~/.zshrc\n")
			mappings, err := dotfiles.LoadManifest(dotfiles.Checkout{Dir: repo}, home)
			Expect(err).NotTo(HaveOccurred())
			Expect(mappings).To(HaveLen(1))
			Expect(mappings[0].Source).To(Equal(filepath.Join(repo, "zsh", ".zshrc")))
			Expect(mappings[0].Destination).To(Equal(filepath.Join(home, ".zshrc")))
		})

		It("explains a missing manifest", func() {
			_, err := dotfiles.LoadManifest(dotfiles.Checkout{Dir: repo}, home)
			Expect(err).To(MatchError(ContainSubstring("no devex-dotfiles.yaml")))
		})

		DescribeTable("rejects unsafe entries",
			func(manifest, message string) {
				Expect(load(manifest)).To(MatchError(ContainSubstring(message)))
			},
			Entry("source traversal", "files:\n  - source: ../outside\n    destination: ~/.x\n", "inside the repository"),
			Entry("absolute source", "files:\n  - source: /etc/passwd\n    destination: ~/.x\n", "inside the repository"),
			Entry("git internals", "files:\n  - source: .git/config\n    destination: ~/.x\n", "inside .git"),
			Entry("missing source", "files:\n  - source: nope\n    destination: ~/.x\n", "nope"),
			Entry("destination outside home", "files:\n  - source: zsh/.zshrc\n    destination: /etc/zshrc\n", "inside your home directory"),
			Entry("destination traversal", "files:\n  - source: zsh/.zshrc\n    destination: ~/../other/.zshrc\n", "traversal"),
			Entry("DevEx directory", "files:\n  - source: zsh/.zshrc\n    destination: ~/.local/share/devex/x\n", "DevEx's own directory"),
			Entry("duplicate destination", "files:\n  - source: zsh/.zshrc\n    destination: ~/.zshrc\n  - source: zsh/.zshrc\n    destination: ~/.zshrc\n", "listed twice"),
			Entry("no files", "files: []\n", "lists no files"),
		)

		It("never deploys a symlink, even one pointing inside the repository", func() {
			Expect(os.Symlink("/etc/passwd", filepath.Join(repo, "evil"))).To(Succeed())
			Expect(load("files:\n  - source: evil\n    destination: ~/.evil\n")).To(MatchError(ContainSubstring("symlinks are not deployed")))
		})
	})

	It("records, reads and clears the sync state", func() {
		home := GinkgoT().TempDir()
		GinkgoT().Setenv("HOME", home)
		_, found, err := dotfiles.ReadState(home)
		Expect(err).NotTo(HaveOccurred())
		Expect(found).To(BeFalse())
		Expect(dotfiles.WriteState(home, dotfiles.State{Repository: "/srv/d", Commit: "abc"})).To(Succeed())
		state, found, err := dotfiles.ReadState(home)
		Expect(err).NotTo(HaveOccurred())
		Expect(found).To(BeTrue())
		Expect(state.Commit).To(Equal("abc"))
		Expect(dotfiles.ClearState(home)).To(Succeed())
		_, found, _ = dotfiles.ReadState(home)
		Expect(found).To(BeFalse())
	})
})
