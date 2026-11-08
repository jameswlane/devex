package main_test

import (
	main "github.com/jameswlane/devex/packages/package-manager-dnf"
	"github.com/jameswlane/devex/packages/plugin-sdk/sdktest"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Unattended dnf execution", func() {
	It("completes mutating operations with stdin closed", func() {
		recorder := sdktest.NewRecorder(GinkgoT(), "dnf", "rpm")
		recorder.Script("dnf", []string{"repoquery", "--installed", "--quiet", "--cacheonly", "example"}, sdktest.Response{
			Stdout: "example-0:1.0-1.fc42.x86_64\n",
		})
		sdktest.CloseStdin(GinkgoT())
		GinkgoT().Setenv("DEVEX_UNATTENDED", "1")
		plugin := main.NewDnfPlugin()
		for _, command := range []string{"install", "remove", "update", "upgrade"} {
			err := plugin.Execute(command, []string{"example"})

			Expect(err).To(Succeed())
		}
	})
})
