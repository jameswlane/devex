package main_test

import (
	main "github.com/jameswlane/devex/packages/package-manager-rpm"
	"github.com/jameswlane/devex/packages/plugin-sdk/sdktest"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Unattended rpm execution", func() {
	It("completes mutating operations with stdin closed", func() {
		sdktest.NewRecorder(GinkgoT(), "rpm")
		sdktest.CloseStdin(GinkgoT())
		GinkgoT().Setenv("DEVEX_UNATTENDED", "1")
		plugin := main.NewRpmPlugin()
		for _, command := range []string{"install", "remove", "update"} {
			err := plugin.Execute(command, []string{"example"})
			if command == "update" {
				Expect(err).To(HaveOccurred())
				continue
			}
			Expect(err).To(Succeed())
		}
	})
})
