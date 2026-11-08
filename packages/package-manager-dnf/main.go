package main

// Build timestamp: 2025-09-03 17:41:19

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
)

var version = "dev" // Set by goreleaser

// DnfPlugin implements the DNF package manager
type DnfPlugin struct {
	*sdk.PackageManagerPlugin
}

// NewDnfPlugin creates a new DNF plugin
func NewDnfPlugin() *DnfPlugin {
	info := sdk.PluginInfo{
		Name:        "package-manager-dnf",
		Version:     version,
		Description: "DNF package manager for Fedora/RHEL",
		Author:      "DevEx Team",
		Repository:  "https://github.com/jameswlane/devex",
		Tags:        []string{"dnf", "fedora", "rhel", "linux"},
		Commands: []sdk.PluginCommand{
			{Name: "upgrade", Description: "Upgrade installed packages"},
			{
				Name:        "install",
				Description: "Install packages using DNF",
				Usage:       "Install one or more packages with dependency resolution",
			},
			{
				Name:        "remove",
				Description: "Remove packages using DNF",
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

	return &DnfPlugin{
		PackageManagerPlugin: sdk.NewPackageManagerPlugin(info, "dnf"),
	}
}

// Execute handles command execution
func (p *DnfPlugin) Execute(command string, args []string) error {
	p.EnsureAvailable()

	ctx := context.Background()

	switch command {
	case "install":
		return p.handleInstall(ctx, args)
	case "remove":
		return p.handleRemove(ctx, args)
	case "upgrade":
		return sdk.ExecCommandWithContext(ctx, true, "dnf", append([]string{"upgrade", "--assumeyes"}, args...)...)
	case "update":
		return p.handleUpdate(ctx, args)
	case "search":
		return p.handleSearch(ctx, args)
	case "list":
		return p.handleList(ctx, args)
	default:
		return fmt.Errorf("unknown command: %s", command)
	}
}

func (p *DnfPlugin) handleInstall(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no packages specified")
	}

	fmt.Printf("Installing packages: %s\n", strings.Join(args, ", "))

	// Install packages using the package manager
	cmdArgs := append([]string{"install", "--assumeyes"}, args...)
	return sdk.ExecCommandWithContext(ctx, true, "dnf", cmdArgs...)
}

func (p *DnfPlugin) handleRemove(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no packages specified")
	}

	// DNF can exit successfully when no installed packages match. Check every
	// package specification before starting the transaction so mixed requests
	// cannot silently remove only the installed subset.
	for _, spec := range args {
		if err := requireInstalledMatch(ctx, spec); err != nil {
			return err
		}
	}

	fmt.Printf("Removing packages: %s\n", strings.Join(args, ", "))

	cmdArgs := append([]string{"remove", "--assumeyes"}, args...)
	return sdk.ExecCommandWithContext(ctx, true, "dnf", cmdArgs...)
}

// requireInstalledMatch resolves spec the way `dnf remove` does (names, globs,
// NEVRAs and provides) against installed packages only. The query reads the
// local RPM database, so it needs no elevation or network access.
func requireInstalledMatch(ctx context.Context, spec string) error {
	if strings.HasPrefix(spec, "-") {
		return fmt.Errorf("cannot remove package %q: package specifications must not start with '-'", spec)
	}
	output, err := sdk.ExecCommandOutputWithContext(ctx, "dnf", "repoquery", "--installed", "--quiet", "--cacheonly", spec)
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			if stderr := strings.TrimSpace(string(exitErr.Stderr)); stderr != "" {
				return fmt.Errorf("cannot remove package %q: installed-package query failed: %s: %w", spec, stderr, err)
			}
		}
		return fmt.Errorf("cannot remove package %q: installed-package query failed: %w", spec, err)
	}
	if strings.TrimSpace(output) == "" {
		return fmt.Errorf("cannot remove package %q: no installed package matches", spec)
	}
	return nil
}

func (p *DnfPlugin) handleUpdate(ctx context.Context, args []string) error {
	fmt.Println("Updating package repositories...")
	return sdk.ExecCommandWithContext(ctx, true, "dnf", "makecache")
}

func (p *DnfPlugin) handleSearch(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no search term specified")
	}

	searchTerm := strings.Join(args, " ")
	fmt.Printf("Searching for: %s\n", searchTerm)

	return sdk.ExecCommandWithContext(ctx, false, "dnf", "search", searchTerm)
}

func (p *DnfPlugin) handleList(ctx context.Context, args []string) error {
	return sdk.ExecCommandWithContext(ctx, false, "dnf", "list", "installed")
}

func main() {
	plugin := NewDnfPlugin()
	sdk.HandleArgs(plugin, os.Args[1:])
}
