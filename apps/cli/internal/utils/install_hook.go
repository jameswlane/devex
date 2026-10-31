package utils

import (
	"context"
	"fmt"

	"github.com/jameswlane/devex/apps/cli/internal/types"
	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
)

// InstallHookPrivilege keeps legacy hooks in user scope and rejects typos before
// execution. Explicit system hooks use the same adapter as package operations.
func InstallHookPrivilege(command types.InstallCommand) (sdk.OperationPrivilege, error) {
	scope := sdk.OperationScope(command.Scope)
	if scope == "" {
		scope = sdk.ScopeUser
	}
	if scope != sdk.ScopeUser && scope != sdk.ScopeSystem {
		return sdk.OperationPrivilege{}, fmt.Errorf("invalid install hook scope %q: use user or system", command.Scope)
	}
	if scope == sdk.ScopeSystem && (command.Shell == "" || command.Copy != nil || command.Command != "" || command.Sleep != 0) {
		return sdk.OperationPrivilege{}, fmt.Errorf("system install hooks require a shell command only; use separate user hooks for copies, commands or delays")
	}
	return sdk.OperationPrivilege{Scope: scope}, nil
}

// ExecInstallHookWithContext resolves the invoking account before elevation and
// passes its name as an argv environment assignment, never interpolated shell
// text. System scripts retain shell variables such as VERSION_CODENAME.
func ExecInstallHookWithContext(ctx context.Context, command types.InstallCommand) (string, error) {
	privilege, err := InstallHookPrivilege(command)
	if err != nil {
		return "", err
	}
	userPlan, err := sdk.NewPrivilegeResolver().Plan(sdk.ExecutionRequest{
		Privilege: sdk.OperationPrivilege{Scope: sdk.ScopeUser}, Name: "install-hook",
	})
	if err != nil {
		return "", fmt.Errorf("resolve install hook invoking user: %w", err)
	}
	if privilege.Scope == sdk.ScopeUser {
		return ExecAsUserWithContext(ctx, ExpandUserPlaceholders(command.Shell, *userPlan.IntendedUser))
	}
	privilege.User = userPlan.IntendedUser
	return ExecOperationWithContext(ctx, privilege, "env", "DEVEX_INVOKING_USER="+userPlan.IntendedUser.Username, "bash", "-c", command.Shell)
}
