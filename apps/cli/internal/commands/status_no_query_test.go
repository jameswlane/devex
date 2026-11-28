package commands_test

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v3"

	"github.com/jameswlane/devex/apps/cli/internal/commands"
	"github.com/jameswlane/devex/apps/cli/internal/commands/status"
	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/datastore"
	"github.com/jameswlane/devex/apps/cli/internal/datastore/repository"
	"github.com/jameswlane/devex/apps/cli/internal/installers"
	"github.com/jameswlane/devex/apps/cli/internal/types"
)

var _ = Describe("Status for apps without a package query", func() {
	const binary = "devex-653-probe"
	var (
		repo     types.Repository
		settings config.CrossPlatformSettings
		binDir   string
	)

	runStatus := func(app string) (status.AppStatus, error) {
		cmd := commands.NewStatusCmd(repo, settings)
		var out bytes.Buffer
		cmd.SetOut(&out)
		cmd.SetErr(&bytes.Buffer{})
		cmd.SetArgs([]string{"--app", app, "--format", "json"})
		err := cmd.Execute()
		var results []status.AppStatus
		Expect(json.Unmarshal(out.Bytes(), &results)).To(Succeed(), out.String())
		Expect(results).To(HaveLen(1))
		return results[0], err
	}

	BeforeEach(func() {
		home := GinkgoT().TempDir()
		GinkgoT().Setenv("HOME", home)
		GinkgoT().Setenv("USERPROFILE", home)
		// An empty PATH keeps a real binary of the same name out of the check.
		GinkgoT().Setenv("PATH", GinkgoT().TempDir())
		binDir = filepath.Join(home, ".local", "bin")

		db := datastore.NewInMemorySQLite()
		DeferCleanup(func() { Expect(db.Close()).To(Succeed()) })
		repo = repository.NewRepository(db)
		curlpipe := func(installed string) types.OSConfig {
			return types.OSConfig{InstallMethod: "curlpipe", DownloadURL: "https://example.com/install.sh", InstalledBinary: installed}
		}
		settings = config.CrossPlatformSettings{ApplicationOverrides: []types.CrossPlatformApp{
			{Name: "probe-agent", Linux: curlpipe(binary), MacOS: curlpipe(binary), Windows: curlpipe(binary)},
			// snap has no package-specific installed query in the CLI.
			{Name: "no-query", AllPlatforms: types.OSConfig{InstallMethod: "snap", InstallCommand: "no-query"}},
		}}
	})

	It("reports the app missing before install and after removal of its binary", func() {
		Expect(repo.AddApp("probe-agent")).To(Succeed())

		result, err := runStatus("probe-agent")
		Expect(err).To(HaveOccurred())
		Expect(result.Status).To(Equal("not_installed"))
		Expect(result.Installed).To(BeFalse())

		name := binary
		if runtime.GOOS == "windows" {
			name += ".exe"
		}
		path := filepath.Join(binDir, name)
		Expect(os.MkdirAll(binDir, 0o755)).To(Succeed())
		Expect(os.WriteFile(path, []byte("#!/bin/sh\n"), 0o755)).To(Succeed()) //nolint:gosec // test executable
		result, err = runStatus("probe-agent")
		Expect(err).NotTo(HaveOccurred())
		Expect(result.Status).To(Equal("healthy"))
		Expect(result.Installed).To(BeTrue())

		Expect(os.Remove(path)).To(Succeed())
		result, err = runStatus("probe-agent")
		Expect(err).To(HaveOccurred())
		Expect(result.Status).To(Equal("not_installed"))
	})

	It("reports an app with no installed query and no installed_binary as unknown, not installed", func() {
		// Test mode provides an installer for snap whose IsInstalled always
		// says yes, as the real plugin installer does for methods without a
		// package query.
		installers.EnableTestMode()
		DeferCleanup(installers.DisableTestMode)
		Expect(repo.AddApp("no-query")).To(Succeed())

		result, err := runStatus("no-query")
		Expect(err).NotTo(HaveOccurred())
		Expect(result.Status).To(Equal("unknown"))
		Expect(result.Installed).To(BeFalse())
	})

	It("keeps --fix YAML output parseable for an app it cannot check", func() {
		installers.EnableTestMode()
		DeferCleanup(installers.DisableTestMode)
		Expect(repo.AddApp("no-query")).To(Succeed())
		cmd := commands.NewStatusCmd(repo, settings)
		cmd.SetArgs([]string{"--app", "no-query", "--fix", "--format", "yaml"})

		var err error
		output := captureStatusStdout(func() { err = cmd.Execute() })

		Expect(err).NotTo(HaveOccurred())
		var results []status.AppStatus
		Expect(yaml.Unmarshal([]byte(output), &results)).To(Succeed(), output)
		Expect(results).To(HaveLen(1))
		Expect(results[0].Status).To(Equal("unknown"))
	})
})
