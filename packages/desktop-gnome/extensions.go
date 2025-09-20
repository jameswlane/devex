package main

import (
	"context"
	"fmt"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
)

// ExtensionManager handles GNOME Shell extensions
type ExtensionManager struct{}

// NewExtensionManager creates a new extension manager instance
func NewExtensionManager() *ExtensionManager {
	return &ExtensionManager{}
}

// InstallExtensions installs GNOME Shell extensions
func (em *ExtensionManager) InstallExtensions(ctx context.Context, args []string) error {
	fmt.Println("Installing GNOME extensions...")

	// Check if gnome-extensions tool is available
	if !sdk.CommandExists("gnome-extensions") {
		return fmt.Errorf("gnome-extensions tool not found. Please install gnome-shell-extensions package")
	}

	extensions := em.getRecommendedExtensions()

	fmt.Println("\nRecommended extensions:")
	for i, ext := range extensions {
		fmt.Printf("%d. %s - %s\n", i+1, ext.name, ext.description)
	}

	fmt.Println("\nNote: Extensions should be installed from https://extensions.gnome.org/")
	fmt.Println("Visit the website and install the GNOME Shell integration browser extension.")

	return nil
}

// extensionInfo holds information about GNOME extensions
type extensionInfo struct {
	uuid        string
	name        string
	description string
}

// getRecommendedExtensions returns a list of recommended GNOME extensions
func (em *ExtensionManager) getRecommendedExtensions() []extensionInfo {
	return []extensionInfo{
		{
			uuid:        "dash-to-dock@micxgx.gmail.com",
			name:        "Dash to Dock",
			description: "Transform the dash into a dock",
		},
		{
			uuid:        "appindicatorsupport@rgcjonas.gmail.com",
			name:        "AppIndicator Support",
			description: "Support for tray icons",
		},
		{
			uuid:        "blur-my-shell@aunetx",
			name:        "Blur my Shell",
			description: "Blur effect for GNOME Shell",
		},
		{
			uuid:        "user-theme@gnome-shell-extensions.gcampax.github.com",
			name:        "User Themes",
			description: "Load shell themes from user directory",
		},
		{
			uuid:        "places-menu@gnome-shell-extensions.gcampax.github.com",
			name:        "Places Status Indicator",
			description: "Add a places menu to the top bar",
		},
		{
			uuid:        "workspace-indicator@gnome-shell-extensions.gcampax.github.com",
			name:        "Workspace Indicator",
			description: "Put a workspace indicator in the top bar",
		},
		{
			uuid:        "screenshot-window-sizer@gnome-shell-extensions.gcampax.github.com",
			name:        "Screenshot Window Sizer",
			description: "Easily resize windows for screenshots",
		},
	}
}
