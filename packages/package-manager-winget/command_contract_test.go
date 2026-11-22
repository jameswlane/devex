//go:build !windows

// Recorder fakes are /bin/sh scripts, so these contracts run on Linux and
// macOS. smoke_windows_test.go drives the real winget on Windows.

package main_test

import (
	"context"
	"errors"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	main "github.com/jameswlane/devex/packages/package-manager-winget"
	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
	"github.com/jameswlane/devex/packages/plugin-sdk/sdktest"
)

// A fake exits with the low byte of the HRESULT it stands for, as a Unix
// process would; the plugin maps it back through UseFakeExitCodes.
var fakeHResults = map[int]uint32{
	0x10: 0x8A150010, // NO_APPLICABLE_INSTALLER
	0x14: 0x8A150014, // NO_APPLICATIONS_FOUND
	0x19: 0x8A150019, // COMMAND_REQUIRES_ADMIN
	0x2B: 0x8A15002B, // UPDATE_NOT_APPLICABLE
	0x56: 0x8A150056, // INSTALLER_PROHIBITS_ELEVATION
	0x61: 0x8A150061, // PACKAGE_ALREADY_INSTALLED
	0x0C: 0x8A15010C, // INSTALL_CANCELLED_BY_USER
	0x0D: 0x8A15010D, // INSTALL_ALREADY_INSTALLED
	0xC7: 0x800704C7, // HRESULT_FROM_WIN32(ERROR_CANCELLED)
}

const (
	exitNotFound         = 0x14
	exitNoUpdate         = 0x2B
	exitAlreadyInstalled = 0x61
	exitUACDeclined      = 0xC7
)

var (
	agree   = []string{"--accept-source-agreements", "--disable-interactivity"}
	change  = []string{"--silent", "--accept-package-agreements", "--accept-source-agreements", "--disable-interactivity"}
	removal = []string{"--silent", "--accept-source-agreements", "--disable-interactivity"}
)

func argv(parts ...[]string) []string {
	var out []string
	for _, part := range parts {
		out = append(out, part...)
	}
	return out
}

// operationContract pins the exact native argv and elevation of one advertised
// command that runs winget.
type operationContract struct {
	name       string
	command    string
	args       []string
	nativeArgv [][]string
	elevated   bool
}

var contracts = []operationContract{
	{name: "install", command: "install", args: []string{"Git.Git"}, nativeArgv: [][]string{
		argv([]string{"install", "--id", "Git.Git", "--exact"}, change, []string{"--no-upgrade"}),
	}},
	{name: "install of several packages", command: "install", args: []string{"Git.Git", "sharkdp.bat"}, nativeArgv: [][]string{
		argv([]string{"install", "--id", "Git.Git", "--exact"}, change, []string{"--no-upgrade"}),
		argv([]string{"install", "--id", "sharkdp.bat", "--exact"}, change, []string{"--no-upgrade"}),
	}},
	{name: "user-scope install", command: "install", args: []string{"--scope=user", "Git.Git"}, nativeArgv: [][]string{
		argv([]string{"install", "--id", "Git.Git", "--exact"}, change, []string{"--no-upgrade", "--scope", "user"}),
	}},
	{name: "machine-scope install", command: "install", args: []string{"--scope", "machine", "Docker.DockerDesktop"}, elevated: true, nativeArgv: [][]string{
		argv([]string{"install", "--id", "Docker.DockerDesktop", "--exact"}, change, []string{"--no-upgrade", "--scope", "machine"}),
	}},
	{name: "Store install", command: "install", args: []string{"9NBLGGH4NNS1"}, nativeArgv: [][]string{
		argv([]string{"install", "--id", "9NBLGGH4NNS1", "--exact"}, change, []string{"--no-upgrade"}),
	}},
	{name: "remove", command: "remove", args: []string{"Git.Git"}, nativeArgv: [][]string{
		argv([]string{"uninstall", "--id", "Git.Git", "--exact"}, removal),
	}},
	{name: "machine-scope remove", command: "remove", args: []string{"--scope=machine", "Git.Git"}, elevated: true, nativeArgv: [][]string{
		argv([]string{"uninstall", "--id", "Git.Git", "--exact"}, removal, []string{"--scope", "machine"}),
	}},
	{name: "upgrade", command: "upgrade", args: []string{"Git.Git"}, nativeArgv: [][]string{
		argv([]string{"upgrade", "--id", "Git.Git", "--exact"}, change),
	}},
	{name: "machine-scope upgrade", command: "upgrade", args: []string{"--scope=machine", "Git.Git"}, elevated: true, nativeArgv: [][]string{
		argv([]string{"upgrade", "--id", "Git.Git", "--exact"}, change, []string{"--scope", "machine"}),
	}},
	{name: "update", command: "update", nativeArgv: [][]string{{"source", "update", "--disable-interactivity"}}},
	{name: "search", command: "search", args: []string{"visual", "studio"}, nativeArgv: [][]string{
		argv([]string{"search", "--query", "visual studio"}, agree),
	}},
	{name: "list", command: "list", nativeArgv: [][]string{argv([]string{"list"}, agree)}},
	{name: "is-installed", command: "is-installed", args: []string{"Git.Git"}, nativeArgv: [][]string{
		argv([]string{"list", "--id", "Git.Git", "--exact"}, agree),
	}},
	{name: "machine-scope is-installed", command: "is-installed", args: []string{"--scope=machine", "Git.Git"}, elevated: true, nativeArgv: [][]string{
		argv([]string{"list", "--id", "Git.Git", "--exact", "--scope", "machine"}, agree),
	}},
}

var _ = Describe("winget command contracts", func() {
	var (
		plugin   *main.WingetPlugin
		recorder *sdktest.Recorder
	)

	BeforeEach(func() {
		recorder = sdktest.NewRecorder(GinkgoT(), "winget")
		DeferCleanup(main.UseFakeExitCodes(fakeHResults))
		plugin = main.NewWingetPlugin()
	})

	expectNoNativeCalls := func() {
		GinkgoHelper()
		Expect(recorder.Calls("winget")).To(BeEmpty())
		Expect(recorder.Calls("sudo")).To(BeEmpty())
	}

	It("has a contract for every advertised command", func() {
		covered := map[string]bool{}
		for _, contract := range contracts {
			covered[contract.command] = true
		}
		names := make([]string, 0, len(covered))
		for name := range covered {
			names = append(names, name)
		}
		Expect(names).To(ConsistOf(sdktest.CommandNames(plugin.Info())))
	})

	It("advertises user scope by default and system scope for --scope=machine before plugin launch", func() {
		info := plugin.Info()
		scoped := map[string]bool{"install": true, "is-installed": true, "remove": true, "upgrade": true}
		for _, command := range info.Commands {
			Expect(command.Privilege).NotTo(BeNil(), command.Name)
			Expect(command.Privilege.Scope).To(Equal(sdk.ScopeUser), command.Name)
			if scoped[command.Name] {
				Expect(sdk.PluginOperationPrivilege(info, []string{command.Name, "Git.Git"}).Scope).To(Equal(sdk.ScopeUser), command.Name)
				Expect(sdk.PluginOperationPrivilege(info, []string{command.Name, "--scope=user", "Git.Git"}).Scope).To(Equal(sdk.ScopeUser), command.Name)
				Expect(sdk.PluginOperationPrivilege(info, []string{command.Name, "--scope=machine", "Git.Git"}).Scope).To(Equal(sdk.ScopeSystem), command.Name)
				Expect(sdk.PluginOperationPrivilege(info, []string{command.Name, "--scope", "machine", "Git.Git"}).Scope).To(Equal(sdk.ScopeSystem), command.Name)
			}
		}
	})

	for _, contract := range contracts {
		It(fmt.Sprintf("honors the exact native argv and elevation for %s", contract.name), func() {
			Expect(plugin.Execute(contract.command, contract.args)).To(Succeed())
			calls := make([]sdktest.Call, 0, len(contract.nativeArgv))
			for _, args := range contract.nativeArgv {
				calls = append(calls, sdktest.Call{Args: args, ElevationRequested: contract.elevated})
			}
			Expect(recorder.Calls("winget")).To(Equal(calls))
		})

		It(fmt.Sprintf("propagates the native exit status from %s", contract.name), func() {
			recorder.Script("winget", contract.nativeArgv[0], sdktest.Response{ExitCode: 42, Stderr: "winget failed\n"})
			err := plugin.Execute(contract.command, contract.args)
			Expect(err).To(MatchError(ContainSubstring("exit status 42")))
			Expect(sdk.ExitStatus(err)).To(Equal(42))
			Expect(recorder.Calls("winget")).To(HaveLen(1), "stops at the first failure")
		})

		It(fmt.Sprintf("does not launch %s after cancellation", contract.name), func() {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			Expect(plugin.ExecuteContext(ctx, contract.command, contract.args)).To(MatchError(context.Canceled))
			expectNoNativeCalls()
		})
	}

	DescribeTable("rejects invalid input without invoking winget",
		func(command string, args []string, message string) {
			Expect(plugin.Execute(command, args)).To(MatchError(ContainSubstring(message)))
			expectNoNativeCalls()
		},
		Entry("install without identifiers", "install", nil, "no package identifiers specified"),
		Entry("install with only a scope", "install", []string{"--scope=user"}, "no package identifiers specified"),
		Entry("remove without identifiers", "remove", nil, "no package identifiers specified"),
		Entry("upgrade without identifiers (never upgrades everything)", "upgrade", nil, "no package identifiers specified"),
		Entry("is-installed without identifiers", "is-installed", nil, "no package identifiers specified"),
		Entry("install by display name", "install", []string{"git"}, `invalid winget package identifier "git"`),
		Entry("install with a space", "install", []string{"Git Git"}, "invalid winget package identifier"),
		Entry("install with a path", "install", []string{`C:\tools\git.exe`}, "invalid winget package identifier"),
		Entry("install with an empty segment", "install", []string{"Git..Git"}, "invalid winget package identifier"),
		Entry("install with a trailing dot", "install", []string{"Git.Git."}, "invalid winget package identifier"),
		Entry("install with a blank identifier", "install", []string{""}, "invalid winget package identifier"),
		Entry("install with a lowercase Store ID", "install", []string{"9nblggh4nns1"}, "invalid winget package identifier"),
		Entry("install with one bad identifier among good ones", "install", []string{"Git.Git", "vim"}, `invalid winget package identifier "vim"`),
		Entry("install with a native flag", "install", []string{"--force", "Git.Git"}, `unsupported option "--force"`),
		Entry("install with a short flag", "install", []string{"-e", "Git.Git"}, `unsupported option "-e"`),
		Entry("install with an invalid scope", "install", []string{"--scope=global", "Git.Git"}, `--scope must be user or machine, not "global"`),
		Entry("install with a missing scope value", "install", []string{"Git.Git", "--scope"}, "--scope requires user or machine"),
		Entry("install with conflicting scopes", "install", []string{"--scope=user", "--scope=machine", "Git.Git"}, "conflicting --scope values"),
		Entry("remove with --all", "remove", []string{"--all"}, "unsupported option"),
		Entry("is-installed with a name", "is-installed", []string{"git"}, "invalid winget package identifier"),
		Entry("search without a term", "search", nil, "no search term specified"),
		Entry("search with an option", "search", []string{"--id"}, "invalid search term"),
		Entry("search with a blank term", "search", []string{" "}, "invalid search term"),
		Entry("update with arguments", "update", []string{"Git.Git"}, "update takes no arguments"),
		Entry("list with arguments", "list", []string{"Git.Git"}, "list takes no arguments"),
	)

	It("rejects an unknown command", func() {
		Expect(plugin.Execute("unknown", nil)).To(MatchError("unknown command: unknown"))
		expectNoNativeCalls()
	})

	Context("when winget reports a result code", func() {
		install := argv([]string{"install", "--id", "Git.Git", "--exact"}, change, []string{"--no-upgrade"})
		upgrade := argv([]string{"upgrade", "--id", "Git.Git", "--exact"}, change)
		remove := argv([]string{"uninstall", "--id", "Git.Git", "--exact"}, removal)

		It("treats an already installed package as installed and keeps going", func() {
			recorder.Script("winget", install, sdktest.Response{ExitCode: exitAlreadyInstalled})
			output := sdktest.CaptureStdout(GinkgoT(), func() {
				Expect(plugin.Execute("install", []string{"Git.Git", "sharkdp.bat"})).To(Succeed())
			})
			Expect(output).To(ContainSubstring("Git.Git: already installed; use upgrade to update it"))
			Expect(recorder.Calls("winget")).To(HaveLen(2))
		})

		It("treats an installer's already-installed result as installed", func() {
			recorder.Script("winget", install, sdktest.Response{ExitCode: 0x0D})
			Expect(plugin.Execute("install", []string{"Git.Git"})).To(Succeed())
		})

		It("treats an upgrade with no newer version as current", func() {
			recorder.Script("winget", upgrade, sdktest.Response{ExitCode: exitNoUpdate})
			output := sdktest.CaptureStdout(GinkgoT(), func() {
				Expect(plugin.Execute("upgrade", []string{"Git.Git"})).To(Succeed())
			})
			Expect(output).To(ContainSubstring("Git.Git: already up to date"))
		})

		It("does not treat no-update as success for install", func() {
			recorder.Script("winget", install, sdktest.Response{ExitCode: exitNoUpdate})
			Expect(plugin.Execute("install", []string{"Git.Git"})).To(MatchError(ContainSubstring("failed with 0x8A15002B")))
		})

		It("reports a declined UAC prompt as an authorization denial with the native status", func() {
			recorder.Script("winget", install, sdktest.Response{ExitCode: exitUACDeclined})
			err := plugin.Execute("install", []string{"Git.Git"})
			var auth *sdk.AuthorizationError
			Expect(errors.As(err, &auth)).To(BeTrue(), "expected an authorization error, got %v", err)
			Expect(auth.Kind).To(Equal(sdk.AuthorizationDenied))
			Expect(err).To(MatchError(And(ContainSubstring("elevation was declined for Git.Git"), ContainSubstring("--scope=machine"))))
			Expect(sdk.ExitStatus(err)).To(Equal(exitUACDeclined))
		})

		DescribeTable("adds guidance and keeps winget's exit status",
			func(command string, native []string, exit int, message string) {
				recorder.Script("winget", native, sdktest.Response{ExitCode: exit})
				err := plugin.Execute(command, []string{"Git.Git"})
				Expect(err).To(MatchError(ContainSubstring(message)))
				Expect(sdk.ExitStatus(err)).To(Equal(exit))
				var auth *sdk.AuthorizationError
				Expect(errors.As(err, &auth)).To(BeFalse())
			},
			Entry("unknown identifier on install", "install", install, exitNotFound, "no winget package has the identifier Git.Git"),
			Entry("absent package on remove", "remove", remove, exitNotFound, "Git.Git is not installed"),
			Entry("absent package on upgrade", "upgrade", upgrade, exitNotFound, "Git.Git is not installed"),
			Entry("a cancelled installer", "install", install, 0x0C, "possibly by a declined UAC prompt"),
			Entry("a command that needs administrator rights", "remove", remove, 0x19, "requires administrator rights"),
			Entry("an installer that refuses elevation", "install", install, 0x56, "cannot run from an elevated terminal"),
			Entry("no installer for the system", "install", install, 0x10, "has no installer for this system"),
		)

		It("names the requested scope when no installer matches it", func() {
			native := argv([]string{"install", "--id", "Git.Git", "--exact"}, change, []string{"--no-upgrade", "--scope", "user"})
			recorder.Script("winget", native, sdktest.Response{ExitCode: 0x10})
			Expect(plugin.Execute("install", []string{"--scope=user", "Git.Git"})).To(MatchError(ContainSubstring("has no installer for user scope")))
		})

		It("says when a search finds nothing", func() {
			recorder.Script("winget", argv([]string{"search", "--query", "nothing"}, agree), sdktest.Response{ExitCode: exitNotFound})
			Expect(plugin.Execute("search", []string{"nothing"})).To(MatchError(ContainSubstring(`no winget package matches "nothing"`)))
		})
	})
})
