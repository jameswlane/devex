//go:build !windows

package tui

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/types"
	"github.com/jameswlane/devex/packages/plugin-sdk/sdktest"
)

func aptApp(name, packages string, sources ...types.AptSource) types.CrossPlatformApp {
	return types.CrossPlatformApp{
		Name:        name,
		Description: name + " test app",
		Category:    "Development",
		AllPlatforms: types.OSConfig{
			InstallMethod:    "apt",
			InstallCommand:   packages,
			UninstallCommand: packages,
			AptSources:       sources,
		},
	}
}

var _ = Describe("APT batching", func() {
	var (
		recorder         *sdktest.Recorder
		repo             *MockRepository
		installer        *StreamingInstaller
		installedPkgs    map[string]bool
		originalQuery    func(context.Context, []string) map[string]bool
		aptGetSubcommand = func(call sdktest.Call) string { return call.Args[0] }
	)

	BeforeEach(func() {
		// Bash stands in for a shell pre-install step; the recorder isolates PATH.
		recorder = sdktest.NewRecorder(GinkgoT(), "apt-get", "bash")
		repo = &MockRepository{}
		installer = &StreamingInstaller{
			ctx:      context.Background(),
			config:   DefaultInstallerConfig(),
			executor: NewDefaultCommandExecutor(),
			repo:     repo,
		}
		installedPkgs = map[string]bool{}
		originalQuery = queryInstalledDebPackages
		queryInstalledDebPackages = func(_ context.Context, packages []string) map[string]bool {
			result := map[string]bool{}
			for _, pkg := range packages {
				if installedPkgs[pkg] {
					result[pkg] = true
				}
			}
			return result
		}
		DeferCleanup(func() { queryInstalledDebPackages = originalQuery })
	})

	aptGetCalls := func(subcommand string) []sdktest.Call {
		var calls []sdktest.Call
		for _, call := range recorder.Calls("apt-get") {
			if aptGetSubcommand(call) == subcommand {
				calls = append(calls, call)
			}
		}
		return calls
	}

	It("updates package lists once and installs every apt package in one transaction", func() {
		apps := []types.CrossPlatformApp{aptApp("bat", "bat"), aptApp("fzf", "fzf"), aptApp("databases", "sqlite3 libsqlite3-0")}

		Expect(installer.InstallApps(context.Background(), apps, config.CrossPlatformSettings{})).To(Succeed())

		Expect(aptGetCalls("update")).To(HaveLen(1))
		installs := aptGetCalls("install")
		Expect(installs).To(HaveLen(1))
		Expect(installs[0].Args).To(Equal([]string{"install", "-y", "bat", "fzf", "sqlite3", "libsqlite3-0"}))
		Expect(repo.installedApps).To(ConsistOf("bat", "fzf", "databases"))
	})

	It("skips apps whose packages are already installed and does not record them", func() {
		installedPkgs["gh"] = true
		apps := []types.CrossPlatformApp{aptApp("GitHub CLI", "gh"), aptApp("bat", "bat")}

		Expect(installer.InstallApps(context.Background(), apps, config.CrossPlatformSettings{})).To(Succeed())

		installs := aptGetCalls("install")
		Expect(installs).To(HaveLen(1))
		Expect(installs[0].Args).To(Equal([]string{"install", "-y", "bat"}))
		Expect(repo.installedApps).To(ConsistOf("bat"))
	})

	It("does not touch apt at all when every app is already installed", func() {
		installedPkgs["gh"] = true
		installedPkgs["bat"] = true
		apps := []types.CrossPlatformApp{aptApp("GitHub CLI", "gh"), aptApp("bat", "bat")}

		Expect(installer.InstallApps(context.Background(), apps, config.CrossPlatformSettings{})).To(Succeed())

		Expect(recorder.Calls("apt-get")).To(BeEmpty())
		Expect(repo.installedApps).To(BeEmpty())
	})

	It("falls back to one install per app when the batch fails, without updating again", func() {
		recorder.Script("apt-get", []string{"install", "-y", "bat", "tldr", "fzf"}, sdktest.Response{ExitCode: 100, Stderr: "E: Unable to locate package tldr\n"})
		recorder.Script("apt-get", []string{"install", "-y", "tldr"}, sdktest.Response{ExitCode: 100, Stderr: "E: Unable to locate package tldr\n"})
		apps := []types.CrossPlatformApp{aptApp("bat", "bat"), aptApp("tldr", "tldr"), aptApp("fzf", "fzf")}

		err := installer.InstallApps(context.Background(), apps, config.CrossPlatformSettings{})

		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("tldr"))
		Expect(aptGetCalls("update")).To(HaveLen(1))
		calls := aptGetCalls("install")
		perApp := make([][]string, 0, len(calls))
		for _, call := range calls {
			perApp = append(perApp, call.Args)
		}
		Expect(perApp).To(Equal([][]string{
			{"install", "-y", "bat", "tldr", "fzf"},
			{"install", "-y", "bat"},
			{"install", "-y", "tldr"},
			{"install", "-y", "fzf"},
		}))
		Expect(repo.installedApps).To(ConsistOf("bat", "fzf"))
	})

	It("keeps apps with pre-install steps out of the batch", func() {
		withPreInstall := aptApp("docker", "docker-ce")
		withPreInstall.AllPlatforms.PreInstall = []types.InstallCommand{{Shell: "true"}}
		apps := []types.CrossPlatformApp{aptApp("bat", "bat"), withPreInstall}

		Expect(installer.InstallApps(context.Background(), apps, config.CrossPlatformSettings{})).To(Succeed())

		calls := aptGetCalls("install")
		installs := make([][]string, 0, len(calls))
		for _, call := range calls {
			installs = append(installs, call.Args)
		}
		Expect(installs).To(Equal([][]string{{"install", "-y", "bat"}, {"install", "-y", "docker-ce"}}))
		Expect(aptGetCalls("update")).To(HaveLen(1))
		Expect(recorder.Calls("bash")).To(Equal([]sdktest.Call{{Args: []string{"-c", "true"}}}))
	})

	It("leaves install commands that are not plain package names to the per-app path", func() {
		Expect(aptPackages(types.OSConfig{InstallCommand: "bat fzf"})).To(Equal([]string{"bat", "fzf"}))
		Expect(aptPackages(types.OSConfig{InstallCommand: "--no-install-recommends bat"})).To(BeNil())
		Expect(aptPackages(types.OSConfig{InstallCommand: "bat; rm -rf /"})).To(BeNil())
	})
})
