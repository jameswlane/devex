package main

import (
	"context"
	"fmt"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
)

// WidgetManager handles KDE Plasma widgets
type WidgetManager struct{}

// NewWidgetManager creates a new widget manager instance
func NewWidgetManager() *WidgetManager {
	return &WidgetManager{}
}

// InstallWidgets installs KDE Plasma widgets
func (wm *WidgetManager) InstallWidgets(ctx context.Context, args []string) error {
	fmt.Println("Installing KDE Plasma widgets...")

	// Check if plasmapkg2 is available
	if !sdk.CommandExists("plasmapkg2") {
		return fmt.Errorf("plasmapkg2 tool not found. Please ensure KDE Plasma is properly installed")
	}

	widgets := wm.getRecommendedWidgets()

	fmt.Println("\nRecommended widgets:")
	for i, widget := range widgets {
		fmt.Printf("%d. %s - %s\n", i+1, widget.Name, widget.Description)
	}

	fmt.Println("\nNote: Widgets can be installed from:")
	fmt.Println("  - KDE Store (store.kde.org)")
	fmt.Println("  - System Settings > Workspace > Startup and Shutdown > Plasma")
	fmt.Println("  - Right-click on desktop > Add Widgets")

	return nil
}

// ListWidgets lists installed widgets
func (wm *WidgetManager) ListWidgets(ctx context.Context, args []string) error {
	fmt.Println("Listing KDE Plasma widgets...")

	if !sdk.CommandExists("plasmapkg2") {
		return fmt.Errorf("plasmapkg2 tool not found")
	}

	// List installed plasmoids
	fmt.Println("\nInstalled Plasmoids:")
	if err := sdk.ExecCommandWithContext(ctx, false, "plasmapkg2", "--list"); err != nil {
		fmt.Printf("Warning: Failed to list installed plasmoids: %v\n", err)
	}

	return nil
}

// RemoveWidget removes a widget
func (wm *WidgetManager) RemoveWidget(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("please provide a widget name to remove")
	}

	widgetName := args[0]
	fmt.Printf("Removing widget: %s\n", widgetName)

	if !sdk.CommandExists("plasmapkg2") {
		return fmt.Errorf("plasmapkg2 tool not found")
	}

	if err := sdk.ExecCommandWithContext(ctx, false, "plasmapkg2", "--remove", widgetName); err != nil {
		return fmt.Errorf("failed to remove widget %s: %w", widgetName, err)
	}

	fmt.Printf("✓ Widget '%s' removed successfully!\n", widgetName)
	return nil
}

// ConfigureWidget configures widget settings
func (wm *WidgetManager) ConfigureWidget(ctx context.Context, args []string) error {
	fmt.Println("Widget configuration help:")
	fmt.Println()
	fmt.Println("To configure widgets:")
	fmt.Println("  1. Right-click on the widget")
	fmt.Println("  2. Select 'Configure Widget' or 'Settings'")
	fmt.Println("  3. Adjust settings as needed")
	fmt.Println()
	fmt.Println("To add new widgets:")
	fmt.Println("  1. Right-click on desktop or panel")
	fmt.Println("  2. Select 'Add Widgets'")
	fmt.Println("  3. Browse and add desired widgets")
	fmt.Println()
	fmt.Println("Widget configuration is typically done through the GUI.")

	return nil
}

// getRecommendedWidgets returns a list of recommended KDE Plasma widgets
func (wm *WidgetManager) getRecommendedWidgets() []WidgetInfo {
	return []WidgetInfo{
		{
			Name:        "System Monitor",
			PackageName: "org.kde.plasma.systemmonitor",
			Description: "Monitor system resources like CPU, memory, and network",
			Category:    "System Information",
		},
		{
			Name:        "Weather Widget",
			PackageName: "org.kde.plasma.weather",
			Description: "Display current weather conditions and forecasts",
			Category:    "Online Services",
		},
		{
			Name:        "Digital Clock",
			PackageName: "org.kde.plasma.digitalclock",
			Description: "Customizable digital clock with calendar",
			Category:    "Date & Time",
		},
		{
			Name:        "Network Speed",
			PackageName: "org.kde.netspeedWidget",
			Description: "Monitor network upload and download speeds",
			Category:    "System Information",
		},
		{
			Name:        "System Tray",
			PackageName: "org.kde.plasma.systemtray",
			Description: "Container for system notification area icons",
			Category:    "System",
		},
		{
			Name:        "Task Manager",
			PackageName: "org.kde.plasma.taskmanager",
			Description: "Shows running applications and windows",
			Category:    "Windows and Tasks",
		},
		{
			Name:        "Application Launcher",
			PackageName: "org.kde.plasma.kickoff",
			Description: "Application menu and launcher",
			Category:    "Application Launchers",
		},
		{
			Name:        "Desktop Folder View",
			PackageName: "org.kde.plasma.folder",
			Description: "Display and manage files on the desktop",
			Category:    "File Management",
		},
		{
			Name:        "Notes",
			PackageName: "org.kde.plasma.notes",
			Description: "Sticky notes for quick reminders",
			Category:    "Utilities",
		},
		{
			Name:        "Media Player Controller",
			PackageName: "org.kde.plasma.mediacontroller",
			Description: "Control media playback from the desktop",
			Category:    "Multimedia",
		},
	}
}
