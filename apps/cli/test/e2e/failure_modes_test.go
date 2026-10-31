//go:build e2e

package e2e_test

import (
	"os"
	"path/filepath"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Startup dependency checks", func() {
	It("explains a missing required tool on stderr instead of failing silently", func() {
		home := sandbox.isolatedHome()
		output, err := sandbox.run(home, sandbox.binary, 30*time.Second, map[string]string{"PATH": pathWithout("unzip")}, "--version")
		Expect(err).To(HaveOccurred(), "the CLI must refuse to start without unzip:\n%s", output)
		Expect(output).To(ContainSubstring("missing dependency: unzip"), "the error must name the missing tool")
		Expect(output).To(ContainSubstring("apt-get install"), "the error must say how to install it")
	})
})

var _ = Describe("Plugin bootstrap failure modes", func() {
	It("degrades clearly without downloading when offline and no plugins are installed", func() {
		home := sandbox.isolatedHome()
		// Leave the plugin location absent: this is the fresh-home offline case and
		// lets bootstrap explain that plugin discovery is unavailable.
		output, err := sandbox.run(home, sandbox.binary, 30*time.Second, map[string]string{"DEVEX_OFFLINE": "1"}, "status", "--all", "--no-tui", "--verbose")
		Expect(err).NotTo(HaveOccurred(), "offline status should degrade successfully:\n%s", output)
		Expect(output).To(ContainSubstring("Failed to bootstrap plugins"), "offline bootstrap must explain missing plugins")
		Expect(output).To(ContainSubstring("failed to read plugin directory"), "offline diagnostic must identify the empty plugin location")
		Expect(output).To(ContainSubstring("No applications to check"), "status must still describe the empty application state")
		_, statErr := os.Stat(filepath.Join(home, ".devex/plugins"))
		Expect(os.IsNotExist(statErr)).To(BeTrue(), "offline bootstrap must not create or download plugins")

		// Also exercise an existing, empty plugin directory. Discovery succeeds
		// here and status clearly reports the empty app state without a warning.
		Expect(os.MkdirAll(filepath.Join(home, ".devex/plugins"), 0750)).To(Succeed())
		output, err = sandbox.run(home, sandbox.binary, 30*time.Second, map[string]string{"DEVEX_OFFLINE": "1"}, "status", "--all", "--no-tui", "--verbose")
		Expect(err).NotTo(HaveOccurred(), "offline status with an empty plugin directory must finish: %s", output)
		Expect(output).To(ContainSubstring("No applications to check"), "empty offline state must be clear")
		entries, readErr := os.ReadDir(filepath.Join(home, ".devex/plugins"))
		Expect(readErr).NotTo(HaveOccurred())
		Expect(entries).To(BeEmpty(), "offline mode must leave the plugin directory empty")
	})

	It("reports an unreachable registry and finishes within its timeout", func() {
		home := sandbox.isolatedHome()
		output, err := sandbox.run(home, sandbox.binary, 45*time.Second, map[string]string{"DEVEX_PLUGIN_REGISTRY_URL": "http://127.0.0.1:1"}, "status", "--all", "--no-tui", "--verbose")
		Expect(err).NotTo(HaveOccurred(), "registry failure should degrade successfully:\n%s", output)
		Expect(output).To(ContainSubstring("Failed to download plugin package-manager-apt"), "unreachable registry must produce a clear download diagnostic")
		Expect(output).To(ContainSubstring("127.0.0.1:1"), "registry diagnostic must identify the unreachable endpoint")
		Expect(output).To(ContainSubstring("No applications to check"), "status must remain usable after download failure")
		entries, readErr := os.ReadDir(filepath.Join(home, ".devex/plugins"))
		Expect(readErr).NotTo(HaveOccurred(), "bootstrap creates the empty plugin directory")
		Expect(entries).To(BeEmpty(), "failed downloads must leave no plugins or partial downloads")
	})
})

var _ = Describe("Command execution failure modes", func() {
	It("prints the error message exactly once on stderr and exits nonzero", func() {
		home := sandbox.isolatedHome()
		output, err := sandbox.run(home, sandbox.binary, 30*time.Second, nil, "notacommand")
		Expect(err).To(HaveOccurred(), "unknown command must fail")
		Expect(strings.Count(output, "Error:")).To(Equal(1), "error message must be printed exactly once")
		Expect(output).To(ContainSubstring(`unknown command "notacommand" for "devex"`))
		Expect(output).To(ContainSubstring("Run 'devex --help' for usage."))
		Expect(output).NotTo(ContainSubstring("Error executing root command"))
	})
})
