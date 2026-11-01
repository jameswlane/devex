package main_test

import (
	main "github.com/jameswlane/devex/packages/package-manager-apk"
	"github.com/jameswlane/devex/packages/plugin-sdk/sdktest"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Unattended apk execution", func() {
	It("completes mutating operations with stdin closed", func() {
		sdktest.NewRecorder(GinkgoT(), "apk")
		sdktest.CloseStdin(GinkgoT())
		GinkgoT().Setenv("DEVEX_UNATTENDED", "1")
		plugin := main.NewAPKPlugin()
		for _, command := range []string{"install", "remove", "update", "upgrade"} {
			err := plugin.Execute(command, []string{"example"})

			Expect(err).To(Succeed())
		}
	})
})
