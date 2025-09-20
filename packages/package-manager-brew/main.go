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

// BrewPlugin implements the Homebrew package manager
type BrewPlugin struct {
	*sdk.PackageManagerPlugin
}

// NewBrewPlugin creates a new Homebrew plugin
func NewBrewPlugin() *BrewPlugin {
	info := sdk.PluginInfo{
		Name:        "package-manager-brew",
		Version:     version,
		Description: "Homebrew package manager for macOS and Linux",
		Author:      "DevEx Team",
		Repository:  "https://github.com/jameswlane/devex",
		Tags:        []string{"brew", "homebrew", "macos", "linux"},
		Commands: []sdk.PluginCommand{
			{
				Name:        "install",
				Description: "Install packages using Homebrew",
				Usage:       "Install one or more packages with dependency resolution",
			},
			{
				Name:        "remove",
				Description: "Remove packages using Homebrew",
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

	return &BrewPlugin{
		PackageManagerPlugin: sdk.NewPackageManagerPlugin(info, "brew"),
	}
}

// Execute handles command execution
func (p *BrewPlugin) Execute(command string, args []string) error {
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

func (p *BrewPlugin) handleInstall(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no packages specified")
	}

	fmt.Printf("Installing packages: %s\n", strings.Join(args, ", "))

	// Install packages using the package manager
	cmdArgs := append([]string{"install"}, args...)
	return sdk.ExecCommandWithContext(ctx, true, "brew", cmdArgs...)
}

func (p *BrewPlugin) handleRemove(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no packages specified")
	}

	fmt.Printf("Removing packages: %s\n", strings.Join(args, ", "))

	cmdArgs := append([]string{"remove"}, args...)
	return sdk.ExecCommandWithContext(ctx, true, "brew", cmdArgs...)
}

func (p *BrewPlugin) handleUpdate(ctx context.Context, args []string) error {
	fmt.Println("Updating package repositories...")
	return sdk.ExecCommandWithContext(ctx, true, "brew", "update")
}

func (p *BrewPlugin) handleSearch(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no search term specified")
	}

	searchTerm := strings.Join(args, " ")
	fmt.Printf("Searching for: %s\n", searchTerm)

	return sdk.ExecCommandWithContext(ctx, false, "brew", "search", searchTerm)
}

func (p *BrewPlugin) handleList(ctx context.Context, args []string) error {
	return sdk.ExecCommandWithContext(ctx, false, "brew", "list")
}

func main() {
	plugin := NewBrewPlugin()
	sdk.HandleArgs(plugin, os.Args[1:])
}
