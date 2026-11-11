package main

import (
	"context"
	"os"
	"path/filepath"

	"github.com/jameswlane/devex/packages/plugin-sdk/sdktest"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Docker Engine apt installation", func() {
	var recorder *sdktest.Recorder

	writeRelease := func(contents string) string {
		path := filepath.Join(GinkgoT().TempDir(), "os-release")
		Expect(os.WriteFile(path, []byte(contents), 0o600)).To(Succeed())
		return path
	}

	newInstaller := func(release string) *DockerInstaller {
		installer := NewDockerPlugin()
		installer.osReleasePath = release
		return installer
	}

	DescribeTable("uses the distribution repository and catalog paths",
		func(release, distro, codename string) {
			recorder = sdktest.NewRecorder(GinkgoT(), "apt-get", "install", "curl", "chmod", "dpkg", "tee")
			recorder.Script("dpkg", []string{"--print-architecture"}, sdktest.Response{Stdout: "amd64\n"})
			teeArgs := []string{dockerAptSourcePath}
			recorder.CaptureStdin("tee", teeArgs)

			Expect(newInstaller(writeRelease(release)).installDockerEngineDebian(context.Background())).To(Succeed())

			Expect(recorder.Calls("apt-get")).To(Equal([]sdktest.Call{
				{Args: []string{"update"}, ElevationRequested: true},
				{Args: []string{"install", "-y", "ca-certificates", "curl"}, ElevationRequested: true},
				{Args: []string{"update"}, ElevationRequested: true},
				{Args: []string{"install", "-y", "docker-ce", "docker-ce-cli", "containerd.io", "docker-buildx-plugin", "docker-compose-plugin"}, ElevationRequested: true},
			}))
			Expect(recorder.Calls("install")).To(Equal([]sdktest.Call{{
				Args: []string{"-m", "0755", "-d", "/etc/apt/keyrings"}, ElevationRequested: true,
			}}))
			Expect(recorder.Calls("curl")).To(Equal([]sdktest.Call{{
				Args: []string{"-fsSL", "https://download.docker.com/linux/" + distro + "/gpg", "-o", dockerAptKeyPath}, ElevationRequested: true,
			}}))
			Expect(recorder.Calls("chmod")).To(Equal([]sdktest.Call{{
				Args: []string{"a+r", dockerAptKeyPath}, ElevationRequested: true,
			}}))
			Expect(recorder.Calls("dpkg")).To(Equal([]sdktest.Call{{Args: []string{"--print-architecture"}}}))
			Expect(recorder.Calls("tee")).To(Equal([]sdktest.Call{{Args: teeArgs, ElevationRequested: true}}))
			Expect(recorder.Stdin("tee", teeArgs)).To(Equal([]string{
				"deb [arch=amd64 signed-by=" + dockerAptKeyPath + "] https://download.docker.com/linux/" + distro + " " + codename + " stable\n",
			}))
		},
		Entry("Ubuntu", "ID=ubuntu\nVERSION_CODENAME=noble\n", "ubuntu", "noble"),
		Entry("Debian", "ID=debian\nVERSION_CODENAME=bookworm\n", "debian", "bookworm"),
		Entry("shell-escaped quoted values", "ID=\"debian\"\nPRETTY_NAME=\"Debian \\$ test\"\nVERSION_CODENAME=\"trixie\"\n", "debian", "trixie"),
		Entry("Ubuntu derivative", "ID=linuxmint\nID_LIKE=\"ubuntu debian\"\nUBUNTU_CODENAME=noble\nVERSION_CODENAME=virginia\n", "ubuntu", "noble"),
	)

	It("rejects an unsupported apt distribution before invoking apt-get", func() {
		recorder = sdktest.NewRecorder(GinkgoT(), "apt-get")
		installer := newInstaller(writeRelease("ID=fedora\nVERSION_CODENAME=fedora\n"))

		err := installer.installDockerEngine(context.Background())

		Expect(err).To(MatchError(ContainSubstring("requires Debian, Ubuntu, or a derivative with ID_LIKE")))
		Expect(recorder.Calls("apt-get")).To(BeEmpty())
	})

	It("rejects a supported distribution without a codename", func() {
		recorder = sdktest.NewRecorder(GinkgoT(), "apt-get")
		installer := newInstaller(writeRelease("ID=debian\n"))

		err := installer.installDockerEngineDebian(context.Background())

		Expect(err).To(MatchError(ContainSubstring("requires UBUNTU_CODENAME or VERSION_CODENAME")))
		Expect(recorder.Calls("apt-get")).To(BeEmpty())
	})

	It("returns prerequisite installation failures", func() {
		recorder = sdktest.NewRecorder(GinkgoT(), "apt-get")
		prerequisites := []string{"install", "-y", "ca-certificates", "curl"}
		recorder.Script("apt-get", prerequisites, sdktest.Response{ExitCode: 42, Stderr: "repository unavailable\n"})
		installer := newInstaller(writeRelease("ID=debian\nVERSION_CODENAME=bookworm\n"))

		err := installer.installDockerEngineDebian(context.Background())

		Expect(err).To(MatchError(ContainSubstring("failed to install Docker prerequisites ca-certificates and curl")))
		Expect(recorder.Calls("apt-get")).To(Equal([]sdktest.Call{
			{Args: []string{"update"}, ElevationRequested: true},
			{Args: prerequisites, ElevationRequested: true},
		}))
	})
})
