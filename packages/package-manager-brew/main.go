package main

import (
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

	switch command {
	case "install":
		return p.handleInstall(args)
	case "remove":
		return p.handleRemove(args)
	case "update":
		return p.handleUpdate(args)
	case "search":
		return p.handleSearch(args)
	case "list":
		return p.handleList(args)
	default:
		return fmt.Errorf("unknown command: %s", command)
	}
}

func (p *BrewPlugin) handleInstall(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no packages specified")
	}

	fmt.Printf("Installing packages: %s\n", strings.Join(args, ", "))

	// Install packages using the package manager
	cmdArgs := append([]string{"install"}, args...)
	return sdk.ExecCommand(true, "brew", cmdArgs...)
}

func (p *BrewPlugin) handleRemove(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no packages specified")
	}

	fmt.Printf("Removing packages: %s\n", strings.Join(args, ", "))

	cmdArgs := append([]string{"remove"}, args...)
	return sdk.ExecCommand(true, "brew", cmdArgs...)
}

func (p *BrewPlugin) handleUpdate(args []string) error {
	fmt.Println("Updating package repositories...")
	return sdk.ExecCommand(true, "brew", "update")
}

func (p *BrewPlugin) handleSearch(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no search term specified")
	}

	searchTerm := strings.Join(args, " ")
	fmt.Printf("Searching for: %s\n", searchTerm)

	return sdk.ExecCommand(false, "brew", "search", searchTerm)
}

func (p *BrewPlugin) handleList(args []string) error {
	return sdk.ExecCommand(false, "brew", "list")
}

func main() {
	plugin := NewBrewPlugin()
	sdk.HandleArgs(plugin, os.Args[1:])
}
