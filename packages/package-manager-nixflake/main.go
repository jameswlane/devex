package main

// Build timestamp: 2025-09-03 17:41:19

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
)

var version = "dev" // Set by goreleaser

// defaultFlake qualifies bare package names, so "git" installs nixpkgs#git.
const defaultFlake = "nixpkgs"

// NixflakePlugin implements the Nix Flake package manager
type NixflakePlugin struct {
	*sdk.PackageManagerPlugin
}

// NewNixflakePlugin creates a new Nix Flake plugin
func NewNixflakePlugin() *NixflakePlugin {
	// Every operation acts on the invoking user's profile or flake and must never elevate.
	user := &sdk.OperationPrivilege{Scope: sdk.ScopeUser}
	info := sdk.PluginInfo{
		Name:        "package-manager-nixflake",
		Version:     version,
		Description: "Nix flake package manager",
		Author:      "DevEx Team",
		Repository:  "https://github.com/jameswlane/devex",
		Tags:        []string{"nix", "flake", "functional"},
		Commands: []sdk.PluginCommand{
			{
				Name:        "install",
				Description: "Install packages into the user profile",
				Usage:       "Install flake installables; bare names resolve to nixpkgs#<name> (nix profile install)",
				Privilege:   user,
			},
			{
				Name:        "remove",
				Description: "Remove packages from the user profile",
				Usage:       "Remove profile elements by name (nix profile remove)",
				Privilege:   user,
			},
			{
				Name:        "update",
				Description: "Update flake inputs",
				Usage:       "Update every input of the flake in the current directory (nix flake update)",
				Privilege:   user,
			},
			{
				Name:        "search",
				Description: "Search for packages",
				Usage:       "Search nixpkgs by name or description (nix search nixpkgs)",
				Privilege:   user,
			},
			{
				Name:        "list",
				Description: "List packages",
				Usage:       "List packages installed in the user profile (nix profile list)",
				Privilege:   user,
			},
		},
	}

	return &NixflakePlugin{
		PackageManagerPlugin: sdk.NewPackageManagerPlugin(info, "nix"),
	}
}

// Execute handles command execution
func (p *NixflakePlugin) Execute(command string, args []string) error {
	return p.ExecuteContext(context.Background(), command, args)
}

// ExecuteContext runs a native operation with cancellation and per-command scope.
func (p *NixflakePlugin) ExecuteContext(ctx context.Context, command string, args []string) error {
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
	default:
		return fmt.Errorf("unknown command: %s", command)
	}
}

func (p *NixflakePlugin) handleInstall(ctx context.Context, args []string) error {
	if err := validatePackages(args); err != nil {
		return err
	}

	fmt.Printf("Installing packages: %s\n", strings.Join(args, ", "))

	cmdArgs := []string{"profile", "install"}
	for _, pkg := range args {
		cmdArgs = append(cmdArgs, installable(pkg))
	}
	return p.run(ctx, "install", cmdArgs...)
}

func (p *NixflakePlugin) handleRemove(ctx context.Context, args []string) error {
	if err := validatePackages(args); err != nil {
		return err
	}

	fmt.Printf("Removing packages: %s\n", strings.Join(args, ", "))

	selectors, err := p.profileRemovalSelectors(ctx, args)
	if err != nil {
		return err
	}
	return p.run(ctx, "remove", append([]string{"profile", "remove"}, selectors...)...)
}

func (p *NixflakePlugin) handleUpdate(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("update takes no arguments; it updates every input of the flake in the current directory")
	}
	fmt.Println("Updating flake inputs...")
	return p.run(ctx, "update", "flake", "update")
}

func (p *NixflakePlugin) handleSearch(ctx context.Context, args []string) error {
	searchTerm, err := searchPhrase(args)
	if err != nil {
		return err
	}
	fmt.Printf("Searching for: %s\n", searchTerm)

	return p.run(ctx, "search", "search", defaultFlake, searchTerm)
}

func (p *NixflakePlugin) handleList(ctx context.Context, args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("list takes no arguments")
	}
	return p.run(ctx, "list", "profile", "list")
}

// installable keeps explicit flake references (nixpkgs#git, github:o/r, ./path)
// and qualifies bare package names with the default flake.
func installable(pkg string) string {
	if strings.ContainsAny(pkg, "#:/") || strings.HasPrefix(pkg, ".") {
		return pkg
	}
	return defaultFlake + "#" + pkg
}

// run executes nix with the privilege advertised for operation.
func (p *NixflakePlugin) run(ctx context.Context, operation string, args ...string) error {
	return p.runWithIO(ctx, operation, sdk.OperationIO{Stdin: os.Stdin, Stdout: os.Stdout, Stderr: os.Stderr}, args...)
}

func (p *NixflakePlugin) runWithIO(ctx context.Context, operation string, streams sdk.OperationIO, args ...string) error {
	privilege := sdk.PluginOperationPrivilege(p.Info(), []string{operation})
	if privilege == nil {
		return fmt.Errorf("operation %s has no declared privilege", operation)
	}
	native, lookupErr := exec.LookPath("nix")
	if lookupErr != nil {
		return fmt.Errorf("nix is not available on this system; install Nix and enable the nix-command and flakes experimental features")
	}
	native, err := filepath.Abs(native)
	if err != nil {
		return err
	}
	dir, err := os.Getwd()
	if err != nil {
		return err
	}
	err = sdk.RunOperation(ctx, sdk.ExecutionRequest{Privilege: *privilege, Name: native, Args: args, Dir: dir}, streams)
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

func searchPhrase(args []string) (string, error) {
	searchTerm := strings.TrimSpace(strings.Join(args, " "))
	if searchTerm == "" {
		return "", fmt.Errorf("no search term specified")
	}
	if strings.HasPrefix(searchTerm, "-") {
		return "", fmt.Errorf("search term must not start with '-'")
	}
	return searchTerm, nil
}

func main() {
	plugin := NewNixflakePlugin()
	sdk.HandleArgs(plugin, os.Args[1:])
}

// Profile JSON changed from an indexed array (manifest v1/v2) to named
// elements (v3). Inspect the native tool's output rather than guessing by OS.
func removalSelectors(data []byte, args []string) ([]string, error) {
	var profile struct {
		Version  int             `json:"version"`
		Elements json.RawMessage `json:"elements"`
	}
	if err := json.Unmarshal(data, &profile); err != nil {
		return nil, fmt.Errorf("read Nix profile: %w", err)
	}
	if profile.Version == 3 {
		var elements map[string]json.RawMessage
		if err := json.Unmarshal(profile.Elements, &elements); err != nil || elements == nil {
			return nil, fmt.Errorf("invalid named Nix profile")
		}
		return args, nil
	}
	if profile.Version != 1 && profile.Version != 2 {
		return nil, fmt.Errorf("unsupported Nix profile version %d", profile.Version)
	}
	var elements []profileElement
	if err := json.Unmarshal(profile.Elements, &elements); err != nil {
		return nil, fmt.Errorf("read indexed Nix profile: %w", err)
	}
	return legacyRemovalSelectors(elements, args)
}

type profileElement struct {
	AttrPath string `json:"attrPath"`
}

func legacyRemovalSelectors(elements []profileElement, args []string) ([]string, error) {
	selectors := make([]string, 0, len(args))
	for _, arg := range args {
		// Preserve explicit legacy selectors (index, full attribute path, store path).
		if strings.Contains(arg, "/") || isProfileIndex(arg) {
			selectors = append(selectors, arg)
			continue
		}
		match := ""
		for _, element := range elements {
			if strings.HasSuffix(element.AttrPath, "."+arg) {
				if match != "" {
					return nil, fmt.Errorf("ambiguous profile name %q; use a full attribute path", arg)
				}
				match = element.AttrPath
			}
		}
		if match == "" && strings.Contains(arg, ".") {
			selectors = append(selectors, arg)
			continue
		}
		if match == "" {
			return nil, fmt.Errorf("profile element %q not found; use 'list' to inspect selectors", arg)
		}
		selectors = append(selectors, regexp.QuoteMeta(match))
	}
	return selectors, nil
}

func isProfileIndex(s string) bool {
	for _, c := range s {
		if c < '0' || c > '9' {
			return false
		}
	}
	return s != ""
}

// profileRemovalSelectors probes JSON support without treating metadata, lookup,
// authorization, or cancellation failures as evidence of an older Nix release.
func (p *NixflakePlugin) profileRemovalSelectors(ctx context.Context, args []string) ([]string, error) {
	var metadata, diagnostic bytes.Buffer
	err := p.runWithIO(ctx, "remove", sdk.OperationIO{Stdout: &metadata, Stderr: &diagnostic}, "profile", "list", "--json")
	if !unsupportedProfileJSON(err, diagnostic.String()) {
		_, _ = io.Copy(os.Stderr, &diagnostic)
		if err != nil {
			return nil, err
		}
		return removalSelectors(metadata.Bytes(), args)
	}
	metadata.Reset()
	if err := p.runWithIO(ctx, "remove", sdk.OperationIO{Stdout: &metadata, Stderr: os.Stderr}, "profile", "list"); err != nil {
		return nil, err
	}
	elements, err := plainProfileElements(metadata.String())
	if err != nil {
		return nil, err
	}
	return legacyRemovalSelectors(elements, args)
}

func unsupportedProfileJSON(err error, diagnostic string) bool {
	var auth *sdk.AuthorizationError
	var native *exec.ExitError
	if errors.As(err, &auth) || !errors.As(err, &native) || native.ExitCode() != 1 {
		return false
	}
	// Nix 2.8 Args::parseCmdline emits this UsageError for unsupported flags.
	lines := strings.Split(strings.TrimSpace(diagnostic), "\n")
	if lines[0] != "error: unrecognised flag '--json'" {
		return false
	}
	return len(lines) == 1 || len(lines) == 2 && lines[1] == "Try 'nix --help' for more information."
}

// Nix 2.8 prints: index originalRef#attrPath resolvedRef#attrPath storePath...
// Validate the complete row before using its attribute path as a selector.
func plainProfileElements(data string) ([]profileElement, error) {
	var elements []profileElement
	seen := make(map[int]bool)
	for _, line := range strings.Split(strings.TrimSpace(data), "\n") {
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		invalid := func() ([]profileElement, error) {
			return nil, fmt.Errorf("read plain Nix profile: malformed metadata row")
		}
		if len(fields) < 4 {
			return invalid()
		}
		index, err := strconv.Atoi(fields[0])
		if err != nil || !isProfileIndex(fields[0]) || seen[index] {
			return invalid()
		}
		seen[index] = true
		for _, path := range fields[3:] {
			if !strings.HasPrefix(path, "/nix/store/") || filepath.Base(path) == "store" || filepath.Clean(path) != path || strings.Contains(strings.TrimPrefix(path, "/nix/store/"), "/") {
				return invalid()
			}
		}
		if fields[1] == "-" && fields[2] == "-" {
			elements = append(elements, profileElement{})
			continue
		}
		original := strings.LastIndex(fields[1], "#")
		resolved := strings.LastIndex(fields[2], "#")
		if original <= 0 || resolved <= 0 || original == len(fields[1])-1 || fields[1][original+1:] != fields[2][resolved+1:] {
			return invalid()
		}
		elements = append(elements, profileElement{AttrPath: fields[1][original+1:]})
	}
	return elements, nil
}
