package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
)

// DesktopManager handles core KDE desktop configuration
type DesktopManager struct{}

// NewDesktopManager creates a new desktop manager instance
func NewDesktopManager() *DesktopManager {
	return &DesktopManager{}
}

// Configure applies comprehensive KDE configuration
func (dm *DesktopManager) Configure(ctx context.Context, args []string) error {
	fmt.Println("Configuring KDE Plasma desktop environment...")

	if !sdk.CommandExists("kwriteconfig5") {
		return fmt.Errorf("kwriteconfig5 not found. Please ensure KDE Plasma is properly installed")
	}

	configs := dm.getDefaultConfigurations()

	for _, config := range configs {
		if err := dm.setKDEConfig(ctx, config); err != nil {
			fmt.Printf("Warning: Failed to set %s.%s: %v\n", config.Group, config.Key, err)
		} else {
			fmt.Printf("✓ Set %s.%s to %s\n", config.Group, config.Key, config.Value)
		}
	}

	fmt.Println("KDE Plasma configuration complete!")
	fmt.Println("You may need to restart Plasma or log out for all changes to take effect.")
	return nil
}

// SetBackground sets the desktop wallpaper
func (dm *DesktopManager) SetBackground(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("please provide a path to the wallpaper image")
	}

	wallpaperPath := args[0]

	// Check if file exists
	if _, err := os.Stat(wallpaperPath); err != nil {
		return fmt.Errorf("wallpaper file not found: %s", wallpaperPath)
	}

	// Get absolute path
	absPath, err := filepath.Abs(wallpaperPath)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Set wallpaper using plasma-apply-wallpaperimage if available
	if sdk.CommandExists("plasma-apply-wallpaperimage") {
		if err := executeCommand(ctx, "plasma-apply-wallpaperimage", absPath); err != nil {
			return fmt.Errorf("failed to set wallpaper: %w", err)
		}
	} else {
		// Fallback to kwriteconfig5
		if err := dm.setWallpaperFallback(ctx, absPath); err != nil {
			return fmt.Errorf("failed to set wallpaper: %w", err)
		}
	}

	fmt.Printf("✓ Wallpaper set to: %s\n", wallpaperPath)
	return nil
}

// ConfigurePanel configures the KDE Plasma panel
func (dm *DesktopManager) ConfigurePanel(ctx context.Context, args []string) error {
	fmt.Println("Configuring KDE Plasma panel...")

	if !sdk.CommandExists("kwriteconfig5") {
		return fmt.Errorf("kwriteconfig5 not found")
	}

	panelConfigs := dm.getPanelConfigurations()

	for _, config := range panelConfigs {
		if err := dm.setKDEConfig(ctx, config); err != nil {
			fmt.Printf("Warning: Failed to set %s: %v\n", config.Desc, err)
		} else {
			fmt.Printf("✓ Set %s\n", config.Desc)
		}
	}

	fmt.Println("Panel configuration complete!")
	fmt.Println("You may need to restart Plasma for all changes to take effect.")
	return nil
}

// RestartPlasma restarts the Plasma Shell with proper error recovery
func (dm *DesktopManager) RestartPlasma(ctx context.Context, args []string) error {
	fmt.Println("Restarting KDE Plasma Shell...")

	// Check if plasmashell is running first
	running, err := dm.isProcessRunning(ctx, "plasmashell")
	if err != nil {
		fmt.Printf("Warning: Could not check if plasmashell is running: %v\n", err)
	}

	if !running {
		fmt.Println("plasmashell is not running, starting it...")
		return dm.startPlasmaShell(ctx)
	}

	// Kill plasmashell with timeout
	if err := dm.killProcessWithTimeout(ctx, "plasmashell", 10*time.Second); err != nil {
		return fmt.Errorf("failed to stop plasmashell: %w", err)
	}

	// Wait for process to fully terminate
	time.Sleep(2 * time.Second)

	// Start plasmashell with proper process management
	if err := dm.startPlasmaShell(ctx); err != nil {
		return fmt.Errorf("failed to restart plasmashell: %w", err)
	}

	fmt.Println("✓ Plasma Shell restarted successfully!")
	return nil
}

// getDefaultConfigurations returns the default KDE configurations
func (dm *DesktopManager) getDefaultConfigurations() []KDEConfig {
	return []KDEConfig{
		// Desktop effects
		{
			File:  "kwinrc",
			Group: "Compositing",
			Key:   "Enabled",
			Value: "true",
			Desc:  "Enable desktop compositing",
		},
		{
			File:  "kwinrc",
			Group: "Compositing",
			Key:   "AnimationSpeed",
			Value: "3",
			Desc:  "Animation speed (normal)",
		},
		// Window behavior
		{
			File:  "kwinrc",
			Group: "Windows",
			Key:   "FocusPolicy",
			Value: "ClickToFocus",
			Desc:  "Click to focus windows",
		},
		{
			File:  "kwinrc",
			Group: "Windows",
			Key:   "AutoRaise",
			Value: "false",
			Desc:  "Disable auto-raise",
		},
		// Touchpad settings
		{
			File:  "kcminputrc",
			Group: "Mouse",
			Key:   "ReverseScrollPolarity",
			Value: "true",
			Desc:  "Natural scrolling",
		},
		// Desktop settings
		{
			File:  "kdeglobals",
			Group: "KDE",
			Key:   "SingleClick",
			Value: "false",
			Desc:  "Double-click to open files",
		},
		// Panel settings
		{
			File:  "plasmashellrc",
			Group: "PlasmaViews",
			Key:   "panelVisibility",
			Value: "0",
			Desc:  "Always visible panel",
		},
	}
}

// getPanelConfigurations returns panel-specific configurations
func (dm *DesktopManager) getPanelConfigurations() []KDEConfig {
	return []KDEConfig{
		{
			File:  "plasmashellrc",
			Group: "PlasmaViews",
			Key:   "panelVisibility",
			Value: "0",
			Desc:  "Panel always visible",
		},
		{
			File:  "plasmarc",
			Group: "Theme",
			Key:   "name",
			Value: "default",
			Desc:  "Default plasma theme",
		},
		{
			File:  "kdeglobals",
			Group: "Icons",
			Key:   "Theme",
			Value: "breeze",
			Desc:  "Breeze icon theme",
		},
	}
}

// setKDEConfig sets a KDE configuration using kwriteconfig5
func (dm *DesktopManager) setKDEConfig(ctx context.Context, config KDEConfig) error {
	cmd := exec.CommandContext(ctx, "kwriteconfig5",
		"--file", config.File,
		"--group", config.Group,
		"--key", config.Key,
		config.Value)

	return cmd.Run()
}

// setWallpaperFallback sets wallpaper using kwriteconfig5 fallback method
func (dm *DesktopManager) setWallpaperFallback(ctx context.Context, wallpaperPath string) error {
	// This is a simplified fallback - actual KDE wallpaper setting is complex
	// involving desktop containments and activities
	config := KDEConfig{
		File:  "plasma-org.kde.plasma.desktop-appletsrc",
		Group: "Containments",
		Key:   "wallpaper",
		Value: wallpaperPath,
		Desc:  "Desktop wallpaper",
	}

	return dm.setKDEConfig(ctx, config)
}

// isProcessRunning checks if a process is currently running
func (dm *DesktopManager) isProcessRunning(ctx context.Context, processName string) (bool, error) {
	cmd := exec.CommandContext(ctx, "pgrep", processName)
	err := cmd.Run()
	if err == nil {
		return true, nil
	}
	if exitErr, ok := err.(*exec.ExitError); ok && exitErr.ExitCode() == 1 {
		// pgrep returns 1 when no processes found
		return false, nil
	}
	return false, err
}

// killProcessWithTimeout kills a process with a timeout and fallback to SIGKILL
func (dm *DesktopManager) killProcessWithTimeout(ctx context.Context, processName string, timeout time.Duration) error {
	// Create context with timeout
	ctxTimeout, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// First try gentle termination with SIGTERM
	cmd := exec.CommandContext(ctxTimeout, "killall", "-TERM", processName)
	if err := cmd.Run(); err != nil {
		// If gentle kill fails, try SIGKILL as fallback
		cmd = exec.CommandContext(ctxTimeout, "killall", "-KILL", processName)
		return cmd.Run()
	}

	// Wait a bit for graceful shutdown
	time.Sleep(1 * time.Second)

	// Check if process is still running
	running, err := dm.isProcessRunning(ctx, processName)
	if err != nil {
		return err
	}
	if running {
		// Process still running, force kill
		cmd = exec.CommandContext(ctxTimeout, "killall", "-KILL", processName)
		return cmd.Run()
	}

	return nil
}

// startPlasmaShell starts the plasma shell process with proper resource management
func (dm *DesktopManager) startPlasmaShell(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "plasmashell")

	// Start the process
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start plasmashell: %w", err)
	}

	// Ensure process cleanup on context cancellation
	go func() {
		<-ctx.Done()
		if cmd.Process != nil {
			if err := cmd.Process.Kill(); err != nil {
				// Process might already be dead, log but don't fail
				fmt.Printf("Warning: Failed to kill process: %v\n", err)
			}
		}
	}()

	// Wait a moment to ensure successful start
	time.Sleep(1 * time.Second)

	// Verify the process started successfully
	running, err := dm.isProcessRunning(context.Background(), "plasmashell")
	if err != nil {
		return fmt.Errorf("failed to verify plasmashell started: %w", err)
	}
	if !running {
		return fmt.Errorf("plasmashell failed to start properly")
	}

	return nil
}
