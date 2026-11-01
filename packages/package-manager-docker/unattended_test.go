package main_test

import (
	main "github.com/jameswlane/devex/packages/package-manager-docker"
	"github.com/jameswlane/devex/packages/plugin-sdk/sdktest"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Unattended Docker execution", func() {
	It("pulls and removes images with exact argv and closed stdin", func() {
		recorder := sdktest.NewRecorder(GinkgoT(), "docker")
		sdktest.CloseStdin(GinkgoT())
		GinkgoT().Setenv("DEVEX_UNATTENDED", "1")
		plugin := main.NewDockerPlugin()
		Expect(plugin.Execute("pull", []string{"alpine:latest"})).To(Succeed())
		Expect(plugin.Execute("rmi", []string{"alpine:latest"})).To(Succeed())
		Expect(recorder.Calls("docker")).To(Equal([]sdktest.Call{{Args: []string{"pull", "alpine:latest"}}, {Args: []string{"rmi", "alpine:latest"}}}))
	})
})
