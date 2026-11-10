package main

// Build timestamp: 2025-09-03 17:41:19

import (
	"context"
	"fmt"
	"os"
	"strings"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
)

var version = "dev" // Set by goreleaser

// SnapPlugin implements the Snap package manager
type SnapPlugin struct {
	*sdk.PackageManagerPlugin
}

// NewSnapPlugin creates a new Snap plugin
func NewSnapPlugin() *SnapPlugin {
	system := &sdk.OperationPrivilege{Scope: sdk.ScopeSystem}
	user := &sdk.OperationPrivilege{Scope: sdk.ScopeUser}
	info := sdk.PluginInfo{
		Name:        "package-manager-snap",
		Version:     version,
		Description: "Snap universal package manager",
		Author:      "DevEx Team",
		Repository:  "https://github.com/jameswlane/devex",
		Tags:        []string{"snap", "universal", "linux"},
		Commands: []sdk.PluginCommand{
			{Name: "upgrade", Description: "Upgrade installed packages", Privilege: system},
			{Name: "is-installed", Description: "Check whether every package is installed", Privilege: user},
			{
				Name:        "install",
				Privilege:   system,
				Description: "Install packages using Snap",
				Usage:       "Install one or more packages with dependency resolution",
			},
			{
				Name:        "remove",
				Privilege:   system,
				Description: "Remove packages using Snap",
				Usage:       "Remove one or more packages from the system",
			},
			{
				Name:        "update",
				Privilege:   system,
				Description: "Update installed snaps",
				Usage:       "Refresh all installed snaps or specific snaps to latest versions. snap refresh itself never prompts, but when not run as root it runs through sudo, which may ask for a password unless passwordless sudo is configured; extra snap flags such as --channel=candidate are forwarded unchanged and snap's own exit status is preserved",
			},
			{
				Name:        "search",
				Privilege:   user,
				Description: "Search for packages",
				Usage:       "Search for packages by name or description",
			},
			{
				Name:        "list",
				Privilege:   user,
				Description: "List packages",
				Usage:       "List installed packages",
			},
		},
	}

	return &SnapPlugin{
		PackageManagerPlugin: sdk.NewPackageManagerPlugin(info, "snap"),
	}
}

// Execute handles command execution
func (p *SnapPlugin) Execute(command string, args []string) error {
	return p.ExecuteContext(context.Background(), command, args)
}

// ExecuteContext executes with cancellation.
func (p *SnapPlugin) ExecuteContext(ctx context.Context, command string, args []string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	switch command {
	case "upgrade":
		return p.run(ctx, "upgrade", append([]string{"refresh"}, args...)...)
	case "is-installed":
		return p.handleIsInstalled(ctx, args)
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
	default:
		return fmt.Errorf("unknown command: %s", command)
	}
}

func (p *SnapPlugin) handleInstall(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no packages specified")
	}

	fmt.Printf("Installing packages: %s\n", strings.Join(args, ", "))

	// The CLI sends native options (--classic, --channel=<ch>, ...) ahead of the
	// package names as separate argv elements (see apps/cli package-options docs).
	// `snap install` accepts them in any position and applies them to every
	// named snap, so everything is forwarded unchanged. snap never prompts, so
	// there is no non-interactive flag to add; sudo -n is used when unattended.
	cmdArgs := append([]string{"install"}, args...)
	return p.run(ctx, "install", cmdArgs...)
}

func (p *SnapPlugin) handleRemove(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no packages specified")
	}

	fmt.Printf("Removing packages: %s\n", strings.Join(args, ", "))

	cmdArgs := append([]string{"remove"}, args...)
	return p.run(ctx, "remove", cmdArgs...)
}

// handleUpdate runs `snap refresh`. snap refresh itself never prompts, so no
// extra confirmation flag is added; when not running as root it is invoked
// through sudo; unattended execution uses sudo -n and fails if authorization is
// unavailable. Every argument (snap names and flags such as --channel=candidate)
// is forwarded unchanged as a separate argv element, and snap's native error
// and exit status are returned unmodified.
func (p *SnapPlugin) handleUpdate(ctx context.Context, args []string) error {
	if len(args) == 0 {
		fmt.Println("Refreshing installed snaps...")
	} else {
		fmt.Printf("Refreshing packages: %s\n", strings.Join(args, ", "))
	}

	cmdArgs := append([]string{"refresh"}, args...)
	return p.run(ctx, "update", cmdArgs...)
}

func (p *SnapPlugin) handleSearch(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no search term specified")
	}

	searchTerm := strings.Join(args, " ")
	fmt.Printf("Searching for: %s\n", searchTerm)

	// `snap find` is the documented command; `snap search` is only its alias
	// (snap help find / snapcraft.io docs "snap find").
	return p.run(ctx, "search", "find", searchTerm)
}

func (p *SnapPlugin) handleList(ctx context.Context, args []string) error {
	return p.run(ctx, "list", "list")
}

func main() {
	plugin := NewSnapPlugin()
	sdk.HandleArgs(plugin, os.Args[1:])
}

func (p *SnapPlugin) run(ctx context.Context, operation string, args ...string) error {
	privilege := sdk.PluginOperationPrivilege(p.Info(), []string{operation})
	return sdk.ExecOperationWithContext(ctx, *privilege, "snap", args...)
}
