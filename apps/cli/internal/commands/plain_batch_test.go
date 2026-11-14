//go:build !windows

package commands

import (
	"bytes"
	"context"
	"errors"
	"path/filepath"
	"sync"

	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/installers"
	"github.com/jameswlane/devex/apps/cli/internal/mocks"
	"github.com/jameswlane/devex/apps/cli/internal/state"
	"github.com/jameswlane/devex/apps/cli/internal/tui"
	"github.com/jameswlane/devex/apps/cli/internal/types"
	"github.com/jameswlane/devex/packages/plugin-sdk/sdktest"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/viper"
)

// Plain-install specs must not run real package managers unless they opt in.
var _ = BeforeEach(func() {
	original := preparePackageBatch
	preparePackageBatch = func(context.Context, []types.CrossPlatformApp, types.Repository, config.CrossPlatformSettings) (tui.PackageBatch, error) {
		return tui.PackageBatch{}, nil
	}
	DeferCleanup(func() { preparePackageBatch = original })
})

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

	usePlainBatch := func(installed string) *sdktest.Recorder {
		recorder := sdktest.NewRecorder(GinkgoT(), "apt-get", "dpkg-query")
		recorder.Fallback("dpkg-query", sdktest.Response{Stdout: installed})
		viper.Set("apt-lists-max-age", "0s")
		DeferCleanup(func() { viper.Set("apt-lists-max-age", nil) })
		preparePackageBatch = tui.PreparePackageBatch
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
