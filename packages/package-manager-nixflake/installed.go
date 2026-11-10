package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
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
	var output, diagnostic bytes.Buffer
	err := p.runWithIO(ctx, "is-installed", sdk.OperationIO{Stdout: &output, Stderr: &diagnostic}, "profile", "list", "--json")
	if unsupportedProfileJSON(err, diagnostic.String()) {
		return p.handlePlainIsInstalled(ctx, args)
	}
	_, _ = io.Copy(os.Stderr, &diagnostic)
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
	markInstalledAttrPaths(installed, elements)
	return reportMissing(installed, args)
}

// handlePlainIsInstalled answers from the plain-text `nix profile list` of
// Nix releases (2.8) that predate `--json`; the same listing drives removal.
func (p *NixflakePlugin) handlePlainIsInstalled(ctx context.Context, args []string) error {
	var output bytes.Buffer
	if err := p.runWithIO(ctx, "is-installed", sdk.OperationIO{Stdout: &output, Stderr: os.Stderr}, "profile", "list"); err != nil {
		return err
	}
	elements, err := plainProfileElements(output.String())
	if err != nil {
		return err
	}
	installed := make(map[string]bool)
	markInstalledAttrPaths(installed, elements)
	return reportMissing(installed, args)
}

func markInstalledAttrPaths(installed map[string]bool, elements []profileElement) {
	for _, e := range elements {
		if e.AttrPath != "" {
			installed[e.AttrPath] = true
			parts := strings.Split(e.AttrPath, ".")
			installed[parts[len(parts)-1]] = true
		}
	}
}

func reportMissing(installed map[string]bool, args []string) error {
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
