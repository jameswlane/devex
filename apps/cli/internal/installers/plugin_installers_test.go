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
		Expect(executor.ctx).To(BeIdenticalTo(ctx))
	})

	DescribeTable("preserves plugin discovery errors instead of confirming absence",
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
		Entry("package-specific probe", "apt", "is-installed"),
		Entry("list fallback", "brew", "list"),
	)
})
