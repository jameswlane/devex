package main_test

import (
	main "github.com/jameswlane/devex/packages/package-manager-xbps"
	"github.com/jameswlane/devex/packages/plugin-sdk/sdktest"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Unattended xbps execution", func() {
	It("completes mutating operations with stdin closed", func() {
		recorder := sdktest.NewRecorder(GinkgoT(), "xbps-install", "xbps-remove")
		sdktest.CloseStdin(GinkgoT())
		GinkgoT().Setenv("DEVEX_UNATTENDED", "1")
		plugin := main.NewXbpsPlugin()
		Expect(plugin.Execute("install", []string{"example"})).To(Succeed())
		Expect(plugin.Execute("remove", []string{"example"})).To(Succeed())
		Expect(plugin.Execute("update", nil)).To(Succeed())
		Expect(recorder.Calls("xbps-install")).To(HaveLen(2))
		Expect(recorder.Calls("xbps-remove")).To(HaveLen(1))
	})
})
