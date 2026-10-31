package utils

import (
	"context"
	"fmt"
	"io"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"

	"github.com/spf13/afero"

	"github.com/jameswlane/devex/apps/cli/internal/fs"
	"github.com/jameswlane/devex/apps/cli/internal/log"
)

func CopyFile(source, destination string) error {
	log.Info("Starting file copy", "source", source, "destination", destination)

	srcFile, err := fs.AppFs.Open(source)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer func(srcFile afero.File) {
		err := srcFile.Close()
		if err != nil {
			log.Error("Failed to close source file", err)
		}
	}(srcFile)

	dstFile, err := fs.AppFs.Create(destination)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer func(dstFile afero.File) {
		err := dstFile.Close()
		if err != nil {
			log.Error("Failed to close destination file", err)
		}
	}(dstFile)

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return fmt.Errorf("failed to copy file content: %w", err)
	}

	return nil
}

// CopyUserFileWithContext creates configuration files under the intended user's
// authority, including when the CLI itself was invoked with sudo.
func CopyUserFileWithContext(ctx context.Context, source, destination string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	plan, err := sdk.NewPrivilegeResolver().Plan(sdk.ExecutionRequest{
		Privilege: sdk.OperationPrivilege{Scope: sdk.ScopeUser}, Name: "cp", Args: []string{"--", source, destination},
	})
	if err != nil {
		return err
	}
	if !plan.SwitchUser {
		return CopyFile(source, destination)
	}
	if output, err := ExecOperationWithContext(ctx, plan.Privilege, plan.Name, plan.Args...); err != nil {
		return fmt.Errorf("copy user configuration: %s: %w", output, err)
	}
	return nil
}
