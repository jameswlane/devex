package commands

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/viper"

	"github.com/jameswlane/devex/apps/cli/internal/bootstrap"
	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/configfiles"
	"github.com/jameswlane/devex/apps/cli/internal/mcpserver"
	"github.com/jameswlane/devex/apps/cli/internal/mocks"
	"github.com/jameswlane/devex/apps/cli/internal/state"
	"github.com/jameswlane/devex/apps/cli/internal/types"
)

// mcpRecordingInstaller records package-manager calls instead of running them.
type mcpRecordingInstaller struct {
	mu    *sync.Mutex
	calls *[]string
}

func (i *mcpRecordingInstaller) record(call string) {
	i.mu.Lock()
	defer i.mu.Unlock()
	*i.calls = append(*i.calls, call)
}

func (i *mcpRecordingInstaller) Install(command string, _ types.Repository) error {
	i.record("install " + command)
	return nil
}

func (i *mcpRecordingInstaller) Uninstall(command string, _ types.Repository) error {
	i.record("uninstall " + command)
	return nil
}

func (i *mcpRecordingInstaller) IsInstalled(string) (bool, error) { return true, nil }

var _ = Describe("MCP executor", func() {
	var (
		ctx            context.Context
		home           string
		repo           *mocks.MockRepository
		executor       *mcpExecutor
		progress       *bytes.Buffer
		callsMu        sync.Mutex
		installerCalls []string
		prepared       bool
		pluginInits    int
		pluginInitErr  error
	)

	BeforeEach(func() {
		ctx = context.Background()
		viper.Reset()
		DeferCleanup(viper.Reset)
		home = os.Getenv("HOME")
		repo = mocks.NewMockRepository()
		progress = &bytes.Buffer{}
		executor = &mcpExecutor{repo: repo, settings: config.CrossPlatformSettings{HomeDir: home}, log: progress}
		installerCalls, prepared, pluginInits, pluginInitErr = nil, false, 0, nil

		oldInit, oldPrepare, oldInstaller, oldInstall := initializeMCPPlugins, prepareApplicationInstallers, getUninstallInstaller, installApplication
		DeferCleanup(func() {
			initializeMCPPlugins, prepareApplicationInstallers, getUninstallInstaller, installApplication = oldInit, oldPrepare, oldInstaller, oldInstall
		})
		initializeMCPPlugins = func(context.Context) (*bootstrap.PluginBootstrap, error) {
			pluginInits++
			if pluginInitErr != nil {
				return nil, pluginInitErr
			}
			return &bootstrap.PluginBootstrap{}, nil
		}
		prepareApplicationInstallers = func(_ context.Context, provider pluginBootstrapProvider) error {
			if provider == nil || provider() == nil {
				return errors.New("no plugin bootstrap")
			}
			prepared = true
			return nil
		}
		// Like installers.GetInstaller, package-manager installers exist only
		// after plugins were initialized.
		getUninstallInstaller = func(_ context.Context, _ string) types.BaseInstaller {
			if !prepared {
				return nil
			}
			return &mcpRecordingInstaller{mu: &callsMu, calls: &installerCalls}
		}
	})

	calls := func() []string {
		callsMu.Lock()
		defer callsMu.Unlock()
		return append([]string(nil), installerCalls...)
	}
	define := func(name string, definition types.OSConfig) {
		data, err := json.Marshal(types.CrossPlatformApp{Name: name, AllPlatforms: definition})
		Expect(err).NotTo(HaveOccurred())
		Expect(repo.Set("installed-app-definition:"+normalizeInstallLookup(name), string(data))).To(Succeed())
	}
	track := func(name string, definition types.OSConfig) {
		Expect(repo.AddApp(name)).To(Succeed())
		define(name, definition)
	}
	writeFile := func(path, content string) {
		Expect(os.MkdirAll(filepath.Dir(path), 0o700)).To(Succeed())
		Expect(os.WriteFile(path, []byte(content), 0o600)).To(Succeed())
	}
	// recordConfig writes the managed configuration-file record for app.
	recordConfig := func(app string, paths ...string) {
		result := configfiles.Result{}
		for _, path := range paths {
			writeFile(path, "managed "+filepath.Base(path))
			result.Files = append(result.Files, configfiles.File{Destination: path, SHA256: fmt.Sprintf("%x", sha256.Sum256([]byte("managed "+filepath.Base(path))))})
		}
		data, err := json.Marshal(result)
		Expect(err).NotTo(HaveOccurred())
		writeFile(filepath.Join(home, ".local", "state", "devex", "config-files", fmt.Sprintf("%x.json", sha256.Sum256([]byte(strings.ToLower(app))))), string(data))
	}

	Context("binding uninstall approval to the executed definition", func() {
		var markerA, markerB, dataA, dataB string

		BeforeEach(func() {
			markerA, markerB = filepath.Join(home, "ran-a"), filepath.Join(home, "ran-b")
			GinkgoT().Setenv("DEVEX_TEST_MARKER_A", markerA)
			GinkgoT().Setenv("DEVEX_TEST_MARKER_B", markerB)
			dataA = filepath.Join(home, ".local", "share", "canary", "a")
			dataB = filepath.Join(home, ".local", "share", "canary", "b")
			writeFile(dataA, "a")
			writeFile(dataB, "b")
		})

		canary := func(command string, cleanup ...string) types.OSConfig {
			return types.OSConfig{InstallMethod: "curlpipe", InstallCommand: "https://example.invalid/install.sh", UninstallCommand: command, CleanupFiles: cleanup}
		}

		It("previews the command, deleted paths and a definition digest", func() {
			track("canary", canary(`touch "$DEVEX_TEST_MARKER_A" # https://example.invalid/x?token=SECRET_CANARY`, dataA))
			recordConfig("canary", filepath.Join(home, ".config", "canary", "a.conf"))

			targets, err := executor.PlanUninstall(ctx, []string{"canary"}, mcpserver.UninstallOptions{RemoveConfig: true, RemoveData: true})

			Expect(err).NotTo(HaveOccurred())
			Expect(targets).To(HaveLen(1))
			Expect(targets[0].Command).To(ContainSubstring(`touch "$DEVEX_TEST_MARKER_A"`))
			Expect(targets[0].Command).NotTo(ContainSubstring("SECRET_CANARY"))
			Expect(targets[0].DataPaths).To(Equal([]string{dataA}))
			Expect(targets[0].ConfigFiles).To(Equal([]string{filepath.Join(home, ".config", "canary", "a.conf")}))
			Expect(targets[0].Definition).To(HaveLen(64))
			Expect(markerA).NotTo(BeAnExistingFile())
		})

		It("executes an unchanged approved definition", func() {
			track("canary", canary(`touch "$DEVEX_TEST_MARKER_A"`, dataA))
			configFile := filepath.Join(home, ".config", "canary", "a.conf")
			recordConfig("canary", configFile)
			options := mcpserver.UninstallOptions{RemoveConfig: true, RemoveData: true}
			targets, err := executor.PlanUninstall(ctx, []string{"canary"}, options)
			Expect(err).NotTo(HaveOccurred())

			outcomes, err := executor.Uninstall(ctx, targets, options)

			Expect(err).NotTo(HaveOccurred())
			Expect(outcomes).To(Equal([]mcpserver.Outcome{{App: "canary", Status: "removed"}}))
			Expect(markerA).To(BeAnExistingFile())
			Expect(dataA).NotTo(BeAnExistingFile())
			Expect(configFile).NotTo(BeAnExistingFile())
			_, err = repo.GetApp("canary")
			Expect(err).To(HaveOccurred())
		})

		It("refuses a target whose saved uninstall command changed after approval", func() {
			track("canary", canary(`touch "$DEVEX_TEST_MARKER_A"`))
			targets, err := executor.PlanUninstall(ctx, []string{"canary"}, mcpserver.UninstallOptions{})
			Expect(err).NotTo(HaveOccurred())
			define("canary", canary(`touch "$DEVEX_TEST_MARKER_B"`))

			outcomes, err := executor.Uninstall(ctx, targets, mcpserver.UninstallOptions{})

			Expect(err).NotTo(HaveOccurred())
			Expect(outcomes).To(HaveLen(1))
			Expect(outcomes[0].Status).To(Equal("failed"))
			Expect(outcomes[0].Error).To(ContainSubstring("plan_rejected"))
			Expect(markerA).NotTo(BeAnExistingFile())
			Expect(markerB).NotTo(BeAnExistingFile())
			Expect(repo.GetApp("canary")).NotTo(BeNil())
		})

		It("refuses a target whose cleanup paths changed after approval", func() {
			options := mcpserver.UninstallOptions{RemoveData: true}
			track("canary", canary(`touch "$DEVEX_TEST_MARKER_A"`, dataA))
			targets, err := executor.PlanUninstall(ctx, []string{"canary"}, options)
			Expect(err).NotTo(HaveOccurred())
			define("canary", canary(`touch "$DEVEX_TEST_MARKER_A"`, dataB))

			outcomes, err := executor.Uninstall(ctx, targets, options)

			Expect(err).NotTo(HaveOccurred())
			Expect(outcomes[0].Error).To(ContainSubstring("plan_rejected"))
			Expect(markerA).NotTo(BeAnExistingFile())
			Expect(dataA).To(BeAnExistingFile())
			Expect(dataB).To(BeAnExistingFile())
		})

		It("refuses a target whose managed configuration files changed after approval", func() {
			options := mcpserver.UninstallOptions{RemoveConfig: true}
			track("canary", canary(`touch "$DEVEX_TEST_MARKER_A"`))
			first, second := filepath.Join(home, ".config", "canary", "a.conf"), filepath.Join(home, ".config", "canary", "b.conf")
			recordConfig("canary", first)
			targets, err := executor.PlanUninstall(ctx, []string{"canary"}, options)
			Expect(err).NotTo(HaveOccurred())
			recordConfig("canary", first, second)

			outcomes, err := executor.Uninstall(ctx, targets, options)

			Expect(err).NotTo(HaveOccurred())
			Expect(outcomes[0].Error).To(ContainSubstring("plan_rejected"))
			Expect(markerA).NotTo(BeAnExistingFile())
			Expect(first).To(BeAnExistingFile())
			Expect(second).To(BeAnExistingFile())
		})

		It("refuses to plan removing a protected data path", func() {
			track("canary", canary(`touch "$DEVEX_TEST_MARKER_A"`, home))

			_, err := executor.PlanUninstall(ctx, []string{"canary"}, mcpserver.UninstallOptions{RemoveData: true})

			Expect(err).To(MatchError(ContainSubstring("refusing to plan removal")))
		})
	})

	Context("tracked applications only", func() {
		It("refuses to plan an application that configuration defines but DevEx does not track", func() {
			viper.Set("applications", []any{map[string]any{
				"name": "configured-only", "description": "x", "category": "Testing",
				"linux": map[string]any{"install_method": "apt", "install_command": "configured-only"},
			}})

			_, err := executor.PlanUninstall(ctx, []string{"configured-only"}, mcpserver.UninstallOptions{})

			Expect(err).To(MatchError(ContainSubstring(`"configured-only" is not tracked by DevEx`)))
			Expect(calls()).To(BeEmpty())
		})

		It("refuses a target that stopped being tracked after approval", func() {
			track("tool", types.OSConfig{InstallMethod: "apt", InstallCommand: "tool"})
			targets, err := executor.PlanUninstall(ctx, []string{"tool"}, mcpserver.UninstallOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(repo.DeleteApp("tool")).To(Succeed())

			outcomes, err := executor.Uninstall(ctx, targets, mcpserver.UninstallOptions{})

			Expect(err).NotTo(HaveOccurred())
			Expect(outcomes[0].Status).To(Equal("failed"))
			Expect(outcomes[0].Error).To(ContainSubstring("not tracked by DevEx"))
			Expect(calls()).To(BeEmpty())
		})
	})

	Context("package-manager plugins", func() {
		var targets []mcpserver.UninstallTarget

		BeforeEach(func() {
			track("tool", types.OSConfig{InstallMethod: "apt", InstallCommand: "tool"})
			var err error
			targets, err = executor.PlanUninstall(ctx, []string{"tool"}, mcpserver.UninstallOptions{})
			Expect(err).NotTo(HaveOccurred())
			Expect(targets[0].UnknownEffects).To(BeTrue())
		})

		It("initializes plugins when uninstall is the first mutation", func() {
			outcomes, err := executor.Uninstall(ctx, targets, mcpserver.UninstallOptions{})

			Expect(err).NotTo(HaveOccurred())
			Expect(outcomes).To(Equal([]mcpserver.Outcome{{App: "tool", Status: "removed"}}))
			Expect(pluginInits).To(Equal(1))
			Expect(calls()).To(Equal([]string{"uninstall tool"}))
		})

		It("reports a plugin initialization failure without removing anything", func() {
			pluginInitErr = errors.New("plugin registry unavailable")

			_, err := executor.Uninstall(ctx, targets, mcpserver.UninstallOptions{})

			Expect(err).To(MatchError(ContainSubstring("initialize package-manager plugins: plugin registry unavailable")))
			Expect(calls()).To(BeEmpty())
		})

		It("neither initializes plugins nor removes anything for a cancelled request", func() {
			cancelled, cancel := context.WithCancel(ctx)
			cancel()

			_, err := executor.Uninstall(cancelled, targets, mcpserver.UninstallOptions{})

			Expect(err).To(MatchError(context.Canceled))
			Expect(pluginInits).To(BeZero())
			Expect(calls()).To(BeEmpty())
		})
	})

	Context("installing", func() {
		BeforeEach(func() {
			viper.Set("applications", []any{map[string]any{
				"name": "canary-app", "description": "x", "category": "Testing",
				"linux": map[string]any{"install_method": "apt", "install_command": "canary-app"},
			}})
		})

		It("returns failures outside individual applications with their outcomes", func() {
			dbPath := filepath.Join(GinkgoT().TempDir(), "state.db")
			viper.Set(state.DBPathConfigKey, dbPath)
			installApplication = func(context.Context, types.CrossPlatformApp, config.CrossPlatformSettings, types.Repository) error {
				// Make recording the run fail after the package installed.
				Expect(os.Remove(dbPath)).To(Succeed())
				Expect(os.Mkdir(dbPath, 0o700)).To(Succeed())
				return nil
			}

			outcomes, err := executor.Install(ctx, []string{"canary-app"})

			Expect(outcomes).To(Equal([]mcpserver.Outcome{{App: "canary-app", Status: "installed"}}))
			Expect(err).To(HaveOccurred())
		})

		It("redacts installer errors in progress output", func() {
			installApplication = func(context.Context, types.CrossPlatformApp, config.CrossPlatformSettings, types.Repository) error {
				return errors.New("download https://user:hunter2@example.invalid/x?token=SECRET_CANARY failed")
			}

			outcomes, err := executor.Install(ctx, []string{"canary-app"})

			Expect(err).To(HaveOccurred())
			Expect(outcomes).To(HaveLen(1))
			Expect(progress.String()).To(ContainSubstring("failed canary-app"))
			Expect(progress.String()).NotTo(ContainSubstring("hunter2"))
			Expect(progress.String()).NotTo(ContainSubstring("SECRET_CANARY"))
		})
	})
})
