package sdk

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/user"
	"syscall"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type windowsChildError int

func (e windowsChildError) Error() string { return fmt.Sprintf("child exited with %d", e) }
func (e windowsChildError) ExitCode() int { return int(e) }

var _ = Describe("Windows operation privileges", func() {
	var adapter windowsPrivilegeAdapter
	var resolver PrivilegeResolver
	var elevated, started bool
	var received ExecutionPlan
	var receivedContext context.Context
	var receivedIO OperationIO
	var result error

	BeforeEach(func() {
		elevated, started, result = false, false, nil
		resolver = PrivilegeResolver{
			CurrentUser: func() (*user.User, error) {
				return &user.User{Username: `HOST\alice`, Uid: "S-1-5-21-1000", HomeDir: `C:\Users\alice`}, nil
			},
			Getenv: func(string) string { return "" },
		}
		adapter = windowsPrivilegeAdapter{
			elevated: func() (bool, error) { return elevated, nil },
			execute: func(ctx context.Context, plan ExecutionPlan, streams OperationIO) error {
				started, received, receivedContext, receivedIO = true, plan, ctx, streams
				return result
			},
		}
	})
	planFor := func(scope OperationScope) ExecutionPlan {
		resolver.Elevated = elevated
		plan, err := resolver.Plan(ExecutionRequest{
			Privilege: OperationPrivilege{Scope: scope}, Name: `C:\Program Files\manager.exe`,
			Args: []string{"install", "", "argument with spaces", `quote"and\slashes\`, "& calc.exe", "$(touch /bad)", "日本語"},
		})
		Expect(err).NotTo(HaveOccurred())
		return plan
	}
	requireAuthorization := func(err error) *AuthorizationError {
		var auth *AuthorizationError
		Expect(errors.As(err, &auth)).To(BeTrue())
		Expect(auth.Kind).To(Equal(AuthorizationRequired))
		Expect(auth.NativeExitCode).To(Equal(-1))
		Expect(started).To(BeFalse())
		return auth
	}

	It("runs a user-scope package manager with literal argv, context, identity and streams", func() {
		plan := planFor(ScopeUser)
		ctx, cancel := context.WithCancel(context.Background())
		DeferCleanup(cancel)
		streams := OperationIO{Stdin: bytes.NewBufferString("input"), Stdout: &bytes.Buffer{}, Stderr: &bytes.Buffer{}}
		Expect(adapter.Run(ctx, plan, streams)).To(Succeed())
		Expect(started).To(BeTrue())
		Expect(received).To(Equal(plan))
		Expect(received.Elevate).To(BeFalse())
		Expect(received.SwitchUser).To(BeFalse())
		Expect(received.IntendedUser.UID).To(Equal("S-1-5-21-1000"))
		Expect(receivedContext).To(BeIdenticalTo(ctx))
		Expect(receivedIO).To(Equal(streams))
	})
	It("returns precise elevation-required for administrator operations without launching or prompting", func() {
		auth := requireAuthorization(adapter.Run(context.Background(), planFor(ScopeSystem), OperationIO{}))
		Expect(auth.Reason).To(ContainSubstring("Run as administrator"))
		Expect(auth.Reason).To(ContainSubstring("unattended"))
		Expect(errors.Is(auth, windowsElevationRequired)).To(BeTrue())
		Expect(ExitStatus(auth)).To(Equal(1))
	})
	It("runs system operations with an elevated token independently of the account SID", func() {
		elevated = true
		plan := planFor(ScopeSystem)
		Expect(plan.Elevate).To(BeFalse())
		Expect(adapter.Run(context.Background(), plan, OperationIO{})).To(Succeed())
		Expect(started).To(BeTrue())
		Expect(received).To(Equal(plan))
	})
	It("refuses user-scope execution from an administrator process instead of elevating a package manager", func() {
		elevated = true
		plan := planFor(ScopeUser)
		Expect(plan.SwitchUser).To(BeTrue())
		auth := requireAuthorization(adapter.Run(context.Background(), plan, OperationIO{}))
		Expect(auth.Reason).To(ContainSubstring("non-elevated terminal"))
	})
	It("rechecks the token before execution so stale plans cannot cross scope boundaries", func() {
		plan := planFor(ScopeUser)
		elevated = true
		Expect(requireAuthorization(adapter.Run(context.Background(), plan, OperationIO{}))).NotTo(BeNil())
		plan = planFor(ScopeSystem)
		elevated = false
		Expect(requireAuthorization(adapter.Run(context.Background(), plan, OperationIO{}))).NotTo(BeNil())
	})
	It("fails closed when token inspection fails or is unavailable", func() {
		cause := errors.New("token query failed")
		adapter.elevated = func() (bool, error) { return false, cause }
		plan := planFor(ScopeUser)
		auth := requireAuthorization(adapter.Run(context.Background(), plan, OperationIO{}))
		Expect(errors.Is(auth, cause)).To(BeTrue())
		adapter.elevated = nil
		Expect(requireAuthorization(adapter.Run(context.Background(), plan, OperationIO{}))).NotTo(BeNil())
	})
	It("does not execute or inspect a token for an already canceled request", func() {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		adapter.elevated = func() (bool, error) { Fail("must not inspect token"); return false, nil }
		Expect(adapter.Run(ctx, planFor(ScopeUser), OperationIO{})).To(MatchError(context.Canceled))
		Expect(started).To(BeFalse())
	})
	It("retains authoritative canceled-elevation launch evidence and the native status", func() {
		cause := &os.PathError{Op: "CreateProcess", Path: "manager.exe", Err: windowsElevationCanceled}
		result = cause
		err := adapter.Run(context.Background(), planFor(ScopeUser), OperationIO{})
		var auth *AuthorizationError
		Expect(errors.As(err, &auth)).To(BeTrue())
		Expect(auth.Kind).To(Equal(AuthorizationDenied))
		Expect(ExitStatus(err)).To(Equal(1223))
		Expect(errors.Is(err, cause)).To(BeTrue())
	})
	It("translates a manifest elevation requirement without retrying a user command as administrator", func() {
		result = &os.PathError{Op: "CreateProcess", Path: "manager.exe", Err: windowsElevationRequired}
		err := adapter.Run(context.Background(), planFor(ScopeUser), OperationIO{})
		var auth *AuthorizationError
		Expect(errors.As(err, &auth)).To(BeTrue())
		Expect(auth.Kind).To(Equal(AuthorizationRequired))
		Expect(errors.Is(err, windowsElevationRequired)).To(BeTrue())
	})
	It("preserves child failures even when their exit codes match authorization statuses", func() {
		for _, code := range []int{42, 740, 1223} {
			result = windowsChildError(code)
			err := adapter.Run(context.Background(), planFor(ScopeUser), OperationIO{})
			Expect(err).To(BeIdenticalTo(result))
			Expect(ExitStatus(err)).To(Equal(code))
			var auth *AuthorizationError
			Expect(errors.As(err, &auth)).To(BeFalse())
		}
	})
	It("keeps access-denied and cancellation distinct from denied UAC consent", func() {
		for _, cause := range []error{syscall.Errno(5), context.Canceled, context.DeadlineExceeded} {
			result = cause
			Expect(adapter.Run(context.Background(), planFor(ScopeUser), OperationIO{})).To(BeIdenticalTo(cause))
		}
	})
	It("runs direct commands with the provided stdio and preserves their real exit error", func() {
		// The existing SDK suite uses shell fixtures; Windows needs a real-host
		// integration run with native executables in addition to these mocks.
		adapter.execute = nil
		plan := planFor(ScopeUser)
		plan.Name, plan.Args = "sh", []string{"-c", `read value; printf '%s' "$value"; printf error >&2; exit 42`}
		var stdout, stderr bytes.Buffer
		err := adapter.Run(context.Background(), plan, OperationIO{Stdin: bytes.NewBufferString("literal input\n"), Stdout: &stdout, Stderr: &stderr})
		var exit *exec.ExitError
		Expect(errors.As(err, &exit)).To(BeTrue())
		Expect(ExitStatus(err)).To(Equal(42))
		Expect(stdout.String()).To(Equal("literal input"))
		Expect(stderr.String()).To(Equal("error"))
	})
	It("forwards cancellation to an executing operation", func() {
		ctx, cancel := context.WithCancel(context.Background())
		DeferCleanup(cancel)
		adapter.execute = func(ctx context.Context, _ ExecutionPlan, _ OperationIO) error {
			cancel()
			return ctx.Err()
		}
		Expect(adapter.Run(ctx, planFor(ScopeUser), OperationIO{Stdout: io.Discard})).To(MatchError(context.Canceled))
	})
})
