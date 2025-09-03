package main

// Build timestamp: 2025-09-03 17:41:19

import (
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
	info := sdk.PluginInfo{
		Name:        "package-manager-eopkg",
		Version:     version,
		Description: "Eopkg package manager for Solus",
		Author:      "DevEx Team",
		Repository:  "https://github.com/jameswlane/devex",
		Tags:        []string{"eopkg", "solus", "linux"},
		Commands: []sdk.PluginCommand{
			{
				Name:        "install",
				Description: "Install packages using Eopkg",
				Usage:       "Install one or more packages with dependency resolution",
			},
			{
				Name:        "remove",
				Description: "Remove packages using Eopkg",
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

	return &EopkgPlugin{
		PackageManagerPlugin: sdk.NewPackageManagerPlugin(info, "eopkg"),
	}
}

// Execute handles command execution
func (p *EopkgPlugin) Execute(command string, args []string) error {
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

func (p *EopkgPlugin) handleInstall(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no packages specified")
	}

	fmt.Printf("Installing packages: %s\n", strings.Join(args, ", "))

	// Install packages using the package manager
	cmdArgs := append([]string{"install"}, args...)
	return sdk.ExecCommand(true, "eopkg", cmdArgs...)
}

func (p *EopkgPlugin) handleRemove(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no packages specified")
	}

	fmt.Printf("Removing packages: %s\n", strings.Join(args, ", "))

	cmdArgs := append([]string{"remove"}, args...)
	return sdk.ExecCommand(true, "eopkg", cmdArgs...)
}

func (p *EopkgPlugin) handleUpdate(args []string) error {
	fmt.Println("Updating package repositories...")
	return sdk.ExecCommand(true, "eopkg", "update")
}

func (p *EopkgPlugin) handleSearch(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no search term specified")
	}

	searchTerm := strings.Join(args, " ")
	fmt.Printf("Searching for: %s\n", searchTerm)

	return sdk.ExecCommand(false, "eopkg", "search", searchTerm)
}

func (p *EopkgPlugin) handleList(args []string) error {
	return sdk.ExecCommand(false, "eopkg", "list")
}

func main() {
	plugin := NewEopkgPlugin()
	sdk.HandleArgs(plugin, os.Args[1:])
}
