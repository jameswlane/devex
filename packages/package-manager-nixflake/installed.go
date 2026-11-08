package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
)

// handleIsInstalled distinguishes a successful empty query from native failures.
func (p *NixflakePlugin) handleIsInstalled(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no packages specified")
	}
	for _, pkg := range args {
		if strings.TrimSpace(pkg) == "" || strings.HasPrefix(pkg, "-") {
			return fmt.Errorf("invalid package name: %q", pkg)
		}
	}
	native, err := exec.LookPath("nix")
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
	query := []string{"profile", "list", "--json"}
	var output bytes.Buffer
	err = sdk.RunOperation(ctx, sdk.ExecutionRequest{Privilege: sdk.OperationPrivilege{Scope: sdk.ScopeUser}, Name: native, Args: query, Dir: dir}, sdk.OperationIO{Stdout: &output, Stderr: os.Stderr})
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if err != nil {
		return err
	}
	installed := make(map[string]bool)
	var profile struct {
		Version  int             `json:"version"`
		Elements json.RawMessage `json:"elements"`
	}
	if err := json.Unmarshal(output.Bytes(), &profile); err != nil {
		return fmt.Errorf("read profile: %w", err)
	}
	var elements []profileElement
	switch profile.Version {
	case 1, 2:
		if err := json.Unmarshal(profile.Elements, &elements); err != nil {
			return err
		}
		if elements == nil {
			return fmt.Errorf("invalid indexed profile")
		}
	case 3:
		var named map[string]profileElement
		if err := json.Unmarshal(profile.Elements, &named); err != nil {
			return err
		}
		if named == nil {
			return fmt.Errorf("invalid named profile")
		}
		for name, e := range named {
			installed[name] = true
			elements = append(elements, e)
		}
	default:
		return fmt.Errorf("unsupported Nix profile version %d", profile.Version)
	}
	for _, e := range elements {
		if e.AttrPath != "" {
			installed[e.AttrPath] = true
			parts := strings.Split(e.AttrPath, ".")
			installed[parts[len(parts)-1]] = true
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
