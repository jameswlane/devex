package main

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
)

var version = "dev" // Set by goreleaser

// SystemSetupPlugin implements the System setup plugin
type SystemSetupPlugin struct {
	*sdk.BasePlugin
}

// SystemConfig represents a system configuration item
type SystemConfig struct {
	Name        string
	Description string
	Check       func() bool
	Apply       func() error
}

// NewSystemSetupPlugin creates a new SystemSetup plugin
func NewSystemSetupPlugin() *SystemSetupPlugin {
	info := sdk.PluginInfo{
		Name:        "system-setup",
		Version:     version,
		Description: "System-specific configuration and setup",
		Author:      "DevEx Team",
		Repository:  "https://github.com/jameswlane/devex",
		Tags:        []string{"system", "setup", "configuration"},
		Commands: []sdk.PluginCommand{
			{
				Name:        "configure",
				Description: "Configure system settings",
				Usage:       "Apply system-wide configuration changes",
			},
			{
				Name:        "apply",
				Description: "Apply configuration changes",
				Usage:       "Apply pending system configuration changes",
			},
			{
				Name:        "validate",
				Description: "Validate system configuration",
				Usage:       "Check system configuration for issues",
			},
			{
				Name:        "backup",
				Description: "Backup system configuration",
				Usage:       "Create backup of current system configuration",
			},
		},
	}

	return &SystemSetupPlugin{
		BasePlugin: sdk.NewBasePlugin(info),
	}
}

// Execute handles command execution
func (p *SystemSetupPlugin) Execute(command string, args []string) error {
	ctx := context.Background()

	switch command {
	case "configure":
		return p.handleConfigure(ctx, args)
	case "apply":
		return p.handleApply(ctx, args)
	case "validate":
		return p.handleValidate(ctx, args)
	case "backup":
		return p.handleBackup(ctx, args)
	default:
		return fmt.Errorf("unknown command: %s", command)
	}
}

func (p *SystemSetupPlugin) handleConfigure(ctx context.Context, args []string) error {
	fmt.Printf("Configuring system (%s)...\n", runtime.GOOS)

	// Display system information
	fmt.Printf("Operating System: %s\n", runtime.GOOS)
	fmt.Printf("Architecture: %s\n", runtime.GOARCH)

	hostname, err := os.Hostname()
	if err == nil {
		fmt.Printf("Hostname: %s\n", hostname)
	}

	// Configure based on OS
	switch runtime.GOOS {
	case "linux":
		return p.configureLinux(ctx, args)
	case "darwin":
		return p.configureMacOS(ctx, args)
	case "windows":
		return p.configureWindows(ctx, args)
	default:
		return fmt.Errorf("unsupported operating system: %s", runtime.GOOS)
	}
}

func (p *SystemSetupPlugin) configureLinux(ctx context.Context, args []string) error {
	fmt.Println("\nConfiguring Linux system settings...")

	configs := []SystemConfig{
		{
			Name:        "Increase file watcher limit",
			Description: "Increase inotify watches for development tools",
			Check:       p.checkFileWatcherLimit,
			Apply:       p.applyFileWatcherLimit,
		},
		{
			Name:        "Configure swappiness",
			Description: "Optimize swap usage for development workloads",
			Check:       p.checkSwappiness,
			Apply:       p.applySwappiness,
		},
		{
			Name:        "Set up development directories",
			Description: "Create standard development directories",
			Check:       p.checkDevDirectories,
			Apply:       p.applyDevDirectories,
		},
	}

	// Check and optionally apply each configuration
	for _, config := range configs {
		fmt.Printf("\n%s:\n", config.Name)
		fmt.Printf("  %s\n", config.Description)

		if config.Check() {
			fmt.Println("  ✓ Already configured")
		} else {
			fmt.Println("  ✗ Not configured")
			if len(args) > 0 && args[0] == "--apply" {
				if err := config.Apply(); err != nil {
					fmt.Printf("  ❌ Failed to apply: %v\n", err)
				} else {
					fmt.Println("  ✓ Applied successfully")
				}
			}
		}
	}

	if len(args) == 0 || args[0] != "--apply" {
		fmt.Println("\nTo apply these configurations, run: system-setup configure --apply")
	}

	return nil
}

func (p *SystemSetupPlugin) configureMacOS(ctx context.Context, args []string) error {
	fmt.Println("\nConfiguring macOS system settings...")

	configs := []SystemConfig{
		{
			Name:        "Show hidden files in Finder",
			Description: "Make hidden files visible in Finder",
			Check:       p.checkFinderHiddenFiles,
			Apply:       p.applyFinderHiddenFiles,
		},
		{
			Name:        "Enable developer mode",
			Description: "Enable developer mode for debugging",
			Check:       p.checkDeveloperMode,
			Apply:       p.applyDeveloperMode,
		},
		{
			Name:        "Disable Gatekeeper for development",
			Description: "Allow unsigned applications during development",
			Check:       p.checkGatekeeper,
			Apply:       p.applyGatekeeper,
		},
	}

	// Check and optionally apply each configuration
	for _, config := range configs {
		fmt.Printf("\n%s:\n", config.Name)
		fmt.Printf("  %s\n", config.Description)

		if config.Check() {
			fmt.Println("  ✓ Already configured")
		} else {
			fmt.Println("  ✗ Not configured")
			if len(args) > 0 && args[0] == "--apply" {
				if err := config.Apply(); err != nil {
					fmt.Printf("  ❌ Failed to apply: %v\n", err)
				} else {
					fmt.Println("  ✓ Applied successfully")
				}
			}
		}
	}

	if len(args) == 0 || args[0] != "--apply" {
		fmt.Println("\nTo apply these configurations, run: system-setup configure --apply")
	}

	return nil
}

func (p *SystemSetupPlugin) configureWindows(ctx context.Context, args []string) error {
	fmt.Println("\nConfiguring Windows system settings...")

	configs := []SystemConfig{
		{
			Name:        "Enable developer mode",
			Description: "Enable Windows developer mode",
			Check:       p.checkWindowsDeveloperMode,
			Apply:       p.applyWindowsDeveloperMode,
		},
		{
			Name:        "Configure Windows Terminal",
			Description: "Set up Windows Terminal for development",
			Check:       p.checkWindowsTerminal,
			Apply:       p.applyWindowsTerminal,
		},
		{
			Name:        "Enable WSL",
			Description: "Enable Windows Subsystem for Linux",
			Check:       p.checkWSL,
			Apply:       p.applyWSL,
		},
	}

	// Check and optionally apply each configuration
	for _, config := range configs {
		fmt.Printf("\n%s:\n", config.Name)
		fmt.Printf("  %s\n", config.Description)

		if config.Check() {
			fmt.Println("  ✓ Already configured")
		} else {
			fmt.Println("  ✗ Not configured")
			if len(args) > 0 && args[0] == "--apply" {
				if err := config.Apply(); err != nil {
					fmt.Printf("  ❌ Failed to apply: %v\n", err)
				} else {
					fmt.Println("  ✓ Applied successfully")
				}
			}
		}
	}

	if len(args) == 0 || args[0] != "--apply" {
		fmt.Println("\nTo apply these configurations, run: system-setup configure --apply")
	}

	return nil
}

func (p *SystemSetupPlugin) handleApply(ctx context.Context, args []string) error {
	fmt.Println("Applying system configuration changes...")

	// Call configure with --apply flag
	return p.handleConfigure(ctx, []string{"--apply"})
}

func (p *SystemSetupPlugin) handleValidate(ctx context.Context, args []string) error {
	fmt.Println("Validating system configuration...")

	// Basic system validation
	fmt.Println("System Validation Report:")
	fmt.Printf("✓ Operating System: %s\n", runtime.GOOS)
	fmt.Printf("✓ Architecture: %s\n", runtime.GOARCH)

	// Check if running as root (on Unix systems)
	if runtime.GOOS != "windows" {
		uid := os.Getuid()
		if uid == 0 {
			fmt.Println("⚠ Running as root user")
		} else {
			fmt.Printf("✓ Running as user ID: %d\n", uid)
		}
	}

	// TODO: Implement comprehensive system validation
	return nil
}

func (p *SystemSetupPlugin) handleBackup(ctx context.Context, args []string) error {
	fmt.Println("Backing up system configuration...")

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}

	// Create a backup directory
	backupDir := filepath.Join(homeDir, ".devex", "backups", "system")
	timestamp := time.Now().Format("20060102-150405")
	backupPath := filepath.Join(backupDir, timestamp)

	if err := os.MkdirAll(backupPath, 0755); err != nil {
		return fmt.Errorf("failed to create backup directory: %w", err)
	}

	// Backup system information
	sysInfo := fmt.Sprintf("OS: %s\nArch: %s\nHostname: %s\nBackup Time: %s\n",
		runtime.GOOS, runtime.GOARCH, getHostname(), timestamp)

	infoFile := filepath.Join(backupPath, "system-info.txt")
	if err := os.WriteFile(infoFile, []byte(sysInfo), 0644); err != nil {
		return fmt.Errorf("failed to write system info: %w", err)
	}

	// Backup configuration files based on OS
	switch runtime.GOOS {
	case "linux":
		p.backupLinuxConfigs(backupPath)
	case "darwin":
		p.backupMacOSConfigs(backupPath)
	case "windows":
		p.backupWindowsConfigs(backupPath)
	}

	fmt.Printf("\nBackup completed: %s\n", backupPath)
	return nil
}

// Helper functions for Linux
func (p *SystemSetupPlugin) checkFileWatcherLimit() bool {
	content, err := os.ReadFile("/proc/sys/fs/inotify/max_user_watches")
	if err != nil {
		return false
	}
	limit := strings.TrimSpace(string(content))
	return limit == "524288" || limit == "1048576"
}

func (p *SystemSetupPlugin) applyFileWatcherLimit() error {
	// Apply temporarily
	if err := os.WriteFile("/proc/sys/fs/inotify/max_user_watches", []byte("524288"), 0644); err != nil {
		if !sdk.IsRoot() {
			return fmt.Errorf("requires root privileges")
		}
		return err
	}

	// Make permanent
	sysctlConf := "/etc/sysctl.d/99-devex.conf"
	content := "fs.inotify.max_user_watches=524288\n"
	return sdk.ExecCommandWithContext(context.Background(), true, "sh", "-c", fmt.Sprintf("echo '%s' > %s", content, sysctlConf))
}

func (p *SystemSetupPlugin) checkSwappiness() bool {
	content, err := os.ReadFile("/proc/sys/vm/swappiness")
	if err != nil {
		return false
	}
	swappiness := strings.TrimSpace(string(content))
	return swappiness == "10"
}

func (p *SystemSetupPlugin) applySwappiness() error {
	// Apply temporarily
	if err := os.WriteFile("/proc/sys/vm/swappiness", []byte("10"), 0644); err != nil {
		if !sdk.IsRoot() {
			return fmt.Errorf("requires root privileges")
		}
		return err
	}

	// Make permanent
	sysctlConf := "/etc/sysctl.d/99-devex.conf"
	content := "vm.swappiness=10\n"
	return sdk.ExecCommandWithContext(context.Background(), true, "sh", "-c", fmt.Sprintf("echo '%s' >> %s", content, sysctlConf))
}

func (p *SystemSetupPlugin) checkDevDirectories() bool {
	homeDir, _ := os.UserHomeDir()
	dirs := []string{
		filepath.Join(homeDir, "Development"),
		filepath.Join(homeDir, "Projects"),
		filepath.Join(homeDir, ".devex"),
	}

	for _, dir := range dirs {
		if _, err := os.Stat(dir); os.IsNotExist(err) {
			return false
		}
	}
	return true
}

func (p *SystemSetupPlugin) applyDevDirectories() error {
	homeDir, _ := os.UserHomeDir()
	dirs := []string{
		filepath.Join(homeDir, "Development"),
		filepath.Join(homeDir, "Projects"),
		filepath.Join(homeDir, ".devex"),
		filepath.Join(homeDir, ".devex", "backups"),
		filepath.Join(homeDir, ".devex", "configs"),
	}

	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create %s: %w", dir, err)
		}
	}
	return nil
}

// Helper functions for macOS
func (p *SystemSetupPlugin) checkFinderHiddenFiles() bool {
	output, err := sdk.RunCommand("defaults", "read", "com.apple.finder", "AppleShowAllFiles")
	if err != nil {
		return false
	}
	return strings.TrimSpace(output) == "YES" || strings.TrimSpace(output) == "1"
}

func (p *SystemSetupPlugin) applyFinderHiddenFiles() error {
	if err := sdk.ExecCommandWithContext(context.Background(), false, "defaults", "write", "com.apple.finder", "AppleShowAllFiles", "YES"); err != nil {
		return err
	}
	return sdk.ExecCommandWithContext(context.Background(), false, "killall", "Finder")
}

func (p *SystemSetupPlugin) checkDeveloperMode() bool {
	output, err := sdk.RunCommand("DevToolsSecurity", "-status")
	if err != nil {
		return false
	}
	return strings.Contains(output, "Developer mode is currently enabled")
}

func (p *SystemSetupPlugin) applyDeveloperMode() error {
	return sdk.ExecCommandWithContext(context.Background(), true, "DevToolsSecurity", "-enable")
}

func (p *SystemSetupPlugin) checkGatekeeper() bool {
	output, err := sdk.RunCommand("spctl", "--status")
	if err != nil {
		return false
	}
	return strings.Contains(output, "assessments disabled")
}

func (p *SystemSetupPlugin) applyGatekeeper() error {
	fmt.Println("WARNING: This will disable Gatekeeper security. Use with caution!")
	return sdk.ExecCommandWithContext(context.Background(), true, "spctl", "--master-disable")
}

// Helper functions for Windows
func (p *SystemSetupPlugin) checkWindowsDeveloperMode() bool {
	// Check registry for developer mode
	output, err := sdk.RunCommand("reg", "query", "HKLM\\SOFTWARE\\Microsoft\\Windows\\CurrentVersion\\AppModelUnlock", "/v", "AllowDevelopmentWithoutDevLicense")
	if err != nil {
		return false
	}
	return strings.Contains(output, "0x1")
}

func (p *SystemSetupPlugin) applyWindowsDeveloperMode() error {
	return sdk.ExecCommandWithContext(context.Background(), true, "reg", "add", "HKLM\\SOFTWARE\\Microsoft\\Windows\\CurrentVersion\\AppModelUnlock", "/v", "AllowDevelopmentWithoutDevLicense", "/t", "REG_DWORD", "/d", "1", "/f")
}

func (p *SystemSetupPlugin) checkWindowsTerminal() bool {
	return sdk.CommandExists("wt")
}

func (p *SystemSetupPlugin) applyWindowsTerminal() error {
	if sdk.CommandExists("winget") {
		return sdk.ExecCommandWithContext(context.Background(), false, "winget", "install", "Microsoft.WindowsTerminal")
	}
	return fmt.Errorf("winget not available to install Windows Terminal")
}

func (p *SystemSetupPlugin) checkWSL() bool {
	output, err := sdk.RunCommand("wsl", "--status")
	return err == nil && strings.Contains(output, "Default Version:")
}

func (p *SystemSetupPlugin) applyWSL() error {
	return sdk.ExecCommandWithContext(context.Background(), true, "wsl", "--install")
}

// Backup helper functions
func (p *SystemSetupPlugin) backupLinuxConfigs(backupPath string) {
	configs := []string{
		"/etc/sysctl.conf",
		"/etc/sysctl.d/",
		"/etc/security/limits.conf",
	}

	for _, config := range configs {
		if _, err := os.Stat(config); err == nil {
			name := strings.ReplaceAll(config, "/", "_")
			if err := sdk.ExecCommandWithContext(context.Background(), false, "cp", "-r", config, filepath.Join(backupPath, name)); err != nil {
				fmt.Printf("Warning: Failed to backup %s: %v\n", config, err)
			}
		}
	}
}

func (p *SystemSetupPlugin) backupMacOSConfigs(backupPath string) {
	// Backup macOS preferences
	prefs := []string{
		"com.apple.finder",
		"com.apple.dock",
		"com.apple.Terminal",
	}

	for _, pref := range prefs {
		output, err := sdk.RunCommand("defaults", "read", pref)
		if err == nil {
			prefFile := filepath.Join(backupPath, pref+".plist")
			if err := os.WriteFile(prefFile, []byte(output), 0644); err != nil {
				fmt.Printf("Warning: Failed to write preference file %s: %v\n", prefFile, err)
			}
		}
	}
}

func (p *SystemSetupPlugin) backupWindowsConfigs(backupPath string) {
	// Backup Windows registry keys
	keys := []string{
		"HKLM\\SOFTWARE\\Microsoft\\Windows\\CurrentVersion\\AppModelUnlock",
	}

	for _, key := range keys {
		name := strings.ReplaceAll(key, "\\", "_") + ".reg"
		if err := sdk.ExecCommandWithContext(context.Background(), false, "reg", "export", key, filepath.Join(backupPath, name)); err != nil {
			fmt.Printf("Warning: Failed to export registry key %s: %v\n", key, err)
		}
	}
}

func getHostname() string {
	hostname, err := os.Hostname()
	if err != nil {
		return "unknown"
	}
	return hostname
}

func main() {
	plugin := NewSystemSetupPlugin()
	sdk.HandleArgs(plugin, os.Args[1:])
}
