package main

import (
	"fmt"
	"os"
	"strings"

	sdk "github.com/jameswlane/devex/packages/shared/plugin-sdk"
)

var version = "dev" // Set by goreleaser

// PipPlugin implements the Pip package manager
type PipPlugin struct {
	*sdk.PackageManagerPlugin
}

// NewPipPlugin creates a new Pip plugin
func NewPipPlugin() *PipPlugin {
	info := sdk.PluginInfo{
		Name:        "package-manager-pip",
		Version:     version,
		Description: "Python package installer",
		Author:      "DevEx Team",
		Repository:  "https://github.com/jameswlane/devex",
		Tags:        []string{"pip", "python", "packages"},
		Commands: []sdk.PluginCommand{
			{
				Name:        "install",
				Description: "Install packages using Pip",
				Usage:       "Install one or more packages with dependency resolution",
			},
			{
				Name:        "remove",
				Description: "Remove packages using Pip",
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

	return &PipPlugin{
		PackageManagerPlugin: sdk.NewPackageManagerPlugin(info, "pip"),
	}
}

// Execute handles command execution
func (p *PipPlugin) Execute(command string, args []string) error {
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

func (p *PipPlugin) handleInstall(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no packages specified")
	}

	fmt.Printf("Installing packages: %s\n", strings.Join(args, ", "))

	// Install packages using the package manager
	cmdArgs := append([]string{"install"}, args...)
	return sdk.ExecCommand(true, "pip", cmdArgs...)
}

func (p *PipPlugin) handleRemove(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no packages specified")
	}

	fmt.Printf("Removing packages: %s\n", strings.Join(args, ", "))

	cmdArgs := append([]string{"remove"}, args...)
	return sdk.ExecCommand(true, "pip", cmdArgs...)
}

func (p *PipPlugin) handleUpdate(args []string) error {
	fmt.Println("Updating package repositories...")
	return sdk.ExecCommand(true, "pip", "update")
}

func (p *PipPlugin) handleSearch(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no search term specified")
	}

	searchTerm := strings.Join(args, " ")
	fmt.Printf("Searching for: %s\n", searchTerm)

	return sdk.ExecCommand(false, "pip", "search", searchTerm)
}

func (p *PipPlugin) handleList(args []string) error {
	return sdk.ExecCommand(false, "pip", "list")
}

func main() {
	plugin := NewPipPlugin()
	sdk.HandleArgs(plugin, os.Args[1:])
}
