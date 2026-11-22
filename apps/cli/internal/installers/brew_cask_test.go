package installers

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/jameswlane/devex/apps/cli/internal/testhelper"
	"github.com/jameswlane/devex/apps/cli/internal/types"
	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
	"github.com/jameswlane/devex/packages/plugin-sdk/sdktest"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v3"
)

var _ = Describe("Homebrew Cask dispatch", func() {
	It("carries shipped Cask selections through the real Brew plugin to native argv", func() {
		if sdk.NewPrivilegeResolver().Elevated {
			// Brew runs user-scoped operations; an elevated process (such as
			// GitHub Actions windows-latest) cannot de-elevate by security
			// policy. Linux, macOS, and non-elevated Windows runs cover this.
			Skip("the Brew plugin subprocess requires a non-elevated process for user-scope operations")
		}
		path := testhelper.Executable(GinkgoT().TempDir(), "brew-plugin")
		build := exec.CommandContext(context.Background(), "go", "build", "-o", path, ".")
		build.Dir = "../../../../packages/package-manager-brew"
		build.Env = append(os.Environ(), "GOFLAGS="+os.Getenv("GOFLAGS")+" -modcacherw")
		output, err := build.CombinedOutput()
		Expect(err).NotTo(HaveOccurred(), string(output))

		recorder := sdktest.NewRecorder(GinkgoT(), "brew")
		installer := &PluginBasedInstaller{method: "brew", pluginBootstrap: snapProcessExecutor{path: path}, ctx: context.Background()}
		for _, name := range []string{"ghostty", "docker"} {
			data, err := os.ReadFile(filepath.Join("../../config/applications/development", name+".yaml"))
			Expect(err).NotTo(HaveOccurred())
			var catalog types.CrossPlatformApp
			Expect(yaml.Unmarshal(data, &catalog)).To(Succeed())
			// Select the macOS block on any host.
			app := types.CrossPlatformApp{Name: catalog.Name, AllPlatforms: catalog.MacOS}
			legacy := app.ToLegacyAppConfig()
			if len(legacy.Packages) > 0 || len(legacy.Options) > 0 {
				Expect(installer.InstallPackages(context.Background(), legacy.InstallCommand, legacy.Packages, legacy.Options)).To(Succeed())
			} else {
				Expect(installer.Install(legacy.InstallCommand, nil)).To(Succeed())
			}
			installed, err := installer.IsInstalled(legacy.PackageQueryCommand())
			Expect(err).NotTo(HaveOccurred())
			Expect(installed).To(BeTrue())
			Expect(installer.Uninstall(legacy.WithKindSelector(legacy.UninstallCommand), nil)).To(Succeed())
		}
		Expect(recorder.Calls("brew")).To(Equal([]sdktest.Call{
			{Args: []string{"install", "--cask", "ghostty"}},
			{Args: []string{"list", "--versions", "--cask", "ghostty"}},
			{Args: []string{"uninstall", "--cask", "ghostty"}},
			{Args: []string{"install", "--cask", "docker"}},
			{Args: []string{"list", "--versions", "--cask", "docker"}},
			{Args: []string{"uninstall", "--cask", "docker"}},
		}))
		Expect(recorder.Calls("sudo")).To(BeEmpty())
	})
})
