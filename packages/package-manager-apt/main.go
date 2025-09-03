// packages/plugins/package-manager-apt/main.go
package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/jameswlane/devex/packages/plugin-sdk"
)

var version = "dev" // Set by goreleaser

// APTPlugin implements the APT package manager
type APTPlugin struct {
	*sdk.PackageManagerPlugin
}

// NewAPTPlugin creates a new APT plugin
func NewAPTPlugin() *APTPlugin {
	info := sdk.PluginInfo{
		Name:        "package-manager-apt",
		Version:     version,
		Description: "APT package manager support for Debian/Ubuntu systems",
		Author:      "DevEx Team",
		Repository:  "https://github.com/jameswlane/devex",
		Tags:        []string{"package-manager", "apt", "debian", "ubuntu", "linux"},
		Commands: []sdk.PluginCommand{
			{
				Name:        "install",
				Description: "Install packages using APT",
				Usage:       "Install one or more packages with automatic dependency resolution",
				Flags: map[string]string{
					"yes":        "Automatically answer yes to prompts",
					"update":     "Update package lists before installing",
					"fix-broken": "Attempt to fix broken dependencies",
				},
			},
			{
				Name:        "remove",
				Description: "Remove packages using APT",
				Usage:       "Remove one or more packages from the system",
				Flags: map[string]string{
					"purge":      "Remove packages and their configuration files",
					"autoremove": "Remove automatically installed dependencies that are no longer needed",
				},
			},
			{
				Name:        "update",
				Description: "Update package lists",
				Usage:       "Download package information from all configured sources",
			},
			{
				Name:        "upgrade",
				Description: "Upgrade installed packages",
				Usage:       "Install newer versions of all installed packages",
				Flags: map[string]string{
					"dist-upgrade": "Intelligently handle changing dependencies",
				},
			},
			{
				Name:        "search",
				Description: "Search for packages",
				Usage:       "Search for packages by name or description",
			},
			{
				Name:        "list",
				Description: "List packages",
				Usage:       "List installed packages or search for available packages",
				Flags: map[string]string{
					"installed":  "List only installed packages",
					"upgradable": "List only upgradable packages",
					"manual":     "List manually installed packages",
				},
			},
			{
				Name:        "info",
				Description: "Show package information",
				Usage:       "Display detailed information about a package",
			},
		},
	}

	return &APTPlugin{
		PackageManagerPlugin: sdk.NewPackageManagerPlugin(info, "apt"),
	}
}

// Execute handles command execution
func (p *APTPlugin) Execute(command string, args []string) error {
	p.EnsureAvailable()

	switch command {
	case "install":
		return p.handleInstall(args)
	case "remove":
		return p.handleRemove(args)
	case "update":
		return p.handleUpdate(args)
	case "upgrade":
		return p.handleUpgrade(args)
	case "search":
		return p.handleSearch(args)
	case "list":
		return p.handleList(args)
	case "info":
		return p.handleInfo(args)
	default:
		return fmt.Errorf("unknown command: %s", command)
	}
}

func (p *APTPlugin) handleInstall(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no packages specified")
	}

	fmt.Printf("Installing packages: %s\n", strings.Join(args, ", "))

	// Update package lists first
	fmt.Println("Updating package lists...")
	if err := sdk.ExecCommand(true, "apt", "update"); err != nil {
		fmt.Printf("Warning: failed to update package lists: %v\n", err)
	}

	// Install packages
	cmdArgs := append([]string{"install", "-y"}, args...)
	return sdk.ExecCommand(true, "apt", cmdArgs...)
}

func (p *APTPlugin) handleRemove(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no packages specified")
	}

	fmt.Printf("Removing packages: %s\n", strings.Join(args, ", "))

	cmdArgs := append([]string{"remove", "-y"}, args...)
	return sdk.ExecCommand(true, "apt", cmdArgs...)
}

func (p *APTPlugin) handleUpdate(args []string) error {
	fmt.Println("Updating package lists...")
	return sdk.ExecCommand(true, "apt", "update")
}

func (p *APTPlugin) handleUpgrade(args []string) error {
	fmt.Println("Upgrading installed packages...")

	// Update first
	if err := sdk.ExecCommand(true, "apt", "update"); err != nil {
		return fmt.Errorf("failed to update package lists: %w", err)
	}

	// Then upgrade
	return sdk.ExecCommand(true, "apt", "upgrade", "-y")
}

func (p *APTPlugin) handleSearch(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no search term specified")
	}

	searchTerm := strings.Join(args, " ")
	fmt.Printf("Searching for: %s\n", searchTerm)

	return sdk.ExecCommand(false, "apt", "search", searchTerm)
}

func (p *APTPlugin) handleList(args []string) error {
	if len(args) == 0 {
		// List all installed packages
		return sdk.ExecCommand(false, "apt", "list", "--installed")
	}

	// Handle flags or search terms
	cmdArgs := append([]string{"list"}, args...)
	return sdk.ExecCommand(false, "apt", cmdArgs...)
}

func (p *APTPlugin) handleInfo(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no package specified")
	}

	for _, pkg := range args {
		fmt.Printf("Package information for: %s\n", pkg)
		if err := sdk.ExecCommand(false, "apt", "show", pkg); err != nil {
			fmt.Printf("Failed to get info for %s: %v\n", pkg, err)
		}
		if len(args) > 1 {
			fmt.Println("---")
		}
	}

	return nil
}

func main() {
	plugin := NewAPTPlugin()
	
	// Handle args with potential panic recovery
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(os.Stderr, "Plugin panic recovered: %v\n", r)
			os.Exit(1)
		}
	}()
	
	sdk.HandleArgs(plugin, os.Args[1:])
}
