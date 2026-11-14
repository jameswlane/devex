package installers_test

import (
	"context"
	"io"
	"os"
	"path/filepath"

	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/installers"
	"github.com/jameswlane/devex/apps/cli/internal/types"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Organization policy non-interactive installation", func() {
	var home, policyPath string

	BeforeEach(func() {
		home = GinkgoT().TempDir()
		GinkgoT().Setenv("HOME", home)
		teamDir := filepath.Join(home, "team")
		GinkgoT().Setenv("DEVEX_TEAM_CONFIG_DIR", teamDir)
		policyPath = filepath.Join(teamDir, "policy.yaml")
		Expect(os.MkdirAll(teamDir, 0o750)).To(Succeed())
		installers.EnableTestMode()
		DeferCleanup(installers.DisableTestMode)
	})

	writePolicy := func(contents string) {
		Expect(os.WriteFile(policyPath, []byte(contents), 0o600)).To(Succeed())
	}

	app := func() types.AppConfig {
		return types.AppConfig{BaseConfig: types.BaseConfig{Name: "editor"}, InstallMethod: "apt", InstallCommand: "editor"}
	}

	It("refuses a denied app and runs an allowed app", func() {
		settings := config.CrossPlatformSettings{HomeDir: home}
		writePolicy("version: '1'\napplications:\n  deny: [editor]\n")
		Expect(installers.InstallApp(context.Background(), app(), settings, nil)).To(MatchError(ContainSubstring("applications.deny")))

		writePolicy("version: '1'\napplications:\n  allow: [editor]\n")
		Expect(installers.InstallApp(context.Background(), app(), settings, nil)).To(Succeed())
	})

	It("reports the executing policy on stderr", func() {
		writePolicy("version: '2026.11.1'\nname: acme\n")
		reader, writer, err := os.Pipe()
		Expect(err).NotTo(HaveOccurred())
		original := os.Stderr
		os.Stderr = writer
		DeferCleanup(func() { os.Stderr = original; _ = reader.Close(); _ = writer.Close() })

		installErr := installers.InstallApp(context.Background(), app(), config.CrossPlatformSettings{HomeDir: home}, nil)
		Expect(writer.Close()).To(Succeed())
		os.Stderr = original
		output, err := io.ReadAll(reader)
		Expect(err).NotTo(HaveOccurred())
		Expect(installErr).NotTo(HaveOccurred())
		Expect(string(output)).To(ContainSubstring("Applying organization policy acme 2026.11.1 (" + policyPath + ")"))
	})
})
