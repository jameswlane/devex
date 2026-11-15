package bootstrap

import (
	"bytes"
	"context"
	"fmt"
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
		script := fmt.Sprintf(`#!/bin/sh
if [ "${1-}" = --plugin-info ]; then
 echo '{"name":"package-manager-apt","version":"1.0.0"}'
 exit 0
fi
if [ ! -f %q ]; then
 touch %q
 echo 'E: Failed to fetch https://repo/git.deb 503 Service Unavailable' >&2
 exit 100
fi
printf '{"installed":true}\n'
`, count, count)
		Expect(os.WriteFile(filepath.Join(dir, "devex-plugin-package-manager-apt"), []byte(script), 0700)).To(Succeed())
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
		script := fmt.Sprintf(`#!/bin/sh
if [ "${1-}" = --plugin-info ]; then
 echo '{"name":"package-manager-apt","version":"1.0.0"}'
 exit 0
fi
echo attempt >> %q
echo 'E: Failed to fetch https://repo/git.deb 503 Service Unavailable' >&2
exit 100
`, count)
		Expect(os.WriteFile(filepath.Join(dir, "devex-plugin-package-manager-apt"), []byte(script), 0700)).To(Succeed())
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
		script := fmt.Sprintf(`#!/bin/sh
if [ "${1-}" = --plugin-info ]; then
 echo '{"name":"package-manager-dnf","version":"1.0.0"}'
 exit 0
fi
echo attempt >> %q
if [ "$(wc -l < %q)" -eq 1 ]; then
 echo '>>> Status code: 503 for http://repo/git.rpm (IP: 127.0.0.1)' >&2
 echo 'Failed to download packages' >&2
 echo 'Error: exit status 1' >&2
 exit 1
fi
`, count, count)
		Expect(os.WriteFile(filepath.Join(dir, "devex-plugin-package-manager-dnf"), []byte(script), 0700)).To(Succeed())
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
