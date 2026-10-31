package utils

import (
	"bytes"
	"context"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
)

// ExecAsUser executes legacy shell text as the original invoking user. Supplying
// args selects argv execution; arguments are never interpolated into shell text.
func ExecAsUser(command string, args ...string) (string, error) {
	return ExecAsUserWithContext(context.Background(), command, args...)
}

func ExecAsUserWithContext(ctx context.Context, command string, args ...string) (string, error) {
	name := command
	if len(args) == 0 {
		name, args = "bash", []string{"-c", command}
	}
	return ExecOperationWithContext(ctx, sdk.OperationPrivilege{Scope: sdk.ScopeUser}, name, args...)
}

// ExecOperationWithContext shares the SDK's identity and authorization decisions.
func ExecOperationWithContext(ctx context.Context, privilege sdk.OperationPrivilege, name string, args ...string) (string, error) {
	request := sdk.ExecutionRequest{Privilege: privilege, Name: name, Args: args}
	if _, ok := sdk.DefaultPrivilegeAdapter.(sdk.CommandPrivilegeAdapter); !ok {
		var output bytes.Buffer
		err := sdk.RunOperation(ctx, request, sdk.OperationIO{Stdout: &output, Stderr: &output})
		return output.String(), err
	}
	cmd, err := sdk.PrepareOperation(ctx, request)
	if err != nil {
		return "", err
	}
	return CommandExec.RunCommand(ctx, cmd.Path, cmd.Args[1:]...)
}
