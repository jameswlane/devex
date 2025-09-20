package main

import (
	"fmt"
	"os"
	"path/filepath"
)

// createDesktopEntry creates a .desktop file for GUI AppImages
func (p *AppimagePlugin) createDesktopEntry(binaryName, binaryPath string) error {
	homeDir := os.Getenv("HOME")
	desktopDir := filepath.Join(homeDir, ".local", "share", "applications")

	if err := os.MkdirAll(desktopDir, 0o755); err != nil {
		return fmt.Errorf("failed to create applications directory: %w", err)
	}

	desktopFile := filepath.Join(desktopDir, binaryName+".desktop")
	content := fmt.Sprintf(`[Desktop Entry]
Version=1.0
Type=Application
Name=%s
Comment=AppImage application
Exec=%s
Icon=application-x-executable
Categories=Utility;
Terminal=false
`, binaryName, binaryPath)

	if err := os.WriteFile(desktopFile, []byte(content), 0o644); err != nil {
		return fmt.Errorf("failed to write desktop file: %w", err)
	}

	p.logger.Debug("Created desktop entry: %s", desktopFile)
	return nil
}

// RemoveDesktopEntry removes a .desktop file
func (p *AppimagePlugin) RemoveDesktopEntry(binaryName string) error {
	homeDir := os.Getenv("HOME")
	desktopFile := filepath.Join(homeDir, ".local", "share", "applications", binaryName+".desktop")

	if _, err := os.Stat(desktopFile); os.IsNotExist(err) {
		return nil // File doesn't exist, nothing to remove
	}

	if err := os.Remove(desktopFile); err != nil {
		return fmt.Errorf("failed to remove desktop entry: %w", err)
	}

	p.logger.Debug("Removed desktop entry: %s", desktopFile)
	return nil
}

// UpdateDesktopDatabase updates the desktop database after changes
func (p *AppimagePlugin) UpdateDesktopDatabase() error {
	// This would typically run update-desktop-database if available
	// For now, we'll just log that we should update it
	p.logger.Debug("Desktop entry created - you may need to refresh your application menu")
	return nil
}
