package main_test

import (
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	main "github.com/jameswlane/devex/packages/package-manager-mas"
	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
	"github.com/jameswlane/devex/packages/plugin-sdk/sdktest"
)

// absentStderr is what mas 7.0.0 prints for an app ID with no installed app.
const absentStderr = "Error: No installed apps with ADAM ID 409183694\nWarning: No installed apps found\n"

var _ = Describe("Mac App Store installed-app queries", func() {
	var (
		plugin   *main.MasPlugin
		recorder *sdktest.Recorder
	)

	BeforeEach(func() {
		recorder = sdktest.NewRecorder(GinkgoT(), "mas")
		plugin = main.NewMasPlugin()
	})

	It("reports an installed app", func() {
		recorder.Script("mas", []string{"list", "497799835"}, sdktest.Response{Stdout: "497799835  Xcode  (27.2)\n"})
		Expect(plugin.Execute("is-installed", []string{"497799835"})).To(Succeed())
	})

	It("reports mas's not-installed answer with the SDK status", func() {
		recorder.Script("mas", []string{"list", "409183694"}, sdktest.Response{ExitCode: 1, Stderr: absentStderr})
		err := plugin.Execute("is-installed", []string{"409183694"})
		var absent *sdk.NotInstalledError
		Expect(errors.As(err, &absent)).To(BeTrue(), "expected NotInstalledError, got %v", err)
		Expect(absent.Packages).To(Equal([]string{"409183694"}))
		Expect(sdk.ExitStatus(err)).To(Equal(sdk.ExitCodeNotInstalled))
	})

	It("reports only the missing apps from separate native queries", func() {
		recorder.Script("mas", []string{"list", "497799835"}, sdktest.Response{Stdout: "497799835  Xcode  (27.2)\n"})
		recorder.Script("mas", []string{"list", "409183694"}, sdktest.Response{ExitCode: 1, Stderr: absentStderr})
		err := plugin.Execute("is-installed", []string{"497799835", "409183694"})
		var absent *sdk.NotInstalledError
		Expect(errors.As(err, &absent)).To(BeTrue())
		Expect(absent.Packages).To(Equal([]string{"409183694"}))
		Expect(recorder.Calls("mas")).To(Equal([]sdktest.Call{
			{Args: []string{"list", "497799835"}},
			{Args: []string{"list", "409183694"}},
		}))
	})

	It("does not treat another app's absence message as this app's", func() {
		recorder.Script("mas", []string{"list", "497799835"}, sdktest.Response{ExitCode: 1, Stderr: absentStderr})
		err := plugin.Execute("is-installed", []string{"497799835"})
		var absent *sdk.NotInstalledError
		Expect(errors.As(err, &absent)).To(BeFalse(), "got %v", err)
		Expect(err).To(MatchError(ContainSubstring("failed to check installation status of App Store app 497799835")))
	})

	It("does not mistake a mas failure for absence", func() {
		recorder.Script("mas", []string{"list", "497799835"}, sdktest.Response{ExitCode: 1, Stderr: "Error: Unexpected argument '497799835'\n"})
		err := plugin.Execute("is-installed", []string{"497799835"})
		var absent *sdk.NotInstalledError
		Expect(errors.As(err, &absent)).To(BeFalse())
		Expect(err).To(MatchError(And(
			ContainSubstring("failed to check installation status of App Store app 497799835"),
			ContainSubstring("exit status 1"),
		)))
		Expect(sdk.ExitStatus(err)).To(Equal(1))
	})
})
