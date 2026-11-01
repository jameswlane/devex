package main_test

import (
	main "github.com/jameswlane/devex/packages/package-manager-curlpipe"
	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
	"github.com/jameswlane/devex/packages/plugin-sdk/sdktest"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Unattended curlpipe execution", func() {
	It("downloads and executes a fake installer with stdin closed", func() {
		recorder := sdktest.NewRecorder(GinkgoT(), "curl", "bash")
		sdktest.CloseStdin(GinkgoT())
		GinkgoT().Setenv("DEVEX_UNATTENDED", "1")
		GinkgoT().Setenv("TMPDIR", GinkgoT().TempDir())
		plugin := main.NewCurlpipePlugin()
		const url = "https://get.devex.sh/install"
		Expect(plugin.Execute("install", []string{url, "--force"})).To(Succeed())
		calls := recorder.Calls("curl")
		Expect(calls).To(HaveLen(1))
		Expect(calls[0].Args).To(HaveLen(5))
		script := calls[0].Args[2]
		Expect(calls).To(Equal([]sdktest.Call{{Args: []string{"-fsSL", "-o", script, "--", url}}}))
		Expect(recorder.Calls("bash")).To(Equal([]sdktest.Call{{Args: []string{script}, ElevationRequested: true}}))
		var manual *sdk.ManualRemovalRequiredError
		Expect(plugin.Execute("remove", []string{"example"})).To(BeAssignableToTypeOf(manual))
	})
})
