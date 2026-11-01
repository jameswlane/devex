package main_test

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	main "github.com/jameswlane/devex/packages/package-manager-pacman"
	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
	"github.com/jameswlane/devex/packages/plugin-sdk/sdktest"
)

type operationContract struct {
	command    string
	args       []string
	nativeArgv []string
	elevated   bool
}

var pacmanContracts = []operationContract{
	{
		command:    "install",
		args:       []string{"git", "vim"},
		nativeArgv: []string{"-S", "--needed", "--noconfirm", "git", "vim"},
		elevated:   true,
	},
	{
		command:    "remove",
		args:       []string{"git", "vim"},
		nativeArgv: []string{"-R", "--noconfirm", "git", "vim"},
		elevated:   true,
	},
	{
		command:    "update",
		nativeArgv: []string{"-Syu", "--noconfirm"},
		elevated:   true,
	},
	{
		command:    "search",
		args:       []string{"web", "server"},
		nativeArgv: []string{"-Ss", "web server"},
	},
	{
		command:    "list",
		nativeArgv: []string{"-Q"},
	},
	{
		command:    "is-installed",
		args:       []string{"git", "vim"},
		nativeArgv: []string{"-Q", "git", "vim"},
	},
}

var _ = Describe("Pacman command contracts", func() {
	var (
		plugin   *main.PacmanPlugin
		recorder *sdktest.Recorder
	)

	BeforeEach(func() {
		recorder = sdktest.NewRecorder(GinkgoT(), "pacman")
		plugin = main.NewPacmanPlugin()
	})

	It("has a contract for every advertised command", func() {
		advertised := make([]string, 0, len(plugin.Info().Commands))
		for _, command := range plugin.Info().Commands {
			advertised = append(advertised, command.Name)
		}
		covered := make([]string, 0, len(pacmanContracts))
		for _, contract := range pacmanContracts {
			covered = append(covered, contract.command)
		}
		Expect(covered).To(ConsistOf(advertised))
	})

	for _, contract := range pacmanContracts {
		It(fmt.Sprintf("honors the exact native argv and elevation for %s", contract.command), func() {
			Expect(plugin.Execute(contract.command, contract.args)).To(Succeed())
			Expect(recorder.Calls("pacman")).To(Equal([]sdktest.Call{{
				Args:               contract.nativeArgv,
				ElevationRequested: contract.elevated,
			}}))
			if !contract.elevated {
				Expect(recorder.Calls("sudo")).To(BeEmpty())
			}
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

		It(fmt.Sprintf("preserves native exit status and stderr from %s", contract.command), func() {
			recorder.Script("pacman", contract.nativeArgv, sdktest.Response{
				ExitCode: 42,
				Stderr:   "pacman database failure\n",
			})
			err := plugin.Execute(contract.command, contract.args)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("pacman database failure"))
			Expect(sdk.ExitStatus(err)).To(Equal(42))
			var native *exec.ExitError
			Expect(errors.As(err, &native)).To(BeTrue())
			Expect(native.ExitCode()).To(Equal(42))
			Expect(recorder.Calls("pacman")).To(HaveLen(1))
		})

		It(fmt.Sprintf("does not launch %s or authorize it after cancellation", contract.command), func() {
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			Expect(plugin.ExecuteContext(ctx, contract.command, contract.args)).To(MatchError(context.Canceled))
			Expect(recorder.Calls("pacman")).To(BeEmpty())
			Expect(recorder.Calls("sudo")).To(BeEmpty())
		})
	}

	It("reports a missing package from a mixed installed/missing query", func() {
		recorder.Script("pacman", []string{"-Q", "git", "missing"}, sdktest.Response{
			Stdout:   "git 2.50.0-1\n",
			Stderr:   "error: package 'missing' was not found\n",
			ExitCode: 1,
		})
		err := plugin.Execute("is-installed", []string{"git", "missing"})
		Expect(err).To(HaveOccurred())
		Expect(sdk.ExitStatus(err)).To(Equal(1))
		Expect(err.Error()).To(ContainSubstring("package 'missing' was not found"))
		Expect(recorder.Calls("pacman")).To(Equal([]sdktest.Call{{Args: []string{"-Q", "git", "missing"}}}))
		Expect(recorder.Calls("sudo")).To(BeEmpty())
	})

	It("retains diagnostics when a query failure also uses exit status one", func() {
		recorder.Script("pacman", []string{"-Q", "git"}, sdktest.Response{
			Stderr:   "error: could not open local database\n",
			ExitCode: 1,
		})
		err := plugin.Execute("is-installed", []string{"git"})
		Expect(err).To(HaveOccurred())
		Expect(sdk.ExitStatus(err)).To(Equal(1))
		Expect(err.Error()).To(ContainSubstring("could not open local database"))
	})

	It("keeps the stderr tail without retaining unbounded native output", func() {
		recorder.Script("pacman", []string{"-Q"}, sdktest.Response{
			Stderr:   "discarded prefix" + strings.Repeat("x", 8192) + "\nlast diagnostic\n",
			ExitCode: 7,
		})
		err := plugin.Execute("list", nil)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(HaveSuffix("last diagnostic"))
		Expect(err.Error()).NotTo(ContainSubstring("discarded prefix"))
		Expect(len(err.Error())).To(BeNumerically("<", 4200))
		Expect(sdk.ExitStatus(err)).To(Equal(7))
	})

	It("stops subsequent operations once the context is cancelled", func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		Expect(plugin.ExecuteContext(ctx, "list", nil)).To(Succeed())
		cancel()
		Expect(plugin.ExecuteContext(ctx, "install", []string{"git"})).To(MatchError(context.Canceled))
		Expect(recorder.Calls("pacman")).To(Equal([]sdktest.Call{{Args: []string{"-Q"}}}))
		Expect(recorder.Calls("sudo")).To(BeEmpty())
	})

	It("preserves cancellation at the subprocess boundary without launching a command", func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		newCommand := sdk.NewCommandContext
		sdk.NewCommandContext = func(ctx context.Context, name string, args ...string) *exec.Cmd {
			cancel()
			return newCommand(ctx, name, args...)
		}
		Expect(plugin.ExecuteContext(ctx, "list", nil)).To(MatchError(context.Canceled))
		Expect(recorder.Calls("pacman")).To(BeEmpty())
		Expect(recorder.Calls("sudo")).To(BeEmpty())
	})

	It("describes update as a full system upgrade", func() {
		for _, command := range plugin.Info().Commands {
			if command.Name == "update" {
				Expect(command.Description).To(ContainSubstring("upgrade the entire system"))
				Expect(command.Usage).To(ContainSubstring("-Syu --noconfirm"))
			}
		}
	})

	It("rejects an unknown command", func() {
		Expect(plugin.Execute("unknown", nil)).To(MatchError("unknown command: unknown"))
		Expect(recorder.Calls("pacman")).To(BeEmpty())
	})

	for _, command := range []string{"install", "remove", "search", "is-installed"} {
		It(fmt.Sprintf("rejects missing arguments for %s without invoking pacman", command), func() {
			Expect(plugin.Execute(command, nil)).To(HaveOccurred())
			Expect(recorder.Calls("pacman")).To(BeEmpty())
			Expect(recorder.Calls("sudo")).To(BeEmpty())
		})
		It(fmt.Sprintf("rejects option injection for %s without invoking pacman", command), func() {
			Expect(plugin.Execute(command, []string{"-Sy"})).To(HaveOccurred())
			Expect(recorder.Calls("pacman")).To(BeEmpty())
			Expect(recorder.Calls("sudo")).To(BeEmpty())
		})
	}

	for _, command := range []string{"list", "update"} {
		It(fmt.Sprintf("rejects arguments for %s without invoking pacman", command), func() {
			Expect(plugin.Execute(command, []string{"-Sy"})).To(HaveOccurred())
			Expect(recorder.Calls("pacman")).To(BeEmpty())
			Expect(recorder.Calls("sudo")).To(BeEmpty())
		})
	}
})
