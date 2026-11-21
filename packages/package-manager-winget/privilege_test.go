//go:build !windows

package main_test

import (
	"context"
	"errors"
	"fmt"
	"os/exec"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	main "github.com/jameswlane/devex/packages/package-manager-winget"
	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
	"github.com/jameswlane/devex/packages/plugin-sdk/sdktest"
)

// planRecorder is a privilege adapter that records each plan instead of
// running it, so specs can check the scope the SDK receives on any host.
type planRecorder struct{ plans []sdk.ExecutionPlan }

func (r *planRecorder) Run(_ context.Context, plan sdk.ExecutionPlan, _ sdk.OperationIO) error {
	r.plans = append(r.plans, plan)
	return nil
}

var _ = Describe("winget execution boundary", func() {
	var (
		plugin   *main.WingetPlugin
		recorder *sdktest.Recorder
	)

	expectAuthorization := func(err error, kind sdk.AuthorizationKind) {
		GinkgoHelper()
		var auth *sdk.AuthorizationError
		Expect(errors.As(err, &auth)).To(BeTrue(), "expected an authorization error, got %v", err)
		Expect(auth.Kind).To(Equal(kind))
	}

	It("returns an actionable error when winget is missing", func() {
		recorder = sdktest.NewRecorder(GinkgoT())
		plugin = main.NewWingetPlugin()
		for _, contract := range contracts {
			err := plugin.Execute(contract.command, contract.args)
			Expect(err).To(MatchError(ContainSubstring("winget is not installed or not on PATH; install or update App Installer")), contract.name)
		}
		Expect(recorder.Calls("sudo")).To(BeEmpty())
	})

	It("still validates input before reporting a missing winget", func() {
		recorder = sdktest.NewRecorder(GinkgoT())
		plugin = main.NewWingetPlugin()
		Expect(plugin.Execute("install", []string{"git"})).To(MatchError(ContainSubstring("invalid winget package identifier")))
	})

	Context("with a plan-recording adapter", func() {
		var plans *planRecorder

		BeforeEach(func() {
			recorder = sdktest.NewRecorder(GinkgoT(), "winget")
			plugin = main.NewWingetPlugin()
			plans = &planRecorder{}
			sdk.DefaultPrivilegeAdapter = plans
		})

		for _, contract := range contracts {
			want := sdk.ScopeUser
			if contract.elevated {
				want = sdk.ScopeSystem
			}
			It(fmt.Sprintf("hands %s to the platform adapter at %s scope", contract.name, want), func() {
				Expect(plugin.Execute(contract.command, contract.args)).To(Succeed())
				Expect(plans.plans).To(HaveLen(len(contract.nativeArgv)))
				for i, plan := range plans.plans {
					Expect(plan.Privilege.Scope).To(Equal(want))
					Expect(plan.Name).To(Equal("winget"))
					Expect(plan.Args).To(Equal(contract.nativeArgv[i]))
				}
			})
		}
	})

	Context("with the default sudo adapter", func() {
		BeforeEach(func() {
			recorder = sdktest.NewRecorder(GinkgoT(), "winget")
			plugin = main.NewWingetPlugin()
		})

		for _, contract := range contracts {
			if !contract.elevated {
				continue
			}

			It(fmt.Sprintf("reports a denied elevation for %s as an authorization error without running winget", contract.name), func() {
				sudoArgv := append([]string{"-n", "--", "winget"}, contract.nativeArgv[0]...)
				recorder.Script("sudo", sudoArgv, sdktest.Response{ExitCode: 1, Stderr: "sudo: a password is required\n"})
				err := plugin.Execute(contract.command, contract.args)
				expectAuthorization(err, sdk.AuthorizationDenied)
				Expect(err.Error()).NotTo(ContainSubstring("UAC"))
				Expect(recorder.Calls("winget")).To(BeEmpty())
			})

			It(fmt.Sprintf("requires authorization for %s when no elevation is available", contract.name), func() {
				sdk.DefaultPrivilegeAdapter = sdk.UnixSudoAdapter{LookPath: func(string) (string, error) { return "", exec.ErrNotFound }}
				err := plugin.Execute(contract.command, contract.args)
				expectAuthorization(err, sdk.AuthorizationRequired)
				Expect(recorder.Calls("winget")).To(BeEmpty())
			})
		}
	})
})
