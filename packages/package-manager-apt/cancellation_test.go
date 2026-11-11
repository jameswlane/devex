//go:build unix

package main_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	main "github.com/jameswlane/devex/packages/package-manager-apt"
	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
	"github.com/jameswlane/devex/packages/plugin-sdk/sdktest"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("APT native cancellation", func() {
	It("cancels a running operation and terminates the native process", func() {
		sdktest.NewRecorder(GinkgoT(), "apt")
		started := filepath.Join(GinkgoT().TempDir(), "started")
		original := sdk.NewCommandContext
		sdk.NewCommandContext = func(ctx context.Context, name string, args ...string) *exec.Cmd {
			if filepath.Base(name) == "apt" {
				return exec.CommandContext(ctx, "/bin/sh", "-c", `echo $$ > "$1"; exec /bin/sleep 60`, "fixture", started)
			}
			return original(ctx, name, args...)
		}
		defer func() { sdk.NewCommandContext = original }()

		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		done := make(chan error, 1)
		go func() {
			done <- main.NewAPTPlugin().ExecuteContext(ctx, "search", []string{"git"})
		}()
		Eventually(func() bool {
			_, err := os.Stat(started)
			return err == nil
		}, 5*time.Second).Should(BeTrue())
		data, err := os.ReadFile(started)
		Expect(err).NotTo(HaveOccurred())
		pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
		Expect(err).NotTo(HaveOccurred())
		defer func() { _ = syscall.Kill(pid, syscall.SIGKILL) }()

		cancel()

		Eventually(done, 3*time.Second).Should(Receive(MatchError(context.Canceled)))
		Eventually(func() error { return syscall.Kill(pid, 0) }, 3*time.Second).Should(Equal(syscall.ESRCH))
	})

	It("does not launch an operation after cancellation", func() {
		recorder := sdktest.NewRecorder(GinkgoT(), "apt")
		ctx, cancel := context.WithCancel(context.Background())
		cancel()

		Expect(main.NewAPTPlugin().ExecuteContext(ctx, "search", []string{"git"})).To(MatchError(context.Canceled))
		Expect(recorder.Calls("apt")).To(BeEmpty())
		Expect(recorder.Calls("sudo")).To(BeEmpty())
	})
})
