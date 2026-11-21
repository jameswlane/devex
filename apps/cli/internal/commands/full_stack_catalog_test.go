package commands_test

import (
	"context"
	"io/fs"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	shipconfig "github.com/jameswlane/devex/apps/cli/config"
	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/security"
	"github.com/jameswlane/devex/apps/cli/internal/templates"
	"github.com/jameswlane/devex/apps/cli/internal/types"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v3"
)

var _ = Describe("Full-stack catalog applications", func() {
	var (
		settings config.CrossPlatformSettings
		byName   map[string]types.CrossPlatformApp
	)

	BeforeEach(func() {
		DeferCleanup(types.PinCatalogOSForTesting("linux"))
		settings = config.CrossPlatformSettings{}
		byName = map[string]types.CrossPlatformApp{}
		Expect(fs.WalkDir(shipconfig.Files, "applications", func(path string, entry fs.DirEntry, err error) error {
			if err != nil || entry.IsDir() || filepath.Ext(path) != ".yaml" {
				return err
			}
			data, readErr := fs.ReadFile(shipconfig.Files, path)
			if readErr != nil {
				return readErr
			}
			var app types.CrossPlatformApp
			if unmarshalErr := yaml.Unmarshal(data, &app); unmarshalErr != nil {
				return unmarshalErr
			}
			settings.ApplicationOverrides = append(settings.ApplicationOverrides, app)
			byName[strings.ToLower(app.Name)] = app
			return nil
		})).To(Succeed())
	})

	DescribeTable("resolves each full-stack editor and browser from the embedded catalog as an official Debian/Ubuntu APT package",
		func(ref, pkg, binary, repoSubstring string) {
			resolved, err := settings.GetApplicationByName(ref)
			Expect(err).NotTo(HaveOccurred())
			Expect(resolved.Name).To(Equal(ref))
			Expect(resolved.InstallMethod).To(Equal("apt"))
			Expect(resolved.InstallCommand).To(Equal(pkg))
			Expect(resolved.UninstallCommand).To(Equal(pkg))

			app, ok := byName[strings.ToLower(ref)]
			Expect(ok).To(BeTrue(), "embedded catalog must include %s", ref)
			Expect(app.MacOS.InstallMethod).To(BeEmpty())
			Expect(app.Windows.InstallMethod).To(BeEmpty())

			requiredOS := make([]string, 0, len(app.Linux.PlatformRequirements))
			for _, req := range app.Linux.PlatformRequirements {
				requiredOS = append(requiredOS, req.OS)
			}
			Expect(requiredOS).To(ContainElements("debian", "ubuntu"))

			validator := security.NewCommandValidatorWithConfig(security.SecurityLevelModerate, nil)
			for _, distro := range []string{"debian", "ubuntu"} {
				cfg := app.LinuxConfigForDistribution(distro)
				Expect(cfg.InstallMethod).To(Equal("apt"), distro)
				Expect(cfg.InstallCommand).To(Equal(pkg), distro)
				Expect(cfg.InstalledBinary).To(Equal(binary), distro)
				Expect(cfg.UninstallCommand).To(Equal(pkg), distro)
				Expect(cfg.OfficialSupport).To(BeTrue(), distro)
				Expect(cfg.PreInstall).To(HaveLen(1), distro)
				hook := cfg.PreInstall[0]
				Expect(hook.Scope).To(Equal("system"), distro)
				Expect(hook.Shell).To(ContainSubstring(repoSubstring), distro)
				Expect(hook.Shell).To(ContainSubstring("apt-get update"), distro)
				Expect(hook.Shell).NotTo(ContainSubstring("sudo"), distro)
				Expect(validator.ValidateCommandForApp(hook.Shell, app.Name)).To(Succeed())

				if runtime.GOOS != "windows" {
					cmd := exec.CommandContext(context.Background(), "bash", "-n")
					cmd.Stdin = strings.NewReader(hook.Shell)
					output, synErr := cmd.CombinedOutput()
					Expect(synErr).NotTo(HaveOccurred(), "%s hook syntax: %s", ref, output)
				}
			}
		},
		Entry("Visual Studio Code", "Visual Studio Code", "code", "code", "https://packages.microsoft.com/repos/code"),
		Entry("Google Chrome", "Google Chrome", "google-chrome-stable", "google-chrome", "https://dl.google.com/linux/chrome/deb/"),
		Entry("Firefox Developer Edition", "Firefox Developer Edition", "firefox-devedition", "firefox-devedition", "https://packages.mozilla.org/apt"),
	)

	It("rejects non-amd64 architectures before running apt for Google Chrome", func() {
		if runtime.GOOS == "windows" {
			Skip("Linux apt pre-install hook execution requires bash")
		}
		app, ok := byName["google chrome"]
		Expect(ok).To(BeTrue())
		script := "dpkg() { echo arm64; }\napt-get() { echo 'apt-get-called'; exit 99; }\n" + app.Linux.PreInstall[0].Shell
		cmd := exec.CommandContext(context.Background(), "bash", "-c", script)
		output, err := cmd.CombinedOutput()
		Expect(err).To(HaveOccurred())
		Expect(string(output)).To(ContainSubstring("only available for amd64"))
		Expect(string(output)).NotTo(ContainSubstring("apt-get-called"))
	})

	It("resolves every full-stack template step without snap on Debian and Ubuntu", func() {
		manager := templates.NewTemplateManager(GinkgoT().TempDir())
		spec, err := manager.GetTemplateSpec("full-stack")
		Expect(err).NotTo(HaveOccurred())

		steps := spec.Lifecycle[templates.LifecycleInstall]
		Expect(steps).NotTo(BeEmpty())

		resolvedRefs := make([]string, 0, len(steps))
		for _, step := range steps {
			if step.App == nil {
				continue
			}
			Expect(step.App.Installer).NotTo(Equal("snap"), "step %s must not use snap", step.ID)
			if step.App.App != nil {
				Expect(step.App.App.InstallMethod).NotTo(Equal("snap"), "step %s must not use snap", step.ID)
			}
			if step.App.Ref != "" {
				resolved, resolveErr := settings.GetApplicationByName(step.App.Ref)
				Expect(resolveErr).NotTo(HaveOccurred(), "ref %s must resolve from embedded catalog", step.App.Ref)
				Expect(resolved.InstallMethod).NotTo(Equal("snap"), "ref %s must not use snap", step.App.Ref)
				app, ok := byName[strings.ToLower(resolved.Name)]
				Expect(ok).To(BeTrue())
				for _, distro := range []string{"debian", "ubuntu"} {
					Expect(app.LinuxConfigForDistribution(distro).InstallMethod).NotTo(Equal("snap"), "%s on %s", step.App.Ref, distro)
				}
				resolvedRefs = append(resolvedRefs, step.App.Ref)
			}
		}
		Expect(resolvedRefs).To(ContainElements("docker", "Visual Studio Code", "Google Chrome", "Firefox Developer Edition"))
	})
})
