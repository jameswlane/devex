package utils

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jameswlane/devex/apps/cli/internal/security"
	"github.com/jameswlane/devex/apps/cli/internal/shellrc"

	"github.com/jameswlane/devex/apps/cli/internal/types"
	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
)

// InstallHookPrivilege keeps legacy hooks in user scope and rejects typos before
// execution. Explicit system hooks use the same adapter as package operations.
func InstallHookPrivilege(command types.InstallCommand) (sdk.OperationPrivilege, error) {
	if err := command.Validate(); err != nil {
		return sdk.OperationPrivilege{}, err
	}
	scope := sdk.OperationScope(command.Scope)
	if scope == "" {
		scope = sdk.ScopeUser
	}
	if scope != sdk.ScopeUser && scope != sdk.ScopeSystem {
		return sdk.OperationPrivilege{}, fmt.Errorf("invalid install hook scope %q: use user or system", command.Scope)
	}
	if scope == sdk.ScopeSystem && (command.Shell == "" || command.Copy != nil || command.Command != "" || command.Sleep != 0 || len(command.ShellInit) > 0 || len(command.Path) > 0) {
		return sdk.OperationPrivilege{}, fmt.Errorf("system install hooks require a shell command only; use separate user hooks for copies, commands or delays")
	}
	return sdk.OperationPrivilege{Scope: scope}, nil
}

// InstallHookPolicy applies the configured validator and application overrides to
// the original script, before adding execution wrappers or resolving privileges.
type InstallHookPolicy struct {
	Validator *security.CommandValidator
	AppName   string
}

// HookPolicyError identifies a rejected script, separately from an optional hook's
// runtime failure. Callers must surface policy rejections to the user.
type HookPolicyError struct {
	AppName string
	Err     error
	// Command marks a rejected legacy `command:` hook, whose installer-side
	// checks security.yaml cannot override.
	Command bool
}

func (e *HookPolicyError) Error() string {
	if e.Command {
		return fmt.Sprintf("install hook policy rejected command for app %q: %v", e.AppName, e.Err)
	}
	return fmt.Sprintf("install hook policy rejected shell for app %q: %v; review the hook and security.yaml level or app_overrides before retrying", e.AppName, e.Err)
}

func (e *HookPolicyError) Unwrap() error { return e.Err }

// ExecInstallHookWithContext resolves the invoking account before elevation and
// passes identity through the SDK, never interpolated shell text. Bash expands
// variables in the target account environment; the script stays one argv value.
func ExecInstallHookWithContext(ctx context.Context, command types.InstallCommand, policies ...InstallHookPolicy) (string, error) {
	privilege, err := InstallHookPrivilege(command)
	if err != nil {
		return "", err
	}
	policy := InstallHookPolicy{}
	if len(policies) > 0 {
		policy = policies[0]
	}
	orgPolicy, err := security.LoadOrgPolicy("")
	if err != nil {
		return "", err
	}
	if policy.Validator == nil {
		policy.Validator = security.NewCommandValidatorWithConfig(security.SecurityLevelModerate, nil).WithOrgPolicy(orgPolicy)
	}
	if err := orgPolicy.CheckHook(command.Shell, privilege.Scope); err != nil {
		return "", &HookPolicyError{AppName: policy.AppName, Err: err}
	}
	if err := policy.Validator.ValidateCommandForApp(command.Shell, policy.AppName); err != nil {
		return "", &HookPolicyError{AppName: policy.AppName, Err: err}
	}
	userPlan, err := sdk.NewPrivilegeResolver().Plan(sdk.ExecutionRequest{
		Privilege: sdk.OperationPrivilege{Scope: sdk.ScopeUser}, Name: "install-hook",
	})
	if err != nil {
		return "", fmt.Errorf("resolve install hook invoking user: %w", err)
	}
	privilege.User = userPlan.IntendedUser
	var output string
	err = guardHookScope(userPlan.IntendedUser.HomeDir, policy.AppName, func() (runErr error) {
		if privilege.Scope == sdk.ScopeUser {
			output, runErr = ExecOperationWithContext(ctx, privilege, "bash", "-c", NonInteractiveSudoShell(command.Shell))
		} else {
			output, runErr = ExecOperationWithContext(ctx, privilege, "env", "DEVEX_INVOKING_USER="+userPlan.IntendedUser.Username, "bash", "-c", NonInteractiveSudoShell(command.Shell))
		}
		return runErr
	})
	return output, err
}

// GuardHookScope runs a hook that does not go through ExecInstallHookWithContext
// (such as a legacy command hook) with the same protected-file guard.
func GuardHookScope(appName string, fn func() error) error {
	userPlan, err := sdk.NewPrivilegeResolver().Plan(sdk.ExecutionRequest{
		Privilege: sdk.OperationPrivilege{Scope: sdk.ScopeUser}, Name: "install-hook",
	})
	if err != nil {
		return fmt.Errorf("resolve install hook invoking user: %w", err)
	}
	return guardHookScope(userPlan.IntendedUser.HomeDir, appName, fn)
}

// IsHookRejection reports whether err is a policy or scope rejection, which
// must fail the app even when the hook is optional.
func IsHookRejection(err error) bool {
	var policyErr *HookPolicyError
	var scopeErr *HookScopeError
	return errors.As(err, &policyErr) || errors.As(err, &scopeErr)
}

// NonInteractiveSudoShell prevents catalog shell hooks and pipelines from prompting
// after the setup UI has taken ownership of the terminal.
func NonInteractiveSudoShell(command string) string {
	if !strings.Contains(command, "sudo ") {
		return command
	}
	return "sudo() { command sudo -n \"$@\"; }; export -f sudo\n" + command
}

// ApplyShellActions executes only declarative actions; shell text is never evaluated here.
func ApplyShellActions(ctx context.Context, command types.InstallCommand) error {
	if len(command.ShellInit) == 0 && len(command.Path) == 0 {
		return nil
	}
	_, err := shellrc.ApplyActions(ctx, command.ShellInit, command.Path)
	return err
}
