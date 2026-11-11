//go:build linux

package setup

import (
	"context"
	"os"
	"path/filepath"

	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/mocks"
	"github.com/jameswlane/devex/apps/cli/internal/platform"
	"github.com/jameswlane/devex/apps/cli/internal/tui"
	"github.com/jameswlane/devex/apps/cli/internal/types"
	"github.com/jameswlane/devex/packages/plugin-sdk/sdktest"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v3"
)

var _ = Describe("Implicit dependency execution contract", func() {
	var catalogDocker types.CrossPlatformApp

	BeforeEach(func() {
		data, err := os.ReadFile(filepath.Join("..", "..", "..", "config", "applications", "development", "docker.yaml"))
		Expect(err).NotTo(HaveOccurred())
		Expect(yaml.Unmarshal(data, &catalogDocker)).To(Succeed())
	})

	databaseModel := func(apps ...types.CrossPlatformApp) *SetupModel {
		return &SetupModel{step: StepConfirmation,
			system: SystemInfo{shells: []string{"bash"}, databases: []string{"PostgreSQL"},
				detectedPlatform: platform.DetectionResult{OS: "linux", Distribution: "ubuntu"}},
			selections: UISelections{selectedDBs: map[int]bool{0: true}, skipDefaultApps: true},
			settings:   config.CrossPlatformSettings{ApplicationOverrides: apps},
		}
	}

	install := func(model *SetupModel) {
		installer := tui.NewStreamingInstallerWithExecutor(nil, mocks.NewMockRepository(), context.Background(), tui.NewDefaultCommandExecutor(), model.settings)
		Expect(installer.InstallApps(context.Background(), model.buildAppList(), model.settings)).To(Succeed())
	}

	Context("Docker for database selections", func() {
		It("plans the catalog Docker entry and shows its repository, service and group changes", func() {
			model := databaseModel(catalogDocker)
			plan := model.installPlan()
			Expect(plan[0].app.Name).To(Equal("Docker"))
			Expect(plan[0].group).To(Equal(planDependency))
			Expect(plan[0].reason).To(Equal("databases: PostgreSQL"))

			view := model.renderInstallPlan()
			Expect(view).To(ContainSubstring("download.docker.com"))
			Expect(view).To(ContainSubstring("systemctl enable --now docker.service"))
			Expect(view).To(ContainSubstring(`usermod -aG docker`))
			Expect(view).To(ContainSubstring("docker run -d --name postgres16"))
			Expect(view).NotTo(ContainSubstring("docker: docker-ce"))
		})

		It("falls back to the distribution package, never a bare docker-ce command", func() {
			docker := databaseModel().getDockerApp()
			Expect(docker.Linux.InstallMethod).To(Equal("apt"))
			Expect(docker.Linux.InstallCommand).To(Equal("docker.io"))
			Expect(docker.Linux.InstalledBinary).To(Equal("docker"))
		})

		It("skips an existing Docker and runs only the displayed container command", func() {
			recorder := sdktest.NewRecorder(GinkgoT(), "docker", "apt-get", "dpkg-query")
			recorder.Script("docker", []string{"container", "inspect", "postgres16"}, sdktest.Response{ExitCode: 1})
			install(databaseModel(catalogDocker))

			Expect(recorder.Calls("apt-get")).To(BeEmpty())
			calls := recorder.Calls("docker")
			Expect(calls).To(HaveLen(3))
			Expect(calls[0].Args).To(Equal([]string{"info"}))
			Expect(calls[1].Args).To(Equal([]string{"container", "inspect", "postgres16"}))
			Expect(calls[2].Args).To(Equal([]string{"run", "-d", "--name", "postgres16", "--restart", "unless-stopped", "-p", "127.0.0.1:5432:5432", "-e", "POSTGRES_HOST_AUTH_METHOD=trust", "postgres:16"}))
			Expect(calls[2].ElevationRequested).To(BeFalse())
		})

		It("creates the container with sudo while the session cannot reach the daemon", func() {
			recorder := sdktest.NewRecorder(GinkgoT(), "docker", "apt-get", "dpkg-query")
			recorder.Script("docker", []string{"info"}, sdktest.Response{ExitCode: 1})
			recorder.Script("docker", []string{"container", "inspect", "postgres16"}, sdktest.Response{ExitCode: 1})
			install(databaseModel(catalogDocker))

			calls := recorder.Calls("docker")
			Expect(calls).To(HaveLen(3))
			Expect(calls[1].ElevationRequested).To(BeTrue())
			Expect(calls[2].Args[0]).To(Equal("run"))
			Expect(calls[2].ElevationRequested).To(BeTrue())
		})

		It("leaves an existing container unchanged", func() {
			recorder := sdktest.NewRecorder(GinkgoT(), "docker", "apt-get", "dpkg-query")
			install(databaseModel(catalogDocker))

			for _, call := range recorder.Calls("docker") {
				Expect(call.Args[0]).NotTo(Equal("run"))
			}
		})
	})

	Context("mise for language selections", func() {
		It("runs only the displayed mise tool installs when mise is present", func() {
			recorder := sdktest.NewRecorder(GinkgoT(), "mise", "curl")
			// Real bash runs the displayed mise command; the fake mise comes first on PATH.
			GinkgoT().Setenv("PATH", os.Getenv("PATH")+":/usr/bin:/bin")
			recorder.Script("mise", []string{"where", "node@lts"}, sdktest.Response{ExitCode: 1})
			model := &SetupModel{step: StepConfirmation,
				system:     SystemInfo{shells: []string{"bash"}, languages: []string{"Node.js"}, detectedPlatform: platform.DetectionResult{OS: "linux", Distribution: "ubuntu"}},
				selections: UISelections{selectedLangs: map[int]bool{0: true}, skipDefaultApps: true},
			}
			view := model.renderInstallPlan()
			Expect(view).To(ContainSubstring("mise: node@lts"))
			install(model)

			Expect(recorder.Calls("curl")).To(BeEmpty())
			var uses [][]string
			for _, call := range recorder.Calls("mise") {
				if call.Args[0] == "use" {
					uses = append(uses, call.Args)
				}
			}
			Expect(uses).To(Equal([][]string{{"use", "--global", "node@lts"}}))
		})
	})
})
