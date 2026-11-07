package main

// Build timestamp: 2025-09-03 17:41:19

import (
	"context"
	"fmt"
	"os"
	"strings"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
)

var version = "dev" // Set by goreleaser

// defaultFlake qualifies bare package names, so "git" installs nixpkgs#git.
const defaultFlake = "nixpkgs"

// NixflakePlugin implements the Nix Flake package manager
type NixflakePlugin struct {
	*sdk.PackageManagerPlugin
}

// NewNixflakePlugin creates a new Nix Flake plugin
func NewNixflakePlugin() *NixflakePlugin {
	// Every operation acts on the invoking user's profile or flake and must never elevate.
	user := &sdk.OperationPrivilege{Scope: sdk.ScopeUser}
	info := sdk.PluginInfo{
		Name:        "package-manager-nixflake",
		Version:     version,
		Description: "Nix flake package manager",
		Author:      "DevEx Team",
		Repository:  "https://github.com/jameswlane/devex",
		Tags:        []string{"nix", "flake", "functional"},
		Commands: []sdk.PluginCommand{
			{
				Name:        "install",
				Description: "Install packages into the user profile",
				Usage:       "Install flake installables; bare names resolve to nixpkgs#<name> (nix profile install)",
				Privilege:   user,
			},
			{
				Name:        "remove",
				Description: "Remove packages from the user profile",
				Usage:       "Remove profile elements by name (nix profile remove)",
				Privilege:   user,
			},
			{
				Name:        "update",
				Description: "Update flake inputs",
				Usage:       "Update every input of the flake in the current directory (nix flake update)",
				Privilege:   user,
			},
			{
				Name:        "search",
				Description: "Search for packages",
				Usage:       "Search nixpkgs by name or description (nix search nixpkgs)",
				Privilege:   user,
			},
			{
				Name:        "list",
				Description: "List packages",
				Usage:       "List packages installed in the user profile (nix profile list)",
				Privilege:   user,
			},
		},
	}

	return &NixflakePlugin{
		PackageManagerPlugin: sdk.NewPackageManagerPlugin(info, "nix"),
	}
}

// Execute handles command execution
func (p *NixflakePlugin) Execute(command string, args []string) error {
	return p.ExecuteContext(context.Background(), command, args)
}

// ExecuteContext runs a native operation with cancellation and per-command scope.
func (p *NixflakePlugin) ExecuteContext(ctx context.Context, command string, args []string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	switch command {
	case "install":
		return p.handleInstall(ctx, args)
	case "remove":
		return p.handleRemove(ctx, args)
	case "update":
		return p.handleUpdate(ctx, args)
	case "search":
		return p.handleSearch(ctx, args)
	case "list":
		return p.handleList(ctx, args)
	default:
		return fmt.Errorf("unknown command: %s", command)
	}
}

func (p *NixflakePlugin) handleInstall(ctx context.Context, args []string) error {
	if err := validatePackages(args); err != nil {
		return err
	}

	fmt.Printf("Installing packages: %s\n", strings.Join(args, ", "))

	cmdArgs := []string{"profile", "install"}
	for _, pkg := range args {
		cmdArgs = append(cmdArgs, installable(pkg))
	}
	return p.run(ctx, "install", cmdArgs...)
}

func (p *NixflakePlugin) handleRemove(ctx context.Context, args []string) error {
	if err := validatePackages(args); err != nil {
		return err
	}

	fmt.Printf("Removing packages: %s\n", strings.Join(args, ", "))

	return p.run(ctx, "remove", append([]string{"profile", "remove"}, args...)...)
}

func (p *NixflakePlugin) handleUpdate(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("update takes no arguments; it updates every input of the flake in the current directory")
	}
	fmt.Println("Updating flake inputs...")
	return p.run(ctx, "update", "flake", "update")
}

func (p *NixflakePlugin) handleSearch(ctx context.Context, args []string) error {
	searchTerm, err := searchPhrase(args)
	if err != nil {
		return err
	}
	fmt.Printf("Searching for: %s\n", searchTerm)

	return p.run(ctx, "search", "search", defaultFlake, searchTerm)
}

func (p *NixflakePlugin) handleList(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("list takes no arguments")
	}
	return p.run(ctx, "list", "profile", "list")
}

// installable keeps explicit flake references (nixpkgs#git, github:o/r, ./path)
// and qualifies bare package names with the default flake.
func installable(pkg string) string {
	if strings.ContainsAny(pkg, "#:/") || strings.HasPrefix(pkg, ".") {
		return pkg
	}
	return defaultFlake + "#" + pkg
}

// run executes nix with the privilege advertised for operation.
func (p *NixflakePlugin) run(ctx context.Context, operation string, args ...string) error {
	privilege := sdk.PluginOperationPrivilege(p.Info(), []string{operation})
	if privilege == nil {
		return fmt.Errorf("operation %s has no declared privilege", operation)
	}
	if !p.IsAvailable() {
		return fmt.Errorf("nix is not available on this system; install Nix and enable the nix-command and flakes experimental features")
	}
	err := sdk.ExecOperationWithContext(ctx, *privilege, "nix", args...)
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

func searchPhrase(args []string) (string, error) {
	searchTerm := strings.TrimSpace(strings.Join(args, " "))
	if searchTerm == "" {
		return "", fmt.Errorf("no search term specified")
	}
	if strings.HasPrefix(searchTerm, "-") {
		return "", fmt.Errorf("search term must not start with '-'")
	}
	return searchTerm, nil
}

func main() {
	plugin := NewNixflakePlugin()
	sdk.HandleArgs(plugin, os.Args[1:])
}
