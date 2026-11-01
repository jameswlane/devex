package main_test

import (
	main "github.com/jameswlane/devex/packages/package-manager-apt"
	"github.com/jameswlane/devex/packages/plugin-sdk/sdktest"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Unattended APT execution", func() {
	It("retains -y for every package mutation and disables debconf with stdin closed", func() {
		recorder := sdktest.NewRecorder(GinkgoT(), "apt", "apt-get", "apt-cache", "dpkg-query")
		sdktest.CloseStdin(GinkgoT())
		GinkgoT().Setenv("DEVEX_UNATTENDED", "1")
		recorder.Script("apt", []string{"--version"}, sdktest.Response{Stdout: "apt 2.8.0 (amd64)\n"})
		recorder.Script("apt-cache", []string{"policy", "git"}, sdktest.Response{Stdout: "Candidate: 1.0\n"})
		recorder.Script("dpkg-query", []string{"-W", "-f=${Status}", "git"}, sdktest.Response{Stdout: "install ok installed"})
		plugin := main.NewAPTPlugin()
		Expect(plugin.Execute("install", []string{"git"})).To(Succeed())
		Expect(plugin.Execute("remove", []string{"git"})).To(Succeed())
		Expect(plugin.Execute("update", nil)).To(Succeed())
		Expect(plugin.Execute("upgrade", nil)).To(Succeed())
		Expect(recorder.Calls("apt-get")).To(Equal([]sdktest.Call{
			{Args: []string{"update"}, ElevationRequested: true},
			{Args: []string{"install", "-y", "git"}, ElevationRequested: true},
			{Args: []string{"remove", "-y", "git"}, ElevationRequested: true},
			{Args: []string{"update"}, ElevationRequested: true},
			{Args: []string{"update"}, ElevationRequested: true},
			{Args: []string{"upgrade", "-y"}, ElevationRequested: true},
		}))
		Expect(recorder.Environment("apt-get", "DEBIAN_FRONTEND")).To(Equal([]string{"noninteractive", "noninteractive", "noninteractive", "noninteractive", "noninteractive", "noninteractive"}))
	})
})

var _ = Describe("Elevated APT environment", func() {
	It("passes DEBIAN_FRONTEND to sudo before --, so sudo doesn't run it as the command", func() {
		recorder := sdktest.NewRecorder(GinkgoT(), "apt", "apt-get")
		sdktest.CloseStdin(GinkgoT())
		GinkgoT().Setenv("DEVEX_UNATTENDED", "1")
		plugin := main.NewAPTPlugin()

		Expect(plugin.Execute("update", nil)).To(Succeed())

		Expect(recorder.Calls("sudo")).To(ContainElement(sdktest.Call{Args: []string{"-n", "DEBIAN_FRONTEND=noninteractive", "--", "apt-get", "update"}}))
		Expect(recorder.Environment("apt-get", "DEBIAN_FRONTEND")).To(Equal([]string{"noninteractive"}))
	})
})
