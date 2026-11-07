package main

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
)

// AppimagePlugin implements the AppImage package manager
type AppimagePlugin struct {
	*sdk.PackageManagerPlugin
	logger sdk.Logger
}

// AppImageVersion information
type AppImageVersion struct {
	Type    string // Type 1 or Type 2
	Version string // Version string if available
}

// Execute handles command execution
func (p *AppimagePlugin) Execute(command string, args []string) error {
	switch command {
	case "is-installed":
		return p.handleIsInstalled(args)
	case "validate-url":
		return p.handleValidateURL(args)
	case "install":
		return p.handleInstall(args)
	case "remove":
		return p.handleRemove(args)
	case "list":
		return p.handleList(args)
	default:
		return fmt.Errorf("unknown command: %s", command)
	}
}

// handleInstall installs AppImage applications
func (p *AppimagePlugin) handleInstall(args []string) error {
	if len(args) < 2 {
		return fmt.Errorf("usage: <download_url> <binary_name> [flags]")
	}

	downloadURL := args[0]
	binaryName := args[1]

	// Validate parameters first
	if err := p.validateAppImageParameters(downloadURL, binaryName); err != nil {
		return fmt.Errorf("parameter validation failed: %w", err)
	}

	// Parse flags
	installLocation := "gui" // default to GUI apps
	for i := 2; i < len(args); i++ {
		switch args[i] {
		case "--gui":
			installLocation = "gui"
		case "--cli":
			installLocation = "cli"
		}
	}

	p.logger.Printf("Installing AppImage: %s as %s\n", downloadURL, binaryName)

	// Check if already installed
	if installed, err := p.isAppImageInstalled(binaryName); err != nil {
		p.logger.Warning("Failed to check installation status: %v", err)
	} else if installed {
		p.logger.Printf("AppImage %s is already installed, skipping\n", binaryName)
		return nil
	}

	// Validate URL accessibility
	if err := p.validateURLAccessibility(downloadURL); err != nil {
		return fmt.Errorf("URL validation failed: %w", err)
	}

	// Install the AppImage
	if err := p.installAppImage(downloadURL, binaryName, installLocation); err != nil {
		return fmt.Errorf("failed to install AppImage: %w", err)
	}

	p.logger.Success("AppImage %s installed successfully", binaryName)
	return nil
}

// handleRemove removes AppImage applications
func (p *AppimagePlugin) handleRemove(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no AppImage names specified")
	}

	// Validate all binary names first
	for _, binaryName := range args {
		if err := p.validateBinaryName(binaryName); err != nil {
			return fmt.Errorf("invalid binary name '%s': %w", binaryName, err)
		}
	}

	p.logger.Printf("Removing AppImages: %s\n", strings.Join(args, ", "))

	// Remove every requested AppImage, then report every failure so a later
	// success cannot hide an earlier partial removal.
	var failures []error
	for _, binaryName := range args {
		if err := p.removeAppImage(binaryName); err != nil {
			failures = append(failures, fmt.Errorf("failed to remove AppImage %s: %w", binaryName, err))
		}
	}

	return errors.Join(failures...)
}

// removeAppImage removes one AppImage from both install locations and its
// desktop entry. Each failed removal is logged as it happens and returned.
func (p *AppimagePlugin) removeAppImage(binaryName string) error {
	installed, lookupErr := p.isAppImageInstalled(binaryName)
	if lookupErr != nil {
		// The status is unknown, so still try to remove both locations.
		p.logger.Warning("Failed to check installation status: %v", lookupErr)
	} else if !installed {
		p.logger.Printf("AppImage %s is not installed, skipping\n", binaryName)
		return nil
	}

	homeDir := os.Getenv("HOME")
	targets := []struct {
		label string
		path  string
	}{
		{"from GUI location", filepath.Join(homeDir, "Applications", binaryName)},
		{"from CLI location", filepath.Join(homeDir, ".local", "bin", binaryName)},
		{"desktop entry", filepath.Join(homeDir, ".local", "share", "applications", binaryName+".desktop")},
	}

	var failures []error
	removed := false
	for i, target := range targets {
		err := os.Remove(target.path)
		switch {
		case err == nil:
			// Only the two binary locations count as removing the AppImage.
			if i < 2 {
				removed = true
			}
		case errors.Is(err, fs.ErrNotExist):
		default:
			p.logger.Warning("Failed to remove %s: %v", target.label, err)
			failures = append(failures, err)
		}
	}

	if len(failures) > 0 {
		return errors.Join(failures...)
	}
	if !removed && lookupErr != nil {
		return fmt.Errorf("could not confirm removal: %w", lookupErr)
	}
	if removed {
		p.logger.Success("AppImage %s removed successfully", binaryName)
	} else {
		p.logger.Printf("⚠️ AppImage %s not found\n", binaryName)
	}
	return nil
}

// handleList lists installed AppImages. Each directory is listed even if the
// other cannot be read; any read failure is returned after both are shown.
func (p *AppimagePlugin) handleList(args []string) error {
	homeDir := os.Getenv("HOME")
	locations := []struct {
		heading string
		dir     string
	}{
		{"🖥️ GUI Applications (~/Applications):\n", filepath.Join(homeDir, "Applications")},
		{"⚙️ CLI Tools (~/.local/bin):\n", filepath.Join(homeDir, ".local", "bin")},
	}

	p.logger.Printf("Installed AppImages:\n")

	var failures []error
	for _, location := range locations {
		p.logger.Printf("\n")
		p.logger.Printf("%s", location.heading)
		if err := p.listAppImagesInDir(location.dir); err != nil {
			p.logger.Printf("  (Error reading directory: %v)\n", err)
			failures = append(failures, fmt.Errorf("failed to list AppImages in %s: %w", location.dir, err))
		}
	}

	return errors.Join(failures...)
}

// handleIsInstalled checks if an AppImage is installed
func (p *AppimagePlugin) handleIsInstalled(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no AppImage name specified")
	}

	binaryName := args[0]
	if err := p.validateBinaryName(binaryName); err != nil {
		return fmt.Errorf("invalid binary name: %w", err)
	}

	installed, err := p.isAppImageInstalled(binaryName)
	if err != nil {
		return fmt.Errorf("failed to check whether AppImage %s is installed: %w", binaryName, err)
	}
	if !installed {
		return &sdk.NotInstalledError{Packages: []string{binaryName}}
	}
	p.logger.Printf("AppImage %s is installed\n", binaryName)
	return nil
}

// handleValidateURL validates AppImage download URLs
func (p *AppimagePlugin) handleValidateURL(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no URL specified")
	}

	// Check every URL, then report every inaccessible one.
	var failures []error
	for _, downloadURL := range args {
		p.logger.Printf("Validating URL: %s\n", downloadURL)

		if err := p.validateURLAccessibility(downloadURL); err != nil {
			p.logger.Printf("❌ %s: %v\n", downloadURL, err)
			failures = append(failures, fmt.Errorf("URL validation failed for %s: %w", downloadURL, err))
		} else {
			p.logger.Printf("✅ %s: URL is accessible\n", downloadURL)
		}
	}

	return errors.Join(failures...)
}

// isAppImageInstalled checks if an AppImage is installed in either location
func (p *AppimagePlugin) isAppImageInstalled(binaryName string) (bool, error) {
	homeDir := os.Getenv("HOME")

	// Installed if either location holds an executable. Absent only when both
	// are confirmed missing or not executable; a stat failure in one location
	// (e.g. permission denied) is reported only if the other doesn't confirm it.
	var lookupErr error
	for _, path := range []string{
		filepath.Join(homeDir, "Applications", binaryName),
		filepath.Join(homeDir, ".local", "bin", binaryName),
	} {
		info, err := os.Stat(path)
		switch {
		case err == nil:
			if info.Mode()&0o111 != 0 {
				return true, nil
			}
		case errors.Is(err, fs.ErrNotExist):
		default:
			if lookupErr == nil {
				lookupErr = fmt.Errorf("check %s: %w", path, err)
			}
		}
	}

	if lookupErr != nil {
		return false, lookupErr
	}
	return false, nil
}

// listAppImagesInDir lists AppImages in a given directory
func (p *AppimagePlugin) listAppImagesInDir(dir string) error {
	files, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		// A location that was never created holds no AppImages.
		p.logger.Printf("  (No AppImages found)\n")
		return nil
	}
	if err != nil {
		return err
	}

	count := 0
	var failures []error
	for _, file := range files {
		if file.IsDir() {
			continue
		}

		// Check if file is executable
		info, err := file.Info()
		if errors.Is(err, fs.ErrNotExist) {
			continue // removed while listing
		}
		if err != nil {
			// Keep listing the rest; the failure is returned below.
			failures = append(failures, fmt.Errorf("inspect %s: %w", filepath.Join(dir, file.Name()), err))
			continue
		}

		if info.Mode()&0o111 != 0 {
			p.logger.Printf("  %s (%s)\n", file.Name(), p.formatFileSize(info.Size()))
			count++
		}
	}

	if count == 0 && len(failures) == 0 {
		p.logger.Printf("  (No AppImages found)\n")
	}

	return errors.Join(failures...)
}

// formatFileSize formats file size in human-readable format
func (p *AppimagePlugin) formatFileSize(size int64) string {
	const unit = 1024
	if size < unit {
		return fmt.Sprintf("%d B", size)
	}
	div, exp := int64(unit), 0
	for n := size / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(size)/float64(div), "KMGTPE"[exp])
}
