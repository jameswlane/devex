package utils

import (
	"context"
	"fmt"
	"io"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"

	"github.com/spf13/afero"

	"github.com/jameswlane/devex/apps/cli/internal/execinputs"
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

func writeCopy(destination string, data []byte) error {
	dstFile, err := fs.AppFs.Create(destination)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	if _, err := dstFile.Write(data); err != nil {
		_ = dstFile.Close()
		return fmt.Errorf("failed to copy file content: %w", err)
	}
	return dstFile.Close()
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
	if _, pinned := execinputs.Pinned(ctx); pinned {
		// A confirmed install copies only the confirmed content.
		data, err := afero.ReadFile(fs.AppFs, source)
		if err != nil {
			return fmt.Errorf("failed to open source file: %w", err)
		}
		if err := execinputs.VerifyFile(ctx, source, data); err != nil {
			return err
		}
		if !plan.SwitchUser {
			return writeCopy(destination, data)
		}
		// cp runs as the intended user and rereads the source; it was verified
		// just above.
	}
	if !plan.SwitchUser {
		return CopyFile(source, destination)
	}
	if output, err := ExecOperationWithContext(ctx, plan.Privilege, plan.Name, plan.Args...); err != nil {
		return fmt.Errorf("copy user configuration: %s: %w", output, err)
	}
	return nil
}
