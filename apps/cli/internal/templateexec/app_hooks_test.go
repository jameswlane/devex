package templateexec_test

import (
	"context"
	"errors"
	"os/user"

	"github.com/jameswlane/devex/apps/cli/internal/templateexec"
	"github.com/jameswlane/devex/apps/cli/internal/templates"
	"github.com/jameswlane/devex/apps/cli/internal/types"
	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type appHookRecorder struct {
	events []string
	plans  []sdk.ExecutionPlan
	ctx    context.Context
	err    error
}

func (r *appHookRecorder) Run(ctx context.Context, plan sdk.ExecutionPlan, _ sdk.OperationIO) error {
	r.ctx = ctx
	r.plans = append(r.plans, plan)
	r.events = append(r.events, plan.Args[len(plan.Args)-1])
	return r.err
}

func (r *appHookRecorder) ExecutePlugin(_ context.Context, _ string, _ []string) error {
	r.events = append(r.events, "install")
	return nil
}

var _ = Describe("Catalog application hooks in templates", func() {
	var recorder *appHookRecorder
	var step templates.PlannedStep
	BeforeEach(func() {
		resolver, adapter := sdk.NewPrivilegeResolver, sdk.DefaultPrivilegeAdapter
		DeferCleanup(func() { sdk.NewPrivilegeResolver, sdk.DefaultPrivilegeAdapter = resolver, adapter })
		sdk.NewPrivilegeResolver = func() sdk.PrivilegeResolver {
			return sdk.PrivilegeResolver{
				CurrentUser: func() (*user.User, error) {
					return &user.User{Username: "alice", Uid: "1000", Gid: "1000", HomeDir: "/home/alice"}, nil
				},
				LookupUser: func(string) (*user.User, error) {
					return &user.User{Username: "alice", Uid: "1000", Gid: "1000", HomeDir: "/home/alice"}, nil
				},
				Getenv: func(string) string { return "" },
			}
		}
		recorder = &appHookRecorder{}
		sdk.DefaultPrivilegeAdapter = recorder
		step = templates.PlannedStep{
			ID: "docker", Type: templates.StepTypeApp,
			Request: templates.ExecutionRequest{Plugin: "package-manager-apt", Command: "install", Args: []string{"docker-ce"}},
			App: &types.AppConfig{
				BaseConfig:  types.BaseConfig{Name: "Docker"},
				PreInstall:  []types.InstallCommand{{Scope: "system", Shell: "repository"}},
				PostInstall: []types.InstallCommand{{Scope: "system", Shell: "service-and-group"}},
			},
		}
	})

	It("executes repository, package, and service hooks in order using the caller context", func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		_, err := (templateexec.PluginStepExecutor{PluginExecutor: recorder}).ExecuteStep(ctx, step)
		Expect(err).NotTo(HaveOccurred())
		Expect(recorder.events).To(Equal([]string{"repository", "install", "service-and-group"}))
		Expect(recorder.ctx).To(BeIdenticalTo(ctx))
		Expect(recorder.plans).To(HaveLen(2))
		for _, plan := range recorder.plans {
			Expect(plan.Privilege.Scope).To(Equal(sdk.ScopeSystem))
			Expect(plan.Elevate).To(BeTrue())
			Expect(plan.IntendedUser.Username).To(Equal("alice"))
		}
	})

	It("stops before package installation when repository setup fails", func() {
		recorder.err = errors.New("repository unavailable")
		_, err := (templateexec.PluginStepExecutor{PluginExecutor: recorder}).ExecuteStep(context.Background(), step)
		Expect(err).To(MatchError(ContainSubstring("Docker pre-install hook: repository unavailable")))
		Expect(recorder.events).To(Equal([]string{"repository"}))
	})

	It("does not run install hooks during removal", func() {
		step.Request.Command = "remove"
		_, err := (templateexec.PluginStepExecutor{PluginExecutor: recorder}).ExecuteStep(context.Background(), step)
		Expect(err).NotTo(HaveOccurred())
		Expect(recorder.plans).To(BeEmpty())
	})
})
