package platform

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"time"
)

// Platform represents the detected platform information
type Platform struct {
	OS              string   // windows, darwin, linux
	Distribution    string   // ubuntu, fedora, arch, etc. (Linux only)
	DesktopEnv      string   // gnome, kde, xfce, etc.
	Version         string   // OS/distro version
	Architecture    string   // amd64, arm64, etc.
	PackageManagers []string // apt, yum, pacman, brew, choco, etc.
}

// Detector handles platform detection with caching
type Detector struct {
	mu             sync.RWMutex
	cachedPlatform *Platform
}

// NewDetector creates a new platform detector
func NewDetector() *Detector {
	return &Detector{}
}

// DetectPlatform detects the current platform and available package managers with caching
func (d *Detector) DetectPlatform() (*Platform, error) {
	// Check if we already have cached platform info
	d.mu.RLock()
	if d.cachedPlatform != nil {
		cached := *d.cachedPlatform // Return a copy to prevent mutation
		d.mu.RUnlock()
		return &cached, nil
	}
	d.mu.RUnlock()

	// Acquire write lock for detection
	d.mu.Lock()
	defer d.mu.Unlock()

	// Double-check in case another goroutine populated the cache
	if d.cachedPlatform != nil {
		cached := *d.cachedPlatform
		return &cached, nil
	}

	// Perform actual detection
	platform := &Platform{
		OS:           runtime.GOOS,
		Architecture: runtime.GOARCH,
	}

	switch platform.OS {
	case "linux":
		if err := d.detectLinuxDistro(platform); err != nil {
			return nil, fmt.Errorf("failed to detect Linux distro: %w", err)
		}
	case "darwin":
		if err := d.detectMacOSVersion(platform); err != nil {
			return nil, fmt.Errorf("failed to detect macOS version: %w", err)
		}
	case "windows":
		if err := d.detectWindowsVersion(platform); err != nil {
			return nil, fmt.Errorf("failed to detect Windows version: %w", err)
		}
	}

	// Detect desktop environment (Linux only for now)
	if platform.OS == "linux" {
		d.detectDesktopEnvironment(platform)
	}

	// Detect available package managers
	d.detectPackageManagers(platform)

	// Cache the result
	d.cachedPlatform = platform

	return platform, nil
}

// detectLinuxDistro detects the Linux distribution
func (d *Detector) detectLinuxDistro(platform *Platform) error {
	// Try /etc/os-release first (most modern distributions)
	if osRelease, err := d.parseOSRelease(); err == nil {
		platform.Distribution = osRelease["ID"]
		platform.Version = osRelease["VERSION_ID"]
		return nil
	}

	// Fallback methods for older systems
	distroChecks := map[string]string{
		"ubuntu":   "/etc/debian_version",
		"debian":   "/etc/debian_version",
		"fedora":   "/etc/fedora-release",
		"centos":   "/etc/centos-release",
		"rhel":     "/etc/redhat-release",
		"arch":     "/etc/arch-release",
		"opensuse": "/etc/SuSE-release",
	}

	for distro, file := range distroChecks {
		if _, err := os.Stat(file); err == nil {
			platform.Distribution = distro
			break
		}
	}

	if platform.Distribution == "" {
		platform.Distribution = "unknown"
	}

	return nil
}

// parseOSRelease parses /etc/os-release file
func (d *Detector) parseOSRelease() (map[string]string, error) {
	file, err := os.Open("/etc/os-release")
	if err != nil {
		return nil, err
	}
	defer file.Close()

	result := make(map[string]string)
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := scanner.Text()
		if strings.Contains(line, "=") {
			parts := strings.SplitN(line, "=", 2)
			key := parts[0]
			value := strings.Trim(parts[1], `"`)
			result[key] = value
		}
	}

	return result, scanner.Err()
}

// detectMacOSVersion detects macOS version
func (d *Detector) detectMacOSVersion(platform *Platform) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "sw_vers", "-productVersion")
	output, err := cmd.Output()
	if err != nil {
		return err
	}

	platform.Version = strings.TrimSpace(string(output))
	platform.Distribution = "macos"
	return nil
}

// detectWindowsVersion detects Windows version
func (d *Detector) detectWindowsVersion(platform *Platform) error {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "powershell", "-Command", "(Get-CimInstance Win32_OperatingSystem).Version")
	output, err := cmd.Output()
	if err != nil {
		// Fallback to older method
		cmd = exec.CommandContext(ctx, "cmd", "/c", "ver")
		output, err = cmd.Output()
		if err != nil {
			return err
		}
	}

	platform.Version = strings.TrimSpace(string(output))
	platform.Distribution = "windows"
	return nil
}

// detectDesktopEnvironment detects the desktop environment on Linux
func (d *Detector) detectDesktopEnvironment(platform *Platform) {
	// Check common environment variables
	envVars := []string{
		"XDG_CURRENT_DESKTOP",
		"DESKTOP_SESSION",
		"GDMSESSION",
		"KDE_SESSION_VERSION",
	}

	for _, envVar := range envVars {
		if value := os.Getenv(envVar); value != "" {
			desktop := strings.ToLower(value)
			// Normalize common desktop environment names
			switch {
			case strings.Contains(desktop, "gnome"):
				platform.DesktopEnv = "gnome"
				return
			case strings.Contains(desktop, "kde") || strings.Contains(desktop, "plasma"):
				platform.DesktopEnv = "kde"
				return
			case strings.Contains(desktop, "xfce"):
				platform.DesktopEnv = "xfce"
				return
			case strings.Contains(desktop, "mate"):
				platform.DesktopEnv = "mate"
				return
			case strings.Contains(desktop, "cinnamon"):
				platform.DesktopEnv = "cinnamon"
				return
			case strings.Contains(desktop, "lxde"):
				platform.DesktopEnv = "lxde"
				return
			case strings.Contains(desktop, "unity"):
				platform.DesktopEnv = "unity"
				return
			}
		}
	}

	// Fallback to process detection
	processes := []string{"gnome-session", "kded4", "kded5", "xfce4-session", "mate-session"}
	for _, proc := range processes {
		if d.processExists(proc) {
			switch proc {
			case "gnome-session":
				platform.DesktopEnv = "gnome"
			case "kded4", "kded5":
				platform.DesktopEnv = "kde"
			case "xfce4-session":
				platform.DesktopEnv = "xfce"
			case "mate-session":
				platform.DesktopEnv = "mate"
			}
			return
		}
	}

	// Default to unknown if we can't detect
	platform.DesktopEnv = "unknown"
}

// processExists checks if a process is currently running
func (d *Detector) processExists(name string) bool {
	// Validate process name to prevent command injection
	if !isValidProcessName(name) {
		return false
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "pgrep", name)
	err := cmd.Run()
	return err == nil
}

// isValidProcessName validates process names to prevent command injection
func isValidProcessName(name string) bool {
	// Allow only alphanumeric characters, hyphens, underscores, and dots
	// This covers legitimate process names while preventing injection
	matched, _ := regexp.MatchString(`^[a-zA-Z0-9_.-]+$`, name)
	return matched && len(name) > 0 && len(name) < 64
}

// detectPackageManagers detects available package managers on the system
func (d *Detector) detectPackageManagers(platform *Platform) {
	packageManagers := []string{}

	switch platform.OS {
	case "linux":
		linuxPMs := map[string]string{
			"apt":      "apt",
			"yum":      "yum",
			"dnf":      "dnf",
			"pacman":   "pacman",
			"zypper":   "zypper",
			"emerge":   "emerge",
			"apk":      "apk",
			"snap":     "snap",
			"flatpak":  "flatpak",
			"appimage": "appimagetool",
		}
		for pm, cmd := range linuxPMs {
			if d.commandExists(cmd) {
				packageManagers = append(packageManagers, pm)
			}
		}

	case "darwin":
		macPMs := map[string]string{
			"brew": "brew",
			"port": "port",
			"fink": "fink",
			"nix":  "nix",
		}
		for pm, cmd := range macPMs {
			if d.commandExists(cmd) {
				packageManagers = append(packageManagers, pm)
			}
		}

	case "windows":
		winPMs := map[string]string{
			"choco":  "choco",
			"scoop":  "scoop",
			"winget": "winget",
			"nix":    "nix",
		}
		for pm, cmd := range winPMs {
			if d.commandExists(cmd) {
				packageManagers = append(packageManagers, pm)
			}
		}
	}

	platform.PackageManagers = packageManagers
}

// commandExists checks if a command exists in PATH
func (d *Detector) commandExists(cmd string) bool {
	// Validate command name to prevent path traversal
	if !isValidCommandName(cmd) {
		return false
	}

	_, err := exec.LookPath(cmd)
	return err == nil
}

// isValidCommandName validates command names to prevent path traversal and injection
func isValidCommandName(cmd string) bool {
	// Allow only alphanumeric characters, hyphens, underscores, and dots
	// No path separators or special characters
	matched, _ := regexp.MatchString(`^[a-zA-Z0-9_.-]+$`, cmd)
	return matched && len(cmd) > 0 && len(cmd) < 64 && !strings.Contains(cmd, "/") && !strings.Contains(cmd, "\\")
}

// GetRequiredPlugins returns the list of plugins needed for this platform
func (p *Platform) GetRequiredPlugins() []string {
	plugins := []string{}

	// Add package manager plugins with priority-based selection
	// Primary package managers are preferred over secondary ones
	primaryPMs := p.getPrimaryPackageManagers()
	for _, pm := range primaryPMs {
		plugins = append(plugins, fmt.Sprintf("package-manager-%s", pm))
	}

	// Add OS-specific plugins (only if they exist)
	switch p.OS {
	case "linux":
		// TODO: Add system-setup when it exists in registry
		// plugins = append(plugins, "system-setup")

		// Skip distro-specific plugins for now - they don't exist yet
		// TODO: Add distro plugins when they're implemented
		// if p.Distribution != "unknown" && p.Distribution != "" {
		//     plugins = append(plugins, fmt.Sprintf("distro-%s", p.Distribution))
		// }

		// Add desktop environment plugin if detected (these do exist)
		if p.DesktopEnv != "unknown" && p.DesktopEnv != "" {
			plugins = append(plugins, fmt.Sprintf("desktop-%s", p.DesktopEnv))
		}
	case "darwin":
		// TODO: Add system-setup for macOS when it exists in registry
		// plugins = append(plugins, "system-setup")
		// TODO: Add macOS desktop plugins when implemented
		// plugins = append(plugins, "desktop-macos")
	case "windows":
		// TODO: Add system-setup for Windows when it exists in registry
		// plugins = append(plugins, "system-setup")
		// TODO: Add Windows desktop plugins when implemented
		// plugins = append(plugins, "desktop-windows")
	}

	// Add essential tool plugins
	plugins = append(plugins, "tool-shell")
	plugins = append(plugins, "tool-git")

	return plugins
}

// getPrimaryPackageManagers returns the primary package managers for the platform
// This prioritizes native package managers over third-party ones
func (p *Platform) getPrimaryPackageManagers() []string {
	// Define priority order for each OS
	priorities := map[string][]string{
		"linux": {
			// Native package managers first
			"apt", "dnf", "yum", "pacman", "zypper", "emerge", "apk",
			// Universal package managers second
			"flatpak", "snap", "appimage",
		},
		"darwin": {
			"brew", // Homebrew is the de-facto standard on macOS
			"port", "fink", "nix",
		},
		"windows": {
			"winget", // Microsoft's official package manager
			"choco", "scoop", "nix",
		},
	}

	result := []string{}
	seen := make(map[string]bool)

	// Get priority order for current OS
	order, ok := priorities[p.OS]
	if !ok {
		// If OS not in priorities, return all detected package managers
		return p.PackageManagers
	}

	// Add package managers in priority order
	foundNativePackageManager := false
	for _, pm := range order {
		for _, detected := range p.PackageManagers {
			if pm == detected && !seen[pm] {
				// Check if this is a universal package manager
				isUniversal := pm == "flatpak" || pm == "snap" || pm == "appimage"

				// For Linux, only include one native package manager, but allow all universal ones
				if p.OS == "linux" && !isUniversal && foundNativePackageManager {
					continue // Skip additional native package managers
				}

				result = append(result, pm)
				seen[pm] = true

				// Mark that we found a native package manager
				if p.OS == "linux" && !isUniversal {
					foundNativePackageManager = true
				}
			}
		}
	}

	// Add any remaining detected package managers not in priority list
	for _, pm := range p.PackageManagers {
		if !seen[pm] {
			result = append(result, pm)
		}
	}

	return result
}

// String returns a human-readable representation of the platform
func (p *Platform) String() string {
	if p.OS == "linux" {
		return fmt.Sprintf("%s %s (%s %s)", p.Distribution, p.Version, p.OS, p.Architecture)
	}
	return fmt.Sprintf("%s %s (%s)", p.OS, p.Version, p.Architecture)
}
