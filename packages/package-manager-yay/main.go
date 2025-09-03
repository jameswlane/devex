package main

import (
	"fmt"
	"os"
	"strings"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
)

var version = "dev" // Set by goreleaser

// YayPlugin implements the Yay package manager
type YayPlugin struct {
	*sdk.PackageManagerPlugin
}

// NewYayPlugin creates a new Yay plugin
func NewYayPlugin() *YayPlugin {
	info := sdk.PluginInfo{
		Name:        "package-manager-yay",
		Version:     version,
		Description: "Yay AUR helper for Arch Linux",
		Author:      "DevEx Team",
		Repository:  "https://github.com/jameswlane/devex",
		Tags:        []string{"yay", "aur", "arch", "linux"},
		Commands: []sdk.PluginCommand{
			{
				Name:        "install",
				Description: "Install packages using Yay",
				Usage:       "Install one or more packages with dependency resolution",
			},
			{
				Name:        "remove",
				Description: "Remove packages using Yay",
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

	return &YayPlugin{
		PackageManagerPlugin: sdk.NewPackageManagerPlugin(info, "yay"),
	}
}

// Execute handles command execution
func (p *YayPlugin) Execute(command string, args []string) error {
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

func (p *YayPlugin) handleInstall(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no packages specified")
	}

	fmt.Printf("Installing packages: %s\n", strings.Join(args, ", "))

	// Install packages using the package manager
	cmdArgs := append([]string{"install"}, args...)
	return sdk.ExecCommand(true, "yay", cmdArgs...)
}

func (p *YayPlugin) handleRemove(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no packages specified")
	}

	fmt.Printf("Removing packages: %s\n", strings.Join(args, ", "))

	cmdArgs := append([]string{"remove"}, args...)
	return sdk.ExecCommand(true, "yay", cmdArgs...)
}

func (p *YayPlugin) handleUpdate(args []string) error {
	fmt.Println("Updating package repositories...")
	return sdk.ExecCommand(true, "yay", "update")
}

func (p *YayPlugin) handleSearch(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no search term specified")
	}

	searchTerm := strings.Join(args, " ")
	fmt.Printf("Searching for: %s\n", searchTerm)

	return sdk.ExecCommand(false, "yay", "search", searchTerm)
}

func (p *YayPlugin) handleList(args []string) error {
	return sdk.ExecCommand(false, "yay", "list")
}

func main() {
	plugin := NewYayPlugin()
	sdk.HandleArgs(plugin, os.Args[1:])
}
