package main_test

import (
	main "github.com/jameswlane/devex/packages/package-manager-zypper"
	"github.com/jameswlane/devex/packages/plugin-sdk/sdktest"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Unattended zypper execution", func() {
	It("completes mutating operations with stdin closed", func() {
		recorder := sdktest.NewRecorder(GinkgoT(), "zypper")
		sdktest.CloseStdin(GinkgoT())
		GinkgoT().Setenv("DEVEX_UNATTENDED", "1")
		plugin := main.NewZypperPlugin()
		Expect(plugin.Execute("install", []string{"example"})).To(Succeed())
		Expect(plugin.Execute("remove", []string{"example"})).To(Succeed())
		Expect(plugin.Execute("update", nil)).To(Succeed())
		Expect(plugin.Execute("upgrade", []string{"example"})).To(Succeed())
		Expect(plugin.Execute("install", []string{"-t", "pattern", "devel_basis"})).To(Succeed())
		Expect(plugin.Execute("remove", []string{"-t", "pattern", "devel_basis"})).To(Succeed())
		calls := recorder.Calls("zypper")
		Expect(calls).To(HaveLen(6))
		for _, call := range calls {
			// zypper's global --non-interactive must precede the subcommand.
			Expect(call.Args[0]).To(Equal("--non-interactive"))
		}
	})
})
