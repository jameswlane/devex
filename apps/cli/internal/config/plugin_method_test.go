package config_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/testhelper"
	"github.com/jameswlane/devex/apps/cli/internal/types"
)

var _ = Describe("plugin install method in the catalog (#757)", func() {
	var settings config.CrossPlatformSettings

	BeforeEach(func() {
		home := GinkgoT().TempDir()
		testhelper.SetProfileEnv(GinkgoT(), home)
		GinkgoT().Setenv("DEVEX_TEAM_CONFIG_DIR", "")
		settings = config.CrossPlatformSettings{HomeDir: home}
	})

	It("ships a mise entry that installs through the mise plugin", func() {
		Expect(config.WriteDefaultConfig(settings.GetConfigDir())).To(Succeed())
		loaded, err := config.LoadCrossPlatformSettings(settings.HomeDir)
		Expect(err).NotTo(HaveOccurred())

		var mise *types.CrossPlatformApp
		for _, app := range loaded.GetAllApps() {
			if app.Name == "mise" {
				mise = &app
				break
			}
		}
		Expect(mise).NotTo(BeNil())
		for _, cfg := range []types.OSConfig{mise.Linux, mise.MacOS} {
			Expect(cfg.InstallMethod).To(Equal(types.InstallMethodPlugin))
			Expect(cfg.InstallCommand).To(Equal("package-manager-mise ensure-installed"))
			Expect(cfg.InstalledBinary).To(Equal("mise"))
		}
		Expect(mise.Windows.InstallMethod).To(BeEmpty())
	})

	It("has every mise-installed entry reach the mise entry through its dependencies", func() {
		Expect(config.WriteDefaultConfig(settings.GetConfigDir())).To(Succeed())
		loaded, err := config.LoadCrossPlatformSettings(settings.HomeDir)
		Expect(err).NotTo(HaveOccurred())

		languages := map[string][]string{}
		for _, language := range loaded.ProgrammingLanguages {
			languages[strings.ToLower(language.Name)] = language.GetOSConfig().Dependencies
		}
		var reaches func([]string, map[string]bool) bool
		reaches = func(dependencies []string, seen map[string]bool) bool {
			for _, dependency := range dependencies {
				key := strings.ToLower(dependency)
				if key == "mise" {
					return true
				}
				if next, ok := languages[key]; ok && !seen[key] {
					seen[key] = true
					if reaches(next, seen) {
						return true
					}
				}
			}
			return false
		}

		var missing []string
		for _, app := range append(loaded.GetAllApps(), loaded.ProgrammingLanguages...) {
			for osName, cfg := range map[string]types.OSConfig{"linux": app.Linux, "macos": app.MacOS, "all_platforms": app.AllPlatforms} {
				for i, candidate := range append([]types.OSConfig{cfg}, cfg.Alternatives...) {
					if candidate.InstallMethod == "mise" && !reaches(candidate.Dependencies, map[string]bool{}) {
						missing = append(missing, fmt.Sprintf("%s %s #%d", app.Name, osName, i))
					}
				}
			}
		}
		Expect(missing).To(BeEmpty())
	})

	It("rejects a plugin install_command that doesn't name a plugin and a command", func() {
		path := filepath.Join(settings.GetUserConfigDir(), "applications", "tools", "broken.yaml")
		Expect(os.MkdirAll(filepath.Dir(path), 0o750)).To(Succeed())
		Expect(os.WriteFile(path, []byte("name: broken\ndescription: Broken\ncategory: Development Tools\ndefault: false\nlinux:\n  install_method: plugin\n  install_command: ../bin/sh\n"), 0o600)).To(Succeed())

		result, err := config.ValidateConfig(context.Background(), settings, true)
		Expect(err).NotTo(HaveOccurred())
		Expect(result.Valid).To(BeFalse())
		Expect(result.Diagnostics).To(ContainElement(And(
			HaveField("File", path),
			HaveField("Field", "linux.install_command"),
			HaveField("Message", ContainSubstring("must name a plugin and a command")),
		)))
	})
})
