package sdk_test

import (
	"context"
	"errors"
	"os/exec"
	"os/user"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Operation privilege contract", func() {
	It("constructs portable required and denied results for external adapters", func() {
		cause := errors.New("native authorization failure")
		required := sdk.NewAuthorizationRequiredError("authorize first", cause)
		Expect(required.Kind).To(Equal(sdk.AuthorizationRequired))
		Expect(required.NativeExitCode).To(Equal(-1))
		Expect(errors.Is(required, cause)).To(BeTrue())
		denied := sdk.NewAuthorizationDeniedError("rejected", 1223, cause)
		Expect(denied.Kind).To(Equal(sdk.AuthorizationDenied))
		Expect(sdk.ExitStatus(denied)).To(Equal(1223))
		Expect(errors.Is(denied, cause)).To(BeTrue())
	})
	var resolver sdk.PrivilegeResolver
	var account *user.User
	BeforeEach(func() {
		account = &user.User{Username: "alice", Uid: "1000", HomeDir: "/Users/alice"}
		resolver = sdk.PrivilegeResolver{
			Elevated:    false,
			CurrentUser: func() (*user.User, error) { return account, nil },
			LookupUser: func(name string) (*user.User, error) {
				if name == "alice" {
					return &user.User{Username: "alice", Uid: "1000", HomeDir: "/Users/alice"}, nil
				}
				return nil, errors.New("unknown account")
			},
			Getenv: func(string) string { return "" },
		}
	})
	planUser := func() (sdk.ExecutionPlan, error) {
		return resolver.Plan(sdk.ExecutionRequest{Privilege: sdk.OperationPrivilege{Scope: sdk.ScopeUser}, Name: "builder", Args: []string{"argument with spaces", "$(touch /bad)"}})
	}
	It("honors the current non-elevated account's HOME without trusting environment identity", func() {
		resolver.Getenv = func(key string) string {
			switch key {
			case "HOME", "USERPROFILE":
				return "/tmp/isolated-home"
			case "USER", "SUDO_USER":
				return "root"
			default:
				return ""
			}
		}
		identity, err := resolver.ResolveUser(nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(identity.HomeDir).To(Equal("/tmp/isolated-home"))
		Expect(identity.Username).To(Equal("alice"))
		Expect(identity.UID).To(Equal("1000"))
		Expect(account.HomeDir).To(Equal("/Users/alice"), "resolution must not mutate the cached account")
	})
	It("keeps the account home for elevated and explicitly selected users", func() {
		resolver.Getenv = func(key string) string {
			if key == "HOME" || key == "USERPROFILE" {
				return "/tmp/untrusted-home"
			}
			return ""
		}
		identity, err := resolver.ResolveUser(&sdk.UserIdentity{Username: "alice", UID: "1000", HomeDir: "/tmp/supplied-home"})
		Expect(err).NotTo(HaveOccurred())
		Expect(identity.HomeDir).To(Equal("/Users/alice"))
		resolver.Elevated = true
		identity, err = resolver.ResolveUser(nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(identity.HomeDir).To(Equal("/Users/alice"))
	})
	It("runs a non-root user operation without sudo and keeps argv intact", func() {
		plan, err := planUser()
		Expect(err).NotTo(HaveOccurred())
		Expect(plan.IntendedUser.HomeDir).To(Equal("/Users/alice"))
		Expect(plan.Elevate).To(BeFalse())
		Expect(plan.SwitchUser).To(BeFalse())
		adapter := sdk.UnixSudoAdapter{LookPath: func(string) (string, error) { Fail("must not look up sudo"); return "", nil }}
		cmd, err := adapter.Prepare(context.Background(), plan)
		Expect(err).NotTo(HaveOccurred())
		Expect(cmd.Args).To(Equal([]string{"builder", "argument with spaces", "$(touch /bad)"}))
	})
	It("restores the original user and account home when the CLI is root", func() {
		resolver.Elevated = true
		account = &user.User{Username: "root", Uid: "0", HomeDir: "/root"}
		resolver.Getenv = func(key string) string {
			if key == "SUDO_USER" {
				return "alice"
			}
			return ""
		}
		plan, err := planUser()
		Expect(err).NotTo(HaveOccurred())
		Expect(plan.SwitchUser).To(BeTrue())
		adapter := sdk.UnixSudoAdapter{LookPath: func(string) (string, error) { return "/usr/bin/sudo", nil }}
		cmd, err := adapter.Prepare(context.Background(), plan)
		Expect(err).NotTo(HaveOccurred())
		Expect(cmd.Args).To(Equal([]string{"/usr/bin/sudo", "-n", "-H", "-u", "alice", "--", "env", "HOME=/Users/alice", "USER=alice", "LOGNAME=alice", "builder", "argument with spaces", "$(touch /bad)"}))
	})
	It("refuses root user operations without an original non-root identity", func() {
		resolver.Elevated = true
		account = &user.User{Username: "root", Uid: "0"}
		_, err := planUser()
		var auth *sdk.AuthorizationError
		Expect(errors.As(err, &auth)).To(BeTrue())
		Expect(auth.Kind).To(Equal(sdk.AuthorizationRequired))
	})
	It("requires an explicit scope and validates intended account identity", func() {
		_, err := resolver.Plan(sdk.ExecutionRequest{Name: "build"})
		Expect(err).To(MatchError(ContainSubstring("scope")))
		_, err = resolver.Plan(sdk.ExecutionRequest{Name: "build", Privilege: sdk.OperationPrivilege{Scope: sdk.ScopeUser, User: &sdk.UserIdentity{Username: "alice", UID: "0"}}})
		Expect(err).To(MatchError(ContainSubstring("matching UID")))
	})
	It("returns authorization-required when sudo is missing instead of running unelevated", func() {
		plan, err := resolver.Plan(sdk.ExecutionRequest{Privilege: sdk.OperationPrivilege{Scope: sdk.ScopeSystem}, Name: "install"})
		Expect(err).NotTo(HaveOccurred())
		Expect(plan.Elevate).To(BeTrue())
		cmd, err := (sdk.UnixSudoAdapter{LookPath: func(string) (string, error) { return "", exec.ErrNotFound }}).Prepare(context.Background(), plan)
		Expect(cmd).To(BeNil())
		var auth *sdk.AuthorizationError
		Expect(errors.As(err, &auth)).To(BeTrue())
		Expect(auth.Kind).To(Equal(sdk.AuthorizationRequired))
		Expect(auth.NativeExitCode).To(Equal(-1))
		Expect(errors.Is(err, exec.ErrNotFound)).To(BeTrue())
	})
	It("returns a typed denial and preserves the platform exit error", func() {
		plan, err := resolver.Plan(sdk.ExecutionRequest{Privilege: sdk.OperationPrivilege{Scope: sdk.ScopeSystem}, Name: "install"})
		Expect(err).NotTo(HaveOccurred())
		native := exec.CommandContext(context.Background(), "sh", "-c", "exit 23").Run()
		_, err = (sdk.UnixSudoAdapter{
			LookPath:  func(string) (string, error) { return "/usr/bin/sudo", nil },
			Authorize: func(context.Context, string) error { return native },
		}).Prepare(context.Background(), plan)
		var auth *sdk.AuthorizationError
		Expect(errors.As(err, &auth)).To(BeTrue())
		Expect(auth.Kind).To(Equal(sdk.AuthorizationDenied))
		Expect(auth.NativeExitCode).To(Equal(23))
		Expect(sdk.ExitStatus(err)).To(Equal(23))
		var exit *exec.ExitError
		Expect(errors.As(err, &exit)).To(BeTrue())
	})
	It("puts requested environment assignments before -- so sudo doesn't run them as the command", func() {
		plan, err := resolver.Plan(sdk.ExecutionRequest{Privilege: sdk.OperationPrivilege{Scope: sdk.ScopeSystem}, Name: "apt-get", Args: []string{"update"}, Env: []string{"DEBIAN_FRONTEND=noninteractive"}})
		Expect(err).NotTo(HaveOccurred())
		cmd, err := (sdk.UnixSudoAdapter{
			LookPath:  func(string) (string, error) { return "/usr/bin/sudo", nil },
			Authorize: func(context.Context, string) error { return nil },
		}).Prepare(context.Background(), plan)
		Expect(err).NotTo(HaveOccurred())
		Expect(cmd.Args).To(Equal([]string{"/usr/bin/sudo", "-n", "DEBIAN_FRONTEND=noninteractive", "--", "apt-get", "update"}))
	})
	It("keeps command failure distinct from successful authorization", func() {
		plan, err := resolver.Plan(sdk.ExecutionRequest{Privilege: sdk.OperationPrivilege{Scope: sdk.ScopeSystem}, Name: "install"})
		Expect(err).NotTo(HaveOccurred())
		cmd, err := (sdk.UnixSudoAdapter{
			LookPath:  func(string) (string, error) { return "/usr/bin/sudo", nil },
			Authorize: func(context.Context, string) error { return nil },
		}).Prepare(context.Background(), plan)
		Expect(err).NotTo(HaveOccurred())
		Expect(cmd.Args).To(Equal([]string{"/usr/bin/sudo", "-n", "--", "install"}))
		native := exec.CommandContext(context.Background(), "sh", "-c", "exit 42").Run()
		Expect(sdk.ExitStatus(native)).To(Equal(42))
		var auth *sdk.AuthorizationError
		Expect(errors.As(native, &auth)).To(BeFalse())
	})
	It("does not prompt to execute system operations already running as root", func() {
		resolver.Elevated = true
		account = &user.User{Username: "root", Uid: "0"}
		plan, err := resolver.Plan(sdk.ExecutionRequest{Privilege: sdk.OperationPrivilege{Scope: sdk.ScopeSystem}, Name: "install"})
		Expect(err).NotTo(HaveOccurred())
		Expect(plan.Elevate).To(BeFalse())
	})
	It("retains cancellation instead of calling cancellation an authorization denial", func() {
		plan, err := resolver.Plan(sdk.ExecutionRequest{Privilege: sdk.OperationPrivilege{Scope: sdk.ScopeSystem}, Name: "install"})
		Expect(err).NotTo(HaveOccurred())
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err = (sdk.UnixSudoAdapter{LookPath: func(string) (string, error) { return "sudo", nil }, Authorize: func(context.Context, string) error { return context.Canceled }}).Prepare(ctx, plan)
		Expect(err).To(MatchError(context.Canceled))
	})
	It("represents an elevated platform token independently of a root UID", func() {
		resolver.Elevated = true
		plan, err := resolver.Plan(sdk.ExecutionRequest{Privilege: sdk.OperationPrivilege{Scope: sdk.ScopeSystem}, Name: "native-system-operation"})
		Expect(err).NotTo(HaveOccurred())
		Expect(plan.IntendedUser.UID).To(Equal("1000"))
		Expect(plan.Elevate).To(BeFalse())
	})
	It("leaves legacy command metadata unspecified", func() {
		Expect(sdk.PluginOperationPrivilege(sdk.PluginInfo{Commands: []sdk.PluginCommand{{Name: "install"}}}, []string{"install"})).To(BeNil())
	})
})
