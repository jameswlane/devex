package setup

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
)

var _ = Describe("mise bootstrap verification", func() {
	It("requires mise's release-key signature before running mise.run on Linux and macOS", func() {
		app := (&SetupModel{}).getMiseApp()
		for _, osConfig := range []struct {
			name         string
			verification sdk.ArtifactVerification
			url          string
		}{{"linux", app.Linux.Verification, app.Linux.DownloadURL}, {"macos", app.MacOS.Verification, app.MacOS.DownloadURL}} {
			Expect(osConfig.url).To(Equal(sdk.MiseInstallScriptURL), osConfig.name)
			Expect(osConfig.verification).To(Equal(sdk.MiseInstallVerification()), osConfig.name)
		}
	})

	It("installs mise for the user, not under sudo (#698)", func() {
		app := (&SetupModel{}).getMiseApp()
		Expect(app.Linux.InstallScope).To(Equal("user"))
		Expect(app.MacOS.InstallScope).To(Equal("user"))
	})
})
