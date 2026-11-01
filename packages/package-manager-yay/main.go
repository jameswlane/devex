package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
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
				Privilege:   &sdk.OperationPrivilege{Scope: sdk.ScopeUser},
				Description: "Install packages using Yay",
				Usage:       "Install one or more packages with dependency resolution",
			},
			{
				Name:        "remove",
				Privilege:   &sdk.OperationPrivilege{Scope: sdk.ScopeUser},
				Description: "Remove packages using Yay",
				Usage:       "Remove one or more packages from the system",
			},
			{
				Name:        "update",
				Privilege:   &sdk.OperationPrivilege{Scope: sdk.ScopeUser},
				Description: "Synchronize repositories and upgrade all packages",
				Usage:       "Perform a full system and AUR upgrade",
			},
			{
				Name:        "search",
				Privilege:   &sdk.OperationPrivilege{Scope: sdk.ScopeUser},
				Description: "Search for packages",
				Usage:       "Search for packages by name or description",
			},
			{
				Name:        "list",
				Privilege:   &sdk.OperationPrivilege{Scope: sdk.ScopeUser},
				Description: "List packages",
				Usage:       "List installed packages",
			},
			{
				Name:        "is-installed",
				Privilege:   &sdk.OperationPrivilege{Scope: sdk.ScopeUser},
				Description: "Check whether packages are installed",
				Usage:       "Returns the native query status for one or more packages",
			},
		},
	}

	return &YayPlugin{
		PackageManagerPlugin: sdk.NewPackageManagerPlugin(info, "yay"),
	}
}

// Execute handles command execution
func (p *YayPlugin) Execute(command string, args []string) error {
	ctx := context.Background()

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

func validateOperands(args []string, missing string) error {
	if len(args) == 0 {
		return fmt.Errorf("%s", missing)
	}
	for _, arg := range args {
		if strings.TrimSpace(arg) == "" || strings.HasPrefix(arg, "-") {
			return fmt.Errorf("invalid operand %q: provide package names or search terms, not yay options", arg)
		}
	}
	return nil
}

func (p *YayPlugin) handleInstall(ctx context.Context, args []string) error {
	if err := validateOperands(args, "no packages specified"); err != nil {
		return err
	}
	return p.runYay(ctx, true, append([]string{"-S", "--needed", "--noconfirm", "--answerdiff=None", "--answerclean=None"}, args...)...)
}

func (p *YayPlugin) handleRemove(ctx context.Context, args []string) error {
	if err := validateOperands(args, "no packages specified"); err != nil {
		return err
	}
	return p.runYay(ctx, false, append([]string{"-R", "--noconfirm"}, args...)...)
}

func (p *YayPlugin) handleUpdate(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("update takes no arguments; it synchronizes and upgrades all packages")
	}
	// Arch does not support partial upgrades: never refresh with -Sy alone.
	return p.runYay(ctx, true, "-Syu", "--noconfirm")
}

func (p *YayPlugin) handleSearch(ctx context.Context, args []string) error {
	if err := validateOperands(args, "no search term specified"); err != nil {
		return err
	}
	return p.runYay(ctx, false, append([]string{"-Ss"}, args...)...)
}

func (p *YayPlugin) handleList(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("list takes no arguments; use is-installed to query packages")
	}
	return p.runYay(ctx, false, "-Q")
}

func (p *YayPlugin) handleIsInstalled(ctx context.Context, args []string) error {
	if err := validateOperands(args, "no packages specified"); err != nil {
		return err
	}
	return p.runYay(ctx, false, append([]string{"-Q"}, args...)...)
}

func (p *YayPlugin) runYay(ctx context.Context, builds bool, args ...string) error {
	// Resolve the invoking account even in an elevated DevEx process, then pin
	// every command to it. The SDK adapter switches identity before running yay.
	identity, err := sdk.NewPrivilegeResolver().ResolveUser(nil)
	if err != nil {
		return fmt.Errorf("resolve yay build user: %w", err)
	}
	if identity.UID == "0" {
		return sdk.NewAuthorizationRequiredError("yay must never run as root; run DevEx as a regular user or preserve the invoking account with SUDO_USER", nil)
	}
	privilege := sdk.OperationPrivilege{Scope: sdk.ScopeUser, User: identity}
	if _, err := exec.LookPath("yay"); err != nil {
		return fmt.Errorf("yay is unavailable: install yay from a trusted Arch source as a regular user, then retry: %w", err)
	}
	if builds {
		if _, err := exec.LookPath("git"); err != nil {
			return fmt.Errorf("git is required for AUR builds; install prerequisites with sudo pacman -S --needed git base-devel: %w", err)
		}
		if err := sdk.RunOperation(ctx, sdk.ExecutionRequest{Privilege: privilege, Name: "pacman", Args: []string{"-Q", "base-devel"}}, sdk.OperationIO{}); err != nil {
			return fmt.Errorf("cannot verify base-devel for AUR builds; install prerequisites with sudo pacman -S --needed git base-devel: %w", err)
		}
	}
	// yay alone authorizes its native pacman step through sudo. Never elevate yay.
	if err := sdk.ExecOperationWithContext(ctx, privilege, "yay", args...); err != nil {
		return fmt.Errorf("yay %s failed; check the output above and sudo access for yay's pacman step: %w", args[0], err)
	}
	return nil
}

func main() {
	plugin := NewYayPlugin()
	sdk.HandleArgs(plugin, os.Args[1:])
}
