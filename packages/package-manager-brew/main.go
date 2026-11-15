package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"strings"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
)

var version = "dev" // Set by goreleaser

// brewPrefixes are Homebrew's supported install locations: Apple Silicon,
// Intel macOS, then Linux. They are checked when brew is not on PATH, which is
// common for noninteractive shells that never ran `brew shellenv`.
var brewPrefixes = []string{
	"/opt/homebrew/bin/brew",
	"/usr/local/bin/brew",
	"/home/linuxbrew/.linuxbrew/bin/brew",
}

// brewPackageName accepts formula and Cask tokens, optionally tap-qualified
// (user/tap/name) or versioned (python@3.12). It never accepts an option.
var brewPackageName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9+_.@-]*(/[A-Za-z0-9][A-Za-z0-9+_.@-]*){0,2}$`)

var userScope = &sdk.OperationPrivilege{Scope: sdk.ScopeUser}

// BrewPlugin implements the Homebrew package manager
type BrewPlugin struct {
	*sdk.PackageManagerPlugin
	brew string
}

// NewBrewPlugin creates a new Homebrew plugin
func NewBrewPlugin() *BrewPlugin {
	info := sdk.PluginInfo{
		Name:        "package-manager-brew",
		Version:     version,
		Description: "Homebrew package manager for macOS and Linux",
		Author:      "DevEx Team",
		Repository:  "https://github.com/jameswlane/devex",
		Tags:        []string{"brew", "homebrew", "macos", "linux"},
		Commands: []sdk.PluginCommand{
			{
				Name:        "install",
				Privilege:   userScope,
				Description: "Install packages using Homebrew",
				Usage:       "install [--cask|--formula] PACKAGE...",
			},
			{
				Name:        "remove",
				Privilege:   userScope,
				Description: "Remove packages using Homebrew",
				Usage:       "remove [--cask|--formula] PACKAGE...",
			},
			{
				Name:        "is-installed",
				Privilege:   userScope,
				Description: "Check whether all requested packages are installed",
				Usage:       "is-installed [--cask|--formula] PACKAGE... (no selector checks formulae, then Casks)",
			},
			{
				Name:        "update",
				Privilege:   userScope,
				Description: "Update package repositories",
				Usage:       "Fetch the newest Homebrew and formula/Cask definitions",
			},
			{
				Name:        "upgrade",
				Privilege:   userScope,
				Description: "Upgrade installed packages",
				Usage:       "upgrade [--cask|--formula] [PACKAGE...] (no packages upgrades everything outdated)",
			},
			{
				Name:        "search",
				Privilege:   userScope,
				Description: "Search for packages",
				Usage:       "Search for packages by name or description",
			},
			{
				Name:        "list",
				Privilege:   userScope,
				Description: "List packages",
				Usage:       "List installed packages",
			},
		},
	}

	return &BrewPlugin{
		PackageManagerPlugin: sdk.NewPackageManagerPlugin(info, "brew"),
	}
}

// findBrew returns the Homebrew executable from PATH or a standard prefix.
func findBrew() (string, error) {
	if path, err := exec.LookPath("brew"); err == nil {
		return path, nil
	}
	for _, candidate := range brewPrefixes {
		if info, err := os.Stat(candidate); err == nil && info.Mode().IsRegular() && info.Mode()&0o111 != 0 {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("homebrew is not installed: brew was not found on PATH or in %s; install it from https://brew.sh, then add it to your shell with `eval \"$(<prefix>/bin/brew shellenv)\"`", strings.Join(brewPrefixes, ", "))
}

// Execute handles command execution
func (p *BrewPlugin) Execute(command string, args []string) error {
	switch command {
	case "install", "remove", "is-installed", "update", "upgrade", "search", "list":
	default:
		return fmt.Errorf("unknown command: %s", command)
	}
	brew, err := findBrew()
	if err != nil {
		return err
	}
	p.brew = brew

	ctx := context.Background()

	switch command {
	case "install":
		return p.handleInstall(ctx, args)
	case "remove":
		return p.handleRemove(ctx, args)
	case "is-installed":
		return p.handleIsInstalled(ctx, args)
	case "update":
		return p.handleUpdate(ctx, args)
	case "upgrade":
		return p.handleUpgrade(ctx, args)
	case "search":
		return p.handleSearch(ctx, args)
	default:
		return p.handleList(ctx, args)
	}
}

// parseSelection separates an optional --cask/--formula selector from package
// names. Any other option is rejected so callers cannot inject brew flags.
func parseSelection(args []string) (selector string, packages []string, err error) {
	for _, arg := range args {
		switch {
		case arg == "--cask" || arg == "--formula":
			if selector != "" && selector != arg {
				return "", nil, fmt.Errorf("use either --cask or --formula, not both")
			}
			selector = arg
		case strings.HasPrefix(arg, "-"):
			return "", nil, fmt.Errorf("unsupported Homebrew option %q: only --cask and --formula are accepted", arg)
		case !brewPackageName.MatchString(arg) || strings.Contains(arg, ".."):
			return "", nil, fmt.Errorf("invalid Homebrew package name %q", arg)
		default:
			packages = append(packages, arg)
		}
	}
	return selector, packages, nil
}

// selectionArgs builds verb [selector] packages...
func selectionArgs(verb, selector string, packages []string) []string {
	args := []string{verb}
	if selector != "" {
		args = append(args, selector)
	}
	return append(args, packages...)
}

func (p *BrewPlugin) handleInstall(ctx context.Context, args []string) error {
	selector, packages, err := parseSelection(args)
	if err != nil {
		return err
	}
	if len(packages) == 0 {
		return fmt.Errorf("no packages specified")
	}

	fmt.Printf("Installing packages: %s\n", strings.Join(packages, ", "))
	return p.runBrew(ctx, selectionArgs("install", selector, packages)...)
}

func (p *BrewPlugin) handleRemove(ctx context.Context, args []string) error {
	selector, packages, err := parseSelection(args)
	if err != nil {
		return err
	}
	if len(packages) == 0 {
		return fmt.Errorf("no packages specified")
	}

	fmt.Printf("Removing packages: %s\n", strings.Join(packages, ", "))
	return p.runBrew(ctx, selectionArgs("uninstall", selector, packages)...)
}

func (p *BrewPlugin) handleUpdate(ctx context.Context, args []string) error {
	if len(args) > 0 {
		return fmt.Errorf("update takes no arguments; use upgrade to upgrade packages")
	}
	fmt.Println("Updating package repositories...")
	return p.runBrew(ctx, "update")
}

func (p *BrewPlugin) handleUpgrade(ctx context.Context, args []string) error {
	selector, packages, err := parseSelection(args)
	if err != nil {
		return err
	}
	return p.runBrew(ctx, selectionArgs("upgrade", selector, packages)...)
}

// handleIsInstalled checks each package with `brew list --versions`, which
// exits 1 without diagnostics when a package is absent. Without a selector a
// package counts as installed when it is either a formula or a Cask, matching
// how `brew install NAME` resolves names.
func (p *BrewPlugin) handleIsInstalled(ctx context.Context, args []string) error {
	selector, packages, err := parseSelection(args)
	if err != nil {
		return err
	}
	if len(packages) == 0 {
		return fmt.Errorf("no packages specified")
	}
	selectors := []string{selector}
	if selector == "" {
		selectors = []string{"--formula", "--cask"}
	}
	var missing []string
	for _, pkg := range packages {
		installed := false
		for _, kind := range selectors {
			found, err := p.listed(ctx, kind, pkg)
			if err != nil {
				return err
			}
			if found {
				installed = true
				break
			}
		}
		if !installed {
			missing = append(missing, pkg)
		}
	}
	if len(missing) > 0 {
		return &sdk.NotInstalledError{Packages: missing}
	}
	return nil
}

func (p *BrewPlugin) listed(ctx context.Context, selector, pkg string) (bool, error) {
	var stdout, stderr bytes.Buffer
	err := sdk.RunOperation(ctx, sdk.ExecutionRequest{Privilege: *userScope, Name: p.brew, Args: []string{"list", "--versions", selector, pkg}}, sdk.OperationIO{Stdout: &stdout, Stderr: &stderr})
	if err == nil {
		_, _ = os.Stdout.Write(stdout.Bytes())
		return true, nil
	}
	var exitErr *exec.ExitError
	// Homebrew uses status 1 both for absence and for failures; only a silent
	// status 1 means the query ran and found nothing.
	if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 && strings.TrimSpace(stderr.String()) == "" {
		return false, nil
	}
	_, _ = os.Stderr.Write(stderr.Bytes())
	return false, fmt.Errorf("homebrew query failed for %q: %w", pkg, err)
}

func (p *BrewPlugin) handleSearch(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no search term specified")
	}

	searchTerm := strings.Join(args, " ")
	if strings.HasPrefix(searchTerm, "-") {
		return fmt.Errorf("search term %q must not start with '-'", searchTerm)
	}
	fmt.Printf("Searching for: %s\n", searchTerm)

	return sdk.ExecOperationWithContext(ctx, *userScope, p.brew, "search", searchTerm)
}

func (p *BrewPlugin) handleList(ctx context.Context, args []string) error {
	return sdk.ExecOperationWithContext(ctx, *userScope, p.brew, "list")
}

func main() {
	plugin := NewBrewPlugin()
	sdk.HandleArgs(plugin, os.Args[1:])
}

// runBrew disables Homebrew's prompts for DevEx's mutating operations.
func (p *BrewPlugin) runBrew(ctx context.Context, args ...string) error {
	return sdk.RunOperation(ctx, sdk.ExecutionRequest{Privilege: *userScope, Name: p.brew, Args: args, Env: []string{"NONINTERACTIVE=1"}}, sdk.OperationIO{Stdout: os.Stdout, Stderr: os.Stderr})
}
