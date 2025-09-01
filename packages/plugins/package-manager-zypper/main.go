package main

import (
	"fmt"
	"os"
	"strings"

	sdk "github.com/jameswlane/devex/packages/shared/plugin-sdk"
)

var version = "dev" // Set by goreleaser

// ZypperPlugin implements the Zypper package manager
type ZypperPlugin struct {
	*sdk.PackageManagerPlugin
}

// NewZypperPlugin creates a new Zypper plugin
func NewZypperPlugin() *ZypperPlugin {
	info := sdk.PluginInfo{
		Name:        "package-manager-zypper",
		Version:     version,
		Description: "Zypper package manager for openSUSE",
		Author:      "DevEx Team",
		Repository:  "https://github.com/jameswlane/devex",
		Tags:        []string{"zypper", "opensuse", "suse", "linux"},
		Commands: []sdk.PluginCommand{
			{
				Name:        "install",
				Description: "Install packages using Zypper",
				Usage:       "Install one or more packages with dependency resolution",
			},
			{
				Name:        "remove",
				Description: "Remove packages using Zypper",
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

	return &ZypperPlugin{
		PackageManagerPlugin: sdk.NewPackageManagerPlugin(info, "zypper"),
	}
}

// Execute handles command execution
func (p *ZypperPlugin) Execute(command string, args []string) error {
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

func (p *ZypperPlugin) handleInstall(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no packages specified")
	}

	fmt.Printf("Installing packages: %s\n", strings.Join(args, ", "))

	// Install packages using the package manager
	cmdArgs := append([]string{"install"}, args...)
	return sdk.ExecCommand(true, "zypper", cmdArgs...)
}

func (p *ZypperPlugin) handleRemove(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no packages specified")
	}

	fmt.Printf("Removing packages: %s\n", strings.Join(args, ", "))

	cmdArgs := append([]string{"remove"}, args...)
	return sdk.ExecCommand(true, "zypper", cmdArgs...)
}

func (p *ZypperPlugin) handleUpdate(args []string) error {
	fmt.Println("Updating package repositories...")
	return sdk.ExecCommand(true, "zypper", "update")
}

func (p *ZypperPlugin) handleSearch(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no search term specified")
	}

	searchTerm := strings.Join(args, " ")
	fmt.Printf("Searching for: %s\n", searchTerm)

	return sdk.ExecCommand(false, "zypper", "search", searchTerm)
}

func (p *ZypperPlugin) handleList(args []string) error {
	return sdk.ExecCommand(false, "zypper", "list")
}

func main() {
	plugin := NewZypperPlugin()
	sdk.HandleArgs(plugin, os.Args[1:])
}
