package main

// Build timestamp: 2025-09-03 17:41:19

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
)

var version = "dev" // Set by goreleaser

// RPM installs package files and queries the local database only; repository
// operations belong to a front end such as dnf or zypper.
const (
	errUpdateUnsupported = "rpm has no repositories to refresh; use the dnf or zypper plugin to update repositories"
	errSearchUnsupported = "rpm cannot search repositories; use the dnf or zypper plugin to search, or list to query installed packages"
)

// RpmPlugin implements the RPM package manager
type RpmPlugin struct {
	*sdk.PackageManagerPlugin
}

// NewRpmPlugin creates a new RPM plugin
func NewRpmPlugin() *RpmPlugin {
	system := &sdk.OperationPrivilege{Scope: sdk.ScopeSystem}
	user := &sdk.OperationPrivilege{Scope: sdk.ScopeUser}
	info := sdk.PluginInfo{
		Name:        "package-manager-rpm",
		Version:     version,
		Description: "RPM package manager",
		Author:      "DevEx Team",
		Repository:  "https://github.com/jameswlane/devex",
		Tags:        []string{"rpm", "redhat", "linux"},
		Commands: []sdk.PluginCommand{
			{
				Name:        "install",
				Description: "Install package files using RPM",
				Usage:       "Install one or more local .rpm files (rpm --install); dependencies are not resolved",
				Privilege:   system,
			},
			{
				Name:        "is-installed",
				Description: "Check whether every package is installed",
				Usage:       "Check installed packages by name (rpm --query)",
				Privilege:   user,
			},
			{
				Name:        "remove",
				Description: "Remove packages using RPM",
				Usage:       "Remove one or more installed packages (rpm --erase)",
				Privilege:   system,
			},
			{
				Name:        "update",
				Description: "Update package repositories (unsupported)",
				Usage:       "Unsupported: rpm has no repositories; use dnf or zypper",
				Privilege:   user,
			},
			{
				Name:        "search",
				Description: "Search for packages (unsupported)",
				Usage:       "Unsupported: rpm has no repository search; use dnf or zypper",
				Privilege:   user,
			},
			{
				Name:        "list",
				Description: "List packages",
				Usage:       "List installed packages (rpm --query --all)",
				Privilege:   user,
			},
		},
	}

	return &RpmPlugin{
		PackageManagerPlugin: sdk.NewPackageManagerPlugin(info, "rpm"),
	}
}

// Execute handles command execution
func (p *RpmPlugin) Execute(command string, args []string) error {
	return p.ExecuteContext(context.Background(), command, args)
}

// ExecuteContext runs a native operation with cancellation and per-command scope.
func (p *RpmPlugin) ExecuteContext(ctx context.Context, command string, args []string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	switch command {
	case "install":
		return p.handleInstall(ctx, args)
	case "is-installed":
		return p.handleIsInstalled(ctx, args)
	case "remove":
		return p.handleRemove(ctx, args)
	case "update":
		return fmt.Errorf("%s", errUpdateUnsupported)
	case "search":
		return fmt.Errorf("%s", errSearchUnsupported)
	case "list":
		return p.handleList(ctx, args)
	default:
		return fmt.Errorf("unknown command: %s", command)
	}
}

func (p *RpmPlugin) handleInstall(ctx context.Context, args []string) error {
	if err := validatePackages(args); err != nil {
		return err
	}
	for _, file := range args {
		if err := validateRPMFile(file); err != nil {
			return err
		}
	}

	fmt.Printf("Installing packages: %s\n", strings.Join(args, ", "))

	var stderr bytes.Buffer
	err := p.runWithIO(ctx, "install", os.Stdout, io.MultiWriter(os.Stderr, &stderr), nil, append([]string{"--install"}, args...)...)
	if err != nil && strings.Contains(stderr.String(), "Failed dependencies") {
		return fmt.Errorf("rpm does not resolve dependencies; use dnf or zypper to install them (for example, dnf install ./file.rpm): %w", err)
	}
	return err
}

func validateRPMFile(file string) error {
	if strings.Contains(file, "://") {
		return fmt.Errorf("remote RPM URLs are not supported: %q; download and verify the file yourself, then pass its local path, or use dnf or zypper to install URLs", file)
	}
	if !strings.HasSuffix(strings.ToLower(file), ".rpm") {
		return fmt.Errorf("rpm installs package files, not repository packages: %q is not an .rpm file; use the dnf or zypper plugin to install by name", file)
	}
	info, err := os.Stat(file)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("RPM package file %q does not exist; pass an existing local .rpm file", file)
		}
		return fmt.Errorf("cannot access RPM package file %q: %w", file, err)
	}
	if !info.Mode().IsRegular() {
		return fmt.Errorf("RPM package file %q is not a regular file; pass a local .rpm file", file)
	}
	return nil
}

func (p *RpmPlugin) handleRemove(ctx context.Context, args []string) error {
	if err := validatePackages(args); err != nil {
		return err
	}

	fmt.Printf("Removing packages: %s\n", strings.Join(args, ", "))

	return p.run(ctx, "remove", append([]string{"--erase"}, args...)...)
}

func (p *RpmPlugin) handleList(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("list takes no arguments")
	}
	return p.run(ctx, "list", "--query", "--all")
}

// run executes rpm with the privilege advertised for operation.
func (p *RpmPlugin) run(ctx context.Context, operation string, args ...string) error {
	return p.runWithIO(ctx, operation, os.Stdout, os.Stderr, nil, args...)
}

// runWithIO is run with caller-supplied output streams and extra environment.
func (p *RpmPlugin) runWithIO(ctx context.Context, operation string, stdout, stderr io.Writer, env []string, args ...string) error {
	privilege := sdk.PluginOperationPrivilege(p.Info(), []string{operation})
	if privilege == nil {
		return fmt.Errorf("operation %s has no declared privilege", operation)
	}
	if !p.IsAvailable() {
		return fmt.Errorf("rpm is not available on this system; this plugin requires an RPM-based distribution")
	}
	err := sdk.RunOperation(ctx, sdk.ExecutionRequest{Privilege: *privilege, Name: "rpm", Args: args, Env: env}, sdk.OperationIO{Stdin: os.Stdin, Stdout: stdout, Stderr: stderr})
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return err
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

func main() {
	plugin := NewRpmPlugin()
	sdk.HandleArgs(plugin, os.Args[1:])
}
