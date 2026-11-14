//go:build !windows

package tui

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/log"
	"github.com/jameswlane/devex/apps/cli/internal/mocks"
	"github.com/jameswlane/devex/apps/cli/internal/types"
	"github.com/jameswlane/devex/packages/plugin-sdk/sdktest"
	"github.com/spf13/viper"
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
		recorder = sdktest.NewRecorder(GinkgoT(), "apt-get", "bash", "flatpak", "mise")
		recorder.Script("flatpak", []string{"info", "org.example.App"}, sdktest.Response{ExitCode: 1})
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

	It("does not add sources or batch packages for failed requirements", func() {
		GinkgoT().Setenv("DEVEX_MISSING_REQUIREMENT_TEST", "")
		app := aptApp("example", "example", types.AptSource{SourceName: "/etc/apt/sources.list.d/example.list", SourceRepo: "deb https://example.test stable main"})
		app.AllPlatforms.SystemRequirements.RequiredEnvVars = []string{"DEVEX_MISSING_REQUIREMENT_TEST"}
		Expect(installer.prepareAptRun(context.Background(), []types.CrossPlatformApp{app})).To(Succeed())
		Expect(recorder.Calls("apt-get")).To(BeEmpty())
		Expect(recorder.Calls("bash")).To(BeEmpty())
		Expect(recorder.Calls("sudo")).To(BeEmpty())
		Expect(installer.apt.sourcesAdded).To(BeEmpty())
		Expect(installer.apt.batchInstalled).To(BeEmpty())
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

	It("finishes APT preparation before scheduled installations", func() {
		signal := make(chan struct{})
		installer.executor = &preparedExecutor{CommandExecutor: NewDefaultCommandExecutor(), prepared: signal}
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

	It("retries an installed app whose lifecycle did not finish and records it finished", func() {
		installedPkgs["gh"] = true
		installedPkgs["bat"] = true
		tracked := mocks.NewMockRepository()
		installer.repo = tracked
		Expect(RecordLifecycle(tracked, "GitHub CLI", false)).To(Succeed())
		apps := []types.CrossPlatformApp{aptApp("GitHub CLI", "gh"), aptApp("bat", "bat")}

		Expect(installer.InstallApps(context.Background(), apps, config.CrossPlatformSettings{})).To(Succeed())

		installs := aptGetCalls("install")
		Expect(installs).To(HaveLen(1))
		Expect(installs[0].Args).To(Equal([]string{"install", "-y", "gh"}))
		Expect(tracked.GetApp("GitHub CLI")).NotTo(BeNil())
		Expect(LifecycleUnfinished(tracked, "GitHub CLI")).To(BeFalse())
		Expect(tracked.Get(lifecycleKey("GitHub CLI"))).To(Equal(lifecycleComplete))
		// bat was installed outside a DevEx lifecycle, so it is still skipped.
		_, err := tracked.GetApp("bat")
		Expect(err).To(HaveOccurred())
	})

	It("keeps the lifecycle unfinished when an optional post-install hook fails, so the next run retries it", func() {
		recorder.Fallback("bash", sdktest.Response{ExitCode: 1})
		tracked := mocks.NewMockRepository()
		installer.repo = tracked
		app := aptApp("GitHub CLI", "gh")
		app.AllPlatforms.PostInstall = []types.InstallCommand{{Shell: "gh extension install example/ext"}}

		Expect(installer.InstallApps(context.Background(), []types.CrossPlatformApp{app}, config.CrossPlatformSettings{})).To(MatchError(ContainSubstring("post-install step failed")))
		Expect(tracked.GetApp("GitHub CLI")).NotTo(BeNil())
		Expect(LifecycleUnfinished(tracked, "GitHub CLI")).To(BeTrue())

		// The packages are now installed, but the hook still has to run again.
		installedPkgs["gh"] = true
		hooksBefore := len(recorder.Calls("bash"))
		Expect(installer.InstallApps(context.Background(), []types.CrossPlatformApp{app}, config.CrossPlatformSettings{})).To(HaveOccurred())
		Expect(len(recorder.Calls("bash"))).To(BeNumerically(">", hooksBefore))
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
	It("batches several apt apps with post-install hooks and runs every hook", func() {
		a, b := aptApp("bat", "bat"), aptApp("fzf", "fzf")
		a.AllPlatforms.PostInstall = []types.InstallCommand{{Shell: "true"}}
		b.AllPlatforms.PostInstall = []types.InstallCommand{{Shell: "false"}}
		Expect(installer.InstallApps(context.Background(), []types.CrossPlatformApp{a, b}, config.CrossPlatformSettings{})).To(Succeed())
		Expect(aptGetCalls("install")).To(HaveLen(1))
		Expect(aptGetCalls("install")[0].Args).To(Equal([]string{"install", "-y", "bat", "fzf"}))
		Expect(recorder.Calls("bash")).To(HaveLen(2))
	})
	It("detects installed apt apps even when another app has a pre-install hook", func() {
		installedPkgs["bat"] = true
		a := aptApp("fzf", "fzf")
		a.AllPlatforms.PreInstall = []types.InstallCommand{{Shell: "true"}}
		Expect(installer.InstallApps(context.Background(), []types.CrossPlatformApp{aptApp("bat", "bat"), a}, config.CrossPlatformSettings{})).To(Succeed())
		Expect(repo.installedApps).To(Equal([]string{"fzf"}))
	})
	It("uses the configured list cache age", func() {
		old := viper.Get("apt-lists-max-age")
		DeferCleanup(func() { viper.Set("apt-lists-max-age", old) })
		viper.Set("apt-lists-max-age", "2h")
		path := GinkgoT().TempDir()
		Expect(os.WriteFile(filepath.Join(path, "mirror_Packages"), nil, 0600)).To(Succeed())
		age := time.Now().Add(-time.Hour)
		Expect(os.Chtimes(path, age, age)).To(Succeed())
		installer.apt.listsPath = path
		Expect(installer.ensureAptLists(context.Background())).To(Succeed())
		Expect(aptGetCalls("update")).To(BeEmpty())
	})
	It("batches flatpak apps and preserves per-app recording", func() {
		a, b := aptApp("one", "org.test.One"), aptApp("two", "org.test.Two")
		a.AllPlatforms.InstallMethod = "flatpak"
		b.AllPlatforms.InstallMethod = "flatpak"
		recorder.Script("flatpak", []string{"info", "org.test.One"}, sdktest.Response{ExitCode: 1})
		recorder.Script("flatpak", []string{"info", "org.test.Two"}, sdktest.Response{ExitCode: 1})
		Expect(installer.InstallApps(context.Background(), []types.CrossPlatformApp{a, b}, config.CrossPlatformSettings{})).To(Succeed())
		var installs []sdktest.Call
		for _, call := range recorder.Calls("flatpak") {
			if call.Args[0] == "install" {
				installs = append(installs, call)
			}
		}
		Expect(installs).To(Equal([]sdktest.Call{{Args: []string{"install", "-y", "org.test.One", "org.test.Two"}}}))
		Expect(repo.installedApps).To(ConsistOf("one", "two"))
	})

	It("falls back to individual flatpak installs after a failed batch", func() {
		a, b := aptApp("one", "org.test.One"), aptApp("two", "org.test.Two")
		a.AllPlatforms.InstallMethod = "flatpak"
		b.AllPlatforms.InstallMethod = "flatpak"
		recorder.Script("flatpak", []string{"info", "org.test.One"}, sdktest.Response{ExitCode: 1})
		recorder.Script("flatpak", []string{"info", "org.test.Two"}, sdktest.Response{ExitCode: 1})
		recorder.Script("flatpak", []string{"install", "-y", "org.test.One", "org.test.Two"}, sdktest.Response{ExitCode: 1})
		Expect(installer.InstallApps(context.Background(), []types.CrossPlatformApp{a, b}, config.CrossPlatformSettings{})).To(Succeed())
		var installs []sdktest.Call
		for _, call := range recorder.Calls("flatpak") {
			if call.Args[0] == "install" {
				installs = append(installs, call)
			}
		}
		Expect(installs).To(HaveLen(3))
		Expect(repo.installedApps).To(ConsistOf("one", "two"))
	})
	It("skips installed mise and flatpak apps before running hooks", func() {
		a, b := aptApp("node", "node@22"), aptApp("desktop", "org.test.App")
		a.AllPlatforms.InstallMethod = "mise"
		b.AllPlatforms.InstallMethod = "flatpak"
		a.AllPlatforms.PostInstall = []types.InstallCommand{{Shell: "true"}}
		Expect(installer.InstallApps(context.Background(), []types.CrossPlatformApp{a, b}, config.CrossPlatformSettings{})).To(Succeed())
		Expect(repo.installedApps).To(BeEmpty())
		Expect(recorder.Calls("bash")).To(BeEmpty())
	})
	It("batches mise activation while preserving the global-use command", func() {
		a, b := aptApp("node", "node@22"), aptApp("python", "python@3.13")
		a.AllPlatforms.InstallMethod = "mise"
		b.AllPlatforms.InstallMethod = "mise"
		recorder.Script("mise", []string{"where", "node@22"}, sdktest.Response{ExitCode: 1})
		recorder.Script("mise", []string{"where", "python@3.13"}, sdktest.Response{ExitCode: 1})
		Expect(installer.InstallApps(context.Background(), []types.CrossPlatformApp{a, b}, config.CrossPlatformSettings{})).To(Succeed())
		Expect(recorder.Calls("bash")).To(HaveLen(1))
		Expect(recorder.Calls("bash")[0].Args[1]).To(ContainSubstring("mise use --global node@22 python@3.13"))
	})

	It("does not rewrite an unchanged source or invalidate fresh lists", func() {
		source := filepath.Join(GinkgoT().TempDir(), "example.list")
		Expect(os.WriteFile(source, []byte("deb https://example.test stable main\n"), 0600)).To(Succeed())
		installer.apt.listsPath = GinkgoT().TempDir()
		Expect(os.WriteFile(filepath.Join(installer.apt.listsPath, "mirror_Packages"), nil, 0600)).To(Succeed())
		Expect(installer.addAptSources(context.Background(), types.OSConfig{AptSources: []types.AptSource{{SourceName: source, SourceRepo: "deb https://example.test stable main"}}})).To(Succeed())
		Expect(installer.ensureAptLists(context.Background())).To(Succeed())
		Expect(installer.apt.sourcesAdded).To(BeEmpty())
		Expect(aptGetCalls("update")).To(BeEmpty())
	})
	It("refreshes populated lists when cache reuse is disabled", func() {
		old := viper.Get("apt-lists-max-age")
		DeferCleanup(func() { viper.Set("apt-lists-max-age", old) })
		viper.Set("apt-lists-max-age", "0")
		installer.apt.listsPath = GinkgoT().TempDir()
		Expect(os.WriteFile(filepath.Join(installer.apt.listsPath, "mirror_Packages"), nil, 0600)).To(Succeed())
		Expect(installer.ensureAptLists(context.Background())).To(Succeed())
		Expect(aptGetCalls("update")).To(HaveLen(1))
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

// The executor checks observable preparation order without timing assumptions.
type preparedExecutor struct {
	CommandExecutor
	prepared chan struct{}
}

func (e *preparedExecutor) ExecuteCommand(ctx context.Context, command string) (*exec.Cmd, error) {
	if strings.Contains(command, "apt-get update") {
		close(e.prepared)
	}
	if strings.HasPrefix(command, "flatpak install") {
		select {
		case <-e.prepared:
		default:
			return nil, fmt.Errorf("app started before APT preparation")
		}
	}
	return e.CommandExecutor.ExecuteCommand(ctx, command)
}
