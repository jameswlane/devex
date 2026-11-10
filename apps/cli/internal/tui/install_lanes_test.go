//go:build !windows

package tui

import (
	"context"
	"time"

	"github.com/jameswlane/devex/apps/cli/internal/types"
	"github.com/spf13/viper"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("APT mirror guidance", func() {
	It("warns only above twenty seconds and suggests both connectivity and mirror remedies", func() {
		Expect(aptMirrorWarning(20 * time.Second)).To(BeEmpty())
		Expect(aptMirrorWarning(time.Second)).To(BeEmpty())
		warning := aptMirrorWarning(21 * time.Second)
		Expect(warning).To(ContainSubstring("IPv4/IPv6"))
		Expect(warning).To(ContainSubstring("Acquire::ForceIPv4=true"))
		Expect(warning).To(ContainSubstring("closer mirror"))
	})
})

var _ = Describe("Shared lane progress", func() {
	It("keeps both active apps visible and counts each completion once", func() {
		apps := []types.CrossPlatformApp{aptApp("bat", "bat"), aptApp("node", "node")}
		model := NewModel(apps)
		model.Update(AppStartedMsg{AppName: "bat", AppIndex: 0})
		model.Update(AppStartedMsg{AppName: "node", AppIndex: 1})
		Expect(model.activeApps).To(HaveLen(2))
		model.Update(AppCompleteMsg{AppName: "node"})
		Expect(model.activeApps).To(HaveKey("bat"))
		Expect(model.currentApp).To(Equal(0))
		model.Update(AppCompleteMsg{AppName: "node"})
		Expect(model.completedApps).To(Equal(int64(1)))
		model.Update(AppCompleteMsg{AppName: "bat"})
		Expect(model.activeApps).To(BeEmpty())
		Expect(model.completedApps).To(Equal(int64(2)))
	})
})

var _ = Describe("Scheduled TUI installs", func() {
	It("prepares first and honors selected dependency order", func() {
		old := viper.Get("install-concurrency")
		viper.Set("install-concurrency", 2)
		defer viper.Set("install-concurrency", old)
		apps := []types.CrossPlatformApp{aptApp("consumer", "consumer"), aptApp("base", "base")}
		apps[0].AllPlatforms.Dependencies = []string{"base"}
		var order []string
		Expect(runInstallLanes(context.Background(), apps, func(context.Context, []types.CrossPlatformApp) error { order = append(order, "prepare"); return nil }, func(_ context.Context, _ int, app types.CrossPlatformApp) error {
			order = append(order, app.Name)
			return nil
		})).To(Succeed())
		Expect(order).To(Equal([]string{"prepare", "base", "consumer"}))
	})
})
