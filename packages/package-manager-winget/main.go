package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
)

var version = "dev" // Set by goreleaser

const errMissingWinget = "winget is not installed or not on PATH; install or update App Installer from the Microsoft Store (https://aka.ms/getwinget), then open a new terminal and retry"

// Flags every query or change passes so winget never waits for a prompt.
var (
	sourceAgreement = []string{"--accept-source-agreements", "--disable-interactivity"}
	packageChange   = []string{"--silent", "--accept-package-agreements", "--accept-source-agreements", "--disable-interactivity"}
)

// WingetPlugin implements the Windows Package Manager through winget.
type WingetPlugin struct {
	*sdk.PackageManagerPlugin
}

// NewWingetPlugin creates a new winget plugin.
func NewWingetPlugin() *WingetPlugin {
	// install, remove and upgrade run as the current, non-elevated user unless
	// --scope=machine asks for system scope; see privilegeFor.
	user := &sdk.OperationPrivilege{Scope: sdk.ScopeUser}
	info := sdk.PluginInfo{
		Name:        "package-manager-winget",
		Version:     version,
		Description: "Windows Package Manager (winget)",
		Author:      "DevEx Team",
		Repository:  "https://github.com/jameswlane/devex",
		Tags:        []string{"winget", "windows", "package-manager"},
		Commands: []sdk.PluginCommand{
			{
				Name:        "install",
				Description: "Install packages by winget package identifier",
				Usage:       "Install packages (winget install --id <id> --exact --no-upgrade ...); --scope=machine needs an elevated terminal",
				Flags:       map[string]string{"scope": "user or machine; machine runs at system scope"},
				Privilege:   user,
			},
			{
				Name:        "is-installed",
				Description: "Check whether every package is installed",
				Usage:       "Check installed packages by identifier (winget list --id <id> --exact)",
				Flags:       map[string]string{"scope": "user or machine installed-package filter"},
				Privilege:   user,
			},
			{
				Name:        "remove",
				Description: "Uninstall packages by winget package identifier",
				Usage:       "Uninstall packages (winget uninstall --id <id> --exact ...); --scope=machine needs an elevated terminal",
				Flags:       map[string]string{"scope": "user or machine; machine runs at system scope"},
				Privilege:   user,
			},
			{
				Name:        "update",
				Description: "Refresh winget source metadata",
				Usage:       "Refresh source indexes without changing packages (winget source update)",
				Privilege:   user,
			},
			{
				Name:        "upgrade",
				Description: "Upgrade selected packages",
				Usage:       "Upgrade only the given packages (winget upgrade --id <id> --exact ...); --scope=machine needs an elevated terminal",
				Flags:       map[string]string{"scope": "user or machine; machine runs at system scope"},
				Privilege:   user,
			},
			{
				Name:        "search",
				Description: "Search winget sources",
				Usage:       "Search configured sources (winget search --query <term>)",
				Privilege:   user,
			},
			{
				Name:        "list",
				Description: "List installed packages",
				Usage:       "List installed packages (winget list)",
				Privilege:   user,
			},
		},
	}

	return &WingetPlugin{
		PackageManagerPlugin: sdk.NewPackageManagerPlugin(info, "winget"),
	}
}

// Execute handles command execution.
func (p *WingetPlugin) Execute(command string, args []string) error {
	return p.ExecuteContext(context.Background(), command, args)
}

// ExecuteContext runs a native operation with cancellation and per-command scope.
func (p *WingetPlugin) ExecuteContext(ctx context.Context, command string, args []string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	switch command {
	case "install", "remove", "upgrade":
		return p.handleChange(ctx, command, args)
	case "is-installed":
		return p.handleIsInstalled(ctx, args)
	case "update":
		if len(args) != 0 {
			return fmt.Errorf("update takes no arguments; it refreshes winget sources, use upgrade <id>... to upgrade packages")
		}
		return p.run(ctx, sdk.ScopeUser, "source", "update", "--disable-interactivity")
	case "search":
		return p.handleSearch(ctx, args)
	case "list":
		if len(args) != 0 {
			return fmt.Errorf("list takes no arguments; use is-installed <id> to check specific packages")
		}
		return p.run(ctx, sdk.ScopeUser, append([]string{"list"}, sourceAgreement...)...)
	default:
		return fmt.Errorf("unknown command: %s", command)
	}
}

// nativeVerb maps a plugin command to the winget command it runs.
var nativeVerb = map[string]string{"install": "install", "remove": "uninstall", "upgrade": "upgrade"}

// handleChange installs, removes or upgrades each package with its own winget
// call, so a failure names the package and keeps winget's exit status.
func (p *WingetPlugin) handleChange(ctx context.Context, command string, args []string) error {
	request, err := parseRequest(args)
	if err != nil {
		return err
	}
	if !p.IsAvailable() {
		return fmt.Errorf("%s", errMissingWinget)
	}
	verb := nativeVerb[command]
	for _, id := range request.ids {
		argv := []string{verb, "--id", id, "--exact"}
		if command == "remove" {
			// uninstall has no package agreements to accept.
			argv = append(argv, "--silent", "--accept-source-agreements", "--disable-interactivity")
		} else {
			argv = append(argv, packageChange...)
		}
		if command == "install" {
			// Keep install distinct from upgrade: an installed package is left alone.
			argv = append(argv, "--no-upgrade")
		}
		argv = append(argv, request.scopeArgs()...)

		fmt.Printf("Running winget %s: %s\n", verb, id)
		err := p.run(ctx, request.privilege(), argv...)
		if err == nil {
			continue
		}
		if ctx.Err() != nil {
			return err
		}
		if code, ok := hresult(err); ok && isAlreadyDone(command, code) {
			fmt.Printf("%s: %s\n", id, alreadyDoneMessage(command))
			continue
		}
		return explain(command, id, request.scope, err)
	}
	return nil
}

func (p *WingetPlugin) handleSearch(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no search term specified")
	}
	for _, term := range args {
		if strings.TrimSpace(term) == "" || strings.HasPrefix(term, "-") || strings.ContainsAny(term, "\x00\r\n") {
			return fmt.Errorf("invalid search term: %q", term)
		}
	}
	argv := append([]string{"search", "--query", strings.Join(args, " ")}, sourceAgreement...)
	err := p.run(ctx, sdk.ScopeUser, argv...)
	if code, ok := hresult(err); ok && code == hrNoApplicationsFound && ctx.Err() == nil {
		return fmt.Errorf("no winget package matches %q: %w", strings.Join(args, " "), err)
	}
	return err
}

// run executes winget at the given scope. On Windows the SDK runs user scope
// only from a non-elevated terminal and system scope only from an elevated one.
func (p *WingetPlugin) run(ctx context.Context, scope sdk.OperationScope, args ...string) error {
	if !p.IsAvailable() {
		return fmt.Errorf("%s", errMissingWinget)
	}
	err := sdk.RunOperation(ctx, sdk.ExecutionRequest{Privilege: sdk.OperationPrivilege{Scope: scope}, Name: "winget", Args: args},
		sdk.OperationIO{Stdin: os.Stdin, Stdout: os.Stdout, Stderr: os.Stderr})
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return err
}

func main() {
	plugin := NewWingetPlugin()
	sdk.HandleArgs(plugin, os.Args[1:])
}
