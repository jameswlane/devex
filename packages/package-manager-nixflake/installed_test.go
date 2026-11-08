package main_test

import (
	"context"
	"errors"

	main "github.com/jameswlane/devex/packages/package-manager-nixflake"
	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
	"github.com/jameswlane/devex/packages/plugin-sdk/sdktest"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Installed package query contract", func() {
	var recorder *sdktest.Recorder
	var plugin *main.NixflakePlugin
	argv := []string{"profile", "list", "--json"}
	BeforeEach(func() { recorder = sdktest.NewRecorder(GinkgoT(), "nix"); plugin = main.NewNixflakePlugin() })
	It("queries exact native argv without sudo", func() {
		recorder.Script("nix", argv, sdktest.Response{Stdout: "{\"version\":3,\"elements\":{\"git\":{\"attrPath\":\"legacyPackages.x86_64-linux.git\"}}}"})
		Expect(plugin.Execute("is-installed", []string{"git"})).To(Succeed())
		Expect(recorder.Calls("nix")).To(Equal([]sdktest.Call{{Args: argv}}))
		Expect(recorder.Calls("sudo")).To(BeEmpty())
	})
	It("reports absence with the SDK status", func() {
		recorder.Script("nix", argv, sdktest.Response{Stdout: "{\"version\":3,\"elements\":{}}"})
		err := plugin.Execute("is-installed", []string{"git"})
		Expect(err).To(BeAssignableToTypeOf(&sdk.NotInstalledError{}))
		Expect(sdk.ExitStatus(err)).To(Equal(sdk.ExitCodeNotInstalled))
	})
	It("reports only missing packages from a multi-package query", func() {
		recorder.Script("nix", argv, sdktest.Response{Stdout: "{\"version\":3,\"elements\":{\"git\":{\"attrPath\":\"legacyPackages.x86_64-linux.git\"}}}"})

		err := plugin.Execute("is-installed", []string{"git", "missing"})
		var absent *sdk.NotInstalledError
		Expect(errors.As(err, &absent)).To(BeTrue())
		Expect(absent.Packages).To(Equal([]string{"missing"}))
	})
	It("runs unattended queries without input or sudo", func() {
		GinkgoT().Setenv("CI", "true")
		sdktest.CloseStdin(GinkgoT())
		recorder.Script("nix", argv, sdktest.Response{Stdout: "{\"version\":3,\"elements\":{\"git\":{\"attrPath\":\"legacyPackages.x86_64-linux.git\"}}}"})
		Expect(plugin.Execute("is-installed", []string{"git"})).To(Succeed())
		Expect(recorder.Calls("sudo")).To(BeEmpty())
	})

	It("rejects corrupt native JSON as an operational failure", func() {
		recorder.Script("nix", argv, sdktest.Response{Stdout: "invalid"})
		err := plugin.Execute("is-installed", []string{"git"})
		Expect(err).To(HaveOccurred())
		var absent *sdk.NotInstalledError
		Expect(errors.As(err, &absent)).To(BeFalse())
	})

	It("preserves query failures rather than reporting absence", func() {
		recorder.Script("nix", argv, sdktest.Response{ExitCode: 42})
		Expect(sdk.ExitStatus(plugin.Execute("is-installed", []string{"git"}))).To(Equal(42))
	})
	It("rejects invalid queries before execution", func() {
		for _, args := range [][]string{nil, {"--help"}, {" "}} {
			Expect(plugin.Execute("is-installed", args)).To(HaveOccurred())
		}
		Expect(recorder.Calls("nix")).To(BeEmpty())
	})
	It("honors cancellation", func() {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		Expect(plugin.ExecuteContext(ctx, "is-installed", []string{"git"})).To(MatchError(context.Canceled))
		Expect(recorder.Calls("nix")).To(BeEmpty())
	})
	It("upgrades legacy indexed profiles with the legacy selector", func() {
		recorder.Script("nix", []string{"profile", "list", "--json"}, sdktest.Response{Stdout: `{"version":2,"elements":[]}`})
		Expect(plugin.Execute("upgrade", nil)).To(Succeed())
		Expect(operationCalls(recorder, "nix")).To(Equal([]sdktest.Call{{Args: []string{"profile", "upgrade", ".*"}}}))
	})
	It("upgrades pre-JSON Nix with the legacy selector", func() {
		recorder.Script("nix", []string{"profile", "list", "--json"}, sdktest.Response{ExitCode: 1, Stderr: "error: unrecognised flag '--json'"})
		Expect(plugin.Execute("upgrade", nil)).To(Succeed())
		Expect(operationCalls(recorder, "nix")).To(Equal([]sdktest.Call{{Args: []string{"profile", "upgrade", ".*"}}}))
	})
	It("does not upgrade when profile inspection fails", func() {
		recorder.Script("nix", []string{"profile", "list", "--json"}, sdktest.Response{ExitCode: 42})
		Expect(sdk.ExitStatus(plugin.Execute("upgrade", nil))).To(Equal(42))
		Expect(operationCalls(recorder, "nix")).To(BeEmpty())
	})

	It("rejects incomplete JSON metadata", func() {
		recorder.Script("nix", argv, sdktest.Response{Stdout: `{"version":2,"elements":null}`})
		err := plugin.Execute("is-installed", []string{"git"})
		Expect(err).To(HaveOccurred())
		var absent *sdk.NotInstalledError
		Expect(errors.As(err, &absent)).To(BeFalse())
	})

})
