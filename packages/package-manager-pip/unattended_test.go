package main_test

import (
	main "github.com/jameswlane/devex/packages/package-manager-pip"
	"github.com/jameswlane/devex/packages/plugin-sdk/sdktest"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Unattended pip execution", func() {
	It("completes all mutations with exact argv and closed stdin", func() {
		recorder := sdktest.NewRecorder(GinkgoT(), "pip", "which")
		sdktest.CloseStdin(GinkgoT())
		GinkgoT().Setenv("DEVEX_UNATTENDED", "1")
		plugin := main.NewPipPlugin()
		Expect(plugin.Execute("install", []string{"requests"})).To(Succeed())
		Expect(plugin.Execute("remove", []string{"requests"})).To(Succeed())
		Expect(plugin.Execute("update", []string{"requests"})).To(Succeed())
		Expect(recorder.Calls("pip")).To(Equal([]sdktest.Call{
			{Args: []string{"--no-input", "install", "requests"}, ElevationRequested: true},
			{Args: []string{"--no-input", "uninstall", "-y", "requests"}, ElevationRequested: true},
			{Args: []string{"--no-input", "install", "--upgrade", "pip"}, ElevationRequested: true},
			{Args: []string{"--no-input", "install", "--upgrade", "requests"}, ElevationRequested: true},
		}))
	})
})
