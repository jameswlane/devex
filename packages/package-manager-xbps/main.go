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

// XbpsPlugin implements the XBPS package manager
type XbpsPlugin struct {
	*sdk.PackageManagerPlugin
}

// NewXbpsPlugin creates a new XBPS plugin
func NewXbpsPlugin() *XbpsPlugin {
	system := &sdk.OperationPrivilege{Scope: sdk.ScopeSystem}
	user := &sdk.OperationPrivilege{Scope: sdk.ScopeUser}
	info := sdk.PluginInfo{
		Name:        "package-manager-xbps",
		Version:     version,
		Description: "XBPS package manager for Void Linux",
		Author:      "DevEx Team",
		Repository:  "https://github.com/jameswlane/devex",
		Tags:        []string{"xbps", "void", "linux"},
		Commands: []sdk.PluginCommand{
			{Name: "upgrade", Description: "Upgrade installed packages", Privilege: system},
			{Name: "is-installed", Description: "Check whether every package is installed", Privilege: user},
			{
				Name:        "install",
				Description: "Install packages using XBPS",
				Usage:       "Install one or more packages with dependency resolution (xbps-install --yes)",
				Privilege:   system,
			},
			{
				Name:        "remove",
				Description: "Remove packages using XBPS",
				Usage:       "Remove one or more packages from the system (xbps-remove --yes)",
				Privilege:   system,
			},
			{
				Name:        "update",
				Description: "Update package repositories",
				Usage:       "Synchronize remote repository index files (xbps-install --sync)",
				Privilege:   system,
			},
			{
				Name:        "search",
				Description: "Search for packages",
				Usage:       "Search repository packages by name or description (xbps-query -R --search)",
				Privilege:   user,
			},
			{
				Name:        "list",
				Description: "List packages",
				Usage:       "List installed packages (xbps-query --list-pkgs)",
				Privilege:   user,
			},
		},
	}

	return &XbpsPlugin{
		PackageManagerPlugin: sdk.NewPackageManagerPlugin(info, "xbps-install"),
	}
}

// Execute handles command execution
func (p *XbpsPlugin) Execute(command string, args []string) error {
	return p.ExecuteContext(context.Background(), command, args)
}

// ExecuteContext runs a native operation with cancellation and per-command scope.
func (p *XbpsPlugin) ExecuteContext(ctx context.Context, command string, args []string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	switch command {
	case "upgrade":
		return p.run(ctx, "upgrade", "xbps-install", append([]string{"--yes", "--update"}, args...)...)
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

// Native commands follow xbps-install(1), xbps-remove(1) and xbps-query(1):
// `--yes` (-y) answers every question, `--sync` (-S) refreshes repository
// indexes, and `--update` (-u) upgrades installed packages, so upgrade is the
// documented `xbps-install -yu`. Installing and removing use separate tools
// (xbps-install / xbps-remove). Queries (`xbps-query -R --search`, `--list-pkgs`)
// are read-only and run without sudo.
func (p *XbpsPlugin) handleInstall(ctx context.Context, args []string) error {
	if err := validatePackages(args); err != nil {
		return err
	}

	fmt.Printf("Installing packages: %s\n", strings.Join(args, ", "))

	return p.run(ctx, "install", "xbps-install", append([]string{"--yes"}, args...)...)
}

func (p *XbpsPlugin) handleRemove(ctx context.Context, args []string) error {
	if err := validatePackages(args); err != nil {
		return err
	}

	fmt.Printf("Removing packages: %s\n", strings.Join(args, ", "))

	return p.run(ctx, "remove", "xbps-remove", append([]string{"--yes"}, args...)...)
}

func (p *XbpsPlugin) handleUpdate(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("update takes no arguments; it synchronizes every repository index")
	}
	fmt.Println("Updating package repositories...")
	return p.run(ctx, "update", "xbps-install", "--sync")
}

func (p *XbpsPlugin) handleSearch(ctx context.Context, args []string) error {
	searchTerm, err := searchPhrase(args)
	if err != nil {
		return err
	}
	fmt.Printf("Searching for: %s\n", searchTerm)

	// -R enables repository mode; --repository would consume the next argument as a URL.
	return p.run(ctx, "search", "xbps-query", "-R", "--search", searchTerm)
}

func (p *XbpsPlugin) handleList(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("list takes no arguments")
	}
	return p.run(ctx, "list", "xbps-query", "--list-pkgs")
}

// run executes an XBPS tool with the privilege advertised for operation.
func (p *XbpsPlugin) run(ctx context.Context, operation, name string, args ...string) error {
	privilege := sdk.PluginOperationPrivilege(p.Info(), []string{operation})
	if privilege == nil {
		return fmt.Errorf("operation %s has no declared privilege", operation)
	}
	if !sdk.CommandExists(name) {
		return fmt.Errorf("%s is not available on this system; this plugin requires Void Linux XBPS", name)
	}
	err := sdk.ExecOperationWithContext(ctx, *privilege, name, args...)
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
	plugin := NewXbpsPlugin()
	sdk.HandleArgs(plugin, os.Args[1:])
}
