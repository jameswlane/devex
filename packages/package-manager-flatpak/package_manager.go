package main

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
)

// FlatpakInstaller implements Flatpak package manager functionality
type FlatpakInstaller struct {
	*sdk.PackageManagerPlugin
	logger         sdk.Logger
	flatpakVersion *FlatpakVersion
	versionCached  bool
}

// FlatpakVersion information
type FlatpakVersion struct {
	Major int
	Minor int
	Patch int
}

// Execute handles command execution
func (f *FlatpakInstaller) Execute(command string, args []string) error {
	ctx := context.Background()

	switch command {
	case "ensure-installed":
		return f.handleEnsureInstalled(ctx, args)
	case "add-flathub":
		return f.handleAddFlathub(ctx, args)
	}

	// For all other commands, ensure Flatpak is available
	f.EnsureAvailable()

	switch command {
	case "install":
		return f.handleInstall(ctx, args)
	case "remove":
		return f.handleRemove(ctx, args)
	case "update":
		return f.handleUpdate(ctx, args)
	case "search":
		return f.handleSearch(ctx, args)
	case "list":
		return f.handleList(ctx, args)
	case "remote-add":
		return f.handleRemoteAdd(ctx, args)
	case "remote-remove":
		return f.handleRemoteRemove(ctx, args)
	case "remote-list":
		return f.handleRemoteList(ctx, args)
	case "is-installed":
		return f.handleIsInstalled(ctx, args)
	case "info":
		return f.handleInfo(ctx, args)
	default:
		return fmt.Errorf("unknown command: %s", command)
	}
}

// getFlatpakVersion detects the Flatpak version with caching
func (f *FlatpakInstaller) getFlatpakVersion() (*FlatpakVersion, error) {
	if f.versionCached && f.flatpakVersion != nil {
		return f.flatpakVersion, nil
	}

	output, err := sdk.ExecCommandOutputWithContext(context.Background(), "flatpak", "--version")
	if err != nil {
		return nil, fmt.Errorf("failed to detect Flatpak version: %w", err)
	}

	// Parse version from output like "Flatpak 1.14.4"
	versionRegex := regexp.MustCompile(`Flatpak\s+(\d+)\.(\d+)\.(\d+)`)
	matches := versionRegex.FindStringSubmatch(output)
	if len(matches) < 4 {
		// Try alternate format
		versionRegex = regexp.MustCompile(`(\d+)\.(\d+)\.(\d+)`)
		matches = versionRegex.FindStringSubmatch(output)
		if len(matches) < 4 {
			return nil, fmt.Errorf("failed to parse Flatpak version from output: %s", output)
		}
	}

	var major, minor, patch int
	_, _ = fmt.Sscanf(matches[1], "%d", &major)
	_, _ = fmt.Sscanf(matches[2], "%d", &minor)
	_, _ = fmt.Sscanf(matches[3], "%d", &patch)

	f.flatpakVersion = &FlatpakVersion{
		Major: major,
		Minor: minor,
		Patch: patch,
	}
	f.versionCached = true

	f.logger.Debug("Detected Flatpak version", "version", fmt.Sprintf("%d.%d.%d", major, minor, patch))
	return f.flatpakVersion, nil
}

// parseFlatpakCommand extracts remote and app ID from command
func parseFlatpakCommand(command string) (remote string, appID string) {
	// Handle commands with explicit remote (e.g., "flathub org.mozilla.Firefox")
	parts := strings.SplitN(command, " ", 2)
	if len(parts) == 2 {
		return parts[0], parts[1]
	}
	// No explicit remote specified
	return "", command
}

// handleInstall installs Flatpak applications
func (f *FlatpakInstaller) handleInstall(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no applications specified")
	}

	f.logger.Printf("Installing applications: %s\n", strings.Join(args, ", "))

	for _, app := range args {
		if err := f.validateAppID(app); err != nil {
			return fmt.Errorf("invalid application ID '%s': %w", app, err)
		}

		// Parse app command
		remote, appID := parseFlatpakCommand(app)

		// Check if already installed
		if installed, _ := f.isAppInstalled(ctx, appID); installed {
			f.logger.Printf("Application %s is already installed, skipping\n", appID)
			continue
		}

		// Build install command
		var installCmd []string
		if remote != "" {
			installCmd = []string{"install", "-y", remote, appID}
		} else {
			installCmd = []string{"install", "-y", appID}
		}

		f.logger.Printf("Installing %s...\n", appID)
		if err := sdk.ExecCommandWithContext(ctx, false, "flatpak", installCmd...); err != nil {
			return fmt.Errorf("failed to install %s: %w", appID, err)
		}
	}

	f.logger.Success("Successfully installed applications")
	return nil
}

// handleRemove removes Flatpak applications
func (f *FlatpakInstaller) handleRemove(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no applications specified")
	}

	f.logger.Printf("Removing applications: %s\n", strings.Join(args, ", "))

	for _, app := range args {
		if err := f.validateAppID(app); err != nil {
			return fmt.Errorf("invalid application ID '%s': %w", app, err)
		}

		if installed, _ := f.isAppInstalled(ctx, app); !installed {
			f.logger.Printf("Application %s is not installed, skipping\n", app)
			continue
		}

		f.logger.Printf("Removing %s...\n", app)
		if err := sdk.ExecCommandWithContext(ctx, false, "flatpak", "uninstall", "-y", app); err != nil {
			return fmt.Errorf("failed to remove %s: %w", app, err)
		}
	}

	f.logger.Success("Successfully removed applications")
	return nil
}

// handleUpdate updates all installed applications and runtimes
func (f *FlatpakInstaller) handleUpdate(ctx context.Context, args []string) error {
	f.logger.Println("Updating all installed applications and runtimes...")

	if err := sdk.ExecCommandWithContext(ctx, false, "flatpak", "update", "-y"); err != nil {
		return fmt.Errorf("failed to update applications: %w", err)
	}

	f.logger.Success("Applications and runtimes updated successfully")
	return nil
}

// handleSearch searches for applications
func (f *FlatpakInstaller) handleSearch(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no search term specified")
	}

	searchTerm := strings.Join(args, " ")
	if err := f.validateSearchTerm(searchTerm); err != nil {
		return fmt.Errorf("invalid search term: %w", err)
	}

	f.logger.Printf("Searching for: %s\n", searchTerm)
	return sdk.ExecCommandWithContext(ctx, false, "flatpak", "search", searchTerm)
}

// handleList lists installed applications
func (f *FlatpakInstaller) handleList(ctx context.Context, args []string) error {
	// Default to listing applications
	listArgs := []string{"list", "--app"}

	// Handle specific flags
	for _, arg := range args {
		switch arg {
		case "--runtime":
			listArgs = []string{"list", "--runtime"}
		case "--all":
			listArgs = []string{"list"}
		}
	}

	return sdk.ExecCommandWithContext(ctx, false, "flatpak", listArgs...)
}

// handleIsInstalled checks if applications are installed
func (f *FlatpakInstaller) handleIsInstalled(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no applications specified")
	}

	allInstalled := true
	for _, app := range args {
		if err := f.validateAppID(app); err != nil {
			return fmt.Errorf("invalid application ID '%s': %w", app, err)
		}

		installed, err := f.isAppInstalled(ctx, app)
		if err != nil {
			return fmt.Errorf("failed to check installation status of %s: %w", app, err)
		}

		if installed {
			f.logger.Success("Application %s is installed", app)
		} else {
			f.logger.ErrorMsg("Application %s is not installed", app)
			allInstalled = false
		}
	}

	if !allInstalled {
		return fmt.Errorf("one or more applications are not installed")
	}
	return nil
}

// handleInfo shows application information
func (f *FlatpakInstaller) handleInfo(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no application specified")
	}

	for i, app := range args {
		if err := f.validateAppID(app); err != nil {
			return fmt.Errorf("invalid application ID '%s': %w", app, err)
		}

		f.logger.Printf("Information for: %s\n", app)

		if err := sdk.ExecCommandWithContext(ctx, false, "flatpak", "info", app); err != nil {
			f.logger.ErrorMsg("Failed to get info for %s: %v", app, err)
		}

		if i < len(args)-1 {
			fmt.Println("---")
		}
	}

	return nil
}

// isAppInstalled checks if a Flatpak application is installed
func (f *FlatpakInstaller) isAppInstalled(ctx context.Context, appID string) (bool, error) {
	output, err := sdk.ExecCommandOutputWithContext(ctx, "flatpak", "list", "--app")
	if err != nil {
		return false, err
	}

	// Check if app ID is in the list
	return strings.Contains(output, appID), nil
}
