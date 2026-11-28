package installers

import (
	"context"
	"errors"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
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
	DescribeTable("checks requested packages and forwards the caller context", func(method string) {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		executor := &recordingPluginExecutor{}
		installer := &PluginBasedInstaller{method: method, pluginBootstrap: executor, ctx: ctx}

		installed, err := installer.IsInstalled("sudo install -y git curl")
		Expect(err).NotTo(HaveOccurred())
		Expect(installed).To(BeTrue())
		Expect(executor.name).To(Equal("package-manager-" + method))
		Expect(executor.args).To(Equal([]string{"is-installed", "git", "curl"}))
		Expect(executor.ctx.Done()).To(Equal(ctx.Done()))
	}, Entry("APT", "apt"), Entry("DNF", "dnf"), Entry("YUM", "yum"))

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
		Entry("dnf probe", "dnf", "is-installed"),
		Entry("yum probe", "yum", "is-installed"),
		Entry("flatpak probe", "flatpak", "is-installed"),
		Entry("pip probe", "pip", "is-installed"),
		Entry("brew probe", "brew", "is-installed"),
		Entry("mas probe", "mas", "is-installed"),
		Entry("winget probe", "winget", "is-installed"),
		Entry("rpm probe", "rpm", "is-installed"),
	)

	DescribeTable("reports a package the plugin found absent as not installed, without an error",
		func(method string) {
			executor := &recordingPluginExecutor{err: &sdk.NotInstalledError{Packages: []string{"git"}}}
			installer := &PluginBasedInstaller{method: method, pluginBootstrap: executor, ctx: context.Background()}

			installed, err := installer.IsInstalled("git")
			Expect(err).NotTo(HaveOccurred())
			Expect(installed).To(BeFalse())
			Expect(executor.args).To(Equal([]string{"is-installed", "git"}))
		},
		Entry("apt", "apt"),
		Entry("deb", "deb"),
		Entry("dnf", "dnf"),
		Entry("yum", "yum"),
		Entry("flatpak", "flatpak"),
		Entry("mise", "mise"),
		Entry("pip", "pip"),
		Entry("appimage", "appimage"),
		Entry("brew", "brew"),
		Entry("mas", "mas"),
		Entry("winget", "winget"),
		Entry("rpm", "rpm"),
	)

	Describe("rpm package names (#421)", func() {
		var (
			original func(context.Context, string) (string, error)
			read     []string
		)
		BeforeEach(func() {
			original = readRPMPackageName
			read = nil
			readRPMPackageName = func(_ context.Context, file string) (string, error) {
				read = append(read, file)
				return "tool", nil
			}
		})
		AfterEach(func() { readRPMPackageName = original })

		It("reports an installed package by the name inside its .rpm file", func() {
			executor := &recordingPluginExecutor{}
			installer := &PluginBasedInstaller{method: "rpm", pluginBootstrap: executor, ctx: context.Background()}

			installed, err := installer.IsInstalled("/tmp/tool-1.0-1.x86_64.rpm")
			Expect(err).NotTo(HaveOccurred())
			Expect(installed).To(BeTrue())
			Expect(read).To(Equal([]string{"/tmp/tool-1.0-1.x86_64.rpm"}))
			Expect(executor.name).To(Equal("package-manager-rpm"))
			Expect(executor.args).To(Equal([]string{"is-installed", "tool"}))
		})

		It("reports a package the plugin found absent (exit 3) as not installed", func() {
			executor := &recordingPluginExecutor{err: &sdk.NotInstalledError{Packages: []string{"tool"}}}
			installer := &PluginBasedInstaller{method: "rpm", pluginBootstrap: executor, ctx: context.Background()}

			installed, err := installer.IsInstalled("/tmp/tool-1.0-1.x86_64.rpm")
			Expect(err).NotTo(HaveOccurred())
			Expect(installed).To(BeFalse())
			Expect(executor.args).To(Equal([]string{"is-installed", "tool"}))
		})

		It("removes the package by the name inside its .rpm file", func() {
			executor := &recordingPluginExecutor{}
			installer := &PluginBasedInstaller{method: "rpm", pluginBootstrap: executor, ctx: context.Background()}

			Expect(installer.Uninstall("/tmp/tool-1.0-1.x86_64.rpm", nil)).To(Succeed())
			Expect(executor.args).To(Equal([]string{"remove", "tool"}))
		})

		It("passes package names through without reading a file", func() {
			executor := &recordingPluginExecutor{}
			installer := &PluginBasedInstaller{method: "rpm", pluginBootstrap: executor, ctx: context.Background()}

			_, err := installer.IsInstalled("tool")
			Expect(err).NotTo(HaveOccurred())
			Expect(read).To(BeEmpty())
			Expect(executor.args).To(Equal([]string{"is-installed", "tool"}))
		})

		It("reads only a local regular file, never a URL or a FIFO-like special file", func() {
			_, err := original(context.Background(), "https://example.com/tool.rpm")
			Expect(err).To(MatchError(ContainSubstring("URL")))
			dir := filepath.Join(GinkgoT().TempDir(), "not-a-file.rpm")
			Expect(os.Mkdir(dir, 0o750)).To(Succeed())
			_, err = original(context.Background(), dir)
			Expect(err).To(MatchError(ContainSubstring("not a regular file")))
		})

		It("returns an error, not absence, when the .rpm file can't be read", func() {
			readRPMPackageName = func(context.Context, string) (string, error) {
				return "", errors.New("open failed")
			}
			executor := &recordingPluginExecutor{}
			installer := &PluginBasedInstaller{method: "rpm", pluginBootstrap: executor, ctx: context.Background()}

			installed, err := installer.IsInstalled("/tmp/gone.rpm")
			Expect(installed).To(BeFalse())
			Expect(err).To(MatchError(ContainSubstring("set uninstall_command to the package name")))
			Expect(executor.args).To(BeNil())
			Expect(installer.Uninstall("/tmp/gone.rpm", nil)).To(MatchError(ContainSubstring("cannot read the RPM package name")))
			Expect(executor.args).To(BeNil())
		})
	})

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
		Entry("pacman plugin", "pacman"),
	)

	It("allows removal when a list-only plugin's list operation would fail offline", func() {
		executor := &recordingPluginExecutor{
			err: errors.New("offline metadata failure"),
		}
		installer := &PluginBasedInstaller{method: "pacman", pluginBootstrap: executor, ctx: context.Background()}

		// IsInstalled skips probe and treats package as present
		installed, err := installer.IsInstalled("git")
		Expect(err).NotTo(HaveOccurred())
		Expect(installed).To(BeTrue())
		Expect(executor.args).To(BeNil())

		// Reset error for successful removal
		executor.err = nil
		err = installer.Uninstall("sudo remove -y git", nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(executor.name).To(Equal("package-manager-pacman"))
		Expect(executor.args).To(Equal([]string{"remove", "git"}))
	})
})
