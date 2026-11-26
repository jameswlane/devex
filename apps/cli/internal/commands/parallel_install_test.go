package commands

import (
	"bytes"
	"context"
	"path/filepath"
	"time"

	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/mocks"
	"github.com/jameswlane/devex/apps/cli/internal/tui"
	"github.com/jameswlane/devex/apps/cli/internal/types"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/viper"
)

var _ = Describe("Parallel plain installations", func() {
	It("overlaps independent applications and writes complete output lines", func() {
		oldInstall, oldPrepare, oldBatch := installApplication, prepareApplicationInstallers, preparePackageBatch
		oldLimit := viper.Get("install-concurrency")
		DeferCleanup(func() {
			installApplication, prepareApplicationInstallers, preparePackageBatch = oldInstall, oldPrepare, oldBatch
			viper.Set("install-concurrency", oldLimit)
		})
		GinkgoT().Setenv("DEVEX_STATE_DB", filepath.Join(GinkgoT().TempDir(), "state.db"))
		viper.Set("install-concurrency", 2)
		prepareApplicationInstallers = func(context.Context, pluginBootstrapProvider) error { return nil }
		preparePackageBatch = func(context.Context, []types.CrossPlatformApp, types.Repository, config.CrossPlatformSettings) (tui.PackageBatch, error) {
			return tui.PackageBatch{}, nil
		}
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		entered := make(chan string, 2)
		release := make(chan struct{})
		done := make(chan error, 1)
		installApplication = func(ctx context.Context, app types.CrossPlatformApp, _ config.CrossPlatformSettings, _ types.Repository) error {
			entered <- app.Name
			select {
			case <-release:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		apps := []types.CrossPlatformApp{{Name: "one", AllPlatforms: types.OSConfig{InstallMethod: "mise"}}, {Name: "two", AllPlatforms: types.OSConfig{InstallMethod: "flatpak"}}}
		var output bytes.Buffer
		go func() {
			done <- runPlainInstall(ctx, &output, apps, mocks.NewMockRepository(), config.CrossPlatformSettings{}, nil)
		}()
		Eventually(entered, 10*time.Second).Should(Receive())
		Eventually(entered, 10*time.Second).Should(Receive())
		close(release)
		Eventually(done, 10*time.Second).Should(Receive(Succeed()))
		Expect(output.String()).To(ContainSubstring("installed one\n"))
		Expect(output.String()).To(ContainSubstring("installed two\n"))
		Expect(output.String()).To(ContainSubstring("installed 2 of 2 applications\n"))
	})
})
