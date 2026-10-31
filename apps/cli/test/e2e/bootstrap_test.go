//go:build e2e

package e2e_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Fresh sandbox CLI lifecycle", Ordered, func() {
	var plugins []string
	var original map[string]pluginState

	It("prints the CLI version without an interactive terminal", func() {
		// Cobra provides --version; there is no `version` subcommand.
		Expect(sandbox.cli("--version")).To(MatchRegexp(`(?m)^devex version \S+`), "CLI must print its build version")
	})

	It("bootstraps plugins and creates secure runtime directories", func() {
		sandbox.cli("list")
		dirs := sandbox.paths(sandbox.home, nil)
		for _, path := range []string{dirs.Data, dirs.Plugins(), dirs.State, dirs.Logs()} {
			info, err := os.Stat(path)
			Expect(err).NotTo(HaveOccurred(), "first run must create %s", path)
			Expect(info.IsDir()).To(BeTrue(), "%s must be a directory", path)
			Expect(info.Mode().Perm()&0002).To(BeZero(), "%s must not be world-writable", path)
		}
		database := filepath.Join(dirs.State, "datastore.db")
		info, err := os.Stat(database)
		Expect(err).NotTo(HaveOccurred(), "CLI initializes the SQLite datastore")
		Expect(info.Mode().IsRegular()).To(BeTrue(), "datastore must be a regular file")
		Expect(info.Mode().Perm()&0002).To(BeZero(), "datastore must not be world-writable")
		logs, err := filepath.Glob(filepath.Join(dirs.Logs(), "devex-*.log"))
		Expect(err).NotTo(HaveOccurred())
		Expect(logs).NotTo(BeEmpty(), "first run must write a CLI log")
		for _, path := range logs {
			info, err := os.Stat(path)
			Expect(err).NotTo(HaveOccurred())
			Expect(info.Mode().Perm()&0002).To(BeZero(), "log %s must not be world-writable", path)
		}
		// LoadCrossPlatformSettings only reads defaults. `setup`/`init` writes config;
		// list/version must not manufacture user configuration on an empty HOME.
		for _, path := range []string{dirs.Config, dirs.Defaults()} {
			_, err = os.Stat(path)
			Expect(os.IsNotExist(err)).To(BeTrue(), "plain CLI startup does not create configuration at %s", path)
		}
	})

	It("downloads exactly the platform plugins with registry SHA256 and SDK version metadata", func() {
		plugins = expectedPlugins()
		entries, err := os.ReadDir(sandbox.paths(sandbox.home, nil).Plugins())
		Expect(err).NotTo(HaveOccurred())
		installed := make([]string, 0, len(entries))
		for _, entry := range entries {
			if strings.HasPrefix(entry.Name(), "devex-plugin-") {
				installed = append(installed, strings.TrimPrefix(entry.Name(), "devex-plugin-"))
			}
		}
		Expect(installed).To(ConsistOf(plugins), "plugin set must match distro, available universal package managers, and DEVEX_E2E_DESKTOP=%s", sandbox.desktop)
		for _, desktop := range []string{"gnome", "kde"} {
			if desktop != sandbox.desktop {
				_, err := os.Stat(pluginPath(sandbox.home, "desktop-"+desktop))
				Expect(os.IsNotExist(err)).To(BeTrue(), "desktop-%s must be absent for %s", desktop, sandbox.desktop)
			}
		}
		for _, name := range plugins {
			By("verifying " + name)
			path := pluginPath(sandbox.home, name)
			info, err := os.Stat(path)
			Expect(err).NotTo(HaveOccurred(), "%s must exist", name)
			Expect(info.Mode().IsRegular()).To(BeTrue(), "%s must be a regular binary", name)
			Expect(info.Mode().Perm()&0111).NotTo(BeZero(), "%s must be executable", name)
			Expect(info.Mode().Perm()&0002).To(BeZero(), "%s must not be world-writable", name)
			Expect(checksum(path)).To(Equal(registryChecksum(name)), "%s SHA256 must match production registry for this OS/arch", name)
			// plugin-sdk HandleArgs exposes --plugin-info, not --version.
			output, err := sandbox.run(sandbox.home, path, 15*time.Second, nil, "--plugin-info")
			Expect(err).NotTo(HaveOccurred(), "%s --plugin-info failed: %s", name, output)
			var metadata struct {
				Name    string `json:"name"`
				Version string `json:"version"`
			}
			Expect(json.Unmarshal([]byte(output), &metadata)).To(Succeed(), "%s must return SDK JSON metadata", name)
			Expect(metadata.Name).To(Equal(name), "plugin identity must match %s", name)
			Expect(metadata.Version).NotTo(BeEmpty(), "%s must report its version", name)
		}
		original = snapshotPlugins(sandbox.home)
	})

	It("lists applications and reports status non-interactively", func() {
		sandbox.cli("list", "installed")
		Expect(sandbox.cli("status", "--all", "--no-tui")).To(ContainSubstring("No applications to check"), "fresh sandbox has no installed applications")
	})

	It("leaves plugin checksums, mtimes, modes, and directory contents unchanged on another run", func() {
		// Separate runs by more than coarse filesystem timestamp resolution.
		time.Sleep(1100 * time.Millisecond)
		sandbox.cli("list")
		Expect(snapshotPlugins(sandbox.home)).To(Equal(original), "repeat bootstrap must not download or replace any plugin")
	})
})
