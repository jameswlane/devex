//go:build !windows

package main_test

import (
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	main "github.com/jameswlane/devex/packages/package-manager-winget"
	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
	"github.com/jameswlane/devex/packages/plugin-sdk/sdktest"
)

var _ = Describe("winget installed-package queries", func() {
	var (
		plugin   *main.WingetPlugin
		recorder *sdktest.Recorder
		query    = func(id string) []string { return argv([]string{"list", "--id", id, "--exact"}, agree) }
	)

	BeforeEach(func() {
		recorder = sdktest.NewRecorder(GinkgoT(), "winget")
		DeferCleanup(main.UseFakeExitCodes(fakeHResults))
		plugin = main.NewWingetPlugin()
	})

	It("reports an installed package", func() {
		recorder.Script("winget", query("Git.Git"), sdktest.Response{Stdout: "Name Id      Version\nGit  Git.Git 2.47.0\n"})
		Expect(plugin.Execute("is-installed", []string{"Git.Git"})).To(Succeed())
	})

	It("queries machine-scope packages at system scope", func() {
		machineQuery := argv([]string{"list", "--id", "Docker.DockerDesktop", "--exact", "--scope", "machine"}, agree)
		recorder.Script("winget", machineQuery, sdktest.Response{ExitCode: exitNotFound})
		err := plugin.Execute("is-installed", []string{"--scope=machine", "Docker.DockerDesktop"})
		var absent *sdk.NotInstalledError
		Expect(errors.As(err, &absent)).To(BeTrue(), "expected NotInstalledError, got %v", err)
		Expect(absent.Packages).To(Equal([]string{"Docker.DockerDesktop"}))
		Expect(recorder.Calls("winget")).To(Equal([]sdktest.Call{{Args: machineQuery, ElevationRequested: true}}))
	})

	It("reports winget's no-installed-package answer with the SDK status", func() {
		recorder.Script("winget", query("sharkdp.bat"), sdktest.Response{ExitCode: exitNotFound, Stdout: "No installed package found matching input criteria.\n"})
		err := plugin.Execute("is-installed", []string{"sharkdp.bat"})
		var absent *sdk.NotInstalledError
		Expect(errors.As(err, &absent)).To(BeTrue(), "expected NotInstalledError, got %v", err)
		Expect(absent.Packages).To(Equal([]string{"sharkdp.bat"}))
		Expect(sdk.ExitStatus(err)).To(Equal(sdk.ExitCodeNotInstalled))
	})

	It("reports only the missing packages from separate native queries", func() {
		recorder.Script("winget", query("sharkdp.bat"), sdktest.Response{ExitCode: exitNotFound})
		err := plugin.Execute("is-installed", []string{"Git.Git", "sharkdp.bat"})
		var absent *sdk.NotInstalledError
		Expect(errors.As(err, &absent)).To(BeTrue())
		Expect(absent.Packages).To(Equal([]string{"sharkdp.bat"}))
		Expect(recorder.Calls("winget")).To(Equal([]sdktest.Call{{Args: query("Git.Git")}, {Args: query("sharkdp.bat")}}))
	})

	It("does not mistake a winget failure for absence", func() {
		recorder.Script("winget", query("Git.Git"), sdktest.Response{ExitCode: 42, Stderr: "Failed when opening source(s)\n"})
		err := plugin.Execute("is-installed", []string{"Git.Git"})
		var absent *sdk.NotInstalledError
		Expect(errors.As(err, &absent)).To(BeFalse())
		Expect(err).To(MatchError(And(
			ContainSubstring("failed to check installation status of winget package Git.Git"),
			ContainSubstring("exit status 42"),
		)))
		Expect(sdk.ExitStatus(err)).To(Equal(42))
	})

	It("does not read a low exit status as an HRESULT without the fake mapping", func() {
		DeferCleanup(main.UseFakeExitCodes(nil))
		recorder.Script("winget", query("Git.Git"), sdktest.Response{ExitCode: exitNotFound})
		err := plugin.Execute("is-installed", []string{"Git.Git"})
		var absent *sdk.NotInstalledError
		Expect(errors.As(err, &absent)).To(BeFalse(), "exit 20 is not NO_APPLICATIONS_FOUND")
	})
})
