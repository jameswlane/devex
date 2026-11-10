package main_test

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	main "github.com/jameswlane/devex/packages/package-manager-nixflake"
	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
	"github.com/jameswlane/devex/packages/plugin-sdk/sdktest"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

const unsupportedJSON = "error: unrecognised flag '--json'\nTry 'nix --help' for more information.\n"

var _ = Describe("Nix 2.8 plain profile compatibility", func() {
	var recorder *sdktest.Recorder
	BeforeEach(func() { recorder = sdktest.NewRecorder(GinkgoT(), "nix") })
	legacyProbe := func() {
		recorder.Script("nix", []string{"profile", "list", "--json"}, sdktest.Response{ExitCode: 1, Stderr: unsupportedJSON})
	}
	It("retries the documented 2.8 listing and removes the complete recorded attribute path", func() {
		legacyProbe()
		data, err := os.ReadFile(filepath.Join("testdata", "profile-2.8.txt"))
		Expect(err).NotTo(HaveOccurred())
		recorder.Script("nix", []string{"profile", "list"}, sdktest.Response{Stdout: string(data)})
		Expect(main.NewNixflakePlugin().Execute("remove", []string{"spotify"})).To(Succeed())
		Expect(recorder.Calls("nix")).To(Equal([]sdktest.Call{
			{Args: []string{"profile", "list", "--json"}},
			{Args: []string{"profile", "list"}},
			{Args: []string{"profile", "remove", `legacyPackages\.x86_64-linux\.spotify`}},
		}))
	})
	It("answers is-installed from the plain 2.8 listing when --json is unsupported", func() {
		legacyProbe()
		data, err := os.ReadFile(filepath.Join("testdata", "profile-2.8.txt"))
		Expect(err).NotTo(HaveOccurred())
		recorder.Script("nix", []string{"profile", "list"}, sdktest.Response{Stdout: string(data)})
		plugin := main.NewNixflakePlugin()
		Expect(plugin.Execute("is-installed", []string{"spotify", "zoom-us"})).To(Succeed())
		err = plugin.Execute("is-installed", []string{"spotify", "git"})
		var absent *sdk.NotInstalledError
		Expect(errors.As(err, &absent)).To(BeTrue())
		Expect(absent.Packages).To(Equal([]string{"git"}))
		Expect(recorder.Calls("nix")).To(HaveLen(4))
		Expect(recorder.Calls("sudo")).To(BeEmpty())
	})
	It("does not report absence when the plain listing is malformed or fails", func() {
		legacyProbe()
		recorder.Script("nix", []string{"profile", "list"}, sdktest.Response{Stdout: "garbage\n"})
		err := main.NewNixflakePlugin().Execute("is-installed", []string{"git"})
		Expect(err).To(HaveOccurred())
		var absent *sdk.NotInstalledError
		Expect(errors.As(err, &absent)).To(BeFalse())
		recorder.Script("nix", []string{"profile", "list"}, sdktest.Response{ExitCode: 42})
		Expect(sdk.ExitStatus(main.NewNixflakePlugin().Execute("is-installed", []string{"git"}))).To(Equal(42))
	})
	DescribeTable("does not reinterpret native metadata failures as old Nix", func(diagnostic string) {
		recorder.Script("nix", []string{"profile", "list", "--json"}, sdktest.Response{ExitCode: 42, Stderr: diagnostic})
		err := main.NewNixflakePlugin().Execute("remove", []string{"git"})
		Expect(sdk.ExitStatus(err)).To(Equal(42))
		Expect(recorder.Calls("nix")).To(Equal([]sdktest.Call{{Args: []string{"profile", "list", "--json"}}}))
	},
		Entry("a non-usage exit with the exact flag diagnostic", unsupportedJSON),
		Entry("store failure", "error: cannot connect to daemon\n"),
		Entry("an unrelated unsupported flag", "error: unrecognised flag '--profile'\n"),
		Entry("unsupported flag quoted in another error", "error: evaluation failed: error: unrecognised flag '--json'\n"),
		Entry("unsupported flag followed by other errors", "error: unrecognised flag '--json'\nerror: profile unreadable\n"),
	)
	It("preserves failure from the older listing without removing anything", func() {
		legacyProbe()
		recorder.Script("nix", []string{"profile", "list"}, sdktest.Response{ExitCode: 42, Stderr: "error: cannot read profile\n"})
		err := main.NewNixflakePlugin().Execute("remove", []string{"git"})
		Expect(sdk.ExitStatus(err)).To(Equal(42))
		Expect(recorder.Calls("nix")).To(HaveLen(2))
	})
	It("does not fall back when the native executable cannot be looked up", func() {
		sdktest.NewRecorder(GinkgoT())
		Expect(main.NewNixflakePlugin().Execute("remove", []string{"git"})).To(MatchError(ContainSubstring("nix is not available")))
		Expect(recorder.Calls("nix")).To(BeEmpty())
	})
	It("does not fall back when the invoking account lookup fails", func() {
		GinkgoT().Setenv("SUDO_USER", "missing-user")
		sdk.NewPrivilegeResolver = elevatedResolver(GinkgoT().TempDir())
		err := main.NewNixflakePlugin().Execute("remove", []string{"git"})
		Expect(err).To(MatchError(ContainSubstring("unknown account")))
		Expect(recorder.Calls("nix")).To(BeEmpty())
		Expect(recorder.Calls("sudo")).To(BeEmpty())
	})
	It("preserves native launch errors after an unsupported flag probe", func() {
		legacyProbe()
		original := sdk.NewCommandContext
		sdk.NewCommandContext = func(ctx context.Context, name string, args ...string) *exec.Cmd {
			cmd := original(ctx, name, args...)
			if filepath.Base(name) == "nix" && len(args) == 2 {
				cmd.Err = exec.ErrNotFound
			}
			return cmd
		}
		defer func() { sdk.NewCommandContext = original }()
		err := main.NewNixflakePlugin().Execute("remove", []string{"git"})
		Expect(errors.Is(err, exec.ErrNotFound)).To(BeTrue())
		Expect(recorder.Calls("nix")).To(HaveLen(1))
	})
	DescribeTable("never falls back on typed authorization failures", func(afterProbe bool) {
		calls := 0
		expected := sdk.NewAuthorizationDeniedError("test authorization denial", 1, fmt.Errorf("permission denied"))
		sdk.DefaultPrivilegeAdapter = profileFailureAdapter{run: func(_ context.Context, _ sdk.ExecutionPlan, streams sdk.OperationIO) error {
			calls++
			if afterProbe && calls == 1 {
				// A real ExitError from the fake establishes the supported fallback trigger.
				cmd := exec.Command("/bin/sh", "-c", "exit 1")
				_, _ = io.WriteString(streams.Stderr, unsupportedJSON)
				return cmd.Run()
			}
			_, _ = io.WriteString(streams.Stderr, unsupportedJSON)
			return expected
		}}
		err := main.NewNixflakePlugin().Execute("remove", []string{"git"})
		Expect(err).To(BeIdenticalTo(expected))
		if afterProbe {
			Expect(calls).To(Equal(2))
		} else {
			Expect(calls).To(Equal(1))
		}
		Expect(recorder.Calls("nix")).To(BeEmpty())
	},
		Entry("JSON probe", false),
		Entry("plain listing", true),
	)
	DescribeTable("rejects malformed plain rows before removal", func(metadata string) {
		legacyProbe()
		recorder.Script("nix", []string{"profile", "list"}, sdktest.Response{Stdout: metadata})
		Expect(main.NewNixflakePlugin().Execute("remove", []string{"git"})).To(MatchError(ContainSubstring("malformed metadata")))
		Expect(recorder.Calls("nix")).To(HaveLen(2))
	},
		Entry("incomplete row", "0 flake:nixpkgs#legacyPackages.x86_64-linux.git\n"),
		Entry("non-index row", "warning: something unusual happened here\n"),
		Entry("missing attribute", "0 flake:nixpkgs github:nixos/nixpkgs /nix/store/hash-git\n"),
		Entry("mismatched attributes", "0 flake:nixpkgs#a.git github:nixos/nixpkgs#a.vim /nix/store/hash-git\n"),
		Entry("non-store output", "0 flake:nixpkgs#a.git github:nixos/nixpkgs#a.git /tmp/git\n"),
		Entry("duplicate indexes", "0 - - /nix/store/hash-one\n0 - - /nix/store/hash-two\n"),
		Entry("one valid and one malformed row", "0 flake:nixpkgs#a.git github:nixos/nixpkgs#a.git /nix/store/hash-git\ninvalid\n"),
	)
	It("fails a short-name lookup without calling removal", func() {
		legacyProbe()
		recorder.Script("nix", []string{"profile", "list"}, sdktest.Response{Stdout: "0 - - /nix/store/hash-git\n"})
		Expect(main.NewNixflakePlugin().Execute("remove", []string{"git"})).To(MatchError(ContainSubstring("not found")))
		Expect(recorder.Calls("nix")).To(HaveLen(2))
	})
})

type profileFailureAdapter struct {
	run func(context.Context, sdk.ExecutionPlan, sdk.OperationIO) error
}

func (a profileFailureAdapter) Run(ctx context.Context, plan sdk.ExecutionPlan, streams sdk.OperationIO) error {
	return a.run(ctx, plan, streams)
}
