package utils

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

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

// copyAsUserScript writes stdin to $1 like cp: an existing destination is
// truncated in place and keeps its mode and owner; a new one is created with
// the source mode ($2, octal) less the umask, and is never more permissive
// than that while it is written.
const copyAsUserScript = `if [ -e "$1" ]; then exec cat > "$1"; fi
mode=$(printf '%o' $(( 0$2 & ~0$(umask) ))) || exit 1
umask 077
cat > "$1" && chmod "$mode" "$1"`

// writeCopyAsUser writes data to destination as the operation's intended
// user, through the same sudo -n launch user-scope operations use.
func writeCopyAsUser(ctx context.Context, privilege sdk.OperationPrivilege, destination string, mode os.FileMode, data []byte) error {
	request := sdk.ExecutionRequest{Privilege: privilege, Name: "/bin/sh", Args: []string{"-c", copyAsUserScript, "devex-copy", destination, strconv.FormatUint(uint64(mode), 8)}}
	cmd, err := sdk.PrepareOperation(ctx, request)
	if err != nil {
		return fmt.Errorf("copy user configuration as the intended user: %w; nothing was copied", err)
	}
	// sudo runs non-interactively (-n), so stdin carries only the content.
	cmd.Stdin = bytes.NewReader(data)
	var output bytes.Buffer
	cmd.Stdout, cmd.Stderr = &output, &output
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		return fmt.Errorf("copy user configuration: %s: %w", strings.TrimSpace(output.String()), err)
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
		// Never let another process reread the source: the verified bytes are
		// written under the intended user's authority from stdin.
		info, err := fs.AppFs.Stat(source)
		if err != nil {
			return fmt.Errorf("failed to open source file: %w", err)
		}
		return writeCopyAsUser(ctx, plan.Privilege, destination, info.Mode().Perm(), data)
	}
	if !plan.SwitchUser {
		return CopyFile(source, destination)
	}
	if output, err := ExecOperationWithContext(ctx, plan.Privilege, plan.Name, plan.Args...); err != nil {
		return fmt.Errorf("copy user configuration: %s: %w", output, err)
	}
	return nil
}
