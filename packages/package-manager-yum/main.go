package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
)

var version = "dev"

const backendUnavailable = "no yum or dnf executable found on PATH; package-manager-yum supports RHEL/CentOS 7 with yum and RHEL 8+, Rocky Linux, and AlmaLinux with yum or the dnf-compatible alias"

type YumPlugin struct {
	*sdk.PackageManagerPlugin
	lookPath func(string) (string, error)
}

func NewYumPlugin() *YumPlugin {
	info := sdk.PluginInfo{
		Name:        "package-manager-yum",
		Version:     version,
		Description: "YUM package manager for legacy RHEL/CentOS, with a DNF-compatible fallback",
		Author:      "DevEx Team",
		Repository:  "https://github.com/jameswlane/devex",
		Tags:        []string{"yum", "dnf", "rhel", "centos", "rocky", "almalinux", "linux"},
		Commands: []sdk.PluginCommand{
			{Name: "is-installed", Description: "Check whether all requested RPM packages are installed"},
			{Name: "add-repository", Description: "Add a repository from an HTTPS .repo URL"},
			{Name: "upgrade", Description: "Upgrade installed packages"},
			{Name: "install", Description: "Install packages using YUM", Usage: "Install one or more packages with dependency resolution"},
			{Name: "remove", Description: "Remove packages using YUM", Usage: "Remove one or more packages from the system"},
			{Name: "update", Description: "Update package repositories", Usage: "Refresh package repository metadata with makecache"},
			{Name: "search", Description: "Search for packages", Usage: "Search for packages by name or description"},
			{Name: "list", Description: "List installed packages", Usage: "List installed packages in YUM's human-readable format"},
		},
	}

	return &YumPlugin{
		PackageManagerPlugin: sdk.NewPackageManagerPlugin(info, "yum"),
		lookPath:             exec.LookPath,
	}
}

func (p *YumPlugin) backend() (string, error) {
	for _, executable := range []string{"yum", "dnf"} {
		if _, err := p.lookPath(executable); err == nil {
			return executable, nil
		}
	}
	return "", fmt.Errorf("%s", backendUnavailable)
}

func (p *YumPlugin) Execute(command string, args []string) error {
	switch command {
	case "is-installed", "add-repository", "install", "remove", "upgrade", "update", "search", "list":
	default:
		return fmt.Errorf("unknown command: %s", command)
	}

	backend, err := p.backend()
	if err != nil {
		return err
	}

	ctx := context.Background()
	filtered := make([]string, 0, len(args))
	for _, arg := range args {
		if arg == "--non-interactive" {
			ctx = sdk.WithUnattended(ctx)
			continue
		}
		filtered = append(filtered, arg)
	}
	args = filtered

	switch command {
	case "is-installed":
		return p.handleIsInstalled(ctx, args)
	case "add-repository":
		return p.handleAddRepository(ctx, args)
	case "install":
		return p.handleInstall(ctx, backend, args)
	case "remove":
		return p.handleRemove(ctx, backend, args)
	case "upgrade":
		return sdk.ExecCommandWithContext(ctx, true, backend, append([]string{"upgrade", "--assumeyes"}, args...)...)
	case "update":
		fmt.Println("Updating package repositories...")
		return sdk.ExecCommandWithContext(ctx, true, backend, "makecache", "--assumeyes")
	case "search":
		return p.handleSearch(ctx, backend, args)
	case "list":
		return p.handleList(ctx, backend)
	default:
		panic("unreachable")
	}
}

func (p *YumPlugin) handleInstall(ctx context.Context, backend string, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no packages specified")
	}
	fmt.Printf("Installing packages: %s\n", strings.Join(args, ", "))
	return sdk.ExecCommandWithContext(ctx, true, backend, append([]string{"install", "--assumeyes"}, args...)...)
}

func (p *YumPlugin) handleRemove(ctx context.Context, backend string, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no packages specified")
	}
	// YUM and DNF can exit successfully when nothing matches, so confirm each
	// specification names an installed package or capability first.
	for _, spec := range args {
		if err := requireInstalledMatch(ctx, spec); err != nil {
			return err
		}
	}
	fmt.Printf("Removing packages: %s\n", strings.Join(args, ", "))
	return sdk.ExecCommandWithContext(ctx, true, backend, append([]string{"remove", "--assumeyes"}, args...)...)
}

func (p *YumPlugin) handleSearch(ctx context.Context, backend string, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no search term specified")
	}
	searchTerm := strings.Join(args, " ")
	fmt.Printf("Searching for: %s\n", searchTerm)
	// Each argument is a separate keyword to YUM and DNF.
	return sdk.ExecCommandWithContext(ctx, false, backend, append([]string{"search"}, args...)...)
}

// handleList lists installed packages. YUM 3 only accepts "list installed",
// while DNF5 only accepts "list --installed" (DNF4 takes both), so the
// backend's version decides the syntax.
func (p *YumPlugin) handleList(ctx context.Context, backend string) error {
	output, err := sdk.ExecCommandOutputWithContext(ctx, backend, "--version")
	if err != nil {
		return queryFailure(fmt.Sprintf("detect %s version", backend), err)
	}
	selector := "--installed"
	if strings.HasPrefix(strings.TrimSpace(output), "3.") {
		selector = "installed"
	}
	return sdk.ExecCommandWithContext(ctx, false, backend, "list", selector)
}

func main() {
	plugin := NewYumPlugin()
	sdk.HandleArgs(plugin, os.Args[1:])
}
