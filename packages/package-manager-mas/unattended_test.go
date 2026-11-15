package main_test

import (
	main "github.com/jameswlane/devex/packages/package-manager-mas"
	"github.com/jameswlane/devex/packages/plugin-sdk/sdktest"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Unattended mas execution", func() {
	It("completes mutating operations with stdin closed", func() {
		recorder := sdktest.NewRecorder(GinkgoT(), "mas")
		sdktest.CloseStdin(GinkgoT())
		GinkgoT().Setenv("DEVEX_UNATTENDED", "1")
		plugin := main.NewMasPlugin()
		Expect(plugin.Execute("install", []string{"497799835"})).To(Succeed())
		Expect(plugin.Execute("upgrade", []string{"497799835"})).To(Succeed())
		Expect(plugin.Execute("remove", []string{"497799835"})).To(Succeed())
		Expect(recorder.Calls("mas")).To(HaveLen(3))
	})
})
