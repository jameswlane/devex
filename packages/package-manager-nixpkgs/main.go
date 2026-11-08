package main

// Build timestamp: 2025-09-03 17:41:19

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
)

var version = "dev" // Set by goreleaser

// errUpdateUnsupported explains why update never runs a native command:
// nix-env has no refresh action, and choosing between nix-channel and flake
// inputs would silently pick a channel or flake policy for the user.
const errUpdateUnsupported = "nix-env cannot refresh package sources; run 'nix-channel --update' for channels, or use the nixflake plugin's update for flakes"

// NixpkgsPlugin implements the Nixpkgs package manager
type NixpkgsPlugin struct {
	*sdk.PackageManagerPlugin
}

// NewNixpkgsPlugin creates a new Nixpkgs plugin
func NewNixpkgsPlugin() *NixpkgsPlugin {
	// Every operation acts on the invoking user's profile and must never elevate.
	user := &sdk.OperationPrivilege{Scope: sdk.ScopeUser}
	info := sdk.PluginInfo{
		Name:        "package-manager-nixpkgs",
		Version:     version,
		Description: "Nix packages collection",
		Author:      "DevEx Team",
		Repository:  "https://github.com/jameswlane/devex",
		Tags:        []string{"nix", "nixpkgs", "functional"},
		Commands: []sdk.PluginCommand{
			{Name: "upgrade", Description: "Upgrade installed packages", Privilege: user},
			{Name: "is-installed", Description: "Check whether every package is installed", Privilege: user},
			{
				Name:        "install",
				Description: "Install packages into the user profile",
				Usage:       "Install packages by derivation name (nix-env --install)",
				Privilege:   user,
			},
			{
				Name:        "remove",
				Description: "Remove packages from the user profile",
				Usage:       "Uninstall packages by derivation name (nix-env --uninstall)",
				Privilege:   user,
			},
			{
				Name:        "update",
				Description: "Update package repositories (unsupported)",
				Usage:       "Unsupported: run nix-channel --update, or use the nixflake plugin for flakes",
				Privilege:   user,
			},
			{
				Name:        "search",
				Description: "Search for packages",
				Usage:       "List available packages whose derivation name matches each term (nix-env --query --available)",
				Privilege:   user,
			},
			{
				Name:        "list",
				Description: "List packages",
				Usage:       "List packages installed in the user profile (nix-env --query --installed)",
				Privilege:   user,
			},
		},
	}

	return &NixpkgsPlugin{
		PackageManagerPlugin: sdk.NewPackageManagerPlugin(info, "nix-env"),
	}
}

// Execute handles command execution
func (p *NixpkgsPlugin) Execute(command string, args []string) error {
	return p.ExecuteContext(context.Background(), command, args)
}

// ExecuteContext runs a native operation with cancellation and per-command scope.
func (p *NixpkgsPlugin) ExecuteContext(ctx context.Context, command string, args []string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	switch command {
	case "upgrade":
		return p.run(ctx, "upgrade", append([]string{"--upgrade"}, args...)...)
	case "is-installed":
		return p.handleIsInstalled(ctx, args)
	case "install":
		return p.handleInstall(ctx, args)
	case "remove":
		return p.handleRemove(ctx, args)
	case "update":
		return fmt.Errorf("%s", errUpdateUnsupported)
	case "search":
		return p.handleSearch(ctx, args)
	case "list":
		return p.handleList(ctx, args)
	default:
		return fmt.Errorf("unknown command: %s", command)
	}
}

func (p *NixpkgsPlugin) handleInstall(ctx context.Context, args []string) error {
	if err := validatePackages(args); err != nil {
		return err
	}

	fmt.Printf("Installing packages: %s\n", strings.Join(args, ", "))

	return p.run(ctx, "install", append([]string{"--install"}, args...)...)
}

func (p *NixpkgsPlugin) handleRemove(ctx context.Context, args []string) error {
	if err := validatePackages(args); err != nil {
		return err
	}

	fmt.Printf("Removing packages: %s\n", strings.Join(args, ", "))

	return p.run(ctx, "remove", append([]string{"--uninstall"}, args...)...)
}

// handleSearch passes each term as its own selector: nix-env matches derivation
// names, which never contain spaces, so a joined phrase could never match.
func (p *NixpkgsPlugin) handleSearch(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no search term specified")
	}
	for _, term := range args {
		if strings.TrimSpace(term) == "" || strings.HasPrefix(term, "-") {
			return fmt.Errorf("invalid search term: %q", term)
		}
	}
	fmt.Printf("Searching for: %s\n", strings.Join(args, ", "))

	return p.run(ctx, "search", append([]string{"--query", "--available"}, args...)...)
}

func (p *NixpkgsPlugin) handleList(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("list takes no arguments")
	}
	return p.run(ctx, "list", "--query", "--installed")
}

// run executes nix-env with the privilege advertised for operation.
func (p *NixpkgsPlugin) run(ctx context.Context, operation string, args ...string) error {
	privilege := sdk.PluginOperationPrivilege(p.Info(), []string{operation})
	if privilege == nil {
		return fmt.Errorf("operation %s has no declared privilege", operation)
	}
	native, lookupErr := exec.LookPath("nix-env")
	if lookupErr != nil {
		return fmt.Errorf("nix-env is not available on this system; install Nix")
	}
	native, err := filepath.Abs(native)
	if err != nil {
		return err
	}
	dir, err := os.Getwd()
	if err != nil {
		return err
	}
	err = sdk.RunOperation(ctx, sdk.ExecutionRequest{Privilege: *privilege, Name: native, Args: args, Dir: dir}, sdk.OperationIO{Stdin: os.Stdin, Stdout: os.Stdout, Stderr: os.Stderr})
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return err
}

func validatePackages(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no packages specified")
	}
	for _, pkg := range args {
		if strings.TrimSpace(pkg) == "" || strings.HasPrefix(pkg, "-") {
			return fmt.Errorf("invalid package name: %q", pkg)
		}
	}
	return nil
}

func main() {
	plugin := NewNixpkgsPlugin()
	sdk.HandleArgs(plugin, os.Args[1:])
}
