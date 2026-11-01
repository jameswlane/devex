package main

import (
	"github.com/jameswlane/devex/packages/plugin-sdk/sdktest"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Unattended snap execution", func() {
	It("completes all mutations with exact argv and closed stdin", func() {
		recorder := sdktest.NewRecorder(GinkgoT(), "snap")
		sdktest.CloseStdin(GinkgoT())
		GinkgoT().Setenv("DEVEX_UNATTENDED", "1")
		plugin := NewSnapPlugin()
		Expect(plugin.Execute("install", []string{"example"})).To(Succeed())
		Expect(plugin.Execute("remove", []string{"example"})).To(Succeed())
		Expect(plugin.Execute("update", nil)).To(Succeed())
		Expect(recorder.Calls("snap")).To(Equal([]sdktest.Call{{Args: []string{"install", "example"}, ElevationRequested: true}, {Args: []string{"remove", "example"}, ElevationRequested: true}, {Args: []string{"refresh"}, ElevationRequested: true}}))

	})
})
