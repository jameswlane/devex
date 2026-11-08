package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
)

var derivationVersion = regexp.MustCompile(`-[^a-zA-Z].*$`)

// handleIsInstalled distinguishes a successful empty query from native failures.
func (p *NixpkgsPlugin) handleIsInstalled(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no packages specified")
	}
	for _, pkg := range args {
		if strings.TrimSpace(pkg) == "" || strings.HasPrefix(pkg, "-") {
			return fmt.Errorf("invalid package name: %q", pkg)
		}
	}
	native, err := exec.LookPath("nix-env")
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
	query := []string{"--query", "--installed", "--json"}
	var output bytes.Buffer
	err = sdk.RunOperation(ctx, sdk.ExecutionRequest{Privilege: sdk.OperationPrivilege{Scope: sdk.ScopeUser}, Name: native, Args: query, Dir: dir}, sdk.OperationIO{Stdout: &output, Stderr: os.Stderr})
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if err != nil {
		return err
	}
	installed := make(map[string]bool)
	var entries map[string]struct {
		Name string `json:"name"`
	}
	if err := json.Unmarshal(output.Bytes(), &entries); err != nil {
		return fmt.Errorf("read installed packages: %w", err)
	}
	if entries == nil {
		return fmt.Errorf("invalid installed packages JSON")
	}
	for _, entry := range entries {
		if entry.Name == "" {
			return fmt.Errorf("installed package has no name")
		}
		installed[entry.Name] = true
		installed[derivationVersion.ReplaceAllString(entry.Name, "")] = true
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
