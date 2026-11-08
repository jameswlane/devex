package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
)

// handleIsInstalled distinguishes a successful empty query from native failures.
func (p *XbpsPlugin) handleIsInstalled(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no packages specified")
	}
	for _, pkg := range args {
		if strings.TrimSpace(pkg) == "" || strings.HasPrefix(pkg, "-") {
			return fmt.Errorf("invalid package name: %q", pkg)
		}
	}
	native, err := exec.LookPath("xbps-query")
	if err != nil {
		return err
	}
	native, err = filepath.Abs(native)
	if err != nil {
		return err
	}
	dir, err := os.Getwd()
	if err != nil {
		return err
	}
	query := []string{"--list-pkgs"}
	var output bytes.Buffer
	err = sdk.RunOperation(ctx, sdk.ExecutionRequest{Privilege: sdk.OperationPrivilege{Scope: sdk.ScopeUser}, Name: native, Args: query, Dir: dir}, sdk.OperationIO{Stdout: &output, Stderr: os.Stderr})
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if err != nil {
		return err
	}
	installed := make(map[string]bool)
	for _, line := range strings.Split(output.String(), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 || fields[0] != "ii" {
			continue
		}
		name := fields[1]
		if i := strings.LastIndex(name, "-"); i >= 0 {
			installed[name[:i]] = true
		}
	}
	var missing []string
	for _, pkg := range args {
		if !installed[pkg] {
			missing = append(missing, pkg)
		}
	}
	if len(missing) > 0 {
		return &sdk.NotInstalledError{Packages: missing}
	}
	return nil
}
