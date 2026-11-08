package main_test

import (
	"context"
	"errors"

	main "github.com/jameswlane/devex/packages/package-manager-emerge"
	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
	"github.com/jameswlane/devex/packages/plugin-sdk/sdktest"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Installed package query contract", func() {
	var recorder *sdktest.Recorder
	var plugin *main.EmergePlugin
	argv := []string{"match", "/", "git"}
	BeforeEach(func() { recorder = sdktest.NewRecorder(GinkgoT(), "portageq"); plugin = main.NewEmergePlugin() })
	It("lists installed packages with the bundled query tool", func() {
		Expect(plugin.Execute("list", nil)).To(Succeed())
		Expect(recorder.Calls("portageq")).To(Equal([]sdktest.Call{{Args: []string{"match", "/", ""}}}))
		Expect(recorder.Calls("sudo")).To(BeEmpty())
	})
	It("queries exact native argv without sudo", func() {
		recorder.Script("portageq", argv, sdktest.Response{Stdout: "dev-vcs/git-2.4\n"})
		Expect(plugin.Execute("is-installed", []string{"git"})).To(Succeed())
		Expect(recorder.Calls("portageq")).To(Equal([]sdktest.Call{{Args: argv}}))
		Expect(recorder.Calls("sudo")).To(BeEmpty())
	})
	It("reports absence with the SDK status", func() {
		recorder.Script("portageq", argv, sdktest.Response{Stdout: ""})
		err := plugin.Execute("is-installed", []string{"git"})
		Expect(err).To(BeAssignableToTypeOf(&sdk.NotInstalledError{}))
		Expect(sdk.ExitStatus(err)).To(Equal(sdk.ExitCodeNotInstalled))
	})
	It("reports only missing packages from a multi-package query", func() {
		recorder.Script("portageq", argv, sdktest.Response{Stdout: "dev-vcs/git-2.4\n"})
		recorder.Script("portageq", []string{"match", "/", "missing"}, sdktest.Response{})
		err := plugin.Execute("is-installed", []string{"git", "missing"})
		var absent *sdk.NotInstalledError
		Expect(errors.As(err, &absent)).To(BeTrue())
		Expect(absent.Packages).To(Equal([]string{"missing"}))
	})
	It("runs unattended queries without input or sudo", func() {
		GinkgoT().Setenv("CI", "true")
		sdktest.CloseStdin(GinkgoT())
		recorder.Script("portageq", argv, sdktest.Response{Stdout: "dev-vcs/git-2.4\n"})
		Expect(plugin.Execute("is-installed", []string{"git"})).To(Succeed())
		Expect(recorder.Calls("sudo")).To(BeEmpty())
	})

	It("preserves query failures rather than reporting absence", func() {
		recorder.Script("portageq", argv, sdktest.Response{ExitCode: 42})
		Expect(sdk.ExitStatus(plugin.Execute("is-installed", []string{"git"}))).To(Equal(42))
	})
	It("rejects invalid queries before execution", func() {
		for _, args := range [][]string{nil, {"--help"}, {" "}} {
			Expect(plugin.Execute("is-installed", args)).To(HaveOccurred())
		}
		Expect(recorder.Calls("portageq")).To(BeEmpty())
	})
	It("honors cancellation", func() {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		Expect(plugin.ExecuteContext(ctx, "is-installed", []string{"git"})).To(MatchError(context.Canceled))
		Expect(recorder.Calls("portageq")).To(BeEmpty())
	})
	It("preserves failures from the installed listing", func() {
		recorder.Script("portageq", []string{"match", "/", ""}, sdktest.Response{ExitCode: 42})
		Expect(sdk.ExitStatus(plugin.Execute("list", nil))).To(Equal(42))
	})

})
