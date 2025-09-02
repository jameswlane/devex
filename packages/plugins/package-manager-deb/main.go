package main

import (
	"fmt"
	"os"
	"strings"

	sdk "github.com/jameswlane/devex/packages/shared/plugin-sdk"
)

var version = "dev" // Set by goreleaser

// DebPlugin implements the DEB package manager
type DebPlugin struct {
	*sdk.PackageManagerPlugin
}

// NewDebPlugin creates a new DEB plugin
func NewDebPlugin() *DebPlugin {
	info := sdk.PluginInfo{
		Name:        "package-manager-deb",
		Version:     version,
		Description: "Debian package installer",
		Author:      "DevEx Team",
		Repository:  "https://github.com/jameswlane/devex",
		Tags:        []string{"deb", "debian", "package"},
		Commands: []sdk.PluginCommand{
			{
				Name:        "install",
				Description: "Install packages using DEB",
				Usage:       "Install one or more packages with dependency resolution",
			},
			{
				Name:        "remove",
				Description: "Remove packages using DEB",
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

	return &DebPlugin{
		PackageManagerPlugin: sdk.NewPackageManagerPlugin(info, "dpkg"),
	}
}

// Execute handles command execution
func (p *DebPlugin) Execute(command string, args []string) error {
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

func (p *DebPlugin) handleInstall(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no packages specified")
	}

	fmt.Printf("Installing packages: %s\n", strings.Join(args, ", "))

	// Install packages using the package manager
	cmdArgs := append([]string{"install"}, args...)
	return sdk.ExecCommand(true, "dpkg", cmdArgs...)
}

func (p *DebPlugin) handleRemove(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no packages specified")
	}

	fmt.Printf("Removing packages: %s\n", strings.Join(args, ", "))

	cmdArgs := append([]string{"remove"}, args...)
	return sdk.ExecCommand(true, "dpkg", cmdArgs...)
}

func (p *DebPlugin) handleUpdate(args []string) error {
	fmt.Println("Updating package repositories...")
	return sdk.ExecCommand(true, "dpkg", "update")
}

func (p *DebPlugin) handleSearch(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no search term specified")
	}

	searchTerm := strings.Join(args, " ")
	fmt.Printf("Searching for: %s\n", searchTerm)

	return sdk.ExecCommand(false, "dpkg", "search", searchTerm)
}

func (p *DebPlugin) handleList(args []string) error {
	return sdk.ExecCommand(false, "dpkg", "list")
}

func main() {
	plugin := NewDebPlugin()
	sdk.HandleArgs(plugin, os.Args[1:])
}
