package main

import (
	"fmt"
	"os"
	"strings"

	sdk "github.com/jameswlane/devex/packages/shared/plugin-sdk"
)

var version = "dev" // Set by goreleaser

// XbpsPlugin implements the XBPS package manager
type XbpsPlugin struct {
	*sdk.PackageManagerPlugin
}

// NewXbpsPlugin creates a new XBPS plugin
func NewXbpsPlugin() *XbpsPlugin {
	info := sdk.PluginInfo{
		Name:        "package-manager-xbps",
		Version:     version,
		Description: "XBPS package manager for Void Linux",
		Author:      "DevEx Team",
		Repository:  "https://github.com/jameswlane/devex",
		Tags:        []string{"xbps", "void", "linux"},
		Commands: []sdk.PluginCommand{
			{
				Name:        "install",
				Description: "Install packages using XBPS",
				Usage:       "Install one or more packages with dependency resolution",
			},
			{
				Name:        "remove",
				Description: "Remove packages using XBPS",
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

	return &XbpsPlugin{
		PackageManagerPlugin: sdk.NewPackageManagerPlugin(info, "xbps-install"),
	}
}

// Execute handles command execution
func (p *XbpsPlugin) Execute(command string, args []string) error {
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

func (p *XbpsPlugin) handleInstall(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no packages specified")
	}

	fmt.Printf("Installing packages: %s\n", strings.Join(args, ", "))

	// Install packages using the package manager
	cmdArgs := append([]string{"install"}, args...)
	return sdk.ExecCommand(true, "xbps-install", cmdArgs...)
}

func (p *XbpsPlugin) handleRemove(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no packages specified")
	}

	fmt.Printf("Removing packages: %s\n", strings.Join(args, ", "))

	cmdArgs := append([]string{"remove"}, args...)
	return sdk.ExecCommand(true, "xbps-install", cmdArgs...)
}

func (p *XbpsPlugin) handleUpdate(args []string) error {
	fmt.Println("Updating package repositories...")
	return sdk.ExecCommand(true, "xbps-install", "update")
}

func (p *XbpsPlugin) handleSearch(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no search term specified")
	}

	searchTerm := strings.Join(args, " ")
	fmt.Printf("Searching for: %s\n", searchTerm)

	return sdk.ExecCommand(false, "xbps-install", "search", searchTerm)
}

func (p *XbpsPlugin) handleList(args []string) error {
	return sdk.ExecCommand(false, "xbps-install", "list")
}

func main() {
	plugin := NewXbpsPlugin()
	sdk.HandleArgs(plugin, os.Args[1:])
}
