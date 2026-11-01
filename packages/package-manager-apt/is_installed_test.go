package main_test

import (
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	main "github.com/jameswlane/devex/packages/package-manager-apt"
	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
	"github.com/jameswlane/devex/packages/plugin-sdk/sdktest"
)

var _ = Describe("is-installed result", func() {
	var recorder *sdktest.Recorder

	BeforeEach(func() {
		recorder = sdktest.NewRecorder(GinkgoT(), "apt", "dpkg-query")
	})

	query := []string{"-W", "-f=${Status}", "example"}

	It("succeeds when dpkg reports the package installed", func() {
		recorder.Script("dpkg-query", query, sdktest.Response{Stdout: "install ok installed"})
		Expect(main.NewAPTPlugin().Execute("is-installed", []string{"example"})).To(Succeed())
	})

	It("reports dpkg-query's not-found exit status 1 as not installed", func() {
		recorder.Script("dpkg-query", query, sdktest.Response{ExitCode: 1, Stderr: "dpkg-query: no packages found matching example\n"})
		err := main.NewAPTPlugin().Execute("is-installed", []string{"example"})
		var missing *sdk.NotInstalledError
		Expect(errors.As(err, &missing)).To(BeTrue())
	})

	It("returns a failed query as an error, not as absence", func() {
		recorder.Script("dpkg-query", query, sdktest.Response{ExitCode: 2, Stderr: "dpkg-query: error: database locked\n"})
		err := main.NewAPTPlugin().Execute("is-installed", []string{"example"})
		Expect(err).To(MatchError(ContainSubstring("dpkg-query failed for example")))
		var missing *sdk.NotInstalledError
		Expect(errors.As(err, &missing)).To(BeFalse())
	})
})
