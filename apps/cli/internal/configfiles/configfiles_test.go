package configfiles_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/jameswlane/devex/apps/cli/internal/configfiles"
	"github.com/jameswlane/devex/apps/cli/internal/types"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestConfigFiles(t *testing.T) { RegisterFailHandler(Fail); RunSpecs(t, "Config files suite") }

var _ = Describe("Config file plans", func() {
	It("previews copy, backup and parent creation without changing any files", func(ctx SpecContext) {
		home := GinkgoT().TempDir()
		GinkgoT().Setenv("HOME", home)
		source, dest := filepath.Join(home, "source"), filepath.Join(home, "destination")
		Expect(os.WriteFile(source, []byte("new"), 0600)).To(Succeed())
		Expect(os.WriteFile(dest, []byte("old"), 0600)).To(Succeed())
		var plan []string
		result := configfiles.Apply(ctx, "preview", []types.ConfigFile{{Source: "$HOME/source", Destination: "~/destination"}, {Source: source, Destination: filepath.Join(home, "new", "config")}}, true, func(line string) { plan = append(plan, line) })
		Expect(result.Warnings).To(BeEmpty())
		Expect(strings.Join(plan, "\n")).To(And(ContainSubstring("Copy "), ContainSubstring("Back up "), ContainSubstring(".devex-bak"), ContainSubstring("create parent directories")))
		data, err := os.ReadFile(dest)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(data)).To(Equal("old"))
		files, err := os.ReadDir(home)
		Expect(err).NotTo(HaveOccurred())
		Expect(files).To(HaveLen(2))
	})
	It("expands the documented variable syntaxes and rejects unknown variables", func() {
		home := GinkgoT().TempDir()
		GinkgoT().Setenv("HOME", home)
		GinkgoT().Setenv("CONFIG_TEST", home)
		for _, path := range []string{"~/config", "$HOME/config", "${CONFIG_TEST}/config", "%CONFIG_TEST%/config"} {
			expanded, err := configfiles.Expand(path)
			Expect(err).NotTo(HaveOccurred())
			Expect(expanded).To(Equal(filepath.Join(home, "config")))
		}
		_, err := configfiles.Expand("$UNSET_CONFIG_TEST/config")
		Expect(err).To(MatchError(ContainSubstring("unset environment variable")))
	})

	Describe("conditional Neovim configuration", func() {
		var home, source, destination string
		var mappings []types.ConfigFile

		BeforeEach(func() {
			home = GinkgoT().TempDir()
			GinkgoT().Setenv("HOME", home)
			source = filepath.Join(home, "source.lua")
			destination = filepath.Join(home, ".config", "nvim", "lazyvim.json")
			Expect(os.WriteFile(source, []byte("managed"), 0600)).To(Succeed())
			mappings = []types.ConfigFile{{
				Source:      source,
				Destination: destination,
				RequiresAny: []string{
					"~/.config/nvim/lua/config/lazy.lua",
					"~/.config/nvim/lazy-lock.json",
				},
			}}
		})

		It("writes nothing when no Neovim configuration exists", func(ctx SpecContext) {
			var plan []string
			result := configfiles.Apply(ctx, "Neovim", mappings, false, func(line string) { plan = append(plan, line) })

			Expect(result.Warnings).To(BeEmpty())
			Expect(destination).NotTo(BeAnExistingFile())
			Expect(filepath.Join(home, ".config")).NotTo(BeADirectory())
			Expect(filepath.Join(home, ".local", "state", "devex")).NotTo(BeADirectory())
			Expect(strings.Join(plan, "\n")).To(ContainSubstring("Skip"))
		})

		DescribeTable("accepts either lazy.nvim marker",
			func(ctx SpecContext, marker string) {
				markerPath := filepath.Join(home, ".config", "nvim", filepath.FromSlash(marker))
				Expect(os.MkdirAll(filepath.Dir(markerPath), 0700)).To(Succeed())
				Expect(os.WriteFile(markerPath, []byte("user config"), 0600)).To(Succeed())

				first := configfiles.Apply(ctx, "Neovim", mappings, false, nil)
				Expect(first.Warnings).To(BeEmpty())
				Expect(destination).To(BeAnExistingFile())
				Expect(first.Files[0].Backup).To(BeEmpty())
				Expect(os.WriteFile(markerPath, []byte("user config"), 0600)).To(Succeed())

				second := configfiles.Apply(ctx, "Neovim", mappings, false, nil)
				Expect(second.Warnings).To(BeEmpty())
				Expect(second.Files[0].Backup).To(BeEmpty())
				data, err := os.ReadFile(markerPath)
				Expect(err).NotTo(HaveOccurred())
				Expect(string(data)).To(Equal("user config"))
				matches, err := filepath.Glob(destination + ".*.devex-bak")
				Expect(err).NotTo(HaveOccurred())
				Expect(matches).To(BeEmpty())
			},
			Entry("LazyVim bootstrap", "lua/config/lazy.lua"),
			Entry("lazy.nvim lockfile", "lazy-lock.json"),
		)

		It("backs up a user-modified destination before replacing it", func(ctx SpecContext) {
			marker := filepath.Join(home, ".config", "nvim", "lazy-lock.json")
			Expect(os.MkdirAll(filepath.Dir(marker), 0700)).To(Succeed())
			Expect(os.WriteFile(marker, []byte("{}"), 0600)).To(Succeed())
			configfiles.Apply(ctx, "Neovim", mappings, false, nil)
			Expect(os.WriteFile(destination, []byte("user edit"), 0600)).To(Succeed())

			result := configfiles.Apply(ctx, "Neovim", mappings, false, nil)

			Expect(result.Warnings).To(BeEmpty())
			backup := result.Files[0].Backup
			Expect(backup).To(HaveSuffix(".devex-bak"))
			data, err := os.ReadFile(backup)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(data)).To(Equal("user edit"))
			data, err = os.ReadFile(destination)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(data)).To(Equal("managed"))
		})
	})
})
