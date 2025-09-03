package main

import (
	"fmt"
	"os"
	"strings"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
)

var version = "dev" // Set by goreleaser

// NixflakePlugin implements the Nix Flake package manager
type NixflakePlugin struct {
	*sdk.PackageManagerPlugin
}

// NewNixflakePlugin creates a new Nix Flake plugin
func NewNixflakePlugin() *NixflakePlugin {
	info := sdk.PluginInfo{
		Name:        "package-manager-nixflake",
		Version:     version,
		Description: "Nix flake package manager",
		Author:      "DevEx Team",
		Repository:  "https://github.com/jameswlane/devex",
		Tags:        []string{"nix", "flake", "functional"},
		Commands: []sdk.PluginCommand{
			{
				Name:        "install",
				Description: "Install packages using Nix Flake",
				Usage:       "Install one or more packages with dependency resolution",
			},
			{
				Name:        "remove",
				Description: "Remove packages using Nix Flake",
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

	return &NixflakePlugin{
		PackageManagerPlugin: sdk.NewPackageManagerPlugin(info, "nix"),
	}
}

// Execute handles command execution
func (p *NixflakePlugin) Execute(command string, args []string) error {
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

func (p *NixflakePlugin) handleInstall(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no packages specified")
	}

	fmt.Printf("Installing packages: %s\n", strings.Join(args, ", "))

	// Install packages using the package manager
	cmdArgs := append([]string{"install"}, args...)
	return sdk.ExecCommand(true, "nix", cmdArgs...)
}

func (p *NixflakePlugin) handleRemove(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no packages specified")
	}

	fmt.Printf("Removing packages: %s\n", strings.Join(args, ", "))

	cmdArgs := append([]string{"remove"}, args...)
	return sdk.ExecCommand(true, "nix", cmdArgs...)
}

func (p *NixflakePlugin) handleUpdate(args []string) error {
	fmt.Println("Updating package repositories...")
	return sdk.ExecCommand(true, "nix", "update")
}

func (p *NixflakePlugin) handleSearch(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no search term specified")
	}

	searchTerm := strings.Join(args, " ")
	fmt.Printf("Searching for: %s\n", searchTerm)

	return sdk.ExecCommand(false, "nix", "search", searchTerm)
}

func (p *NixflakePlugin) handleList(args []string) error {
	return sdk.ExecCommand(false, "nix", "list")
}

func main() {
	plugin := NewNixflakePlugin()
	sdk.HandleArgs(plugin, os.Args[1:])
}
