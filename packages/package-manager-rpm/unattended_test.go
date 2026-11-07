package main_test

import (
	main "github.com/jameswlane/devex/packages/package-manager-rpm"
	"github.com/jameswlane/devex/packages/plugin-sdk/sdktest"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Unattended rpm execution", func() {
	It("completes mutating operations with stdin closed", func() {
		recorder := sdktest.NewRecorder(GinkgoT(), "rpm")
		sdktest.CloseStdin(GinkgoT())
		GinkgoT().Setenv("DEVEX_UNATTENDED", "1")
		plugin := main.NewRpmPlugin()
		Expect(plugin.Execute("install", []string{"example.rpm"})).To(Succeed())
		Expect(plugin.Execute("remove", []string{"example"})).To(Succeed())
		Expect(recorder.Calls("rpm")).To(HaveLen(2))
	})
})
