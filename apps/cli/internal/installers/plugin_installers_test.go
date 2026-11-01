package installers

import (
	"context"
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type recordingPluginExecutor struct {
	ctx  context.Context
	name string
	args []string
	err  error
}

func (r *recordingPluginExecutor) ExecutePluginWithContext(ctx context.Context, name string, args []string) error {
	r.ctx, r.name, r.args = ctx, name, args
	return r.err
}

var _ = Describe("Plugin installation discovery", func() {
	It("preserves the Docker Desktop cask flag for install and removal", func() {
		executor := &recordingPluginExecutor{}
		installer := &PluginBasedInstaller{method: "brew", pluginBootstrap: executor, ctx: context.Background()}
		Expect(installer.Install("--cask docker", nil)).To(Succeed())
		Expect(executor.args).To(Equal([]string{"install", "--cask", "docker"}))
		Expect(installer.Uninstall("--cask docker", nil)).To(Succeed())
		Expect(executor.args).To(Equal([]string{"remove", "--cask", "docker"}))
	})
	It("checks the requested APT packages and forwards the caller context", func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		executor := &recordingPluginExecutor{}
		installer := &PluginBasedInstaller{method: "apt", pluginBootstrap: executor, ctx: ctx}

		installed, err := installer.IsInstalled("sudo install -y git curl")
		Expect(err).NotTo(HaveOccurred())
		Expect(installed).To(BeTrue())
		Expect(executor.name).To(Equal("package-manager-apt"))
		Expect(executor.args).To(Equal([]string{"is-installed", "git", "curl"}))
		Expect(executor.ctx.Done()).To(Equal(ctx.Done()))
	})

	DescribeTable("preserves plugin discovery errors for package-specific probes",
		func(method, operation string) {
			probeErr := errors.New("plugin discovery failed")
			executor := &recordingPluginExecutor{err: probeErr}
			installer := &PluginBasedInstaller{method: method, pluginBootstrap: executor, ctx: context.Background()}

			installed, err := installer.IsInstalled("git")
			Expect(installed).To(BeFalse())
			Expect(err).To(MatchError(ContainSubstring("installation check failed")))
			Expect(errors.Is(err, probeErr)).To(BeTrue())
			Expect(executor.args[0]).To(Equal(operation))
		},
		Entry("apt probe", "apt", "is-installed"),
		Entry("flatpak probe", "flatpak", "is-installed"),
		Entry("pip probe", "pip", "is-installed"),
	)

	DescribeTable("skips the installed probe for plugins without an is-installed operation",
		func(method string) {
			probeErr := errors.New("offline list failure")
			executor := &recordingPluginExecutor{err: probeErr}
			installer := &PluginBasedInstaller{method: method, pluginBootstrap: executor, ctx: context.Background()}

			installed, err := installer.IsInstalled("git")
			Expect(err).NotTo(HaveOccurred())
			Expect(installed).To(BeTrue())
			Expect(executor.args).To(BeNil())
		},
		Entry("dnf plugin", "dnf"),
		Entry("brew plugin", "brew"),
		Entry("pacman plugin", "pacman"),
	)

	It("allows removal when a list-only plugin's list operation would fail offline", func() {
		executor := &recordingPluginExecutor{
			err: errors.New("offline metadata failure"),
		}
		installer := &PluginBasedInstaller{method: "dnf", pluginBootstrap: executor, ctx: context.Background()}

		// IsInstalled skips probe and treats package as present
		installed, err := installer.IsInstalled("git")
		Expect(err).NotTo(HaveOccurred())
		Expect(installed).To(BeTrue())
		Expect(executor.args).To(BeNil())

		// Reset error for successful removal
		executor.err = nil
		err = installer.Uninstall("sudo remove -y git", nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(executor.name).To(Equal("package-manager-dnf"))
		Expect(executor.args).To(Equal([]string{"remove", "git"}))
	})
})
