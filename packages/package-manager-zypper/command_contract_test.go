package main_test

import (
	"context"
	"errors"
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	main "github.com/jameswlane/devex/packages/package-manager-zypper"
	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
	"github.com/jameswlane/devex/packages/plugin-sdk/sdktest"
)

// operationContract pins the exact native argv and elevation of one advertised
// command that runs zypper.
type operationContract struct {
	command    string
	args       []string
	nativeArgv []string
	elevated   bool
}

var contracts = []operationContract{
	{command: "upgrade", nativeArgv: []string{"--non-interactive", "update"}, elevated: true},
	{command: "install", args: []string{"git", "vim"}, nativeArgv: []string{"--non-interactive", "install", "--type", "package", "git", "vim"}, elevated: true},
	{command: "remove", args: []string{"git", "vim"}, nativeArgv: []string{"--non-interactive", "remove", "--type", "package", "git", "vim"}, elevated: true},
	{command: "is-installed", args: []string{"git"}, nativeArgv: isInstalledArgv("package", "git")},
	{command: "update", nativeArgv: []string{"--non-interactive", "refresh"}, elevated: true},
	{command: "search", args: []string{"web", "server"}, nativeArgv: []string{"search", "--search-descriptions", "web server"}},
	{command: "list", nativeArgv: []string{"search", "--installed-only"}},
}

// isInstalledArgv is the exact, local-only query is-installed runs per name.
func isInstalledArgv(kind, name string) []string {
	return []string{"--non-interactive", "--quiet", "--no-refresh", "--disable-repositories",
		"search", "--installed-only", "--match-exact", "--type", kind, "--", name}
}

var _ = Describe("ZYPPER command contracts", func() {
	var (
		plugin   *main.ZypperPlugin
		recorder *sdktest.Recorder
	)

	BeforeEach(func() {
		recorder = sdktest.NewRecorder(GinkgoT(), "zypper")
		plugin = main.NewZypperPlugin()
	})

	expectNoNativeCalls := func() {
		GinkgoHelper()
		Expect(recorder.Calls("zypper")).To(BeEmpty())
		Expect(recorder.Calls("sudo")).To(BeEmpty())
	}

	It("has a contract for every advertised command", func() {
		covered := make([]string, 0, len(contracts))
		for _, contract := range contracts {
			covered = append(covered, contract.command)
		}
		Expect(covered).To(ConsistOf(sdktest.CommandNames(plugin.Info())))
	})

	for _, contract := range contracts {
		It(fmt.Sprintf("honors the exact native argv and elevation for %s", contract.command), func() {
			Expect(plugin.Execute(contract.command, contract.args)).To(Succeed())
			Expect(recorder.Calls("zypper")).To(Equal([]sdktest.Call{{
				Args:               contract.nativeArgv,
				ElevationRequested: contract.elevated,
			}}))
		})

		It(fmt.Sprintf("advertises the execution privilege used by %s", contract.command), func() {
			privilege := sdk.PluginOperationPrivilege(plugin.Info(), []string{contract.command})
			Expect(privilege).NotTo(BeNil())
			scope := sdk.ScopeUser
			if contract.elevated {
				scope = sdk.ScopeSystem
			}
			Expect(privilege.Scope).To(Equal(scope))
		})

		It(fmt.Sprintf("propagates the native exit status from %s", contract.command), func() {
			recorder.Script("zypper", contract.nativeArgv, sdktest.Response{ExitCode: 42, Stderr: "zypper failed\n"})
			err := plugin.Execute(contract.command, contract.args)
			Expect(err).To(MatchError(ContainSubstring("exit status 42")))
			Expect(sdk.ExitStatus(err)).To(Equal(42))
			Expect(recorder.Calls("zypper")).To(HaveLen(1))
		})

		It(fmt.Sprintf("does not launch %s after cancellation", contract.command), func() {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			Expect(plugin.ExecuteContext(ctx, contract.command, contract.args)).To(MatchError(context.Canceled))
			expectNoNativeCalls()
		})
	}

	DescribeTable("rejects invalid input without invoking zypper",
		func(command string, args []string, message string) {
			Expect(plugin.Execute(command, args)).To(MatchError(ContainSubstring(message)))
			expectNoNativeCalls()
		},
		Entry("install without packages", "install", nil, "no packages specified"),
		Entry("remove without packages", "remove", nil, "no packages specified"),
		Entry("install with an option-like package", "install", []string{"--force"}, "invalid package name"),
		Entry("install with an unsupported type", "install", []string{"--type", "product", "openSUSE"}, "unsupported zypper type"),
		Entry("install with conflicting types", "install", []string{"-t", "pattern", "--type=package", "git"}, "selects one type"),
		Entry("install with a type and no value", "install", []string{"git", "-t"}, "requires a value"),
		Entry("install with only a type", "install", []string{"--type=pattern"}, "no packages specified"),
		Entry("install a pattern with a version relation", "install", []string{"-t", "pattern", "devel_basis>=1"}, "invalid pattern name"),
		Entry("install a name with whitespace", "install", []string{"git vim"}, "invalid package name"),
		Entry("is-installed without packages", "is-installed", nil, "no packages specified"),
		Entry("is-installed with a version relation", "is-installed", []string{"git>=2"}, "invalid package name"),
		Entry("is-installed with an option-like package", "is-installed", []string{"--details"}, "invalid package name"),
		Entry("remove with a blank package", "remove", []string{""}, "invalid package name"),
		Entry("upgrade with an option-like package", "upgrade", []string{"--no-recommends"}, "invalid package name"),
		Entry("search without a term", "search", nil, "no search term specified"),
		Entry("search with an option-like term", "search", []string{"--details"}, "must not start with '-'"),
		Entry("update with arguments", "update", []string{"repo-oss"}, "use upgrade to update packages"),
		Entry("list with arguments", "list", []string{"git"}, "list takes no arguments"),
	)

	It("upgrades only the named packages when upgrade receives arguments", func() {
		Expect(plugin.Execute("upgrade", []string{"git", "vim"})).To(Succeed())
		Expect(recorder.Calls("zypper")).To(Equal([]sdktest.Call{{
			Args:               []string{"--non-interactive", "update", "git", "vim"},
			ElevationRequested: true,
		}}))
	})

	DescribeTable("passes the pattern selection to zypper",
		func(command string, args []string, nativeArgv []string, elevated bool) {
			Expect(plugin.Execute(command, args)).To(Succeed())
			Expect(recorder.Calls("zypper")).To(Equal([]sdktest.Call{{Args: nativeArgv, ElevationRequested: elevated}}))
		},
		Entry("install -t pattern", "install", []string{"-t", "pattern", "devel_basis", "devel_C_C++"},
			[]string{"--non-interactive", "install", "--type", "pattern", "devel_basis", "devel_C_C++"}, true),
		Entry("install --type=pattern after the name", "install", []string{"devel_basis", "--type=pattern"},
			[]string{"--non-interactive", "install", "--type", "pattern", "devel_basis"}, true),
		Entry("install a versioned package", "install", []string{"--type", "package", "git>=2.40"},
			[]string{"--non-interactive", "install", "--type", "package", "git>=2.40"}, true),
		Entry("remove --type pattern", "remove", []string{"--type", "pattern", "devel_basis"},
			[]string{"--non-interactive", "remove", "--type", "pattern", "devel_basis"}, true),
		Entry("is-installed --type=pattern", "is-installed", []string{"--type=pattern", "devel_basis"},
			isInstalledArgv("pattern", "devel_basis"), false),
	)

	Context("is-installed", func() {
		It("queries every name and reports the absent ones", func() {
			recorder.Script("zypper", isInstalledArgv("package", "vim"), sdktest.Response{ExitCode: 104, Stdout: "No matching items found.\n"})
			recorder.Script("zypper", isInstalledArgv("package", "nano"), sdktest.Response{ExitCode: 104})
			err := plugin.Execute("is-installed", []string{"git", "vim", "nano"})
			var missing *sdk.NotInstalledError
			Expect(errors.As(err, &missing)).To(BeTrue(), "expected NotInstalledError, got %v", err)
			Expect(missing.Packages).To(Equal([]string{"vim", "nano"}))
			Expect(sdk.ExitStatus(err)).To(Equal(sdk.ExitCodeNotInstalled))
			Expect(recorder.Calls("zypper")).To(HaveLen(3))
		})

		It("reports an absent pattern", func() {
			recorder.Script("zypper", isInstalledArgv("pattern", "devel_basis"), sdktest.Response{ExitCode: 104})
			err := plugin.Execute("is-installed", []string{"-t", "pattern", "devel_basis"})
			var missing *sdk.NotInstalledError
			Expect(errors.As(err, &missing)).To(BeTrue())
			Expect(missing.Packages).To(Equal([]string{"devel_basis"}))
		})

		It("treats any other failure as a failed query with zypper's diagnostic", func() {
			recorder.Script("zypper", isInstalledArgv("package", "git"), sdktest.Response{ExitCode: 7, Stderr: "System management is locked\n"})
			err := plugin.Execute("is-installed", []string{"git", "vim"})
			var missing *sdk.NotInstalledError
			Expect(errors.As(err, &missing)).To(BeFalse())
			Expect(err).To(MatchError(ContainSubstring("System management is locked")))
			Expect(sdk.ExitStatus(err)).To(Equal(7))
			Expect(recorder.Calls("zypper")).To(HaveLen(1))
		})
	})

	DescribeTable("treats zypper's reboot-needed status as success",
		func(command string, args []string, nativeArgv []string) {
			recorder.Script("zypper", nativeArgv, sdktest.Response{ExitCode: 102})
			Expect(plugin.Execute(command, args)).To(Succeed())
		},
		Entry("install", "install", []string{"kernel-default"}, []string{"--non-interactive", "install", "--type", "package", "kernel-default"}),
		Entry("remove", "remove", []string{"kernel-default"}, []string{"--non-interactive", "remove", "--type", "package", "kernel-default"}),
		Entry("upgrade", "upgrade", nil, []string{"--non-interactive", "update"}),
	)

	It("keeps a missing package or pattern as zypper's not-found error", func() {
		recorder.Script("zypper", []string{"--non-interactive", "install", "--type", "pattern", "nope"}, sdktest.Response{ExitCode: 104, Stderr: "No provider of 'nope' found.\n"})
		err := plugin.Execute("install", []string{"-t", "pattern", "nope"})
		Expect(sdk.ExitStatus(err)).To(Equal(104))
	})

	It("rejects an unknown command", func() {
		Expect(plugin.Execute("unknown", nil)).To(MatchError("unknown command: unknown"))
		expectNoNativeCalls()
	})
})
