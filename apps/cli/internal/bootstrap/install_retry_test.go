package bootstrap

import (
	"bytes"
	"context"
	"os"
	"path/filepath"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/viper"
)

var _ = Describe("APT and DNF retry execution boundary", func() {
	BeforeEach(func() {
		viper.Reset()
		DeferCleanup(viper.Reset)
		viper.Set("install-retry-delay", "1ns")
		viper.Set("install-retry-max-delay", "1ns")
	})
	It("retries native stderr failures through the real manager and keeps diagnostics off stdout", func() {
		dir := GinkgoT().TempDir()
		count := filepath.Join(dir, "attempts")
		writeFakePlugin(dir, fakePluginConfig{Name: "package-manager-apt", Mode: "apt-retry", MarkerPath: count})
		bootstrap := &PluginBootstrap{manager: sdk.NewExecutableManager(dir)}
		var stdout, stderr bytes.Buffer
		Expect(bootstrap.ExecutePluginWithIO(context.Background(), "package-manager-apt", []string{"install", "git"}, sdk.OperationIO{Stdout: &stdout, Stderr: &stderr})).To(Succeed())
		Expect(stdout.String()).To(Equal("{\"installed\":true}\n"))
		Expect(stderr.String()).To(ContainSubstring("503 Service Unavailable"))
		Expect(stderr.String()).To(ContainSubstring("attempt 1/3 failed"))
	})
	DescribeTable("does not retry unapproved operation shapes", func(args []string) {
		dir := GinkgoT().TempDir()
		count := filepath.Join(dir, "attempts")
		writeFakePlugin(dir, fakePluginConfig{Name: "package-manager-apt", Mode: "apt-fail", MarkerPath: count})
		bootstrap := &PluginBootstrap{manager: sdk.NewExecutableManager(dir)}
		var stderr bytes.Buffer
		Expect(bootstrap.ExecutePluginWithIO(context.Background(), "package-manager-apt", args, sdk.OperationIO{Stderr: &stderr})).To(HaveOccurred())
		data, err := os.ReadFile(count)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(data)).To(Equal("attempt\n"))
		Expect(stderr.String()).NotTo(ContainSubstring("retrying"))
	}, Entry("removal", []string{"remove", "git"}), Entry("update", []string{"update"}), Entry("destructive suffix", []string{"install", "git-"}))
	It("retries a dnf5 transient download failure once and labels the notice", func() {
		dir := GinkgoT().TempDir()
		count := filepath.Join(dir, "attempts")
		writeFakePlugin(dir, fakePluginConfig{Name: "package-manager-dnf", Mode: "dnf-retry", MarkerPath: count})
		bootstrap := &PluginBootstrap{manager: sdk.NewExecutableManager(dir)}
		var stderr bytes.Buffer
		Expect(bootstrap.ExecutePluginWithIO(context.Background(), "package-manager-dnf", []string{"install", "git"}, sdk.OperationIO{Stderr: &stderr})).To(Succeed())
		data, err := os.ReadFile(count)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(data)).To(Equal("attempt\nattempt\n"))
		Expect(stderr.String()).To(ContainSubstring("Status code: 503"))
		Expect(stderr.String()).To(ContainSubstring("DNF install attempt 1/3 failed"))
	})
})
