package installers

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/jameswlane/devex/apps/cli/internal/types"
	"github.com/jameswlane/devex/packages/plugin-sdk/sdktest"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v3"
)

type snapProcessExecutor struct{ path string }

func (executor snapProcessExecutor) ExecutePluginWithContext(ctx context.Context, _ string, args []string) error {
	output, err := exec.CommandContext(ctx, executor.path, args...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("snap plugin: %w: %s", err, output)
	}
	return nil
}

var _ = Describe("Typed package installation", func() {
	It("carries legacy and structured definitions through the real Snap plugin to native argv", func() {
		path := filepath.Join(GinkgoT().TempDir(), "snap-plugin")
		build := exec.CommandContext(context.Background(), "go", "build", "-o", path, ".")
		build.Dir = "../../../../packages/package-manager-snap"
		// The suite isolates HOME; make its temporary module cache removable.
		build.Env = append(os.Environ(), "GOFLAGS="+os.Getenv("GOFLAGS")+" -modcacherw")
		output, err := build.CombinedOutput()
		Expect(err).NotTo(HaveOccurred(), string(output))
		recorder := sdktest.NewRecorder(GinkgoT(), "snap")
		installer := &PluginBasedInstaller{method: "snap", pluginBootstrap: snapProcessExecutor{path: path}}
		Expect(installer.Install("sudo snap install nvim --classic", nil)).To(Succeed())
		Expect(installer.InstallPackages(context.Background(), "", []string{"nvim", "ghostty"}, []string{"--classic"})).To(Succeed())
		Expect(installer.InstallPackages(context.Background(), `nvim --channel "latest/edge"`, nil, nil)).To(Succeed())
		Expect(recorder.Calls("snap")).To(HaveLen(3))
		calls := recorder.Calls("snap")
		Expect(calls[0].Args).To(Equal([]string{"install", "--classic", "nvim"}))
		Expect(calls[1].Args).To(Equal([]string{"install", "--classic", "nvim", "ghostty"}))
		Expect(calls[2].Args).To(Equal([]string{"install", "--channel=latest/edge", "nvim"}))
	})

	DescribeTable("records native argv without losing identifiers or option values", func(method, command string, packages, options, want []string) {
		recorder := sdktest.NewRecorder(GinkgoT(), "intent")
		installer := &PluginBasedInstaller{method: method, pluginBootstrap: intentExecutor{}, ctx: context.Background()}
		Expect(installer.InstallPackages(context.Background(), command, packages, options)).To(Succeed())
		Expect(recorder.Calls("intent")).To(Equal([]sdktest.Call{{Args: want}}))
	},
		Entry("legacy Snap classic", "snap", "sudo snap install nvim --classic", nil, nil, []string{"install", "--classic", "nvim"}),
		Entry("multiple identifiers", "snap", "", []string{"nvim", "ghostty"}, []string{"--classic"}, []string{"install", "--classic", "nvim", "ghostty"}),
		Entry("separate channel value", "snap", `nvim --channel "latest/edge"`, nil, nil, []string{"install", "--channel=latest/edge", "nvim"}),
		Entry("literal option value", "snap", "", []string{"nvim"}, []string{"--channel", `track with spaces;$(literal)'"`}, []string{"install", `--channel=track with spaces;$(literal)'"`, "nvim"}),
		Entry("literal backslashes", "deb", `"C:\Program Files\archive.deb"`, nil, nil, []string{"install", `C:\Program Files\archive.deb`}),
		Entry("quoted archive", "deb", `sudo apt install "./archive with spaces.deb"`, nil, nil, []string{"install", "./archive with spaces.deb"}),
		Entry("typed manager name is a package", "apt", "", []string{"apt", "sudo"}, nil, []string{"install", "apt", "sudo"}),
	)
	DescribeTable("rejects malformed requests before execution", func(command string, packages, options []string) {
		recorder := sdktest.NewRecorder(GinkgoT(), "intent")
		installer := &PluginBasedInstaller{method: "snap", pluginBootstrap: intentExecutor{}}
		Expect(installer.InstallPackages(context.Background(), command, packages, options)).To(HaveOccurred())
		Expect(recorder.Calls("intent")).To(BeEmpty())
	},
		Entry("missing channel value", "nvim --channel", nil, nil),
		Entry("unknown option", "nvim --unknown", nil, nil),
		Entry("boolean value", "nvim --classic=true", nil, nil),
		Entry("ambiguous representations", "nvim", []string{"ghostty"}, nil),
		Entry("flag as identifier", "", []string{"--classic"}, nil),
		Entry("empty identifier", "", []string{""}, nil),
		Entry("executable without verb", "snap nvim", nil, nil),
		Entry("sudo without verb", "sudo snap nvim", nil, nil),
		Entry("unterminated quote", `nvim --channel "edge`, nil, nil),
		Entry("manager without packages", "sudo snap install", nil, nil),
		Entry("empty request", "", nil, nil),
	)
	It("uses only identifiers for structured discovery and removal", func() {
		executor := &recordingPluginExecutor{}
		installer := &PluginBasedInstaller{method: "apt", pluginBootstrap: executor}
		app := types.AppConfig{InstallMethod: "apt", Packages: []string{"apt", "sudo"}}
		installed, err := installer.IsInstalled(app.PackageQueryCommand())
		Expect(err).NotTo(HaveOccurred())
		Expect(installed).To(BeTrue())
		Expect(executor.args).To(Equal([]string{"is-installed", "apt", "sudo"}))
		Expect(installer.Uninstall(app.PackageQueryCommand(), nil)).To(Succeed())
		Expect(executor.args).To(Equal([]string{"remove", "apt", "sudo"}))
	})

	It("preserves Snap options in shipped neovim and ghostty definitions", func() {
		recorder := sdktest.NewRecorder(GinkgoT(), "intent")
		installer := &PluginBasedInstaller{method: "snap", pluginBootstrap: intentExecutor{}}
		count := 0
		for _, name := range []string{"neovim", "ghostty"} {
			data, err := os.ReadFile(filepath.Join("../../config/applications/development", name+".yaml"))
			Expect(err).NotTo(HaveOccurred())
			var app types.CrossPlatformApp
			Expect(yaml.Unmarshal(data, &app)).To(Succeed())
			var visit func(types.OSConfig)
			visit = func(config types.OSConfig) {
				if config.InstallMethod == "snap" {
					Expect(installer.InstallPackages(context.Background(), config.InstallCommand, config.Packages, config.Options)).To(Succeed())
					count++
				}
				for _, alternative := range config.Alternatives {
					visit(alternative)
				}
			}
			visit(app.Linux)
		}
		Expect(count).To(BeNumerically(">=", 3))
		for _, call := range recorder.Calls("intent") {
			Expect(call.Args).To(ContainElement("--classic"))
			Expect(call.Args).NotTo(ContainElement("snap"))
			Expect(call.Args).NotTo(ContainElement("sudo"))
		}
	})
})
