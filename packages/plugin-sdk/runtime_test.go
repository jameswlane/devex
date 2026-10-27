package sdk_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"sync/atomic"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
)

func writeRuntimeTestPlugin(path, version string, operation string) {
	body := fmt.Sprintf("#!/bin/sh\nif [ \"$1\" = \"--plugin-info\" ]; then\n  printf '{\"name\":\"demo\",\"version\":%q,\"description\":\"test plugin\"}\\n'\n  exit 0\nfi\n%s\n", version, operation)
	Expect(os.WriteFile(path, []byte(body), 0o755)).To(Succeed()) // #nosec G306 - test executable
}

var _ = Describe("Concurrent plugin installation", func() {
	It("serializes installs of the same plugin and verifies the installed bytes", func() {
		pluginDir := GinkgoT().TempDir()
		payload := []byte(fakeExecutable)
		checksum := sha256Hex(payload)
		platformKey := fmt.Sprintf("%s-%s", runtime.GOOS, runtime.GOARCH)
		var downloadCount atomic.Int32

		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/api/v1/plugins/demo":
				_ = json.NewEncoder(w).Encode(sdk.PluginMetadata{
					PluginInfo: sdk.PluginInfo{Name: "demo", Version: "1.2.3"},
					Binaries: map[string]sdk.PlatformBinary{
						platformKey: {URL: "https://example.invalid/demo", Checksum: checksum, Size: int64(len(payload))},
					},
				})
			case "/api/v1/plugins/demo/download/" + platformKey:
				downloadCount.Add(1)
				time.Sleep(50 * time.Millisecond)
				_, _ = w.Write(payload)
			default:
				http.NotFound(w, r)
			}
		}))
		defer server.Close()

		downloader := sdk.NewDownloader(server.URL, pluginDir)
		downloader.SetSilent(true)
		const installers = 8
		start := make(chan struct{})
		results := make(chan error, installers)
		for range installers {
			go func() {
				<-start
				results <- downloader.DownloadPluginWithContext(context.Background(), "demo")
			}()
		}
		close(start)
		for range installers {
			Expect(<-results).To(Succeed())
		}

		Expect(downloadCount.Load()).To(Equal(int32(1)))
		installed, err := os.ReadFile(filepath.Join(pluginDir, executableName("demo")))
		Expect(err).ToNot(HaveOccurred())
		Expect(sha256Hex(installed)).To(Equal(checksum))
		entries, err := os.ReadDir(pluginDir)
		Expect(err).ToNot(HaveOccurred())
		names := make([]string, 0, len(entries))
		for _, entry := range entries {
			names = append(names, entry.Name())
		}
		Expect(names).To(ConsistOf(executableName("demo"), ".devex-plugin-demo.install.json"))
	})
})

var _ = Describe("Plugin execution and discovery", func() {
	var (
		pluginDir string
		manager   *sdk.ExecutableManager
	)

	BeforeEach(func() {
		if runtime.GOOS == "windows" {
			Skip("shell plugin fixtures require a POSIX shell")
		}
		pluginDir = GinkgoT().TempDir()
		manager = sdk.NewExecutableManager(pluginDir)
	})

	It("does not apply the metadata timeout to plugin operations", func() {
		source := filepath.Join(pluginDir, "source")
		writeRuntimeTestPlugin(source, "1.0.0", "sleep 0.2")
		Expect(manager.InstallPlugin(source, "demo")).To(Succeed())
		manager.SetMetadataTimeout(20 * time.Millisecond)

		Expect(manager.ExecutePluginWithContext(context.Background(), "demo", nil)).To(Succeed())
	})

	It("uses the caller's context to cancel plugin operations", func() {
		source := filepath.Join(pluginDir, "source")
		writeRuntimeTestPlugin(source, "1.0.0", "sleep 1")
		Expect(manager.InstallPlugin(source, "demo")).To(Succeed())
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
		defer cancel()

		err := manager.ExecutePluginWithContext(ctx, "demo", nil)
		Expect(err).To(HaveOccurred())
		Expect(errors.Is(ctx.Err(), context.DeadlineExceeded)).To(BeTrue())
	})

	It("forces discovery to observe files installed outside the manager", func() {
		Expect(manager.DiscoverPlugins()).To(Succeed())
		Expect(manager.ListPlugins()).To(BeEmpty())
		writeRuntimeTestPlugin(filepath.Join(pluginDir, executableName("demo")), "2.0.0", "exit 0")

		Expect(manager.DiscoverPlugins()).To(Succeed())
		Expect(manager.ListPlugins()).To(HaveKey("demo"))
		Expect(manager.ListPlugins()["demo"].Version).To(Equal("2.0.0"))
	})

	It("invalidates cached discovery after local installation", func() {
		Expect(manager.DiscoverPlugins()).To(Succeed())
		source := filepath.Join(pluginDir, "source")
		writeRuntimeTestPlugin(source, "1.0.0", "exit 0")

		Expect(manager.InstallPlugin(source, "demo")).To(Succeed())
		Expect(manager.ListPlugins()).To(HaveKey("demo"))
	})

	It("returns plugin directory errors from discovery", func() {
		manager = sdk.NewExecutableManager(filepath.Join(pluginDir, "missing"))

		err := manager.DiscoverPlugins()
		Expect(err).To(HaveOccurred())
		Expect(errors.Is(err, os.ErrNotExist)).To(BeTrue())
	})
})

var _ = Describe("Background updater run lifecycle", func() {
	It("supports repeated Start Stop Start cycles", func() {
		pluginDir := GinkgoT().TempDir()
		downloader := sdk.NewDownloader("https://registry.example.com", pluginDir)
		manager := sdk.NewExecutableManager(pluginDir)
		updater := sdk.NewBackgroundUpdater(downloader, manager)
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		defer updater.Stop()

		for range 20 {
			Expect(updater.Start(ctx)).To(Succeed())
			Expect(updater.IsRunning()).To(BeTrue())
			updater.Stop()
			Expect(updater.IsRunning()).To(BeFalse())
		}
	})
})
