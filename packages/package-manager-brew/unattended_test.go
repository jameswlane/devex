package main

import (
	"github.com/jameswlane/devex/packages/plugin-sdk/sdktest"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Unattended brew execution", func() {
	It("completes all mutations with exact argv and closed stdin", func() {
		recorder := sdktest.NewRecorder(GinkgoT(), "brew")
		sdktest.CloseStdin(GinkgoT())
		GinkgoT().Setenv("DEVEX_UNATTENDED", "1")
		plugin := NewBrewPlugin()
		Expect(plugin.Execute("install", []string{"example"})).To(Succeed())
		Expect(plugin.Execute("remove", []string{"example"})).To(Succeed())
		Expect(plugin.Execute("update", nil)).To(Succeed())
		Expect(recorder.Calls("brew")).To(Equal([]sdktest.Call{{Args: []string{"install", "example"}, ElevationRequested: false}, {Args: []string{"uninstall", "example"}, ElevationRequested: false}, {Args: []string{"update"}, ElevationRequested: false}}))
		Expect(recorder.Environment("brew", "NONINTERACTIVE")).To(Equal([]string{"1", "1", "1"}))
	})
})
