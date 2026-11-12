package devcontainer_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

	Describe("the remote user", func() {
		It("relies on the official images' vscode user", func() {
			result, err := devcontainer.Generate([]types.CrossPlatformApp{apt("curl")}, devcontainer.Options{Image: "mcr.microsoft.com/devcontainers/base:debian-12"})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Spec.RemoteUser).To(Equal("vscode"))
			Expect(result.Dockerfile).NotTo(ContainSubstring("useradd"))
		})

		DescribeTable("is created in the Dockerfile when the base image may not provide it",
			func(options devcontainer.Options, user string) {
				result, err := devcontainer.Generate([]types.CrossPlatformApp{apt("curl")}, options)
				Expect(err).NotTo(HaveOccurred())
				Expect(result.Spec.RemoteUser).To(Equal(user))
				Expect(result.Dockerfile).To(ContainSubstring("if ! id -u " + user + " "))
				Expect(result.Dockerfile).To(ContainSubstring("--user-group " + user))
				// A base image may default to a non-root USER; apt needs root.
				Expect(strings.Index(result.Dockerfile, "USER root")).To(BeNumerically("<", strings.Index(result.Dockerfile, "apt-get")))
			},
			Entry("plain Debian", devcontainer.Options{Image: "debian:12-slim"}, "vscode"),
			Entry("plain Ubuntu", devcontainer.Options{Image: "ubuntu:24.04"}, "vscode"),
			Entry("another user on the default image", devcontainer.Options{RemoteUser: "dev"}, "dev"),
		)

		It("rejects root and malformed user names", func() {
			for _, user := range []string{"root", "Bad", "a b", "x;rm -rf /"} {
				_, err := devcontainer.Generate(nil, devcontainer.Options{RemoteUser: user})
				Expect(err).To(MatchError(ContainSubstring("invalid remote user")), user)
			}
		})
	})

	Context("in a disposable container", Label("docker"), func() {
		BeforeEach(func(ctx SpecContext) {
			if _, err := exec.LookPath("docker"); err != nil {
				Skip("docker is not installed")
			}
			if err := exec.CommandContext(ctx, "docker", "info").Run(); err != nil {
				Skip("docker is not running")
			}
		})
		// cleanup runs docker with a fresh, bounded context: the spec's own
		// context is already cancelled when cleanup runs.
		cleanup := func(args ...string) {
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()
			_ = exec.CommandContext(ctx, "docker", args...).Run()
		}

		DescribeTable("builds the generated Dockerfile and runs a tool as the declared remote user",
			func(ctx SpecContext, image string) {
				result, err := devcontainer.Generate([]types.CrossPlatformApp{apt("curl")}, devcontainer.Options{Image: image})
				Expect(err).NotTo(HaveOccurred())
				data, err := result.Marshal()
				Expect(err).NotTo(HaveOccurred())
				var spec devcontainer.Spec
				Expect(json.Unmarshal(data, &spec)).To(Succeed())
				Expect(spec.RemoteUser).NotTo(BeEmpty())

				dir := GinkgoT().TempDir()
				Expect(os.WriteFile(filepath.Join(dir, "Dockerfile"), []byte(result.Dockerfile), 0o600)).To(Succeed())
				tag := fmt.Sprintf("devex-devcontainer-test:%d", time.Now().UnixNano())
				DeferCleanup(cleanup, "rmi", "-f", tag)
				output, err := exec.CommandContext(ctx, "docker", "build", "-q", "-t", tag, dir).CombinedOutput()
				Expect(err).NotTo(HaveOccurred(), string(output))
				output, err = exec.CommandContext(ctx, "docker", "run", "--rm", "--user", spec.RemoteUser, tag, "sh", "-c", "id -un && curl --version").CombinedOutput()
				Expect(err).NotTo(HaveOccurred(), string(output))
				Expect(string(output)).To(HavePrefix(spec.RemoteUser + "\n"))
				Expect(string(output)).To(ContainSubstring("curl"))
			},
			Entry("default image", SpecTimeout(15*time.Minute), devcontainer.DefaultImage),
			Entry("custom Debian image", SpecTimeout(15*time.Minute), "debian:12-slim"),
		)

		DescribeTable("starts through the Dev Container CLI and executes as the remote user",
			func(ctx SpecContext, apps []types.CrossPlatformApp, image string) {
				if _, err := exec.LookPath("devcontainer"); err != nil {
					Skip("the devcontainer CLI is not installed")
				}
				result, err := devcontainer.Generate(apps, devcontainer.Options{Image: image})
				Expect(err).NotTo(HaveOccurred())
				data, err := result.Marshal()
				Expect(err).NotTo(HaveOccurred())
				workspace := GinkgoT().TempDir()
				Expect(os.MkdirAll(filepath.Join(workspace, ".devcontainer"), 0o755)).To(Succeed())
				Expect(os.WriteFile(filepath.Join(workspace, ".devcontainer", "devcontainer.json"), data, 0o600)).To(Succeed())
				Expect(os.WriteFile(filepath.Join(workspace, ".devcontainer", "Dockerfile"), []byte(result.Dockerfile), 0o600)).To(Succeed())
				DeferCleanup(func() {
					ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
					defer cancel()
					ids, _ := exec.CommandContext(ctx, "docker", "ps", "-aq", "--filter", "label=devcontainer.local_folder="+workspace).Output()
					for _, id := range strings.Fields(string(ids)) {
						cleanup("rm", "-f", id)
					}
				})
				output, err := exec.CommandContext(ctx, "devcontainer", "up", "--workspace-folder", workspace).CombinedOutput()
				Expect(err).NotTo(HaveOccurred(), string(output))
				output, err = exec.CommandContext(ctx, "devcontainer", "exec", "--workspace-folder", workspace, "id", "-un").CombinedOutput()
				Expect(err).NotTo(HaveOccurred(), string(output))
				Expect(strings.TrimSpace(string(output))).To(HaveSuffix(result.Spec.RemoteUser))
			},
			Entry("default image with a Feature", SpecTimeout(20*time.Minute), []types.CrossPlatformApp{{Name: "git"}, apt("curl")}, devcontainer.DefaultImage),
			Entry("custom Debian image with a Feature", SpecTimeout(20*time.Minute), []types.CrossPlatformApp{{Name: "git"}, apt("curl")}, "debian:12-slim"),
		)
	})
})
