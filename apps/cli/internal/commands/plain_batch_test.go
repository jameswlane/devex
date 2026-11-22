//go:build !windows

package commands

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"

	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/installers"
	"github.com/jameswlane/devex/apps/cli/internal/mocks"
	"github.com/jameswlane/devex/apps/cli/internal/security"
	"github.com/jameswlane/devex/apps/cli/internal/state"
	"github.com/jameswlane/devex/apps/cli/internal/tui"
	"github.com/jameswlane/devex/apps/cli/internal/types"
	"github.com/jameswlane/devex/apps/cli/internal/utils"
	"github.com/jameswlane/devex/packages/plugin-sdk/sdktest"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/viper"
)

var _ = Describe("Plain install package batching", func() {
	aptApp := func(name, packages string) types.CrossPlatformApp {
		return types.CrossPlatformApp{Name: name, Description: name + " test app", Category: "Development", AllPlatforms: types.OSConfig{InstallMethod: "apt", InstallCommand: packages, UninstallCommand: packages}}
	}

	It("runs one apt-get update and one batched install, then each app's own lifecycle", func() {
		recorder := sdktest.NewRecorder(GinkgoT(), "apt-get", "dpkg-query")
		recorder.Fallback("dpkg-query", sdktest.Response{Stdout: "gh\tinstall ok installed\n"})
		viper.Set("apt-lists-max-age", "0s")
		DeferCleanup(func() { viper.Set("apt-lists-max-age", nil) })
		preparePackageBatch = tui.PreparePackageBatch
		// Test mode reports the apt installer plugin as available.
		installers.EnableTestMode()
		DeferCleanup(installers.DisableTestMode)
		oldPrepare, oldInstall, oldQuery := prepareApplicationInstallers, installApplication, nativeApplicationInstalled
		DeferCleanup(func() {
			prepareApplicationInstallers, installApplication, nativeApplicationInstalled = oldPrepare, oldInstall, oldQuery
		})
		prepareApplicationInstallers = func(context.Context, pluginBootstrapProvider) error { return nil }
		// After the batch, native state reports every package as present.
		nativeApplicationInstalled = func(context.Context, types.CrossPlatformApp) (bool, error) { return true, nil }
		var mu sync.Mutex
		batched := map[string]bool{}
		installApplication = func(ctx context.Context, app types.CrossPlatformApp, _ config.CrossPlatformSettings, _ types.Repository) error {
			mu.Lock()
			defer mu.Unlock()
			batched[app.Name] = installers.PackagesInstalled(ctx, app.Name)
			return nil
		}

		repo := mocks.NewMockRepository()
		// A stale tracking row must not turn a batch-installed app into a skip.
		Expect(repo.AddApp("bat")).To(Succeed())
		var output bytes.Buffer
		apps := []types.CrossPlatformApp{aptApp("bat", "bat"), aptApp("fzf", "fzf"), aptApp("GitHub CLI", "gh"), aptApp("databases", "sqlite3 libsqlite3-0")}
		Expect(runPlainInstall(context.Background(), &output, apps, repo, config.CrossPlatformSettings{}, nil)).To(Succeed())

		var updates, installs []sdktest.Call
		for _, call := range recorder.Calls("apt-get") {
			switch call.Args[0] {
			case "update":
				updates = append(updates, call)
			case "install":
				installs = append(installs, call)
			}
		}
		Expect(updates).To(HaveLen(1))
		Expect(installs).To(HaveLen(1))
		Expect(installs[0].Args).To(Equal([]string{"install", "-y", "bat", "fzf", "sqlite3", "libsqlite3-0"}))
		Expect(batched).To(Equal(map[string]bool{"bat": true, "fzf": true, "databases": true}))
		Expect(output.String()).To(Equal("installing bat...\ninstalled bat\ninstalling fzf...\ninstalled fzf\nskipped GitHub CLI (already installed)\ninstalling databases...\ninstalled databases\ninstalled 3 of 4 applications\n"))
	})

	usePlainBatch := func(installed string, commands ...string) *sdktest.Recorder {
		recorder := sdktest.NewRecorder(GinkgoT(), append([]string{"apt-get", "dpkg-query"}, commands...)...)
		recorder.Fallback("dpkg-query", sdktest.Response{Stdout: installed})
		viper.Set("apt-lists-max-age", "0s")
		DeferCleanup(func() { viper.Set("apt-lists-max-age", nil) })
		preparePackageBatch = tui.PreparePackageBatch
		prepareAptSources = tui.PrepareAptSources
		installers.EnableTestMode()
		DeferCleanup(installers.DisableTestMode)
		oldPrepare, oldInstall, oldQuery := prepareApplicationInstallers, installApplication, nativeApplicationInstalled
		DeferCleanup(func() {
			prepareApplicationInstallers, installApplication, nativeApplicationInstalled = oldPrepare, oldInstall, oldQuery
		})
		prepareApplicationInstallers = func(context.Context, pluginBootstrapProvider) error { return nil }
		nativeApplicationInstalled = func(context.Context, types.CrossPlatformApp) (bool, error) { return true, nil }
		return recorder
	}

	It("fails an app whose APT key download failed instead of installing the distribution's package", func() {
		// Shell steps run through bash; only the key download fails.
		recorder := usePlainBatch("", "bash")
		dir := GinkgoT().TempDir()
		keyURL := "https://cli.github.com/packages/githubcli-archive-keyring.gpg"
		keyName := filepath.Join(dir, "githubcli-archive-keyring.gpg")
		recorder.Script("bash", []string{"-c", utils.NonInteractiveSudoShell("curl -fsSL " + keyURL + " | sudo -n gpg --dearmor -o " + keyName)}, sdktest.Response{ExitCode: 22, Stderr: "curl: (22) The requested URL returned error: 404\n"})
		gh := aptApp("GitHub CLI", "gh")
		gh.AllPlatforms.AptSources = []types.AptSource{{
			KeySource:      keyURL,
			KeyName:        keyName,
			RequireDearmor: true,
			SourceName:     filepath.Join(dir, "github-cli.list"),
			SourceRepo:     "deb https://cli.github.com/packages stable main",
		}}
		// A later app's source is still written although an earlier one failed.
		other := aptApp("other", "other")
		other.AllPlatforms.AptSources = []types.AptSource{{SourceName: filepath.Join(dir, "other.list"), SourceRepo: "deb https://example.test stable main"}}
		var ran []string
		installApplication = func(_ context.Context, app types.CrossPlatformApp, _ config.CrossPlatformSettings, _ types.Repository) error {
			ran = append(ran, app.Name)
			return nil
		}

		var output bytes.Buffer
		err := runPlainInstall(context.Background(), &output, []types.CrossPlatformApp{gh, other}, mocks.NewMockRepository(), config.CrossPlatformSettings{}, nil)

		Expect(err).To(MatchError(ContainSubstring("GitHub CLI")))
		Expect(ran).To(Equal([]string{"other"}))
		Expect(output.String()).To(ContainSubstring("failed GitHub CLI: prepare APT sources for GitHub CLI: failed to add GPG key"))
		Expect(output.String()).To(ContainSubstring("installed 1 of 2 applications"))
		calls := recorder.Calls("bash")
		shell := make([]string, 0, len(calls))
		for _, call := range calls {
			shell = append(shell, call.Args[1])
		}
		Expect(shell).NotTo(ContainElement(ContainSubstring("github-cli.list")))
		Expect(shell).To(ContainElement(ContainSubstring("tee " + filepath.Join(dir, "other.list"))))
	})

	// An app with options never enters the batch, so the batch never writes its sources.
	unbatchedSourceApp := func(dir string) types.CrossPlatformApp {
		gh := aptApp("GitHub CLI", "")
		gh.AllPlatforms.Packages = []string{"gh"}
		gh.AllPlatforms.Options = []string{"--no-install-recommends"}
		gh.AllPlatforms.AptSources = []types.AptSource{{SourceName: filepath.Join(dir, "github-cli.list"), SourceRepo: "deb https://cli.github.com/packages stable main"}}
		return gh
	}
	shellCommands := func(recorder *sdktest.Recorder) []string {
		calls := recorder.Calls("bash")
		shell := make([]string, 0, len(calls))
		for _, call := range calls {
			shell = append(shell, call.Args[1])
		}
		return shell
	}

	It("writes the APT sources of an app the batch left out before it installs", func() {
		recorder := usePlainBatch("", "bash")
		dir := GinkgoT().TempDir()
		var shellBeforeInstall []string
		var updatesBeforeInstall int
		installApplication = func(context.Context, types.CrossPlatformApp, config.CrossPlatformSettings, types.Repository) error {
			shellBeforeInstall = shellCommands(recorder)
			for _, call := range recorder.Calls("apt-get") {
				if call.Args[0] == "update" {
					updatesBeforeInstall++
				}
			}
			return nil
		}

		var output bytes.Buffer
		Expect(runPlainInstall(context.Background(), &output, []types.CrossPlatformApp{unbatchedSourceApp(dir)}, mocks.NewMockRepository(), config.CrossPlatformSettings{}, nil)).To(Succeed())

		Expect(shellBeforeInstall).To(ContainElement(ContainSubstring("tee " + filepath.Join(dir, "github-cli.list"))))
		Expect(updatesBeforeInstall).To(Equal(1))
		Expect(output.String()).To(ContainSubstring("installed GitHub CLI"))
	})

	It("fails an app the batch left out when its APT sources cannot be written", func() {
		recorder := usePlainBatch("", "bash")
		dir := GinkgoT().TempDir()
		recorder.Fallback("bash", sdktest.Response{ExitCode: 1, Stderr: "sudo: a password is required\n"})
		var ran []string
		installApplication = func(_ context.Context, app types.CrossPlatformApp, _ config.CrossPlatformSettings, _ types.Repository) error {
			ran = append(ran, app.Name)
			return nil
		}

		var output bytes.Buffer
		err := runPlainInstall(context.Background(), &output, []types.CrossPlatformApp{unbatchedSourceApp(dir)}, mocks.NewMockRepository(), config.CrossPlatformSettings{}, nil)

		Expect(err).To(MatchError(ContainSubstring("GitHub CLI")))
		Expect(ran).To(BeEmpty())
		Expect(output.String()).To(ContainSubstring("failed GitHub CLI: prepare APT sources for GitHub CLI: failed to add APT source"))
	})

	It("writes no APT sources for an app the organization policy denies", func() {
		recorder := usePlainBatch("", "bash")
		home := GinkgoT().TempDir()
		teamDir := filepath.Join(home, "team")
		GinkgoT().Setenv("DEVEX_TEAM_CONFIG_DIR", teamDir)
		Expect(os.MkdirAll(teamDir, 0o750)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(teamDir, "policy.yaml"), []byte("version: '1'\napplications:\n  deny: [GitHub CLI]\n"), 0o600)).To(Succeed())
		security.ResetOrgPolicyCache()
		DeferCleanup(security.ResetOrgPolicyCache)
		policyErr := errors.New("application GitHub CLI is denied by organization policy")
		installApplication = func(context.Context, types.CrossPlatformApp, config.CrossPlatformSettings, types.Repository) error {
			return policyErr
		}

		var output bytes.Buffer
		err := runPlainInstall(context.Background(), &output, []types.CrossPlatformApp{unbatchedSourceApp(GinkgoT().TempDir())}, mocks.NewMockRepository(), config.CrossPlatformSettings{HomeDir: home}, nil)

		Expect(err).To(MatchError(ContainSubstring("GitHub CLI")))
		Expect(output.String()).To(ContainSubstring("failed GitHub CLI: " + policyErr.Error()))
		Expect(shellCommands(recorder)).To(BeEmpty())
		Expect(recorder.Calls("apt-get")).To(BeEmpty())
	})

	It("retries an installed app whose DevEx lifecycle did not finish", func() {
		usePlainBatch("gh\tinstall ok installed\nbat\tinstall ok installed\n")
		var ran []string
		hookErr := errors.New("post-install hook failed")
		installApplication = func(_ context.Context, app types.CrossPlatformApp, _ config.CrossPlatformSettings, _ types.Repository) error {
			ran = append(ran, app.Name)
			return hookErr
		}
		repo := mocks.NewMockRepository()
		// An earlier run installed gh's packages, then its post-install hook failed.
		Expect(tui.RecordLifecycle(repo, "GitHub CLI", false)).To(Succeed())
		apps := []types.CrossPlatformApp{aptApp("GitHub CLI", "gh"), aptApp("bat", "bat")}

		var output bytes.Buffer
		Expect(runPlainInstall(context.Background(), &output, apps, repo, config.CrossPlatformSettings{}, nil)).To(MatchError(ContainSubstring("GitHub CLI")))
		Expect(ran).To(Equal([]string{"GitHub CLI"}))
		Expect(output.String()).To(ContainSubstring("skipped bat (already installed)"))
		Expect(tui.LifecycleUnfinished(repo, "GitHub CLI")).To(BeTrue())

		// Once the hook succeeds the lifecycle is finished and later runs skip it.
		installApplication = func(_ context.Context, app types.CrossPlatformApp, _ config.CrossPlatformSettings, _ types.Repository) error {
			ran = append(ran, app.Name)
			return nil
		}
		Expect(runPlainInstall(context.Background(), &output, apps, repo, config.CrossPlatformSettings{}, nil)).To(Succeed())
		Expect(tui.LifecycleUnfinished(repo, "GitHub CLI")).To(BeFalse())
		Expect(repo.GetApp("GitHub CLI")).NotTo(BeNil())
		output.Reset()
		Expect(runPlainInstall(context.Background(), &output, apps, repo, config.CrossPlatformSettings{}, nil)).To(Succeed())
		Expect(ran).To(Equal([]string{"GitHub CLI", "GitHub CLI"}))
		Expect(output.String()).To(ContainSubstring("skipped GitHub CLI (already installed)"))
	})

	It("records the shared batch steps in each app's audit", func() {
		viper.Set(state.DBPathConfigKey, filepath.Join(GinkgoT().TempDir(), "state.db"))
		DeferCleanup(func() { viper.Set(state.DBPathConfigKey, nil) })
		usePlainBatch("")
		installApplication = func(context.Context, types.CrossPlatformApp, config.CrossPlatformSettings, types.Repository) error {
			return nil
		}
		apps := []types.CrossPlatformApp{aptApp("bat", "bat"), aptApp("fzf", "fzf")}

		var output bytes.Buffer
		Expect(runPlainInstall(context.Background(), &output, apps, mocks.NewMockRepository(), config.CrossPlatformSettings{}, nil)).To(Succeed())

		path, err := state.ConfiguredDBPath()
		Expect(err).NotTo(HaveOccurred())
		store, err := state.OpenBoltStore(path, 0o600)
		Expect(err).NotTo(HaveOccurred())
		defer store.Close()
		events, err := store.ListAudit(state.AuditFilter{Mode: "plain", Event: "result"})
		Expect(err).NotTo(HaveOccurred())
		steps := map[string][]string{}
		runs := map[string]bool{}
		for _, event := range events {
			steps[event.Target] = append(steps[event.Target], event.StepID)
			runs[event.RunID] = true
		}
		for _, app := range []string{"bat", "fzf"} {
			Expect(steps[app]).To(ContainElements("apt-update", "install-batch", "registration", "application"), app)
		}
		Expect(runs).To(HaveLen(1))
	})

	It("leaves apps whose installer plugin is unavailable to fail on their own", func() {
		recorder := sdktest.NewRecorder(GinkgoT(), "apt-get", "dpkg-query")
		preparePackageBatch = tui.PreparePackageBatch
		oldPrepare, oldInstall := prepareApplicationInstallers, installApplication
		DeferCleanup(func() { prepareApplicationInstallers, installApplication = oldPrepare, oldInstall })
		prepareApplicationInstallers = func(context.Context, pluginBootstrapProvider) error { return nil }
		installApplication = func(ctx context.Context, app types.CrossPlatformApp, _ config.CrossPlatformSettings, _ types.Repository) error {
			Expect(installers.PackagesInstalled(ctx, app.Name)).To(BeFalse())
			return errors.New("required plugin package-manager-apt is unavailable")
		}

		var output bytes.Buffer
		err := runPlainInstall(context.Background(), &output, []types.CrossPlatformApp{aptApp("jq", "jq")}, mocks.NewMockRepository(), config.CrossPlatformSettings{}, nil)

		Expect(err).To(HaveOccurred())
		Expect(recorder.Calls("apt-get")).To(BeEmpty())
		Expect(output.String()).To(Equal("installing jq...\nfailed jq: required plugin package-manager-apt is unavailable\ninstalled 0 of 1 applications\n"))
	})

	It("batches nothing when an app without its plugin has a pre-install hook", func() {
		recorder := sdktest.NewRecorder(GinkgoT(), "apt-get", "dpkg-query")
		preparePackageBatch = tui.PreparePackageBatch
		// Test mode reports apt, but not mise, as available.
		installers.EnableTestMode()
		DeferCleanup(installers.DisableTestMode)
		tool := types.CrossPlatformApp{Name: "tool", Description: "tool test app", Category: "Development", AllPlatforms: types.OSConfig{InstallMethod: "mise", InstallCommand: "node", PreInstall: []types.InstallCommand{{Shell: "true"}}}}
		batch, err := preparePackageBatch(context.Background(), []types.CrossPlatformApp{tool, aptApp("bat", "bat")}, mocks.NewMockRepository(), config.CrossPlatformSettings{})
		Expect(err).NotTo(HaveOccurred())
		Expect(batch.BatchInstalled).To(BeEmpty())
		Expect(recorder.Calls("apt-get")).To(BeEmpty())
	})
})
