package main

import (
	"context"

	"github.com/jameswlane/devex/packages/plugin-sdk/sdktest"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Docker socket access", func() {
	const socketDenied = "permission denied while trying to connect to the Docker daemon socket at unix:///var/run/docker.sock\n"

	var recorder *sdktest.Recorder

	runArgs := []string{"postgres:16", "--name=devex-postgres", "--detach"}

	BeforeEach(func() {
		recorder = sdktest.NewRecorder(GinkgoT(), "docker")
	})

	Context("when the user isn't in the docker group in this session", func() {
		BeforeEach(func() {
			recorder.Script("docker", []string{"info"}, sdktest.Response{ExitCode: 1, Stderr: socketDenied})
		})

		It("starts the container through sudo", func() {
			Expect(NewDockerPlugin().ExecuteContext(context.Background(), "install", runArgs)).To(Succeed())

			Expect(recorder.Calls("docker")).To(Equal([]sdktest.Call{
				{Args: []string{"info"}},
				{Args: []string{"run", "-d", "--name", "devex-postgres", "postgres:16"}, ElevationRequested: true},
			}))
		})

		It("probes the socket once for every docker command in the run", func() {
			Expect(NewDockerPlugin().ExecuteContext(context.Background(), "remove", []string{"devex-postgres"})).To(Succeed())

			calls := recorder.Calls("docker")
			Expect(calls).To(HaveLen(3))
			Expect(calls[0]).To(Equal(sdktest.Call{Args: []string{"info"}}))
			Expect(calls[1].ElevationRequested).To(BeTrue())
			Expect(calls[2].ElevationRequested).To(BeTrue())
		})

		It("tells the user to log in again when sudo can't run docker", func() {
			recorder.Script("docker", runDockerArgs(), sdktest.Response{ExitCode: 1})

			err := NewDockerPlugin().ExecuteContext(context.Background(), "install", runArgs)

			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("log out and back in"))
		})
	})

	Context("when only standalone docker-compose is installed", func() {
		It("falls back to it through sudo", func() {
			recorder = sdktest.NewRecorder(GinkgoT(), "docker", "docker-compose")
			recorder.Script("docker", []string{"info"}, sdktest.Response{ExitCode: 1, Stderr: socketDenied})
			recorder.Script("docker", []string{"compose", "version"}, sdktest.Response{ExitCode: 1, Stderr: "docker: 'compose' is not a docker command.\n"})

			Expect(NewDockerPlugin().ExecuteContext(context.Background(), "compose", []string{"up"})).To(Succeed())

			Expect(recorder.Calls("docker-compose")).To(Equal([]sdktest.Call{{Args: []string{"up"}, ElevationRequested: true}}))
		})
	})

	Context("when docker fails with another permission error", func() {
		It("runs docker without sudo", func() {
			recorder.Script("docker", []string{"info"}, sdktest.Response{ExitCode: 1, Stderr: "WARNING: Error loading config file: open /home/u/.docker/config.json: permission denied\n"})

			Expect(NewDockerPlugin().dockerNeedsSudo(context.Background())).To(BeFalse())
		})
	})

	Context("when status can't reach the socket even with sudo", func() {
		It("reports the re-login hint instead of a stopped daemon", func() {
			recorder.Script("docker", []string{"info"}, sdktest.Response{ExitCode: 1, Stderr: "permission denied while trying to connect to the docker API at unix:///var/run/docker.sock\n"})

			err := NewDockerPlugin().ExecuteContext(context.Background(), "status", nil)

			Expect(err).To(MatchError(ContainSubstring("log out and back in")))
			Expect(err.Error()).NotTo(ContainSubstring("daemon is not running"))
		})
	})

	Context("when the user can reach the Docker socket", func() {
		It("runs docker without sudo", func() {
			Expect(NewDockerPlugin().ExecuteContext(context.Background(), "install", runArgs)).To(Succeed())

			Expect(recorder.Calls("docker")).To(Equal([]sdktest.Call{
				{Args: []string{"info"}},
				{Args: []string{"run", "-d", "--name", "devex-postgres", "postgres:16"}},
			}))
		})
	})

	Context("when the Docker daemon isn't running yet", func() {
		It("doesn't cache the probe, so a later permission error still elevates", func() {
			recorder.Script("docker", []string{"info"}, sdktest.Response{ExitCode: 1, Stderr: "Cannot connect to the Docker daemon. Is the docker daemon running?\n"})
			plugin := NewDockerPlugin()
			Expect(plugin.dockerNeedsSudo(context.Background())).To(BeFalse())

			recorder.Script("docker", []string{"info"}, sdktest.Response{ExitCode: 1, Stderr: socketDenied})
			Expect(plugin.dockerNeedsSudo(context.Background())).To(BeTrue())
		})
	})
})

func runDockerArgs() []string {
	return []string{"run", "-d", "--name", "devex-postgres", "postgres:16"}
}
