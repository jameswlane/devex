package setup

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"

	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/jameswlane/devex/apps/cli/internal/installers"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/viper"
)

var _ = Describe("Plugin installation progress", func() {
	var model *SetupModel

	BeforeEach(func() {
		GinkgoT().Setenv("HOME", GinkgoT().TempDir())
		viper.Reset()
		viper.Set("offline", true)
		DeferCleanup(viper.Reset)
		DeferCleanup(func() { installers.InitializeWithPluginBootstrap(nil) })
		names := []string{"tool-shell", "tool-git"}
		model = &SetupModel{step: StepPluginInstall, plugins: PluginState{
			requiredPlugins: names, pluginStatuses: initializePluginStatuses(names),
			spinner: initializeSpinner(), timeout: time.Second,
		}}
	})

	It("applies ordered channel events only when Update receives each message", func() {
		events := make(chan tea.Msg, 6)
		model.plugins.pluginEvents = events
		atomic.StoreInt32(&model.plugins.pluginsInstalling, 1)
		events <- PluginInstallMsg{PluginName: "tool-shell", Status: "downloading", BytesDownloaded: 25, TotalBytes: 100}
		events <- PluginInstallMsg{PluginName: "tool-shell", Status: "verifying"}
		events <- PluginInstallMsg{PluginName: "tool-shell", Status: "success"}
		events <- PluginInstallMsg{PluginName: "tool-git", Status: "downloading"}
		events <- PluginInstallMsg{PluginName: "tool-git", Status: "error", Error: errors.New("checksum mismatch")}
		events <- PluginInstallCompleteMsg{SuccessCount: 1, TotalCount: 2, SuccessfulPlugins: []string{"tool-shell"}, Errors: []error{errors.New("checksum mismatch")}}
		close(events)

		cmd := waitForPluginInstallationEvent(events)
		msg := cmd()
		Expect(model.plugins.pluginStatuses[0].Status).To(Equal("pending"))
		_, cmd = model.Update(msg)
		Expect(model.plugins.pluginStatuses[0].Status).To(Equal("downloading"))
		Expect(model.View()).To(ContainSubstring("25%"))
		Expect(model.View()).To(ContainSubstring("0/2 plugins"))
		Expect(model.View()).To(ContainSubstring(model.plugins.spinner.View()))
		_, cmd = model.Update(cmd())
		Expect(model.plugins.pluginStatuses[0].Status).To(Equal("verifying"))
		_, cmd = model.Update(cmd())
		Expect(model.plugins.pluginStatuses[0].Status).To(Equal("success"))
		Expect(model.plugins.pluginStatuses[1].Status).To(Equal("pending"))
		Expect(model.View()).To(ContainSubstring("1/2 plugins"))
		_, cmd = model.Update(cmd())
		Expect(model.plugins.pluginStatuses[1].Status).To(Equal("downloading"))
		_, cmd = model.Update(cmd())
		Expect(model.View()).To(And(ContainSubstring("❌"), ContainSubstring("checksum mismatch"), ContainSubstring("✅"), ContainSubstring("2/2 plugins")))
		_, cmd = model.Update(cmd())
		Expect(cmd).To(BeNil())
		Expect(model.plugins.pluginEvents).To(BeNil())
		Expect(atomic.LoadInt32(&model.plugins.pluginsInstalling)).To(BeZero())
		Expect(model.plugins.successfulPlugins).To(Equal([]string{"tool-shell"}))
	})

	It("starts a real spinner tick and continues ticking only while installing", func() {
		batch, ok := model.startPluginInstallation()().(tea.BatchMsg)
		Expect(ok).To(BeTrue())
		DeferCleanup(model.plugins.pluginCancel)
		Expect(batch).To(HaveLen(3))
		tick, ok := batch[0]().(spinner.TickMsg)
		Expect(ok).To(BeTrue())
		_, next := model.Update(tick)
		Expect(next).NotTo(BeNil())
		Expect(next()).To(BeAssignableToTypeOf(spinner.TickMsg{}))
		model.Update(PluginInstallCompleteMsg{})
		_, next = model.Update(tick)
		Expect(next).To(BeNil())
	})

	It("does not let the worker mutate statuses while the view reads them", func() {
		blockedHome := filepath.Join(GinkgoT().TempDir(), "file")
		Expect(os.WriteFile(blockedHome, nil, 0600)).To(Succeed())
		GinkgoT().Setenv("HOME", blockedHome)
		events := make(chan tea.Msg)
		worker := model.runPluginInstallation(context.Background(), events)
		finished := make(chan struct{})
		go func() {
			defer close(finished)
			worker()
		}()
		var complete PluginInstallCompleteMsg
		for msg := range events {
			Expect(model.View()).To(ContainSubstring("0/2 plugins"))
			Expect(model.plugins.pluginStatuses).To(Equal(initializePluginStatuses(model.plugins.requiredPlugins)))
			if result, ok := msg.(PluginInstallCompleteMsg); ok {
				complete = result
			}
		}
		Eventually(finished).Should(BeClosed())
		Expect(complete.Errors).NotTo(BeEmpty())
		Expect(complete.SuccessCount).To(BeZero())
	})

	It("keeps bootstrap download failures visible through completion", func() {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()
		GinkgoT().Setenv("DEVEX_PLUGIN_REGISTRY_URL", server.URL)
		viper.Set("offline", false)
		model.plugins.timeout = 5 * time.Second
		batch := model.startPluginInstallation()().(tea.BatchMsg)
		finished := make(chan struct{})
		go func() {
			defer close(finished)
			batch[1]()
		}()
		cmd := batch[2]
		sawDownload := false
		for cmd != nil {
			msg := cmd()
			if event, ok := msg.(PluginInstallMsg); ok && event.PluginName == "tool-shell" && event.Status == "downloading" {
				sawDownload = true
				Expect(model.plugins.pluginStatuses[0].Status).To(Equal("pending"))
			}
			_, cmd = model.Update(msg)
		}
		Eventually(finished).Should(BeClosed())
		Expect(sawDownload).To(BeTrue())
		for _, status := range model.plugins.pluginStatuses {
			Expect(status.Status).To(Equal("error"))
			Expect(status.Error).To(ContainSubstring("registry"))
		}
		Expect(model.plugins.successfulPlugins).To(BeEmpty())
		Expect(model.View()).To(And(ContainSubstring("❌"), ContainSubstring("2/2 plugins")))
	})

	It("cancels a blocked progress producer when setup quits", func() {
		batch := model.startPluginInstallation()().(tea.BatchMsg)
		finished := make(chan struct{})
		go func() {
			defer close(finished)
			batch[1]()
		}()
		_, quit := model.Update(tea.KeyMsg{Type: tea.KeyCtrlC})
		Expect(quit).NotTo(BeNil())
		Eventually(finished).Should(BeClosed())
	})

	It("keeps failure reasons visible when registry errors allow setup to continue", func() {
		model.Update(PluginInstallMsg{PluginName: "tool-shell", Status: "error", Error: errors.New("registry unavailable: connection refused")})
		model.Update(PluginInstallCompleteMsg{Errors: []error{errors.New("registry unavailable")}, TotalCount: 2})
		Expect(atomic.LoadInt32(&model.plugins.pluginsInstalled)).To(Equal(int32(1)))
		Expect(model.View()).To(ContainSubstring("connection refused"))
	})
})
