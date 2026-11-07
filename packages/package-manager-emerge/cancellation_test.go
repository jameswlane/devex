//go:build unix

package main_test

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	main "github.com/jameswlane/devex/packages/package-manager-emerge"
	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
	"github.com/jameswlane/devex/packages/plugin-sdk/sdktest"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("native cancellation", func() {
	It("cancels emerge after the native process has started", func() {
		sdktest.NewRecorder(GinkgoT(), "emerge")
		started := filepath.Join(GinkgoT().TempDir(), "started")
		original := sdk.NewCommandContext
		sdk.NewCommandContext = func(ctx context.Context, name string, args ...string) *exec.Cmd {
			if filepath.Base(name) == "emerge" {
				return exec.CommandContext(ctx, "/bin/sh", "-c", `echo started > "$1"; exec /bin/sleep 60`, "fixture", started)
			}
			return original(ctx, name, args...)
		}
		defer func() { sdk.NewCommandContext = original }()
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		done := make(chan error, 1)
		go func() { done <- main.NewEmergePlugin().ExecuteContext(ctx, "search", []string{"git"}) }()
		Eventually(func() bool { _, err := os.Stat(started); return err == nil }, 5*time.Second).Should(BeTrue())
		cancel()
		Eventually(done, 3*time.Second).Should(Receive(MatchError(context.Canceled)))
	})

	It("forwards cancellation through the installed plugin executable to native emerge", func() {
		if runtime.GOOS != "linux" {
			Skip("Linux process signaling fixture")
		}
		root := GinkgoT().TempDir()
		binary := filepath.Join(root, "plugin")
		build := exec.Command("go", "build", "-o", binary, ".")
		output, err := build.CombinedOutput()
		Expect(err).NotTo(HaveOccurred(), string(output))
		bin := filepath.Join(root, "bin")
		Expect(os.Mkdir(bin, 0o700)).To(Succeed())
		started := filepath.Join(root, "started")
		Expect(os.WriteFile(filepath.Join(bin, "emerge"), []byte("#!/bin/sh\necho $$ > \"$STARTED\"\nexec /bin/sleep 60\n"), 0o700)).To(Succeed())
		// This fixture exercises argv and signal forwarding without real elevation.
		Expect(os.WriteFile(filepath.Join(bin, "sudo"), []byte("#!/bin/sh\n[ \"$1\" = -n ] && shift\n[ \"$1\" = -- ] && shift\nexec \"$@\"\n"), 0o700)).To(Succeed())
		GinkgoT().Setenv("PATH", bin+":"+os.Getenv("PATH"))
		GinkgoT().Setenv("STARTED", started)
		GinkgoT().Setenv(sdk.UnattendedEnv, "1")
		manager := sdk.NewExecutableManager(filepath.Join(root, "plugins"))
		Expect(manager.InstallPlugin(binary, "package-manager-emerge")).To(Succeed())
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		var stderr bytes.Buffer
		done := make(chan error, 1)
		go func() {
			done <- manager.ExecutePluginWithIO(ctx, "package-manager-emerge", []string{"install", "git"}, sdk.OperationIO{Stderr: &stderr})
		}()
		Eventually(func() bool { _, err := os.Stat(started); return err == nil }, 10*time.Second).Should(BeTrue())
		data, err := os.ReadFile(started)
		Expect(err).NotTo(HaveOccurred())
		pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
		Expect(err).NotTo(HaveOccurred())
		defer func() { _ = syscall.Kill(pid, syscall.SIGKILL) }()
		cancel()
		Eventually(done, 5*time.Second).Should(Receive(MatchError(context.Canceled)))
		Expect(stderr.String()).To(ContainSubstring("context canceled"))
		Eventually(func() error { return syscall.Kill(pid, 0) }, 3*time.Second).Should(Equal(syscall.ESRCH))
	})
})
