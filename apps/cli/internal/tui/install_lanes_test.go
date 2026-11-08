//go:build !windows

package tui

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/jameswlane/devex/apps/cli/internal/types"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Installation lanes", func() {
	independent := func(name, method string) types.CrossPlatformApp {
		app := aptApp(name, name)
		app.AllPlatforms.InstallMethod = method
		return app
	}
	It("overlaps independent installers with APT preparation and preserves each lane's order", func() {
		apps := []types.CrossPlatformApp{aptApp("bat", "bat"), independent("mise", "curlpipe"), aptApp("fzf", "fzf"), independent("node", "mise"), independent("desktop", "flatpak")}
		started := make(chan struct{})
		otherStarted := make(chan struct{})
		var mu sync.Mutex
		var order []string
		done := make(chan error, 1)
		go func() {
			done <- runInstallLanes(context.Background(), apps, func(ctx context.Context, _ []types.CrossPlatformApp) error {
				close(started)
				select {
				case <-otherStarted:
					return nil
				case <-ctx.Done():
					return ctx.Err()
				}
			}, func(ctx context.Context, _ int, app types.CrossPlatformApp) error {
				if app.Name == "mise" {
					<-started
					close(otherStarted)
				}
				mu.Lock()
				order = append(order, app.Name)
				mu.Unlock()
				return ctx.Err()
			})
		}()
		Eventually(done, time.Second).Should(Receive(Succeed()))
		mu.Lock()
		defer mu.Unlock()
		aptOrder, independentOrder := []string{}, []string{}
		for _, name := range order {
			if name == "bat" || name == "fzf" {
				aptOrder = append(aptOrder, name)
			} else {
				independentOrder = append(independentOrder, name)
			}
		}
		Expect(aptOrder).To(Equal([]string{"bat", "fzf"}))
		Expect(independentOrder).To(Equal([]string{"mise", "node", "desktop"}))
	})
	It("keeps hooks and declared dependencies on a serial path", func() {
		apps := []types.CrossPlatformApp{independent("mise", "mise"), aptApp("bat", "bat")}
		apps[0].AllPlatforms.Dependencies = []string{"bat"}
		var order []string
		Expect(runInstallLanes(context.Background(), apps, func(context.Context, []types.CrossPlatformApp) error { order = append(order, "prepare"); return nil }, func(_ context.Context, _ int, app types.CrossPlatformApp) error {
			order = append(order, app.Name)
			return nil
		})).To(Succeed())
		Expect(order).To(Equal([]string{"prepare", "mise", "bat"}))
	})
	It("continues after an individual failure and returns it", func() {
		expected := errors.New("bad package")
		var names []string
		err := runInstallLanes(context.Background(), []types.CrossPlatformApp{aptApp("bad", "bad"), aptApp("good", "good")}, func(context.Context, []types.CrossPlatformApp) error { return nil }, func(_ context.Context, _ int, app types.CrossPlatformApp) error {
			names = append(names, app.Name)
			if app.Name == "bad" {
				return expected
			}
			return nil
		})
		Expect(err).To(MatchError(expected))
		Expect(names).To(Equal([]string{"bad", "good"}))
	})
	It("cancels the other lane when preparation fails", func() {
		expected := errors.New("prepare failed")
		entered := make(chan struct{})
		err := runInstallLanes(context.Background(), []types.CrossPlatformApp{aptApp("bat", "bat"), independent("node", "mise")}, func(context.Context, []types.CrossPlatformApp) error { <-entered; return expected }, func(ctx context.Context, _ int, _ types.CrossPlatformApp) error {
			close(entered)
			<-ctx.Done()
			return ctx.Err()
		})
		Expect(errors.Is(err, expected)).To(BeTrue())
		Expect(errors.Is(err, context.Canceled)).To(BeTrue())
	})
	It("recovers a lane panic and preserves preceding failures", func() {
		expected := errors.New("earlier failure")
		apps := []types.CrossPlatformApp{aptApp("bad", "bad"), aptApp("panic", "panic")}
		err := runInstallLanes(context.Background(), apps, func(context.Context, []types.CrossPlatformApp) error { return nil }, func(_ context.Context, i int, _ types.CrossPlatformApp) error {
			if i == 0 {
				return expected
			}
			panic("installer exploded")
		})
		Expect(errors.Is(err, expected)).To(BeTrue())
		Expect(err.Error()).To(ContainSubstring("installer exploded"))
	})
	It("cancels and joins the other lane after a preparation panic", func() {
		started := make(chan struct{})
		finished := make(chan struct{})
		err := runInstallLanes(context.Background(), []types.CrossPlatformApp{aptApp("bat", "bat"), independent("node", "mise")}, func(context.Context, []types.CrossPlatformApp) error {
			<-started
			panic("preparation exploded")
		}, func(ctx context.Context, _ int, _ types.CrossPlatformApp) error {
			close(started)
			<-ctx.Done()
			close(finished)
			return ctx.Err()
		})
		Expect(err.Error()).To(ContainSubstring("preparation exploded"))
		Expect(errors.Is(err, context.Canceled)).To(BeTrue())
		Expect(finished).To(BeClosed())
	})

	It("does no work for an already cancelled context", func() {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		err := runInstallLanes(ctx, []types.CrossPlatformApp{independent("node", "mise")}, nil, func(context.Context, int, types.CrossPlatformApp) error { Fail("unexpected install"); return nil })
		Expect(errors.Is(err, context.Canceled)).To(BeTrue())
	})
})

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
