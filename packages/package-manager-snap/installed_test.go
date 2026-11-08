package main

import (
	"context"
	"errors"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
	"github.com/jameswlane/devex/packages/plugin-sdk/sdktest"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Installed package query contract", func() {
	var recorder *sdktest.Recorder
	var plugin *SnapPlugin
	argv := []string{"list"}
	BeforeEach(func() { recorder = sdktest.NewRecorder(GinkgoT(), "snap"); plugin = NewSnapPlugin() })
	It("queries exact native argv without sudo", func() {
		recorder.Script("snap", argv, sdktest.Response{Stdout: "Name Version Rev Tracking Publisher Notes\ngit 1 1 latest/stable test -\n"})
		Expect(plugin.Execute("is-installed", []string{"git"})).To(Succeed())
		Expect(recorder.Calls("snap")).To(Equal([]sdktest.Call{{Args: argv}}))
		Expect(recorder.Calls("sudo")).To(BeEmpty())
	})
	It("reports absence with the SDK status", func() {
		recorder.Script("snap", argv, sdktest.Response{Stdout: "Name Version Rev Tracking Publisher Notes\n"})
		err := plugin.Execute("is-installed", []string{"git"})
		Expect(err).To(BeAssignableToTypeOf(&sdk.NotInstalledError{}))
		Expect(sdk.ExitStatus(err)).To(Equal(sdk.ExitCodeNotInstalled))
	})
	It("reports only missing packages from a multi-package query", func() {
		recorder.Script("snap", argv, sdktest.Response{Stdout: "Name Version Rev Tracking Publisher Notes\ngit 1 1 latest/stable test -\n"})

		err := plugin.Execute("is-installed", []string{"git", "missing"})
		var absent *sdk.NotInstalledError
		Expect(errors.As(err, &absent)).To(BeTrue())
		Expect(absent.Packages).To(Equal([]string{"missing"}))
	})
	It("runs unattended queries without input or sudo", func() {
		GinkgoT().Setenv("CI", "true")
		sdktest.CloseStdin(GinkgoT())
		recorder.Script("snap", argv, sdktest.Response{Stdout: "Name Version Rev Tracking Publisher Notes\ngit 1 1 latest/stable test -\n"})
		Expect(plugin.Execute("is-installed", []string{"git"})).To(Succeed())
		Expect(recorder.Calls("sudo")).To(BeEmpty())
	})

	It("preserves query failures rather than reporting absence", func() {
		recorder.Script("snap", argv, sdktest.Response{ExitCode: 42})
		Expect(sdk.ExitStatus(plugin.Execute("is-installed", []string{"git"}))).To(Equal(42))
	})
	It("rejects invalid queries before execution", func() {
		for _, args := range [][]string{nil, {"--help"}, {" "}} {
			Expect(plugin.Execute("is-installed", args)).To(HaveOccurred())
		}
		Expect(recorder.Calls("snap")).To(BeEmpty())
	})
	It("honors cancellation", func() {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		Expect(plugin.ExecuteContext(ctx, "is-installed", []string{"git"})).To(MatchError(context.Canceled))
		Expect(recorder.Calls("snap")).To(BeEmpty())
	})
})
