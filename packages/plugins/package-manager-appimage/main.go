package main

import (
	"fmt"
	"os"
	"strings"

	sdk "github.com/jameswlane/devex/packages/shared/plugin-sdk"
)

var version = "dev" // Set by goreleaser

// AppimagePlugin implements the AppImage package manager
type AppimagePlugin struct {
	*sdk.PackageManagerPlugin
}

// NewAppimagePlugin creates a new AppImage plugin
func NewAppimagePlugin() *AppimagePlugin {
	info := sdk.PluginInfo{
		Name:        "package-manager-appimage",
		Version:     version,
		Description: "AppImage package manager for Linux",
		Author:      "DevEx Team",
		Repository:  "https://github.com/jameswlane/devex",
		Tags:        []string{"appimage", "linux"},
		Commands: []sdk.PluginCommand{
			{
				Name:        "install",
				Description: "Install packages using AppImage",
				Usage:       "Install one or more packages with dependency resolution",
			},
			{
				Name:        "remove",
				Description: "Remove packages using AppImage",
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

	return &AppimagePlugin{
		PackageManagerPlugin: sdk.NewPackageManagerPlugin(info, "AppImage"),
	}
}

// Execute handles command execution
func (p *AppimagePlugin) Execute(command string, args []string) error {
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

func (p *AppimagePlugin) handleInstall(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no packages specified")
	}

	fmt.Printf("Installing packages: %s\n", strings.Join(args, ", "))

	// Install packages using the package manager
	cmdArgs := append([]string{"install"}, args...)
	return sdk.ExecCommand(true, "AppImage", cmdArgs...)
}

func (p *AppimagePlugin) handleRemove(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no packages specified")
	}

	fmt.Printf("Removing packages: %s\n", strings.Join(args, ", "))

	cmdArgs := append([]string{"remove"}, args...)
	return sdk.ExecCommand(true, "AppImage", cmdArgs...)
}

func (p *AppimagePlugin) handleUpdate(args []string) error {
	fmt.Println("Updating package repositories...")
	return sdk.ExecCommand(true, "AppImage", "update")
}

func (p *AppimagePlugin) handleSearch(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no search term specified")
	}

	searchTerm := strings.Join(args, " ")
	fmt.Printf("Searching for: %s\n", searchTerm)

	return sdk.ExecCommand(false, "AppImage", "search", searchTerm)
}

func (p *AppimagePlugin) handleList(args []string) error {
	return sdk.ExecCommand(false, "AppImage", "list")
}

func main() {
	plugin := NewAppimagePlugin()
	sdk.HandleArgs(plugin, os.Args[1:])
}
