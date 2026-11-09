package platform

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Diagnostic tool versions", func() {
	BeforeEach(func() {
		if runtime.GOOS == "windows" {
			Skip("POSIX probe fixtures")
		}
	})
	It("does not probe tools unless explicitly requested", func() {
		dir := GinkgoT().TempDir()
		GinkgoT().Setenv("PATH", dir)
		Expect(os.WriteFile(filepath.Join(dir, "node"), []byte("#!/bin/sh\nexit 42\n"), 0700)).To(Succeed())
		Expect(GatherSystemInfo("test").PackageManagers).To(BeEmpty())
	})
	It("collects only selected tools and normalizes successful output", func() {
		dir := GinkgoT().TempDir()
		GinkgoT().Setenv("PATH", dir)
		for _, tool := range []string{"node", "npm"} {
			Expect(os.WriteFile(filepath.Join(dir, tool), []byte("#!/bin/sh\nprintf 'v1.2.3\\nextra\\n'\n"), 0700)).To(Succeed())
		}
		Expect(GatherSystemInfoForTools(context.Background(), "test", "node", "node", "unknown").PackageManagers).To(Equal(map[string]string{"node": "v1.2.3"}))
	})
	It("bounds slow probes concurrently and omits failed versions", func() {
		dir := GinkgoT().TempDir()
		GinkgoT().Setenv("PATH", dir)
		for _, tool := range []string{"node", "npm", "go"} {
			Expect(os.WriteFile(filepath.Join(dir, tool), []byte("#!/bin/sh\nexec /bin/sleep 5\n"), 0700)).To(Succeed())
		}
		start := time.Now()
		Expect(GatherSystemInfoForTools(context.Background(), "test", "node", "npm", "go").PackageManagers).To(BeEmpty())
		Expect(time.Since(start)).To(BeNumerically("<", 1200*time.Millisecond))
	})
	It("honors caller cancellation", func() {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		Expect(GatherSystemInfoForTools(ctx, "test", "node").PackageManagers).To(BeEmpty())
	})
})
