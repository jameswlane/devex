package devcontainer_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/jameswlane/devex/apps/cli/internal/devcontainer"
	"github.com/jameswlane/devex/apps/cli/internal/types"
)

func apt(name string, packages ...string) types.CrossPlatformApp {
	command := name
	if len(packages) > 0 {
		command = packages[0]
	}
	return types.CrossPlatformApp{Name: name, Linux: types.OSConfig{InstallMethod: "apt", InstallCommand: command}}
}

var _ = Describe("Devcontainer generation", func() {
	It("maps runtimes to Features and plain apt packages to one layer", func() {
		result, err := devcontainer.Generate([]types.CrossPlatformApp{{Name: "Go"}, {Name: "node.js"}, apt("curl"), apt("ripgrep"), apt("tools", "jq")}, devcontainer.Options{})
		Expect(err).NotTo(HaveOccurred())
		Expect(result.Features).To(Equal([]string{"ghcr.io/devcontainers/features/go:1", "ghcr.io/devcontainers/features/node:1"}))
		Expect(result.AptPackages).To(Equal([]string{"curl", "jq", "ripgrep"}))
		Expect(result.Skipped).To(BeEmpty())
		Expect(result.Dockerfile).To(ContainSubstring("FROM " + devcontainer.DefaultImage))
		Expect(result.Dockerfile).To(ContainSubstring("apt-get install -y --no-install-recommends"))
		Expect(result.Spec.RemoteUser).To(Equal("vscode"))
	})

	DescribeTable("leaves host-only applications out with the reason",
		func(app types.CrossPlatformApp, reason string) {
			result, err := devcontainer.Generate([]types.CrossPlatformApp{app}, devcontainer.Options{})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Skipped).To(ConsistOf(devcontainer.Skipped{App: app.Name, Reason: reason}))
			Expect(result.AptPackages).To(BeEmpty())
		},
		Entry("GUI application", apt("Ghostty"), "desktop application"),
		Entry("desktop-specific", types.CrossPlatformApp{Name: "ext", DesktopEnvironments: []string{"gnome"}, Linux: types.OSConfig{InstallMethod: "apt", InstallCommand: "ext"}}, "desktop application"),
		Entry("flatpak", types.CrossPlatformApp{Name: "f", Linux: types.OSConfig{InstallMethod: "flatpak", InstallCommand: "org.x"}}, "desktop application (flatpak)"),
		Entry("snap", types.CrossPlatformApp{Name: "s", Linux: types.OSConfig{InstallMethod: "snap", InstallCommand: "s"}}, "snap packages need snapd, which containers do not run"),
		Entry("third-party repository", types.CrossPlatformApp{Name: "r", Linux: types.OSConfig{InstallMethod: "apt", InstallCommand: "r", AptSources: []types.AptSource{{SourceRepo: "deb x"}}}}, "needs a third-party apt repository; add the repository to the Dockerfile yourself"),
		Entry("hooks", types.CrossPlatformApp{Name: "h", Linux: types.OSConfig{InstallMethod: "apt", InstallCommand: "h", PostInstall: []types.InstallCommand{{Shell: "echo"}}}}, "has setup hooks that DevEx runs on the host"),
		Entry("other installer", types.CrossPlatformApp{Name: "m", Linux: types.OSConfig{InstallMethod: "curlpipe", InstallCommand: "https://x"}}, "installed with curlpipe; add it to the Dockerfile or a Feature yourself"),
		Entry("no Linux definition", types.CrossPlatformApp{Name: "mac"}, "no Linux install definition"),
		Entry("shell syntax in the command", apt("weird", "pkg && rm -rf /"), "its apt install command is not a plain package list"),
	)

	It("rejects invalid image references", func() {
		_, err := devcontainer.Generate(nil, devcontainer.Options{Image: "ubuntu; rm -rf /"})
		Expect(err).To(MatchError(ContainSubstring("invalid image")))
		Expect(devcontainer.ValidateImage("mcr.microsoft.com/devcontainers/base:debian-12")).To(Succeed())
	})

	It("validates the devcontainer.json it writes", func() {
		result, err := devcontainer.Generate([]types.CrossPlatformApp{{Name: "Rust"}}, devcontainer.Options{Name: "demo"})
		Expect(err).NotTo(HaveOccurred())
		data, err := result.Marshal()
		Expect(err).NotTo(HaveOccurred())
		Expect(devcontainer.Validate(data)).To(Succeed())
		Expect(devcontainer.Validate([]byte(`{"name":"x","build":{"dockerfile":"Dockerfile"},"features":{"ghcr.io/evil/feature:1":{}},"remoteUser":"vscode"}`))).To(MatchError(ContainSubstring("unknown feature")))
		Expect(devcontainer.Validate([]byte(`{"name":"x","build":{"dockerfile":"Dockerfile"},"mounts":["source=/,target=/host"],"remoteUser":"vscode"}`))).To(HaveOccurred())
		Expect(devcontainer.Validate([]byte(`{"build":{}}`))).To(HaveOccurred())
	})

	It("builds the generated Dockerfile in a disposable container", Label("docker"), func(ctx SpecContext) {
		if _, err := exec.LookPath("docker"); err != nil {
			Skip("docker is not installed")
		}
		if err := exec.CommandContext(ctx, "docker", "info").Run(); err != nil {
			Skip("docker is not running")
		}
		result, err := devcontainer.Generate([]types.CrossPlatformApp{apt("curl")}, devcontainer.Options{Image: "debian:12-slim"})
		Expect(err).NotTo(HaveOccurred())
		dir := GinkgoT().TempDir()
		Expect(os.WriteFile(filepath.Join(dir, "Dockerfile"), []byte(result.Dockerfile), 0o600)).To(Succeed())
		tag := "devex-devcontainer-test:" + time.Now().Format("150405")
		DeferCleanup(func() { _ = exec.Command("docker", "rmi", "-f", tag).Run() })
		build := exec.CommandContext(ctx, "docker", "build", "-q", "-t", tag, dir)
		output, err := build.CombinedOutput()
		Expect(err).NotTo(HaveOccurred(), string(output))
		run := exec.CommandContext(ctx, "docker", "run", "--rm", tag, "curl", "--version")
		output, err = run.CombinedOutput()
		Expect(err).NotTo(HaveOccurred(), string(output))
		Expect(string(output)).To(ContainSubstring("curl"))
	}, SpecTimeout(10*time.Minute))
})
