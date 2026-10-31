package main

import (
	"context"
	"errors"
	"os/user"
	"testing"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestSystemSetup(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "System setup suite")
}

type recordingPrivilegeAdapter struct {
	plans []sdk.ExecutionPlan
	err   error
}

func (a *recordingPrivilegeAdapter) Run(_ context.Context, plan sdk.ExecutionPlan, _ sdk.OperationIO) error {
	a.plans = append(a.plans, plan)
	return a.err
}

var _ = Describe("macOS operation authorization", func() {
	var (
		plugin      *SystemSetupPlugin
		adapter     *recordingPrivilegeAdapter
		oldAdapter  sdk.PrivilegeAdapter
		oldResolver func() sdk.PrivilegeResolver
	)

	BeforeEach(func() {
		plugin = NewSystemSetupPlugin()
		adapter = &recordingPrivilegeAdapter{}
		oldAdapter = sdk.DefaultPrivilegeAdapter
		oldResolver = sdk.NewPrivilegeResolver
		sdk.DefaultPrivilegeAdapter = adapter
		sdk.NewPrivilegeResolver = func() sdk.PrivilegeResolver {
			return sdk.PrivilegeResolver{
				CurrentUser: func() (*user.User, error) {
					return &user.User{Username: "alice", Uid: "501", Gid: "20", HomeDir: "/Users/alice"}, nil
				},
				LookupUser: func(string) (*user.User, error) {
					return nil, errors.New("unexpected account lookup")
				},
				Getenv: func(string) string { return "" },
			}
		}
	})

	AfterEach(func() {
		sdk.DefaultPrivilegeAdapter = oldAdapter
		sdk.NewPrivilegeResolver = oldResolver
	})

	It("keeps preference writes and Finder restart in the user session", func() {
		Expect(plugin.applyFinderHiddenFiles(context.Background())).To(Succeed())
		Expect(adapter.plans).To(HaveLen(2))
		for _, plan := range adapter.plans {
			Expect(plan.Privilege.Scope).To(Equal(sdk.ScopeUser))
			Expect(plan.Elevate).To(BeFalse())
			Expect(plan.SwitchUser).To(BeFalse())
		}
		Expect(adapter.plans[0].Name).To(Equal("defaults"))
		Expect(adapter.plans[0].Args).To(Equal([]string{"write", "com.apple.finder", "AppleShowAllFiles", "YES"}))
		Expect(adapter.plans[1].Name).To(Equal("killall"))
		Expect(adapter.plans[1].Args).To(Equal([]string{"Finder"}))
	})

	It("applies only the selected user operation without authorization or security mutations", func() {
		Expect(plugin.configureMacOS(context.Background(), []string{"--apply", "finder-hidden-files"})).To(Succeed())
		Expect(adapter.plans).To(HaveLen(2))
		for _, plan := range adapter.plans {
			Expect(plan.Privilege.Scope).To(Equal(sdk.ScopeUser))
			Expect(plan.Elevate).To(BeFalse())
			Expect(plan.SwitchUser).To(BeFalse())
			Expect(plan.Name).NotTo(BeElementOf("DevToolsSecurity", "spctl"))
		}
		Expect(adapter.plans[0].Name).To(Equal("defaults"))
		Expect(adapter.plans[1].Name).To(Equal("killall"))
	})

	It("requires an explicit operation instead of applying every missing setting", func() {
		Expect(plugin.configureMacOS(context.Background(), []string{"--apply"})).To(MatchError(ContainSubstring("select at least one macOS operation")))
		Expect(adapter.plans).To(BeEmpty())
	})

	It("authorizes only the selected system operation", func() {
		Expect(plugin.configureMacOS(context.Background(), []string{"--apply", "developer-mode"})).To(Succeed())
		Expect(adapter.plans).To(HaveLen(1))
		Expect(adapter.plans[0].Name).To(Equal("DevToolsSecurity"))
		Expect(adapter.plans[0].Elevate).To(BeTrue())
	})

	It("requests system authorization only for developer mode", func() {
		Expect(plugin.applyDeveloperMode(context.Background())).To(Succeed())
		Expect(adapter.plans).To(HaveLen(1))
		Expect(adapter.plans[0].Privilege.Scope).To(Equal(sdk.ScopeSystem))
		Expect(adapter.plans[0].Elevate).To(BeTrue())
		Expect(adapter.plans[0].Name).To(Equal("DevToolsSecurity"))
		Expect(adapter.plans[0].Args).To(Equal([]string{"-enable"}))
	})

	It("requests system authorization for the explicitly selected Gatekeeper operation", func() {
		Expect(plugin.applyGatekeeper(context.Background())).To(Succeed())
		Expect(adapter.plans).To(HaveLen(1))
		Expect(adapter.plans[0].Privilege.Scope).To(Equal(sdk.ScopeSystem))
		Expect(adapter.plans[0].Elevate).To(BeTrue())
		Expect(adapter.plans[0].Name).To(Equal("spctl"))
		Expect(adapter.plans[0].Args).To(Equal([]string{"--master-disable"}))
	})

	It("reports native authorization denial without retrying the operation", func() {
		cause := errors.New("osascript: User canceled (-128)")
		adapter.err = sdk.NewAuthorizationDeniedError("authorization canceled", -128, cause)

		err := plugin.applyDeveloperMode(context.Background())
		var authorization *sdk.AuthorizationError
		Expect(errors.As(err, &authorization)).To(BeTrue())
		Expect(authorization.Kind).To(Equal(sdk.AuthorizationDenied))
		Expect(errors.Is(err, cause)).To(BeTrue())
		Expect(authorization.NativeExitCode).To(Equal(-128))
		Expect(sdk.ExitStatus(err)).To(Equal(1))
		Expect(err).To(MatchError(ContainSubstring("approve the macOS administrator prompt")))
		Expect(adapter.plans).To(HaveLen(1))
	})
})
