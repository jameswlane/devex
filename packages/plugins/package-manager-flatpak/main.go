package main

import (
	"fmt"
	"os"
	"strings"

	sdk "github.com/jameswlane/devex/packages/shared/plugin-sdk"
)

var version = "dev" // Set by goreleaser

// FlatpakPlugin implements the Flatpak package manager
type FlatpakPlugin struct {
	*sdk.PackageManagerPlugin
}

// NewFlatpakPlugin creates a new Flatpak plugin
func NewFlatpakPlugin() *FlatpakPlugin {
	info := sdk.PluginInfo{
		Name:        "package-manager-flatpak",
		Version:     version,
		Description: "Flatpak universal package manager",
		Author:      "DevEx Team",
		Repository:  "https://github.com/jameswlane/devex",
		Tags:        []string{"flatpak", "universal", "linux"},
		Commands: []sdk.PluginCommand{
			{
				Name:        "install",
				Description: "Install packages using Flatpak",
				Usage:       "Install one or more packages with dependency resolution",
			},
			{
				Name:        "remove",
				Description: "Remove packages using Flatpak",
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

	return &FlatpakPlugin{
		PackageManagerPlugin: sdk.NewPackageManagerPlugin(info, "flatpak"),
	}
}

// Execute handles command execution
func (p *FlatpakPlugin) Execute(command string, args []string) error {
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

func (p *FlatpakPlugin) handleInstall(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no packages specified")
	}

	fmt.Printf("Installing packages: %s\n", strings.Join(args, ", "))

	// Install packages using the package manager
	cmdArgs := append([]string{"install"}, args...)
	return sdk.ExecCommand(true, "flatpak", cmdArgs...)
}

func (p *FlatpakPlugin) handleRemove(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no packages specified")
	}

	fmt.Printf("Removing packages: %s\n", strings.Join(args, ", "))

	cmdArgs := append([]string{"remove"}, args...)
	return sdk.ExecCommand(true, "flatpak", cmdArgs...)
}

func (p *FlatpakPlugin) handleUpdate(args []string) error {
	fmt.Println("Updating package repositories...")
	return sdk.ExecCommand(true, "flatpak", "update")
}

func (p *FlatpakPlugin) handleSearch(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no search term specified")
	}

	searchTerm := strings.Join(args, " ")
	fmt.Printf("Searching for: %s\n", searchTerm)

	return sdk.ExecCommand(false, "flatpak", "search", searchTerm)
}

func (p *FlatpakPlugin) handleList(args []string) error {
	return sdk.ExecCommand(false, "flatpak", "list")
}

func main() {
	plugin := NewFlatpakPlugin()
	sdk.HandleArgs(plugin, os.Args[1:])
}
