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

// ZypperPlugin implements the Zypper package manager
type ZypperPlugin struct {
	*sdk.PackageManagerPlugin
}

// NewZypperPlugin creates a new Zypper plugin
func NewZypperPlugin() *ZypperPlugin {
	system := &sdk.OperationPrivilege{Scope: sdk.ScopeSystem}
	user := &sdk.OperationPrivilege{Scope: sdk.ScopeUser}
	info := sdk.PluginInfo{
		Name:        "package-manager-zypper",
		Version:     version,
		Description: "Zypper package manager for openSUSE",
		Author:      "DevEx Team",
		Repository:  "https://github.com/jameswlane/devex",
		Tags:        []string{"zypper", "opensuse", "suse", "linux"},
		Commands: []sdk.PluginCommand{
			{
				Name:        "upgrade",
				Description: "Upgrade installed packages",
				Usage:       "Upgrade all or the named installed packages (zypper --non-interactive update)",
				Privilege:   system,
			},
			{
				Name:        "install",
				Description: "Install packages using Zypper",
				Usage:       "Install one or more packages with dependency resolution (zypper --non-interactive install)",
				Privilege:   system,
			},
			{
				Name:        "remove",
				Description: "Remove packages using Zypper",
				Usage:       "Remove one or more packages from the system (zypper --non-interactive remove)",
				Privilege:   system,
			},
			{
				Name:        "update",
				Description: "Update package repositories",
				Usage:       "Refresh repository metadata without upgrading packages (zypper --non-interactive refresh)",
				Privilege:   system,
			},
			{
				Name:        "search",
				Description: "Search for packages",
				Usage:       "Search for packages by name or description (zypper search --search-descriptions)",
				Privilege:   user,
			},
			{
				Name:        "list",
				Description: "List packages",
				Usage:       "List installed packages (zypper search --installed-only)",
				Privilege:   user,
			},
		},
	}

	return &ZypperPlugin{
		PackageManagerPlugin: sdk.NewPackageManagerPlugin(info, "zypper"),
	}
}

// Execute handles command execution
func (p *ZypperPlugin) Execute(command string, args []string) error {
	return p.ExecuteContext(context.Background(), command, args)
}

// ExecuteContext runs a native operation with cancellation and per-command scope.
func (p *ZypperPlugin) ExecuteContext(ctx context.Context, command string, args []string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	switch command {
	case "install":
		return p.handleInstall(ctx, args)
	case "remove":
		return p.handleRemove(ctx, args)
	case "upgrade":
		return p.handleUpgrade(ctx, args)
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

func (p *ZypperPlugin) handleInstall(ctx context.Context, args []string) error {
	if err := validatePackages(args); err != nil {
		return err
	}

	fmt.Printf("Installing packages: %s\n", strings.Join(args, ", "))

	return p.run(ctx, "install", append([]string{"--non-interactive", "install"}, args...)...)
}

func (p *ZypperPlugin) handleRemove(ctx context.Context, args []string) error {
	if err := validatePackages(args); err != nil {
		return err
	}

	fmt.Printf("Removing packages: %s\n", strings.Join(args, ", "))

	return p.run(ctx, "remove", append([]string{"--non-interactive", "remove"}, args...)...)
}

// handleUpgrade upgrades every installed package, or only the named packages.
func (p *ZypperPlugin) handleUpgrade(ctx context.Context, args []string) error {
	if len(args) != 0 {
		if err := validatePackages(args); err != nil {
			return err
		}
	}
	return p.run(ctx, "upgrade", append([]string{"--non-interactive", "update"}, args...)...)
}

func (p *ZypperPlugin) handleUpdate(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("update takes no arguments; it refreshes every repository, use upgrade to update packages")
	}
	fmt.Println("Updating package repositories...")
	return p.run(ctx, "update", "--non-interactive", "refresh")
}

func (p *ZypperPlugin) handleSearch(ctx context.Context, args []string) error {
	searchTerm, err := searchPhrase(args)
	if err != nil {
		return err
	}
	fmt.Printf("Searching for: %s\n", searchTerm)

	return p.run(ctx, "search", "search", "--search-descriptions", searchTerm)
}

func (p *ZypperPlugin) handleList(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("list takes no arguments")
	}
	return p.run(ctx, "list", "search", "--installed-only")
}

// run executes zypper with the privilege advertised for operation.
func (p *ZypperPlugin) run(ctx context.Context, operation string, args ...string) error {
	privilege := sdk.PluginOperationPrivilege(p.Info(), []string{operation})
	if privilege == nil {
		return fmt.Errorf("operation %s has no declared privilege", operation)
	}
	if !p.IsAvailable() {
		return fmt.Errorf("zypper is not available on this system; this plugin requires openSUSE or SUSE Linux")
	}
	err := sdk.ExecOperationWithContext(ctx, *privilege, "zypper", args...)
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
	plugin := NewZypperPlugin()
	sdk.HandleArgs(plugin, os.Args[1:])
}
