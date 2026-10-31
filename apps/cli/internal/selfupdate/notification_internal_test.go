package selfupdate

import (
	"context"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Detached notification refresh", func() {
	It("finishes without waiting and reads a result filled after foreground cancellation", func() {
		dir := GinkgoT().TempDir()
		ctx, cancel := context.WithCancel(context.Background())
		DeferCleanup(cancel)
		launched := false
		n := startNotification(ctx, "0.3.2", dir, func(_ context.Context, current, cacheDir string) error {
			Expect(current).To(Equal("0.3.2"))
			Expect(cacheDir).To(Equal(dir))
			launched = true
			return nil
		})
		Expect(launched).To(BeTrue())
		Expect(n.Finish()).To(BeEmpty())
		cancel()
		// A child can fill the cache after the foreground command has finished.
		Expect(writeCache(filepath.Join(dir, "update-check.json"), notificationCache{CheckedAt: time.Now(), Latest: "v0.3.3"})).To(BeTrue())
		next := startNotification(context.Background(), "0.3.2", dir, func(context.Context, string, string) error {
			Fail("a fresh cache must not launch another helper")
			return nil
		})
		Expect(next.Finish()).To(Equal("DevEx 0.3.3 is available (you have 0.3.2). Run 'devex self-update'."))
	})
	It("uses a completed negative cache without launching another helper", func() {
		dir := GinkgoT().TempDir()
		Expect(writeCache(filepath.Join(dir, "update-check.json"), notificationCache{CheckedAt: time.Now()})).To(BeTrue())
		n := startNotification(context.Background(), "0.3.2", dir, func(context.Context, string, string) error {
			Fail("a completed failed lookup must be cached too")
			return nil
		})
		Expect(n.Finish()).To(BeEmpty())
	})
	It("does not launch when already cancelled", func() {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		Expect(startNotification(ctx, "0.3.2", GinkgoT().TempDir(), func(context.Context, string, string) error {
			Fail("cancelled invocations must not launch a helper")
			return nil
		})).To(BeNil())
	})
})
