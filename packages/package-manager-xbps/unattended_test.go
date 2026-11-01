package main_test

import (
	main "github.com/jameswlane/devex/packages/package-manager-xbps"
	"github.com/jameswlane/devex/packages/plugin-sdk/sdktest"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Unattended xbps execution", func() {
	It("completes mutating operations with stdin closed", func() {
		sdktest.NewRecorder(GinkgoT(), "xbps-install", "xbps-remove")
		sdktest.CloseStdin(GinkgoT())
		GinkgoT().Setenv("DEVEX_UNATTENDED", "1")
		plugin := main.NewXbpsPlugin()
		for _, command := range []string{"install", "remove", "update"} {
			err := plugin.Execute(command, []string{"example"})

			Expect(err).To(Succeed())
		}
	})
})
