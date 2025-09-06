package main

import (
	"fmt"
	"os"
	"strings"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
)

// handleEnsureInstalled installs Flatpak system-wide if not present
func (f *FlatpakInstaller) handleEnsureInstalled(args []string) error {
	f.logger.Printf("Checking if Flatpak is installed...\n")

	// Check if flatpak command exists
	if sdk.CommandExists("flatpak") {
		if _, err := f.getFlatpakVersion(); err == nil {
			f.logger.Success("Flatpak is already installed")
			return nil
		}
	}

	f.logger.Printf("Installing Flatpak system-wide...\n")

	// Detect OS and install accordingly
	if err := f.installFlatpakForSystem(); err != nil {
		return fmt.Errorf("failed to install Flatpak: %w", err)
	}

	// Verify installation
	if !sdk.CommandExists("flatpak") {
		return fmt.Errorf("Flatpak installation verification failed")
	}

	f.logger.Success("Flatpak installed successfully")
	return nil
}

// installFlatpakForSystem installs Flatpak based on the detected system
func (f *FlatpakInstaller) installFlatpakForSystem() error {
	// Try apt-get first (Debian/Ubuntu)
	if sdk.CommandExists("apt-get") {
		f.logger.Printf("Installing Flatpak using apt-get...\n")
		if err := f.installFlatpakDebian(); err != nil {
			return err
		}
	} else if sdk.CommandExists("dnf") {
		// Try dnf (Fedora/RHEL)
		f.logger.Printf("Installing Flatpak using dnf...\n")
		if err := f.installFlatpakFedora(); err != nil {
			return err
		}
	} else if sdk.CommandExists("pacman") {
		// Try pacman (Arch Linux)
		f.logger.Printf("Installing Flatpak using pacman...\n")
		if err := f.installFlatpakArch(); err != nil {
			return err
		}
	} else if sdk.CommandExists("zypper") {
		// Try zypper (openSUSE)
		f.logger.Printf("Installing Flatpak using zypper...\n")
		if err := f.installFlatpakSUSE(); err != nil {
			return err
		}
	} else {
		return fmt.Errorf("unsupported system: no supported package manager found")
	}

	return nil
}

// installFlatpakDebian installs Flatpak on Debian/Ubuntu systems
func (f *FlatpakInstaller) installFlatpakDebian() error {
	// Update package lists
	if err := sdk.ExecCommand(true, "apt-get", "update"); err != nil {
		f.logger.Warning("Failed to update package lists: %v", err)
	}

	// Install Flatpak
	if err := sdk.ExecCommand(true, "apt-get", "install", "-y", "flatpak"); err != nil {
		return fmt.Errorf("failed to install Flatpak via apt-get: %w", err)
	}

	// Install GNOME Software Flatpak plugin if GNOME is detected
	if f.isGnomeDesktop() {
		f.logger.Printf("GNOME desktop detected, installing GNOME Software Flatpak plugin...\n")
		if err := sdk.ExecCommand(true, "apt-get", "install", "-y", "gnome-software-plugin-flatpak"); err != nil {
			f.logger.Warning("Failed to install GNOME Software Flatpak plugin: %v", err)
		}
	}

	return nil
}

// installFlatpakFedora installs Flatpak on Fedora/RHEL systems
func (f *FlatpakInstaller) installFlatpakFedora() error {
	// Flatpak is usually pre-installed on Fedora, but install if missing
	if err := sdk.ExecCommand(true, "dnf", "install", "-y", "flatpak"); err != nil {
		return fmt.Errorf("failed to install Flatpak via dnf: %w", err)
	}

	return nil
}

// installFlatpakArch installs Flatpak on Arch Linux
func (f *FlatpakInstaller) installFlatpakArch() error {
	if err := sdk.ExecCommand(true, "pacman", "-S", "--noconfirm", "flatpak"); err != nil {
		return fmt.Errorf("failed to install Flatpak via pacman: %w", err)
	}

	return nil
}

// installFlatpakSUSE installs Flatpak on openSUSE systems
func (f *FlatpakInstaller) installFlatpakSUSE() error {
	if err := sdk.ExecCommand(true, "zypper", "install", "-y", "flatpak"); err != nil {
		return fmt.Errorf("failed to install Flatpak via zypper: %w", err)
	}

	return nil
}

// isGnomeDesktop checks if GNOME desktop environment is running
func (f *FlatpakInstaller) isGnomeDesktop() bool {
	// Check common environment variables for GNOME
	desktopSession := os.Getenv("XDG_CURRENT_DESKTOP")
	gdmSession := os.Getenv("GDMSESSION")
	
	return strings.Contains(strings.ToLower(desktopSession), "gnome") ||
		   strings.Contains(strings.ToLower(gdmSession), "gnome")
}