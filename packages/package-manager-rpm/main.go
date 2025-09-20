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

// RpmPlugin implements the RPM package manager
type RpmPlugin struct {
	*sdk.PackageManagerPlugin
}

// NewRpmPlugin creates a new RPM plugin
func NewRpmPlugin() *RpmPlugin {
	info := sdk.PluginInfo{
		Name:        "package-manager-rpm",
		Version:     version,
		Description: "RPM package manager",
		Author:      "DevEx Team",
		Repository:  "https://github.com/jameswlane/devex",
		Tags:        []string{"rpm", "redhat", "linux"},
		Commands: []sdk.PluginCommand{
			{
				Name:        "install",
				Description: "Install packages using RPM",
				Usage:       "Install one or more packages with dependency resolution",
			},
			{
				Name:        "remove",
				Description: "Remove packages using RPM",
				Usage:       "Remove one or more packages from the system",
			},
			{
				Name:        "update",
				Description: "Update package repositories",
				Usage:       "Update package repository information",
			},
			{
				Name:        "search",
				Description: "Search for packages",
				Usage:       "Search for packages by name or description",
			},
			{
				Name:        "list",
				Description: "List packages",
				Usage:       "List installed packages",
			},
		},
	}

	return &RpmPlugin{
		PackageManagerPlugin: sdk.NewPackageManagerPlugin(info, "rpm"),
	}
}

// Execute handles command execution
func (p *RpmPlugin) Execute(command string, args []string) error {
	p.EnsureAvailable()

	ctx := context.Background()

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

func (p *RpmPlugin) handleInstall(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no packages specified")
	}

	fmt.Printf("Installing packages: %s\n", strings.Join(args, ", "))

	// Install packages using the package manager
	cmdArgs := append([]string{"install"}, args...)
	return sdk.ExecCommandWithContext(ctx, true, "rpm", cmdArgs...)
}

func (p *RpmPlugin) handleRemove(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no packages specified")
	}

	fmt.Printf("Removing packages: %s\n", strings.Join(args, ", "))

	cmdArgs := append([]string{"remove"}, args...)
	return sdk.ExecCommandWithContext(ctx, true, "rpm", cmdArgs...)
}

func (p *RpmPlugin) handleUpdate(ctx context.Context, args []string) error {
	fmt.Println("Updating package repositories...")
	return sdk.ExecCommandWithContext(ctx, true, "rpm", "update")
}

func (p *RpmPlugin) handleSearch(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no search term specified")
	}

	searchTerm := strings.Join(args, " ")
	fmt.Printf("Searching for: %s\n", searchTerm)

	return sdk.ExecCommandWithContext(ctx, false, "rpm", "search", searchTerm)
}

func (p *RpmPlugin) handleList(ctx context.Context, args []string) error {
	return sdk.ExecCommandWithContext(ctx, false, "rpm", "list")
}

func main() {
	plugin := NewRpmPlugin()
	sdk.HandleArgs(plugin, os.Args[1:])
}
