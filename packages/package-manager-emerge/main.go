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

// errListUnsupported explains why list never runs a native command: emerge has
// no installed-package listing, and the alternatives need extra packages.
const errListUnsupported = "emerge cannot list installed packages; install app-portage/portage-utils and run 'qlist -I', or app-portage/gentoolkit and run \"equery list '*'\""

// EmergePlugin implements the Emerge package manager
type EmergePlugin struct {
	*sdk.PackageManagerPlugin
}

// NewEmergePlugin creates a new Emerge plugin
func NewEmergePlugin() *EmergePlugin {
	system := &sdk.OperationPrivilege{Scope: sdk.ScopeSystem}
	user := &sdk.OperationPrivilege{Scope: sdk.ScopeUser}
	info := sdk.PluginInfo{
		Name:        "package-manager-emerge",
		Version:     version,
		Description: "Portage package manager for Gentoo",
		Author:      "DevEx Team",
		Repository:  "https://github.com/jameswlane/devex",
		Tags:        []string{"emerge", "portage", "gentoo", "linux"},
		Commands: []sdk.PluginCommand{
			{
				Name:        "install",
				Description: "Install packages using Emerge",
				Usage:       "Install one or more packages with dependency resolution (emerge --ask=n)",
				Privilege:   system,
			},
			{
				Name:        "remove",
				Description: "Remove packages using Emerge",
				Usage:       "Remove packages that nothing else depends on (emerge --ask=n --depclean)",
				Privilege:   system,
			},
			{
				Name:        "update",
				Description: "Update package repositories",
				Usage:       "Synchronize Portage repositories (emerge --ask=n --sync)",
				Privilege:   system,
			},
			{
				Name:        "search",
				Description: "Search for packages",
				Usage:       "Search for packages by name or description (emerge --searchdesc)",
				Privilege:   user,
			},
			{
				Name:        "list",
				Description: "List packages (unsupported)",
				Usage:       "Unsupported: emerge has no installed-package query; use qlist -I or equery list",
				Privilege:   user,
			},
		},
	}

	return &EmergePlugin{
		PackageManagerPlugin: sdk.NewPackageManagerPlugin(info, "emerge"),
	}
}

// Execute handles command execution
func (p *EmergePlugin) Execute(command string, args []string) error {
	return p.ExecuteContext(context.Background(), command, args)
}

// ExecuteContext runs a native operation with cancellation and per-command scope.
func (p *EmergePlugin) ExecuteContext(ctx context.Context, command string, args []string) error {
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
		return p.handleList(args)
	default:
		return fmt.Errorf("unknown command: %s", command)
	}
}

func (p *EmergePlugin) handleInstall(ctx context.Context, args []string) error {
	if err := validatePackages(args); err != nil {
		return err
	}

	fmt.Printf("Installing packages: %s\n", strings.Join(args, ", "))

	return p.run(ctx, "install", append([]string{"--ask=n"}, args...)...)
}

func (p *EmergePlugin) handleRemove(ctx context.Context, args []string) error {
	if err := validatePackages(args); err != nil {
		return err
	}

	fmt.Printf("Removing packages: %s\n", strings.Join(args, ", "))

	return p.run(ctx, "remove", append([]string{"--ask=n", "--depclean"}, args...)...)
}

func (p *EmergePlugin) handleUpdate(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("update takes no arguments; it synchronizes all Portage repositories")
	}
	fmt.Println("Updating package repositories...")
	return p.run(ctx, "update", "--ask=n", "--sync")
}

func (p *EmergePlugin) handleSearch(ctx context.Context, args []string) error {
	searchTerm, err := searchPhrase(args)
	if err != nil {
		return err
	}
	fmt.Printf("Searching for: %s\n", searchTerm)

	return p.run(ctx, "search", "--searchdesc", searchTerm)
}

func (p *EmergePlugin) handleList(args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("list takes no arguments")
	}
	return fmt.Errorf("%s", errListUnsupported)
}

// run executes emerge with the privilege advertised for operation.
func (p *EmergePlugin) run(ctx context.Context, operation string, args ...string) error {
	privilege := sdk.PluginOperationPrivilege(p.Info(), []string{operation})
	if privilege == nil {
		return fmt.Errorf("operation %s has no declared privilege", operation)
	}
	if !p.IsAvailable() {
		return fmt.Errorf("emerge is not available on this system; install sys-apps/portage")
	}
	err := sdk.ExecOperationWithContext(ctx, *privilege, "emerge", args...)
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
	plugin := NewEmergePlugin()
	sdk.HandleArgs(plugin, os.Args[1:])
}
