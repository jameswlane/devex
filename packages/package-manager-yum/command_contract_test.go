package main_test

import (
	"fmt"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	main "github.com/jameswlane/devex/packages/package-manager-yum"
	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
	"github.com/jameswlane/devex/packages/plugin-sdk/sdktest"
)

type operationContract struct {
	command    string
	args       []string
	nativeArgv []string
	elevated   bool
	probes     [][]string
}

var contracts = []operationContract{
	{command: "upgrade", nativeArgv: []string{"upgrade", "--assumeyes"}, elevated: true},
	{command: "install", args: []string{"git", "vim"}, nativeArgv: []string{"install", "--assumeyes", "git", "vim"}, elevated: true},
	{command: "remove", args: []string{"git", "vim"}, nativeArgv: []string{"remove", "--assumeyes", "git", "vim"}, elevated: true},
	{command: "update", nativeArgv: []string{"makecache", "--assumeyes"}, elevated: true},
	{command: "search", args: []string{"web", "server"}, nativeArgv: []string{"search", "web", "server"}},
	{command: "list", nativeArgv: []string{"list", "--installed"}, probes: [][]string{{"--version"}}},
}

var _ = Describe("YUM command contracts", func() {
	var (
		plugin   *main.YumPlugin
		recorder *sdktest.Recorder
	)

	BeforeEach(func() {
		recorder = sdktest.NewRecorder(GinkgoT(), "yum", "dnf", "rpm", "yum-config-manager")
		plugin = main.NewYumPlugin()
	})

	removeFake := func(command string) {
		GinkgoHelper()
		Expect(os.Remove(filepath.Join(os.Getenv("PATH"), command))).To(Succeed())
	}

	expectOnlyBackend := func(backend string, contract operationContract) {
		GinkgoHelper()
		other := "dnf"
		if backend == "dnf" {
			other = "yum"
		}
		expected := []sdktest.Call{}
		for _, probe := range contract.probes {
			expected = append(expected, sdktest.Call{Args: probe})
		}
		expected = append(expected, sdktest.Call{Args: contract.nativeArgv, ElevationRequested: contract.elevated})
		Expect(recorder.Calls(backend)).To(Equal(expected))
		Expect(recorder.Calls(other)).To(BeEmpty())
	}

	It("has a contract for every advertised command", func() {
		covered := []string{"is-installed", "add-repository"}
		for _, contract := range contracts {
			covered = append(covered, contract.command)
		}
		Expect(covered).To(ConsistOf(sdktest.CommandNames(plugin.Info())))
	})

	for _, contract := range contracts {
		contract := contract
		It(fmt.Sprintf("uses real yum with exact argv and elevation for %s", contract.command), func() {
			Expect(plugin.Execute(contract.command, contract.args)).To(Succeed())
			expectOnlyBackend("yum", contract)
		})

		It(fmt.Sprintf("uses dnf as the yum-compatible alias for %s when yum is absent", contract.command), func() {
			removeFake("yum")
			Expect(plugin.Execute(contract.command, contract.args)).To(Succeed())
			expectOnlyBackend("dnf", contract)
		})

		It(fmt.Sprintf("propagates native stderr and exit status from %s", contract.command), func() {
			recorder.Script("yum", contract.nativeArgv, sdktest.Response{ExitCode: 42, Stderr: "yum failed\n"})
			err := plugin.Execute(contract.command, contract.args)
			Expect(err).To(MatchError("exit status 42"))
			Expect(sdk.ExitStatus(err)).To(Equal(42))
		})
	}

	It("preserves native errors from the dnf alias", func() {
		removeFake("yum")
		recorder.Script("dnf", []string{"install", "--assumeyes", "git"}, sdktest.Response{ExitCode: 23, Stderr: "dnf failed\n"})
		err := plugin.Execute("install", []string{"git"})
		Expect(err).To(MatchError("exit status 23"))
		Expect(sdk.ExitStatus(err)).To(Equal(23))
	})

	It("returns a precise support error when neither backend exists", func() {
		removeFake("yum")
		removeFake("dnf")
		err := plugin.Execute("install", []string{"git"})
		Expect(err).To(MatchError("no yum or dnf executable found on PATH; package-manager-yum supports RHEL/CentOS 7 with yum and RHEL 8+, Rocky Linux, and AlmaLinux with yum or the dnf-compatible alias"))
		Expect(recorder.Calls("sudo")).To(BeEmpty())
	})

	It("resolves the backend again for each execution", func() {
		Expect(plugin.Execute("update", nil)).To(Succeed())
		removeFake("yum")
		Expect(plugin.Execute("list", nil)).To(Succeed())
		Expect(recorder.Calls("yum")).To(Equal([]sdktest.Call{{Args: []string{"makecache", "--assumeyes"}, ElevationRequested: true}}))
		Expect(recorder.Calls("dnf")).To(Equal([]sdktest.Call{{Args: []string{"--version"}}, {Args: []string{"list", "--installed"}}}))
	})

	It("uses the YUM 3 list syntax on legacy yum", func() {
		recorder.Script("yum", []string{"--version"}, sdktest.Response{Stdout: "3.4.3\n  Installed: rpm-4.11.3\n"})
		Expect(plugin.Execute("list", nil)).To(Succeed())
		Expect(recorder.Calls("yum")).To(Equal([]sdktest.Call{{Args: []string{"--version"}}, {Args: []string{"list", "installed"}}}))
	})

	It("uses the DNF list syntax when yum is the DNF5 alias", func() {
		recorder.Script("yum", []string{"--version"}, sdktest.Response{Stdout: "dnf5 version 5.2.6.2\n"})
		Expect(plugin.Execute("list", nil)).To(Succeed())
		Expect(recorder.Calls("yum")).To(Equal([]sdktest.Call{{Args: []string{"--version"}}, {Args: []string{"list", "--installed"}}}))
	})

	It("refuses to remove a package that matches nothing installed", func() {
		recorder.Script("rpm", []string{"-q", "--", "ghost"}, sdktest.Response{ExitCode: 1})
		recorder.Script("rpm", []string{"-q", "--whatprovides", "--", "ghost"}, sdktest.Response{ExitCode: 1})
		err := plugin.Execute("remove", []string{"git", "ghost"})
		Expect(err).To(MatchError(`cannot remove package "ghost": no installed package matches`))
		Expect(recorder.Calls("yum")).To(BeEmpty())
	})

	It("reports RPM database failures instead of treating them as missing packages", func() {
		recorder.Script("rpm", []string{"-q", "--", "git"}, sdktest.Response{ExitCode: 1, Stderr: "error: rpmdb: BDB0113 Thread died\n"})
		err := plugin.Execute("remove", []string{"git"})
		Expect(err).To(MatchError(ContainSubstring(`cannot remove package "git": RPM query failed: error: rpmdb: BDB0113 Thread died`)))
		Expect(recorder.Calls("yum")).To(BeEmpty())
	})

	It("removes packages matched by a name glob", func() {
		recorder.Script("rpm", []string{"-qa", "--", "git*"}, sdktest.Response{Stdout: "git-2.43.0-1.el9.x86_64\n"})
		Expect(plugin.Execute("remove", []string{"git*"})).To(Succeed())
		Expect(recorder.Calls("yum")).To(Equal([]sdktest.Call{{Args: []string{"remove", "--assumeyes", "git*"}, ElevationRequested: true}}))
	})

	It("refuses a name glob that matches nothing installed", func() {
		err := plugin.Execute("remove", []string{"ghost*"})
		Expect(err).To(MatchError(`cannot remove package "ghost*": no installed package matches`))
		Expect(recorder.Calls("yum")).To(BeEmpty())
	})

	It("keeps RPM stderr when an is-installed query fails", func() {
		recorder.Script("rpm", []string{"-q", "--", "git"}, sdktest.Response{ExitCode: 1, Stderr: "error: rpmdb open failed\n"})
		err := plugin.Execute("is-installed", []string{"git"})
		Expect(err).To(MatchError(`RPM query failed for "git": error: rpmdb open failed: exit status 1`))
	})

	It("removes a package matched by a provided capability", func() {
		recorder.Script("rpm", []string{"-q", "--", "editor"}, sdktest.Response{ExitCode: 1})
		Expect(plugin.Execute("remove", []string{"editor"})).To(Succeed())
		Expect(recorder.Calls("yum")).To(Equal([]sdktest.Call{{Args: []string{"remove", "--assumeyes", "editor"}, ElevationRequested: true}}))
	})

	It("explains how to get yum-config-manager when it is missing", func() {
		removeFake("yum-config-manager")
		err := plugin.Execute("add-repository", []string{"https://example.com/x.repo"})
		Expect(err).To(MatchError(ContainSubstring("install yum-utils")))
		Expect(recorder.Calls("sudo")).To(BeEmpty())
	})

	DescribeTable("rejects invalid input without invoking a backend",
		func(command string, args []string, message string) {
			Expect(plugin.Execute(command, args)).To(MatchError(ContainSubstring(message)))
			Expect(recorder.Calls("yum")).To(BeEmpty())
			Expect(recorder.Calls("dnf")).To(BeEmpty())
			Expect(recorder.Calls("sudo")).To(BeEmpty())
		},
		Entry("install without packages", "install", nil, "no packages specified"),
		Entry("remove without packages", "remove", nil, "no packages specified"),
		Entry("search without a term", "search", nil, "no search term specified"),
		Entry("unknown command", "unknown", nil, "unknown command: unknown"),
	)
})
