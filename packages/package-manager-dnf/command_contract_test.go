package main_test

import (
	"errors"
	"fmt"
	"os/exec"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	main "github.com/jameswlane/devex/packages/package-manager-dnf"
	"github.com/jameswlane/devex/packages/plugin-sdk/sdktest"
)

type operationContract struct {
	command    string
	args       []string
	currentArg []string
	nativeArgv []string
	elevated   bool
	pending    string
	// queries are package specifications checked before the native call.
	queries []string
}

func installedQuery(spec string) []string {
	return []string{"repoquery", "--installed", "--quiet", "--cacheonly", spec}
}

func installedQueryCall(spec string) sdktest.Call {
	return sdktest.Call{Args: installedQuery(spec)}
}

// scriptInstalled makes each spec resolve to one installed package.
func scriptInstalled(recorder *sdktest.Recorder, specs ...string) {
	for _, spec := range specs {
		recorder.Script("dnf", installedQuery(spec), sdktest.Response{Stdout: spec + "-0:1.0-1.fc42.x86_64\n"})
	}
}

func expectedCalls(contract operationContract, native []string) []sdktest.Call {
	calls := make([]sdktest.Call, 0, len(contract.queries)+1)
	for _, spec := range contract.queries {
		calls = append(calls, installedQueryCall(spec))
	}
	return append(calls, sdktest.Call{Args: native, ElevationRequested: contract.elevated})
}

var dnfContracts = []operationContract{
	{command: "upgrade", currentArg: []string{"upgrade", "--assumeyes"}, nativeArgv: []string{"upgrade", "--assumeyes"}, elevated: true},
	{
		command:    "install",
		args:       []string{"git", "vim"},
		currentArg: []string{"install", "--assumeyes", "git", "vim"},
		nativeArgv: []string{"install", "--assumeyes", "git", "vim"},
		elevated:   true,
	},
	{
		command:    "remove",
		args:       []string{"git", "vim"},
		currentArg: []string{"remove", "--assumeyes", "git", "vim"},
		nativeArgv: []string{"remove", "--assumeyes", "git", "vim"},
		elevated:   true,
		queries:    []string{"git", "vim"},
	},
	{
		command:    "update",
		currentArg: []string{"makecache"},
		nativeArgv: []string{"makecache"},
		elevated:   true,
	},
	{
		command: "search",
		// Search currently treats all CLI words as one native phrase/regex.
		args:       []string{"web", "server"},
		currentArg: []string{"search", "web server"},
		nativeArgv: []string{"search", "web server"},
	},
	{
		command:    "list",
		currentArg: []string{"list", "installed"},
		nativeArgv: []string{"list", "installed"},
	},
}

var _ = Describe("DNF command contracts", func() {
	var (
		plugin   *main.DnfPlugin
		recorder *sdktest.Recorder
	)

	BeforeEach(func() {
		recorder = sdktest.NewRecorder(GinkgoT(), "dnf", "rpm")
		plugin = main.NewDnfPlugin()
	})

	It("has a contract for every advertised command", func() {
		advertised := make([]string, 0, len(plugin.Info().Commands))
		for _, command := range plugin.Info().Commands {
			advertised = append(advertised, command.Name)
		}
		covered := make([]string, 0, len(dnfContracts))
		for _, contract := range dnfContracts {
			covered = append(covered, contract.command)
		}
		Expect(covered).To(ConsistOf(advertised))
	})

	for _, contract := range dnfContracts {
		contract := contract
		It(fmt.Sprintf("routes %s with its current argv and elevation behavior", contract.command), func() {
			scriptInstalled(recorder, contract.queries...)
			Expect(plugin.Execute(contract.command, contract.args)).To(Succeed())
			Expect(recorder.Calls("dnf")).To(Equal(expectedCalls(contract, contract.currentArg)))
		})

		It(fmt.Sprintf("propagates native errors from %s", contract.command), func() {
			scriptInstalled(recorder, contract.queries...)
			recorder.Script("dnf", contract.currentArg, sdktest.Response{
				ExitCode: 42,
				Stderr:   "dnf failed\n",
			})
			Expect(plugin.Execute(contract.command, contract.args)).To(MatchError("exit status 42"))
		})

		assertNativeArgv := func() {
			scriptInstalled(recorder, contract.queries...)
			Expect(plugin.Execute(contract.command, contract.args)).To(Succeed())
			Expect(recorder.Calls("dnf")).To(Equal(expectedCalls(contract, contract.nativeArgv)))
		}
		if contract.pending == "" {
			It(fmt.Sprintf("uses the required native argv for %s", contract.command), assertNativeArgv)
		} else {
			PIt(fmt.Sprintf("uses the required native argv for %s: %s", contract.command, contract.pending), assertNativeArgv)
		}
	}

	It("rejects an unknown command", func() {
		Expect(plugin.Execute("unknown", nil)).To(MatchError("unknown command: unknown"))
		Expect(recorder.Calls("dnf")).To(BeEmpty())
	})

	Describe("remove", func() {
		removal := sdktest.Call{Args: []string{"remove", "--assumeyes", "git", "vim"}, ElevationRequested: true}

		for _, unattendedEnv := range []string{"DEVEX_UNATTENDED", "CI"} {
			It(fmt.Sprintf("uses the exact unattended removal argv with %s", unattendedEnv), func() {
				GinkgoT().Setenv(unattendedEnv, "true")
				sdktest.CloseStdin(GinkgoT())
				scriptInstalled(recorder, "git", "vim")
				Expect(plugin.Execute("remove", []string{"git", "vim"})).To(Succeed())
				Expect(recorder.Calls("dnf")).To(Equal([]sdktest.Call{
					installedQueryCall("git"), installedQueryCall("vim"), removal,
				}))
			})
		}

		It("queries every specification without elevation before the elevated transaction", func() {
			scriptInstalled(recorder, "git", "vim")
			Expect(plugin.Execute("remove", []string{"git", "vim"})).To(Succeed())
			Expect(recorder.Calls("dnf")).To(Equal([]sdktest.Call{
				installedQueryCall("git"), installedQueryCall("vim"), removal,
			}))
			Expect(recorder.Calls("rpm")).To(BeEmpty())
		})

		It("accepts a glob that matches installed packages", func() {
			recorder.Script("dnf", installedQuery("kernel*"), sdktest.Response{
				Stdout: "kernel-0:6.11.4-301.fc42.x86_64\nkernel-core-0:6.11.4-301.fc42.x86_64\n",
			})
			Expect(plugin.Execute("remove", []string{"kernel*"})).To(Succeed())
			Expect(recorder.Calls("dnf")).To(Equal([]sdktest.Call{
				installedQueryCall("kernel*"),
				{Args: []string{"remove", "--assumeyes", "kernel*"}, ElevationRequested: true},
			}))
		})

		DescribeTable("rejects a specification that matches nothing, stopping at the first miss",
			func(packages []string, queried []string) {
				scriptInstalled(recorder, "git")
				// The unscripted "missing" query returns no output, as DNF does
				// when no installed package matches.
				err := plugin.Execute("remove", packages)
				Expect(err).To(MatchError(`cannot remove package "missing": no installed package matches`))
				calls := make([]sdktest.Call, 0, len(queried))
				for _, spec := range queried {
					calls = append(calls, installedQueryCall(spec))
				}
				Expect(recorder.Calls("dnf")).To(Equal(calls))
				Expect(recorder.Calls("sudo")).To(BeEmpty())
			},
			Entry("only a missing package", []string{"missing"}, []string{"missing"}),
			Entry("missing after an installed package", []string{"git", "missing"}, []string{"git", "missing"}),
			Entry("missing before an installed package", []string{"missing", "git"}, []string{"missing"}),
			Entry("missing between installed packages", []string{"git", "missing", "git"}, []string{"git", "missing"}),
		)

		It("rejects option-like specifications without querying DNF", func() {
			err := plugin.Execute("remove", []string{"--all"})
			Expect(err).To(MatchError(ContainSubstring("must not start with '-'")))
			Expect(recorder.Calls("dnf")).To(BeEmpty())
		})

		It("reports query failures with native stderr and keeps the exit status", func() {
			recorder.Script("dnf", installedQuery("git"), sdktest.Response{
				ExitCode: 2, Stderr: "Error: rpmdb open failed\n",
			})
			err := plugin.Execute("remove", []string{"git"})
			Expect(err).To(MatchError(ContainSubstring(`cannot remove package "git": installed-package query failed: Error: rpmdb open failed: exit status 2`)))
			var exitErr *exec.ExitError
			Expect(errors.As(err, &exitErr)).To(BeTrue())
			Expect(exitErr.ExitCode()).To(Equal(2))
			Expect(recorder.Calls("dnf")).To(Equal([]sdktest.Call{installedQueryCall("git")}))
		})

		It("returns a native DNF failure unchanged", func() {
			scriptInstalled(recorder, "git")
			recorder.Script("dnf", []string{"remove", "--assumeyes", "git"}, sdktest.Response{
				ExitCode: 42, Stderr: "transaction failed\n",
			})
			err := plugin.Execute("remove", []string{"git"})
			Expect(err).To(BeAssignableToTypeOf(&exec.ExitError{}))
			Expect(err.(*exec.ExitError).ExitCode()).To(Equal(42))
			Expect(err).To(MatchError("exit status 42"))
		})
	})

	for _, command := range []string{"install", "remove", "search"} {
		command := command
		It(fmt.Sprintf("rejects missing arguments for %s without invoking DNF", command), func() {
			Expect(plugin.Execute(command, nil)).To(HaveOccurred())
			Expect(recorder.Calls("dnf")).To(BeEmpty())
			Expect(recorder.Calls("rpm")).To(BeEmpty())
		})
	}
})
