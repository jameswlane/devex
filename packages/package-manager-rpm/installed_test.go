package main_test

import (
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	main "github.com/jameswlane/devex/packages/package-manager-rpm"
	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
	"github.com/jameswlane/devex/packages/plugin-sdk/sdktest"
)

var _ = Describe("RPM installed-package queries", func() {
	var (
		plugin   *main.RpmPlugin
		recorder *sdktest.Recorder
	)

	BeforeEach(func() {
		recorder = sdktest.NewRecorder(GinkgoT(), "rpm")
		plugin = main.NewRpmPlugin()
	})

	It("reports an installed package", func() {
		recorder.Script("rpm", []string{"--query", "git"}, sdktest.Response{Stdout: "git-2.51.0-1.fc44.x86_64\n"})
		Expect(plugin.Execute("is-installed", []string{"git"})).To(Succeed())
	})

	It("reports a native not-installed answer with the SDK status", func() {
		nativeStderr := "package git is not installed\n"
		recorder.Script("rpm", []string{"--query", "git"}, sdktest.Response{ExitCode: 1, Stderr: nativeStderr})
		var err error
		stderr := captureStderr(func() {
			err = plugin.Execute("is-installed", []string{"git"})
		})
		var absent *sdk.NotInstalledError
		Expect(errors.As(err, &absent)).To(BeTrue(), "expected NotInstalledError, got %v", err)
		Expect(absent.Packages).To(Equal([]string{"git"}))
		Expect(sdk.ExitStatus(err)).To(Equal(sdk.ExitCodeNotInstalled))
		Expect(stderr).To(Equal(nativeStderr))
	})

	It("reports only the missing packages from separate native queries", func() {
		recorder.Script("rpm", []string{"--query", "git"}, sdktest.Response{Stdout: "git-2.51.0-1.fc44.x86_64\n"})
		recorder.Script("rpm", []string{"--query", "missing"}, sdktest.Response{ExitCode: 1, Stderr: "package missing is not installed\n"})
		err := plugin.Execute("is-installed", []string{"git", "missing"})
		var absent *sdk.NotInstalledError
		Expect(errors.As(err, &absent)).To(BeTrue())
		Expect(absent.Packages).To(Equal([]string{"missing"}))
		Expect(recorder.Calls("rpm")).To(Equal([]sdktest.Call{
			{Args: []string{"--query", "git"}},
			{Args: []string{"--query", "missing"}},
		}))
	})

	It("does not mistake an RPM database failure for absence", func() {
		recorder.Script("rpm", []string{"--query", "git"}, sdktest.Response{ExitCode: 1, Stderr: "error: rpmdb open failed\n"})
		err := plugin.Execute("is-installed", []string{"git"})
		var absent *sdk.NotInstalledError
		Expect(errors.As(err, &absent)).To(BeFalse())
		Expect(err).To(MatchError(And(
			ContainSubstring("failed to check installation status of git"),
			ContainSubstring("exit status 1"),
		)))
		Expect(sdk.ExitStatus(err)).To(Equal(1))
	})
})
