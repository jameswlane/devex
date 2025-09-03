package main

import (
	"fmt"
	"os"
	"strings"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
)

var version = "dev" // Set by goreleaser

// CurlpipePlugin implements the Curl Pipe package manager
type CurlpipePlugin struct {
	*sdk.PackageManagerPlugin
}

// NewCurlpipePlugin creates a new Curl Pipe plugin
func NewCurlpipePlugin() *CurlpipePlugin {
	info := sdk.PluginInfo{
		Name:        "package-manager-curlpipe",
		Version:     version,
		Description: "Direct download and installation via curl",
		Author:      "DevEx Team",
		Repository:  "https://github.com/jameswlane/devex",
		Tags:        []string{"curl", "download", "script"},
		Commands: []sdk.PluginCommand{
			{
				Name:        "install",
				Description: "Install packages using Curl Pipe",
				Usage:       "Install one or more packages with dependency resolution",
			},
			{
				Name:        "remove",
				Description: "Remove packages using Curl Pipe",
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

	return &CurlpipePlugin{
		PackageManagerPlugin: sdk.NewPackageManagerPlugin(info, "curl"),
	}
}

// Execute handles command execution
func (p *CurlpipePlugin) Execute(command string, args []string) error {
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

func (p *CurlpipePlugin) handleInstall(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no packages specified")
	}

	fmt.Printf("Installing packages: %s\n", strings.Join(args, ", "))

	// Install packages using the package manager
	cmdArgs := append([]string{"install"}, args...)
	return sdk.ExecCommand(true, "curl", cmdArgs...)
}

func (p *CurlpipePlugin) handleRemove(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no packages specified")
	}

	fmt.Printf("Removing packages: %s\n", strings.Join(args, ", "))

	cmdArgs := append([]string{"remove"}, args...)
	return sdk.ExecCommand(true, "curl", cmdArgs...)
}

func (p *CurlpipePlugin) handleUpdate(args []string) error {
	fmt.Println("Updating package repositories...")
	return sdk.ExecCommand(true, "curl", "update")
}

func (p *CurlpipePlugin) handleSearch(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no search term specified")
	}

	searchTerm := strings.Join(args, " ")
	fmt.Printf("Searching for: %s\n", searchTerm)

	return sdk.ExecCommand(false, "curl", "search", searchTerm)
}

func (p *CurlpipePlugin) handleList(args []string) error {
	return sdk.ExecCommand(false, "curl", "list")
}

func main() {
	plugin := NewCurlpipePlugin()
	sdk.HandleArgs(plugin, os.Args[1:])
}
