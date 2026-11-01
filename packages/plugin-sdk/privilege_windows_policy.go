package sdk

import (
	"context"
	"errors"
	"os/exec"
	"syscall"
)

// Windows launch errors are numeric here so the policy can be tested on any host.
const (
	windowsElevationRequired syscall.Errno = 740
	windowsElevationCanceled syscall.Errno = 1223
)

// windowsPrivilegeAdapter deliberately returns a precise required result instead
// of using ShellExecuteEx/runas, which cannot preserve OperationIO or cancellation
// of a pending consent dialog. Only an already-authorized process runs a command.
// The injected token query and runner let Linux tests exercise Windows policy.
type windowsPrivilegeAdapter struct {
	elevated func() (bool, error)
	execute  func(context.Context, ExecutionPlan, OperationIO) error
}

func (a windowsPrivilegeAdapter) Run(ctx context.Context, plan ExecutionPlan, streams OperationIO) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if a.elevated == nil {
		return authorizationRequired("Windows process token inspection is unavailable; run this operation on Windows", nil)
	}
	elevated, err := a.elevated()
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if err != nil {
		return authorizationRequired("cannot verify Windows process token; retry from a terminal running as the intended user", err)
	}
	switch plan.Privilege.Scope {
	case ScopeUser:
		if elevated || plan.SwitchUser || plan.Elevate {
			return authorizationRequired("Windows user-scope operation requires a non-elevated terminal running as the intended user; automatic de-elevation and account switching are unavailable", nil)
		}
	case ScopeSystem:
		if !elevated || plan.Elevate || plan.SwitchUser {
			return authorizationRequired("Windows administrator operation requires elevation; run only this system operation from a terminal opened with Run as administrator and approve UAC; unattended execution requires an administrator-provisioned elevated process", windowsElevationRequired)
		}
	default:
		return authorizationRequired("Windows operation requires explicit user or system scope", nil)
	}
	execute := a.execute
	if execute == nil {
		execute = func(ctx context.Context, plan ExecutionPlan, streams OperationIO) error {
			cmd := exec.CommandContext(ctx, plan.Name, plan.Args...)
			configureOperationCommand(cmd, plan.ExecutionRequest)
			if plan.Unattended {
				streams.Stdin = nil
			}
			return runPreparedCommand(cmd, streams)
		}
	}
	err = execute(ctx, plan, streams)
	// Only launch errors are evidence of failed authorization. A child exiting
	// with 740 or 1223 is still an ordinary command failure, not a UAC denial.
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return err
	}
	if errors.Is(err, windowsElevationRequired) {
		return authorizationRequired("Windows refused to start the command because it requires elevation; verify the operation scope and run system operations from a terminal opened with Run as administrator", err)
	}
	if errors.Is(err, windowsElevationCanceled) {
		return NewAuthorizationDeniedError("Windows canceled authorization before the command started; approve UAC to run the requested system operation", int(windowsElevationCanceled), err)
	}
	return err
}
