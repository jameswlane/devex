//go:build !windows

package tui

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/log"
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
		recorder = sdktest.NewRecorder(GinkgoT(), "apt-get", "bash", "flatpak")
		repo = &MockRepository{}
		installer = &StreamingInstaller{
			ctx:      context.Background(),
			config:   DefaultInstallerConfig(),
			executor: NewDefaultCommandExecutor(),
			repo:     repo,
		}
		installer.apt.listsPath = filepath.Join(GinkgoT().TempDir(), "missing-lists")
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

	It("runs the independent lane during a blocked APT update", func() {
		signal := make(chan struct{})
		installer.executor = &overlappingExecutor{CommandExecutor: NewDefaultCommandExecutor(), independent: signal}
		app := aptApp("desktop", "org.example.App")
		app.AllPlatforms.InstallMethod = "flatpak"
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		Expect(installer.InstallApps(ctx, []types.CrossPlatformApp{aptApp("bat", "bat"), app}, config.CrossPlatformSettings{})).To(Succeed())
		Expect(aptGetCalls("update")).To(HaveLen(1))
		Expect(aptGetCalls("install")).To(HaveLen(1))
		Expect(repo.installedApps).To(ConsistOf("bat", "desktop"))
	})

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

	It("logs slow-mirror guidance once when the first update exceeds twenty seconds", func() {
		Expect(log.InitFileLogger(false)).To(Succeed())
		DeferCleanup(func() { Expect(log.Close()).To(Succeed()); log.InitTestLogger() })
		calls := 0
		start := time.Now()
		installer.apt.now = func() time.Time {
			calls++
			if calls == 1 {
				return start
			}
			return start.Add(21 * time.Second)
		}
		Expect(installer.ensureAptLists(context.Background())).To(Succeed())
		Expect(installer.ensureAptLists(context.Background())).To(Succeed())
		content, err := os.ReadFile(log.GetLogFile())
		Expect(err).NotTo(HaveOccurred())
		Expect(strings.Count(string(content), "Acquire::ForceIPv4=true")).To(Equal(1))
		Expect(string(content)).To(ContainSubstring("closer mirror"))
		Expect(aptGetCalls("update")).To(HaveLen(1))
	})

	It("skips fresh lists, but refreshes when sources changed", func() {
		installer.apt.listsPath = GinkgoT().TempDir()
		Expect(os.WriteFile(filepath.Join(installer.apt.listsPath, "mirror_Packages"), nil, 0600)).To(Succeed())
		Expect(installer.ensureAptLists(context.Background())).To(Succeed())
		Expect(aptGetCalls("update")).To(BeEmpty())
		installer.apt = aptRunState{listsPath: installer.apt.listsPath, sourcesAdded: map[string]bool{"new": true}}
		Expect(installer.ensureAptLists(context.Background())).To(Succeed())
		Expect(aptGetCalls("update")).To(HaveLen(1))
	})
	It("refreshes an empty lists directory even when it is new", func() {
		installer.apt.listsPath = GinkgoT().TempDir()
		Expect(installer.ensureAptLists(context.Background())).To(Succeed())
		Expect(aptGetCalls("update")).To(HaveLen(1))
	})

	It("refreshes old lists and never repeats a failed update", func() {
		path := GinkgoT().TempDir()
		old := time.Now().Add(-time.Hour)
		Expect(os.Chtimes(path, old, old)).To(Succeed())
		installer.apt.listsPath = path
		recorder.Script("apt-get", []string{"update"}, sdktest.Response{ExitCode: 100})
		Expect(installer.ensureAptLists(context.Background())).NotTo(Succeed())
		Expect(installer.ensureAptLists(context.Background())).NotTo(Succeed())
		Expect(aptGetCalls("update")).To(HaveLen(1))
	})
	It("runs post-install hooks after the batch transaction", func() {
		app := aptApp("bat", "bat")
		app.AllPlatforms.PostInstall = []types.InstallCommand{{Shell: "true"}}
		Expect(installer.InstallApps(context.Background(), []types.CrossPlatformApp{app}, config.CrossPlatformSettings{})).To(Succeed())
		Expect(aptGetCalls("install")).To(HaveLen(1))
		Expect(recorder.Calls("bash")).To(HaveLen(1))
		Expect(repo.installedApps).To(Equal([]string{"bat"}))
	})
	It("uses a filesystem check for existing key files", func() {
		path := filepath.Join(GinkgoT().TempDir(), "key.gpg")
		Expect(os.WriteFile(path, []byte("existing"), 0600)).To(Succeed())
		Expect(installer.addModernGPGKey(context.Background(), "https://example.org/key", path, true)).To(Succeed())
		Expect(recorder.Calls("apt-get")).To(BeEmpty())
	})

	It("leaves install commands that are not plain package names to the per-app path", func() {
		Expect(aptPackages(types.OSConfig{Packages: []string{"bat", "fzf"}})).To(Equal([]string{"bat", "fzf"}))
		Expect(aptPackages(types.OSConfig{Packages: []string{"bat"}, Options: []string{"--unknown"}})).To(BeNil())
		Expect(aptPackages(types.OSConfig{InstallCommand: "bat fzf"})).To(Equal([]string{"bat", "fzf"}))
		Expect(aptPackages(types.OSConfig{InstallCommand: "--no-install-recommends bat"})).To(BeNil())
		Expect(aptPackages(types.OSConfig{InstallCommand: "bat; rm -rf /"})).To(BeNil())
	})
})

// A handshake makes serial execution fail without relying on elapsed timings.
type overlappingExecutor struct {
	CommandExecutor
	independent chan struct{}
}

func (e *overlappingExecutor) ExecuteCommand(ctx context.Context, command string) (*exec.Cmd, error) {
	if strings.Contains(command, "apt-get update") {
		select {
		case <-e.independent:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	if strings.HasPrefix(command, "flatpak install") {
		close(e.independent)
	}
	return e.CommandExecutor.ExecuteCommand(ctx, command)
}
