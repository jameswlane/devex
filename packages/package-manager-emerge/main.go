package main

// Build timestamp: 2025-09-03 17:41:19

import (
	"fmt"
	"os"
	"strings"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
)

var version = "dev" // Set by goreleaser

// EmergePlugin implements the Emerge package manager
type EmergePlugin struct {
	*sdk.PackageManagerPlugin
}

// NewEmergePlugin creates a new Emerge plugin
func NewEmergePlugin() *EmergePlugin {
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
				Usage:       "Install one or more packages with dependency resolution",
			},
			{
				Name:        "remove",
				Description: "Remove packages using Emerge",
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

	return &EmergePlugin{
		PackageManagerPlugin: sdk.NewPackageManagerPlugin(info, "emerge"),
	}
}

// Execute handles command execution
func (p *EmergePlugin) Execute(command string, args []string) error {
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

func (p *EmergePlugin) handleInstall(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no packages specified")
	}

	fmt.Printf("Installing packages: %s\n", strings.Join(args, ", "))

	// Install packages using the package manager
	cmdArgs := append([]string{"install"}, args...)
	return sdk.ExecCommand(true, "emerge", cmdArgs...)
}

func (p *EmergePlugin) handleRemove(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no packages specified")
	}

	fmt.Printf("Removing packages: %s\n", strings.Join(args, ", "))

	cmdArgs := append([]string{"remove"}, args...)
	return sdk.ExecCommand(true, "emerge", cmdArgs...)
}

func (p *EmergePlugin) handleUpdate(args []string) error {
	fmt.Println("Updating package repositories...")
	return sdk.ExecCommand(true, "emerge", "update")
}

func (p *EmergePlugin) handleSearch(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no search term specified")
	}

	searchTerm := strings.Join(args, " ")
	fmt.Printf("Searching for: %s\n", searchTerm)

	return sdk.ExecCommand(false, "emerge", "search", searchTerm)
}

func (p *EmergePlugin) handleList(args []string) error {
	return sdk.ExecCommand(false, "emerge", "list")
}

func main() {
	plugin := NewEmergePlugin()
	sdk.HandleArgs(plugin, os.Args[1:])
}
