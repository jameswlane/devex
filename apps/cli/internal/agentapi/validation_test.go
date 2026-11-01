package agentapi_test

import (
	"context"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"sort"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/jameswlane/devex/apps/cli/internal/agentapi"
	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/types"
)

// snapshot lists every path under root with its size and mode.
func snapshot(root string) []string {
	var entries []string
	_ = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		info, _ := d.Info()
		entries = append(entries, path+"|"+info.Mode().String()+"|"+string(rune(info.Size())))
		return nil
	})
	sort.Strings(entries)
	return entries
}

var _ = Describe("ValidateConfig", func() {
	var settings config.CrossPlatformSettings

	BeforeEach(func() {
		home := GinkgoT().TempDir()
		GinkgoT().Setenv("HOME", home)
		settings = config.CrossPlatformSettings{HomeDir: home}
	})

	It("is valid with an empty diagnostics list, not null, for a clean application set", func() {
		settings.ApplicationOverrides = []types.CrossPlatformApp{
			{Name: "git", Linux: types.OSConfig{InstallMethod: "apt", InstallCommand: "git"}},
		}
		result, err := agentapi.ValidateConfig(context.Background(), settings, true)
		Expect(err).NotTo(HaveOccurred())
		Expect(result.Valid).To(BeTrue())

		data, err := json.Marshal(result)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(data)).To(ContainSubstring(`"diagnostics":[]`))
	})

	It("reports unknown methods and cycles with stable categories", func() {
		settings.ApplicationOverrides = []types.CrossPlatformApp{
			{Name: "A", Linux: types.OSConfig{InstallMethod: "apt", InstallCommand: "a", Dependencies: []string{"B"}}},
			{Name: "B", Linux: types.OSConfig{InstallMethod: "apt", InstallCommand: "b", Dependencies: []string{"A"}}},
			{Name: "C", Linux: types.OSConfig{InstallMethod: "aptt", InstallCommand: "c"}},
		}
		result, err := agentapi.ValidateConfig(context.Background(), settings, false)
		Expect(err).NotTo(HaveOccurred())
		Expect(result.Valid).To(BeFalse())
		Expect(result.Errors).To(Equal(2))
		Expect(result.Diagnostics).To(ContainElements(
			And(HaveField("Category", agentapi.CategoryDependencyCycle), HaveField("Field", "linux.dependencies")),
			And(HaveField("Category", agentapi.CategoryInstallMethodUnknown), HaveField("File", "C"), HaveField("Field", "linux.install_method")),
		))
	})

	It("returns the context error when cancelled", func() {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err := agentapi.ValidateConfig(ctx, settings, false)
		Expect(err).To(MatchError(context.Canceled))
	})

	It("doesn't write anything", func() {
		Expect(config.WriteDefaultConfig(settings.GetConfigDir())).To(Succeed())
		before := snapshot(settings.HomeDir)

		_, err := agentapi.ValidateConfig(context.Background(), settings, true)
		Expect(err).NotTo(HaveOccurred())

		Expect(snapshot(settings.HomeDir)).To(Equal(before))
		_, statErr := os.Stat(filepath.Join(settings.HomeDir, ".local", "state"))
		Expect(os.IsNotExist(statErr)).To(BeTrue(), "validation must not create state")
	})
})
