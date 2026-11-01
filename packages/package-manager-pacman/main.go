package main

// Build timestamp: 2025-09-03 17:41:19

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
)

var version = "dev" // Set by goreleaser

// PacmanPlugin implements the Pacman package manager
type PacmanPlugin struct {
	*sdk.PackageManagerPlugin
}

// NewPacmanPlugin creates a new Pacman plugin
func NewPacmanPlugin() *PacmanPlugin {
	info := sdk.PluginInfo{
		Name:        "package-manager-pacman",
		Version:     version,
		Description: "Pacman package manager for Arch Linux",
		Author:      "DevEx Team",
		Repository:  "https://github.com/jameswlane/devex",
		Tags:        []string{"pacman", "arch", "linux"},
		Commands: []sdk.PluginCommand{
			{
				Name:        "install",
				Description: "Install packages using Pacman",
				Usage:       "Install one or more packages with dependency resolution",
				Privilege:   &sdk.OperationPrivilege{Scope: sdk.ScopeSystem},
			},
			{
				Name:        "remove",
				Description: "Remove packages using Pacman",
				Usage:       "Remove one or more packages from the system",
				Privilege:   &sdk.OperationPrivilege{Scope: sdk.ScopeSystem},
			},
			{
				Name:        "update",
				Description: "Refresh repositories and upgrade the entire system",
				Usage:       "Perform a full rolling-release upgrade using pacman -Syu --noconfirm",
				Privilege:   &sdk.OperationPrivilege{Scope: sdk.ScopeSystem},
			},
			{
				Name:        "search",
				Description: "Search for packages",
				Usage:       "Search for packages by name or description",
				Privilege:   &sdk.OperationPrivilege{Scope: sdk.ScopeUser},
			},
			{
				Name:        "list",
				Description: "List packages",
				Usage:       "List installed packages",
				Privilege:   &sdk.OperationPrivilege{Scope: sdk.ScopeUser},
			},
			{
				Name:        "is-installed",
				Description: "Check whether all requested packages are installed",
				Usage:       "Returns exit code 0 if all packages are installed; preserves native query failures",
				Privilege:   &sdk.OperationPrivilege{Scope: sdk.ScopeUser},
			},
		},
	}

	return &PacmanPlugin{
		PackageManagerPlugin: sdk.NewPackageManagerPlugin(info, "pacman"),
	}
}

// Execute handles command execution
func (p *PacmanPlugin) Execute(command string, args []string) error {
	return p.ExecuteContext(context.Background(), command, args)
}

// ExecuteContext runs a native operation with cancellation and per-command scope.
func (p *PacmanPlugin) ExecuteContext(ctx context.Context, command string, args []string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	switch command {
	case "install":
		return p.handleInstall(ctx, args)
	case "remove":
		return p.handleRemove(ctx, args)
	case "update":
		return p.handleUpdate(ctx, args)
	case "search":
		return p.handleSearch(ctx, args)
	case "list":
		return p.handleList(ctx, args)
	case "is-installed":
		return p.handleIsInstalled(ctx, args)
	default:
		return fmt.Errorf("unknown command: %s", command)
	}
}

func (p *PacmanPlugin) handleInstall(ctx context.Context, args []string) error {
	if err := validatePackages(args); err != nil {
		return err
	}

	fmt.Printf("Installing packages: %s\n", strings.Join(args, ", "))

	// Install packages using the package manager
	cmdArgs := append([]string{"-S", "--needed", "--noconfirm"}, args...)
	return p.run(ctx, "install", cmdArgs...)
}

func (p *PacmanPlugin) handleRemove(ctx context.Context, args []string) error {
	if err := validatePackages(args); err != nil {
		return err
	}

	fmt.Printf("Removing packages: %s\n", strings.Join(args, ", "))

	cmdArgs := append([]string{"-R", "--noconfirm"}, args...)
	return p.run(ctx, "remove", cmdArgs...)
}

func (p *PacmanPlugin) handleUpdate(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("update takes no arguments; it upgrades the entire system")
	}
	fmt.Println("Refreshing repositories and upgrading the entire system...")
	return p.run(ctx, "update", "-Syu", "--noconfirm")
}

func (p *PacmanPlugin) handleSearch(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no search term specified")
	}

	searchTerm := strings.Join(args, " ")
	fmt.Printf("Searching for: %s\n", searchTerm)

	if strings.HasPrefix(searchTerm, "-") {
		return fmt.Errorf("search term must not start with '-'")
	}
	return p.run(ctx, "search", "-Ss", searchTerm)
}

func (p *PacmanPlugin) handleList(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("list takes no arguments; use is-installed to query packages")
	}
	return p.run(ctx, "list", "-Q")
}

func (p *PacmanPlugin) handleIsInstalled(ctx context.Context, args []string) error {
	if err := validatePackages(args); err != nil {
		return err
	}
	return p.run(ctx, "is-installed", append([]string{"-Q"}, args...)...)
}

func validatePackages(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no packages specified")
	}
	for _, pkg := range args {
		if strings.TrimSpace(pkg) == "" || strings.HasPrefix(pkg, "-") {
			return fmt.Errorf("invalid package name: %q", pkg)
		}
	}
	return nil
}

func (p *PacmanPlugin) run(ctx context.Context, operation string, args ...string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	privilege := sdk.PluginOperationPrivilege(p.Info(), []string{operation})
	var stderr stderrTail
	err := sdk.RunOperation(ctx, sdk.ExecutionRequest{
		Privilege: *privilege,
		Name:      "pacman",
		Args:      args,
	}, sdk.OperationIO{Stdin: os.Stdin, Stdout: os.Stdout, Stderr: io.MultiWriter(os.Stderr, &stderr)})
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if err != nil && len(stderr) != 0 {
		// Preserve the native status for sdk.HandleArgs while retaining diagnostics.
		return fmt.Errorf("pacman %s: %w: %s", operation, err, strings.TrimSpace(string(stderr)))
	}
	return err
}

// Keep only the last 4 KiB even when a native command emits extensive output.
type stderrTail []byte

func (t *stderrTail) Write(data []byte) (int, error) {
	const limit = 4096
	n := len(data)
	if n >= limit {
		*t = append((*t)[:0], data[n-limit:]...)
	} else {
		*t = append(*t, data...)
		if len(*t) > limit {
			*t = (*t)[len(*t)-limit:]
		}
	}
	return n, nil
}

func main() {
	plugin := NewPacmanPlugin()
	sdk.HandleArgs(plugin, os.Args[1:])
}
