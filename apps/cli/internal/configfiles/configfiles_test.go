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
})
