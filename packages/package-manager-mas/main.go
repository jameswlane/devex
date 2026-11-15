package main

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

const (
	errMissingMas = "mas is not installed; install it with Homebrew (brew install mas), then retry"
	// The App Store keeps no local package index, so there is nothing to refresh.
	errUpdateUnsupported = "mas has no package index to refresh; use upgrade <app-id>... to install pending App Store updates"
	// mas cannot sign in on current macOS releases; the App Store app owns the session.
	signInHint = "mas needs an Apple Account signed in to the App Store app (open App Store and choose Sign In; mas cannot sign in for you), and install only installs apps this account already got from the App Store"
)

// MasPlugin implements the Mac App Store package manager through mas.
type MasPlugin struct {
	*sdk.PackageManagerPlugin
}

// NewMasPlugin creates a new Mac App Store plugin.
func NewMasPlugin() *MasPlugin {
	system := &sdk.OperationPrivilege{Scope: sdk.ScopeSystem}
	user := &sdk.OperationPrivilege{Scope: sdk.ScopeUser}
	info := sdk.PluginInfo{
		Name:        "package-manager-mas",
		Version:     version,
		Description: "Mac App Store package manager (mas)",
		Author:      "DevEx Team",
		Repository:  "https://github.com/jameswlane/devex",
		Tags:        []string{"mas", "app-store", "macos"},
		Commands: []sdk.PluginCommand{
			{
				Name:        "install",
				Description: "Install App Store apps by app ID",
				Usage:       "Install apps this Apple Account already got from the App Store (mas install <app-id>...)",
				Privilege:   system,
			},
			{
				Name:        "is-installed",
				Description: "Check whether every app is installed",
				Usage:       "Check installed apps by app ID (mas list <app-id>)",
				Privilege:   user,
			},
			{
				Name:        "remove",
				Description: "Uninstall App Store apps by app ID",
				Usage:       "Uninstall apps installed from the App Store (mas uninstall <app-id>...)",
				Privilege:   system,
			},
			{
				Name:        "update",
				Description: "Refresh package metadata (unsupported)",
				Usage:       "Unsupported: the App Store has no local index; use upgrade",
				Privilege:   user,
			},
			{
				Name:        "upgrade",
				Description: "Upgrade selected App Store apps",
				Usage:       "Install pending updates for the given app IDs (mas upgrade <app-id>...)",
				Privilege:   system,
			},
			{
				Name:        "search",
				Description: "Search the App Store",
				Usage:       "Search the App Store by name (mas search <term>...)",
				Privilege:   user,
			},
			{
				Name:        "list",
				Description: "List installed App Store apps",
				Usage:       "List apps installed from the App Store (mas list)",
				Privilege:   user,
			},
		},
	}

	return &MasPlugin{
		PackageManagerPlugin: sdk.NewPackageManagerPlugin(info, "mas"),
	}
}

// Execute handles command execution.
func (p *MasPlugin) Execute(command string, args []string) error {
	return p.ExecuteContext(context.Background(), command, args)
}

// ExecuteContext runs a native operation with cancellation and per-command scope.
func (p *MasPlugin) ExecuteContext(ctx context.Context, command string, args []string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	switch command {
	case "install":
		return p.handleStoreOperation(ctx, "install", "install", args)
	case "is-installed":
		return p.handleIsInstalled(ctx, args)
	case "remove":
		if err := validateAppIDs(args); err != nil {
			return err
		}
		return p.run(ctx, "remove", append([]string{"uninstall"}, args...)...)
	case "update":
		return fmt.Errorf("%s", errUpdateUnsupported)
	case "upgrade":
		return p.handleStoreOperation(ctx, "upgrade", "upgrade", args)
	case "search":
		return p.handleSearch(ctx, args)
	case "list":
		if len(args) != 0 {
			return fmt.Errorf("list takes no arguments; use is-installed <app-id> to check specific apps")
		}
		return p.run(ctx, "list", "list")
	default:
		return fmt.Errorf("unknown command: %s", command)
	}
}

// handleStoreOperation runs an App Store download, which needs a signed-in
// Apple Account. A native failure keeps mas's output and exit status and adds
// the sign-in prerequisite, because mas reports a missing session only through
// Apple's own (often generic) errors.
func (p *MasPlugin) handleStoreOperation(ctx context.Context, operation, verb string, args []string) error {
	if err := validateAppIDs(args); err != nil {
		return err
	}
	if !p.IsAvailable() {
		return fmt.Errorf("%s", errMissingMas)
	}

	fmt.Printf("Running mas %s: %s\n", verb, strings.Join(args, ", "))

	var stderr bytes.Buffer
	err := p.runWithIO(ctx, operation, os.Stdout, io.MultiWriter(os.Stderr, &stderr), append([]string{verb}, args...)...)
	if err != nil && ctx.Err() == nil && !isAuthorization(err) {
		return fmt.Errorf("mas %s failed; %s: %w", verb, signInHint, err)
	}
	return err
}

func (p *MasPlugin) handleSearch(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no search term specified")
	}
	for _, term := range args {
		if strings.TrimSpace(term) == "" || strings.HasPrefix(term, "-") {
			return fmt.Errorf("invalid search term: %q", term)
		}
	}
	return p.run(ctx, "search", append([]string{"search"}, args...)...)
}

// run executes mas with the privilege advertised for operation.
func (p *MasPlugin) run(ctx context.Context, operation string, args ...string) error {
	return p.runWithIO(ctx, operation, os.Stdout, os.Stderr, args...)
}

// runWithIO is run with caller-supplied output streams.
func (p *MasPlugin) runWithIO(ctx context.Context, operation string, stdout, stderr io.Writer, args ...string) error {
	privilege := sdk.PluginOperationPrivilege(p.Info(), []string{operation})
	if privilege == nil {
		return fmt.Errorf("operation %s has no declared privilege", operation)
	}
	if !p.IsAvailable() {
		return fmt.Errorf("%s", errMissingMas)
	}
	err := sdk.RunOperation(ctx, sdk.ExecutionRequest{Privilege: *privilege, Name: "mas", Args: args}, sdk.OperationIO{Stdin: os.Stdin, Stdout: stdout, Stderr: stderr})
	if ctx.Err() != nil {
		return ctx.Err()
	}
	return err
}

// validateAppIDs accepts only numeric App Store IDs (for example Xcode's
// 497799835). mas treats any other argument as a bundle ID or an option, so
// names such as "xcode" are rejected before mas runs.
func validateAppIDs(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no app IDs specified")
	}
	for _, id := range args {
		if !isAppID(id) {
			return fmt.Errorf("invalid App Store app ID %q: use the numeric ID from the app's App Store URL (for example 497799835 for Xcode) or from mas search", id)
		}
	}
	return nil
}

func isAppID(id string) bool {
	if id == "" || len(id) > 19 || id[0] == '0' {
		return false
	}
	for _, c := range id {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

func main() {
	plugin := NewMasPlugin()
	sdk.HandleArgs(plugin, os.Args[1:])
}
