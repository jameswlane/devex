package main

import (
	"context"
	"fmt"
	"strings"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
)

// handleAddFlathub adds the Flathub repository
func (f *FlatpakInstaller) handleAddFlathub(ctx context.Context, args []string) error {
	f.EnsureAvailable()

	f.logger.Printf("Adding Flathub repository...\n")

	// Check if Flathub is already added
	output, err := sdk.ExecCommandOutputWithContext(ctx, "flatpak", "remote-list")
	if err != nil {
		f.logger.Warning("Failed to check existing remotes: %v", err)
	} else if strings.Contains(output, "flathub") {
		f.logger.Success("Flathub repository already configured")
		return nil
	}

	// Add Flathub remote
	flathubURL := "https://dl.flathub.org/repo/flathub.flatpakrepo"

	// Determine installation level (user vs system)
	installArgs := []string{"remote-add", "--if-not-exists", "flathub", flathubURL}

	// Check for explicit user/system flags
	systemWide := false
	for _, arg := range args {
		if arg == "--system" {
			systemWide = true
			installArgs = []string{"remote-add", "--if-not-exists", "--system", "flathub", flathubURL}
			break
		}
	}

	if err := sdk.ExecCommandWithContext(ctx, systemWide, "flatpak", installArgs...); err != nil {
		return fmt.Errorf("failed to add Flathub repository: %w", err)
	}

	f.logger.Success("Flathub repository added successfully")
	return nil
}

// handleRemoteAdd adds a new remote repository
func (f *FlatpakInstaller) handleRemoteAdd(ctx context.Context, args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("remote-add requires: <name> <url>")
	}

	remoteName := args[0]
	remoteURL := args[1]

	// Validate remote name and URL
	if err := f.validateRemoteName(remoteName); err != nil {
		return fmt.Errorf("invalid remote name: %w", err)
	}

	if err := f.validateRemoteURL(remoteURL); err != nil {
		return fmt.Errorf("invalid remote URL: %w", err)
	}

	f.logger.Printf("Adding remote: %s (%s)\n", remoteName, remoteURL)

	addArgs := []string{"remote-add", "--if-not-exists", remoteName, remoteURL}

	// Check for system-wide installation flag
	systemWide := f.hasSystemFlag(args[2:])
	if systemWide {
		addArgs = []string{"remote-add", "--if-not-exists", "--system", remoteName, remoteURL}
	}

	if err := sdk.ExecCommandWithContext(ctx, systemWide, "flatpak", addArgs...); err != nil {
		return fmt.Errorf("failed to add remote %s: %w", remoteName, err)
	}

	f.logger.Success("Successfully added remote: %s", remoteName)
	return nil
}

// handleRemoteRemove removes a remote repository
func (f *FlatpakInstaller) handleRemoteRemove(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no remote name specified")
	}

	remoteName := args[0]
	if err := f.validateRemoteName(remoteName); err != nil {
		return fmt.Errorf("invalid remote name: %w", err)
	}

	f.logger.Printf("Removing remote: %s\n", remoteName)

	// Check if remote exists
	if exists, err := f.remoteExists(ctx, remoteName); err != nil {
		f.logger.Warning("Failed to check if remote exists: %v", err)
	} else if !exists {
		f.logger.Printf("Remote %s does not exist, skipping\n", remoteName)
		return nil
	}

	if err := sdk.ExecCommandWithContext(ctx, false, "flatpak", "remote-delete", remoteName); err != nil {
		return fmt.Errorf("failed to remove remote %s: %w", remoteName, err)
	}

	f.logger.Success("Successfully removed remote: %s", remoteName)
	return nil
}

// handleRemoteList lists configured remotes
func (f *FlatpakInstaller) handleRemoteList(ctx context.Context, args []string) error {
	f.logger.Println("Configured Flatpak remotes:")
	return sdk.ExecCommandWithContext(ctx, false, "flatpak", "remote-list")
}

// remoteExists checks if a remote repository exists
func (f *FlatpakInstaller) remoteExists(ctx context.Context, remoteName string) (bool, error) {
	output, err := sdk.ExecCommandOutputWithContext(ctx, "flatpak", "remote-list")
	if err != nil {
		return false, err
	}

	// Check if remote name appears in the output
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, remoteName) {
			return true, nil
		}
	}

	return false, nil
}

// hasSystemFlag checks if system-wide installation is requested
func (f *FlatpakInstaller) hasSystemFlag(args []string) bool {
	for _, arg := range args {
		if arg == "--system" {
			return true
		}
	}
	return false
}

// GetFlathubURL returns the standard Flathub repository URL
func (f *FlatpakInstaller) GetFlathubURL() string {
	return "https://dl.flathub.org/repo/flathub.flatpakrepo"
}

// GetKDEAppsURL returns the KDE Apps repository URL
func (f *FlatpakInstaller) GetKDEAppsURL() string {
	return "https://distribute.kde.org/kdeapps.flatpakrepo"
}

// GetGnomeNightlyURL returns the GNOME Nightly repository URL
func (f *FlatpakInstaller) GetGnomeNightlyURL() string {
	return "https://nightly.gnome.org/gnome-nightly.flatpakrepo"
}
