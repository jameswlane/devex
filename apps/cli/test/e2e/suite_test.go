//go:build e2e

package e2e_test

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var sandbox suiteEnvironment

func TestE2E(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "DevEx sandboxed Linux E2E suite")
}

var _ = BeforeSuite(func() {
	// Check authorization before creating artifacts, directories, or subprocesses.
	Expect(os.Getenv("DEVEX_E2E_SANDBOX")).To(Equal("1"), "refusing to mutate HOME without DEVEX_E2E_SANDBOX=1")
	Expect(runtime.GOOS).To(Equal("linux"), "phase 1 requires a Linux sandbox")
	Expect(GinkgoParallelProcess()).To(Equal(1), "the shared HOME suite must run serially")
	suiteConfig, _ := GinkgoConfiguration()
	Expect(suiteConfig.ParallelTotal).To(Equal(1), "do not run this stateful suite with --ginkgo.parallel")
	var err error
	sandbox.home, err = os.UserHomeDir()
	Expect(err).NotTo(HaveOccurred(), "resolve sandbox HOME")
	Expect(filepath.IsAbs(sandbox.home)).To(BeTrue(), "HOME must be absolute")
	dirs := sandbox.paths(sandbox.home, nil)
	for _, path := range []string{filepath.Join(sandbox.home, ".devex"), dirs.Config, dirs.Data, dirs.State, dirs.Cache} {
		_, err = os.Lstat(path)
		Expect(os.IsNotExist(err)).To(BeTrue(), "sandbox HOME must start without %s (got %v)", path, err)
	}
	sandbox.binary = envDefault("DEVEX_E2E_BINARY", "devex")
	sandbox.registry = envDefault("DEVEX_E2E_REGISTRY_URL", "https://registry.devex.sh")
	sandbox.desktop = envDefault("DEVEX_E2E_DESKTOP", "none")
	Expect(sandbox.desktop).To(BeElementOf("gnome", "kde", "none"), "DEVEX_E2E_DESKTOP must be gnome, kde, or none")
	if sandbox.desktop != "gnome" {
		// The CLI also detects desktops from running processes, not only XDG_CURRENT_DESKTOP.
		Expect(processRunning("gnome-session")).To(BeFalse(), "a gnome-session process is running; DEVEX_E2E_DESKTOP=%s needs a sandbox without a GNOME session", sandbox.desktop)
	}
	if sandbox.desktop != "kde" {
		for _, name := range []string{"kded4", "kded5"} {
			Expect(processRunning(name)).To(BeFalse(), "a %s process is running; DEVEX_E2E_DESKTOP=%s needs a sandbox without a KDE session", name, sandbox.desktop)
		}
	}
	sandbox.artifacts = artifactDirectory()
	sandbox.homes = []string{sandbox.home}
	sandbox.work, err = os.MkdirTemp("", "devex-e2e-work-")
	Expect(err).NotTo(HaveOccurred(), "create isolated CLI working directory")
})

var _ = AfterSuite(func() {
	if sandbox.work != "" {
		Expect(os.RemoveAll(sandbox.work)).To(Succeed(), "remove suite working directory")
	}
})

var _ = ReportAfterEach(func(report SpecReport) {
	if report.Failed() && sandbox.artifacts != "" {
		sandbox.collectArtifacts(report.LeafNodeText)
	}
})
