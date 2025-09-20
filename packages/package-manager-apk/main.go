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

// APKPlugin implements the APK package manager
type APKPlugin struct {
	*sdk.PackageManagerPlugin
}

// NewAPKPlugin creates a new APK plugin
func NewAPKPlugin() *APKPlugin {
	info := sdk.PluginInfo{
		Name:        "package-manager-apk",
		Version:     version,
		Description: "APK package manager support for Alpine Linux",
		Author:      "DevEx Team",
		Repository:  "https://github.com/jameswlane/devex",
		Tags:        []string{"package-manager", "apk", "alpine", "linux"},
		Commands: []sdk.PluginCommand{
			{
				Name:        "install",
				Description: "Install packages using APK",
				Usage:       "Install one or more packages with dependency resolution",
				Flags: map[string]string{
					"no-cache": "Do not use cached packages",
					"update":   "Update repositories before installing",
					"virtual":  "Create a virtual package",
				},
			},
			{
				Name:        "remove",
				Description: "Remove packages using APK",
				Usage:       "Remove one or more packages from the system",
				Flags: map[string]string{
					"purge": "Remove configuration files as well",
				},
			},
			{
				Name:        "update",
				Description: "Update package repositories",
				Usage:       "Download package information from all configured repositories",
			},
			{
				Name:        "upgrade",
				Description: "Upgrade installed packages",
				Usage:       "Install newer versions of all installed packages",
				Flags: map[string]string{
					"available": "Reinstall packages that are available in repositories",
				},
			},
			{
				Name:        "search",
				Description: "Search for packages",
				Usage:       "Search for packages by name or description",
			},
			{
				Name:        "list",
				Description: "List packages",
				Usage:       "List installed packages or available packages",
				Flags: map[string]string{
					"installed":  "List only installed packages",
					"available":  "List available packages",
					"upgradable": "List upgradable packages",
				},
			},
			{
				Name:        "info",
				Description: "Show package information",
				Usage:       "Display detailed information about a package",
			},
		},
	}

	return &APKPlugin{
		PackageManagerPlugin: sdk.NewPackageManagerPlugin(info, "apk"),
	}
}

// Execute handles command execution
func (p *APKPlugin) Execute(command string, args []string) error {
	ctx := context.Background()

	switch command {
	case "install":
		return p.handleInstall(ctx, args)
	case "remove":
		return p.handleRemove(ctx, args)
	case "update":
		return p.handleUpdate(ctx, args)
	case "upgrade":
		return p.handleUpgrade(ctx, args)
	case "search":
		return p.handleSearch(ctx, args)
	case "list":
		return p.handleList(ctx, args)
	case "info":
		return p.handleInfo(ctx, args)
	default:
		return fmt.Errorf("unknown command: %s", command)
	}
}

func (p *APKPlugin) handleInstall(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no packages specified")
	}

	fmt.Printf("Installing packages: %s\n", strings.Join(args, ", "))

	// Update repositories first
	fmt.Println("Updating package repositories...")
	if err := sdk.ExecCommandWithContext(ctx, true, "apk", "update"); err != nil {
		fmt.Printf("Warning: failed to update repositories: %v\n", err)
	}

	// Install packages
	cmdArgs := append([]string{"add", "--no-cache"}, args...)
	return sdk.ExecCommandWithContext(ctx, true, "apk", cmdArgs...)
}

func (p *APKPlugin) handleRemove(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no packages specified")
	}

	fmt.Printf("Removing packages: %s\n", strings.Join(args, ", "))

	cmdArgs := append([]string{"del"}, args...)
	return sdk.ExecCommandWithContext(ctx, true, "apk", cmdArgs...)
}

func (p *APKPlugin) handleUpdate(ctx context.Context, args []string) error {
	fmt.Println("Updating package repositories...")
	return sdk.ExecCommandWithContext(ctx, true, "apk", "update")
}

func (p *APKPlugin) handleUpgrade(ctx context.Context, args []string) error {
	fmt.Println("Upgrading installed packages...")

	// Update first
	if err := sdk.ExecCommandWithContext(ctx, true, "apk", "update"); err != nil {
		return fmt.Errorf("failed to update repositories: %w", err)
	}

	// Then upgrade
	return sdk.ExecCommandWithContext(ctx, true, "apk", "upgrade")
}

func (p *APKPlugin) handleSearch(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no search term specified")
	}

	searchTerm := strings.Join(args, " ")
	fmt.Printf("Searching for: %s\n", searchTerm)

	return sdk.ExecCommandWithContext(ctx, false, "apk", "search", searchTerm)
}

func (p *APKPlugin) handleList(ctx context.Context, args []string) error {
	if len(args) == 0 {
		// List all installed packages
		return sdk.ExecCommandWithContext(ctx, false, "apk", "list", "--installed")
	}

	// Handle specific arguments
	cmdArgs := append([]string{"list"}, args...)
	return sdk.ExecCommandWithContext(ctx, false, "apk", cmdArgs...)
}

func (p *APKPlugin) handleInfo(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no package specified")
	}

	for _, pkg := range args {
		fmt.Printf("Package information for: %s\n", pkg)
		if err := sdk.ExecCommandWithContext(ctx, false, "apk", "info", pkg); err != nil {
			fmt.Printf("Failed to get info for %s: %v\n", pkg, err)
		}
		if len(args) > 1 {
			fmt.Println("---")
		}
	}

	return nil
}

func main() {
	plugin := NewAPKPlugin()
	sdk.HandleArgs(plugin, os.Args[1:])
}
