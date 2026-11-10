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

// EopkgPlugin implements the Eopkg package manager
type EopkgPlugin struct {
	*sdk.PackageManagerPlugin
}

// NewEopkgPlugin creates a new Eopkg plugin
func NewEopkgPlugin() *EopkgPlugin {
	system := &sdk.OperationPrivilege{Scope: sdk.ScopeSystem}
	user := &sdk.OperationPrivilege{Scope: sdk.ScopeUser}
	info := sdk.PluginInfo{
		Name:        "package-manager-eopkg",
		Version:     version,
		Description: "Eopkg package manager for Solus",
		Author:      "DevEx Team",
		Repository:  "https://github.com/jameswlane/devex",
		Tags:        []string{"eopkg", "solus", "linux"},
		Commands: []sdk.PluginCommand{
			{Name: "upgrade", Description: "Upgrade installed packages", Privilege: system},
			{Name: "is-installed", Description: "Check whether every package is installed", Privilege: user},
			{
				Name:        "install",
				Description: "Install packages using Eopkg",
				Usage:       "Install one or more packages with dependency resolution (eopkg install --yes-all)",
				Privilege:   system,
			},
			{
				Name:        "remove",
				Description: "Remove packages using Eopkg",
				Usage:       "Remove one or more packages from the system (eopkg remove --yes-all)",
				Privilege:   system,
			},
			{
				Name:        "update",
				Description: "Update package repositories",
				Usage:       "Refresh all repository indexes (eopkg update-repo)",
				Privilege:   system,
			},
			{
				Name:        "search",
				Description: "Search for packages",
				Usage:       "Search for packages by name or description",
				Privilege:   user,
			},
			{
				Name:        "list",
				Description: "List packages",
				Usage:       "List installed packages (eopkg list-installed)",
				Privilege:   user,
			},
		},
	}

	return &EopkgPlugin{
		PackageManagerPlugin: sdk.NewPackageManagerPlugin(info, "eopkg"),
	}
}

// Execute handles command execution
func (p *EopkgPlugin) Execute(command string, args []string) error {
	return p.ExecuteContext(context.Background(), command, args)
}

// ExecuteContext runs a native operation with cancellation and per-command scope.
func (p *EopkgPlugin) ExecuteContext(ctx context.Context, command string, args []string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	switch command {
	case "upgrade":
		return p.run(ctx, "upgrade", append([]string{"upgrade", "--yes-all"}, args...)...)
	case "is-installed":
		return p.handleIsInstalled(ctx, args)
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

// Native commands follow eopkg(1): `install`, `remove` and `upgrade` take
// `--yes-all` (-y, "assume yes in all yes/no queries") so they never prompt;
// `update-repo` refreshes repository indexes; `search` and `list-installed`
// are read-only and run without sudo.
func (p *EopkgPlugin) handleInstall(ctx context.Context, args []string) error {
	if err := validatePackages(args); err != nil {
		return err
	}

	fmt.Printf("Installing packages: %s\n", strings.Join(args, ", "))

	return p.run(ctx, "install", append([]string{"install", "--yes-all"}, args...)...)
}

func (p *EopkgPlugin) handleRemove(ctx context.Context, args []string) error {
	if err := validatePackages(args); err != nil {
		return err
	}

	fmt.Printf("Removing packages: %s\n", strings.Join(args, ", "))

	return p.run(ctx, "remove", append([]string{"remove", "--yes-all"}, args...)...)
}

func (p *EopkgPlugin) handleUpdate(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("update takes no arguments; it refreshes every repository index")
	}
	fmt.Println("Updating package repositories...")
	return p.run(ctx, "update", "update-repo")
}

func (p *EopkgPlugin) handleSearch(ctx context.Context, args []string) error {
	searchTerm, err := searchPhrase(args)
	if err != nil {
		return err
	}
	fmt.Printf("Searching for: %s\n", searchTerm)

	return p.run(ctx, "search", "search", searchTerm)
}

func (p *EopkgPlugin) handleList(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("list takes no arguments")
	}
	return p.run(ctx, "list", "list-installed")
}

// run executes eopkg with the privilege advertised for operation.
func (p *EopkgPlugin) run(ctx context.Context, operation string, args ...string) error {
	privilege := sdk.PluginOperationPrivilege(p.Info(), []string{operation})
	if privilege == nil {
		return fmt.Errorf("operation %s has no declared privilege", operation)
	}
	if !p.IsAvailable() {
		return fmt.Errorf("eopkg is not available on this system; this plugin requires Solus")
	}
	err := sdk.ExecOperationWithContext(ctx, *privilege, "eopkg", args...)
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
	plugin := NewEopkgPlugin()
	sdk.HandleArgs(plugin, os.Args[1:])
}
