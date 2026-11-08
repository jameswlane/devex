package main_test

import (
	"context"
	"errors"

	main "github.com/jameswlane/devex/packages/package-manager-nixpkgs"
	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
	"github.com/jameswlane/devex/packages/plugin-sdk/sdktest"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Installed package query contract", func() {
	var recorder *sdktest.Recorder
	var plugin *main.NixpkgsPlugin
	argv := []string{"--query", "--installed", "--json"}
	BeforeEach(func() { recorder = sdktest.NewRecorder(GinkgoT(), "nix-env"); plugin = main.NewNixpkgsPlugin() })
	It("queries exact native argv without sudo", func() {
		recorder.Script("nix-env", argv, sdktest.Response{Stdout: "{\"git\":{\"name\":\"git-2.4\"}}"})
		Expect(plugin.Execute("is-installed", []string{"git"})).To(Succeed())
		Expect(recorder.Calls("nix-env")).To(Equal([]sdktest.Call{{Args: argv}}))
		Expect(recorder.Calls("sudo")).To(BeEmpty())
	})
	It("reports absence with the SDK status", func() {
		recorder.Script("nix-env", argv, sdktest.Response{Stdout: "{}"})
		err := plugin.Execute("is-installed", []string{"git"})
		Expect(err).To(BeAssignableToTypeOf(&sdk.NotInstalledError{}))
		Expect(sdk.ExitStatus(err)).To(Equal(sdk.ExitCodeNotInstalled))
	})
	It("reports only missing packages from a multi-package query", func() {
		recorder.Script("nix-env", argv, sdktest.Response{Stdout: "{\"git\":{\"name\":\"git-2.4\"}}"})

		err := plugin.Execute("is-installed", []string{"git", "missing"})
		var absent *sdk.NotInstalledError
		Expect(errors.As(err, &absent)).To(BeTrue())
		Expect(absent.Packages).To(Equal([]string{"missing"}))
	})
	It("runs unattended queries without input or sudo", func() {
		GinkgoT().Setenv("CI", "true")
		sdktest.CloseStdin(GinkgoT())
		recorder.Script("nix-env", argv, sdktest.Response{Stdout: "{\"git\":{\"name\":\"git-2.4\"}}"})
		Expect(plugin.Execute("is-installed", []string{"git"})).To(Succeed())
		Expect(recorder.Calls("sudo")).To(BeEmpty())
	})

	It("rejects corrupt native JSON as an operational failure", func() {
		recorder.Script("nix-env", argv, sdktest.Response{Stdout: "invalid"})
		err := plugin.Execute("is-installed", []string{"git"})
		Expect(err).To(HaveOccurred())
		var absent *sdk.NotInstalledError
		Expect(errors.As(err, &absent)).To(BeFalse())
	})

	It("preserves query failures rather than reporting absence", func() {
		recorder.Script("nix-env", argv, sdktest.Response{ExitCode: 42})
		Expect(sdk.ExitStatus(plugin.Execute("is-installed", []string{"git"}))).To(Equal(42))
	})
	It("rejects invalid queries before execution", func() {
		for _, args := range [][]string{nil, {"--help"}, {" "}} {
			Expect(plugin.Execute("is-installed", args)).To(HaveOccurred())
		}
		Expect(recorder.Calls("nix-env")).To(BeEmpty())
	})
	It("honors cancellation", func() {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		Expect(plugin.ExecuteContext(ctx, "is-installed", []string{"git"})).To(MatchError(context.Canceled))
		Expect(recorder.Calls("nix-env")).To(BeEmpty())
	})
	It("rejects incomplete JSON metadata", func() {
		recorder.Script("nix-env", argv, sdktest.Response{Stdout: `{"entry":{}}`})
		err := plugin.Execute("is-installed", []string{"git"})
		Expect(err).To(HaveOccurred())
		var absent *sdk.NotInstalledError
		Expect(errors.As(err, &absent)).To(BeFalse())
	})

})
