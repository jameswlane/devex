package platform

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"runtime"
	"strings"
	"time"
)

// SystemInfo contains comprehensive system information for debugging
type SystemInfo struct {
	// Basic platform info
	OS           string
	Distribution string
	Version      string
	Architecture string
	DesktopEnv   string

	// Runtime info
	GoVersion     string
	DevExVersion  string
	Username      string
	HomeDir       string
	Shell         string
	KernelVersion string

	// Package managers and their versions
	PackageManagers map[string]string

	// System resources
	CPUCount int

	// Environment
	Path       string
	CurrentDir string
	Timestamp  time.Time
}

// GatherSystemInfo collects comprehensive system information for debug logging
func GatherSystemInfo(devexVersion string) *SystemInfo {
	ctx := context.Background()
	platform := DetectPlatform()

	info := &SystemInfo{
		OS:              platform.OS,
		Distribution:    platform.Distribution,
		Version:         platform.Version,
		Architecture:    platform.Architecture,
		DesktopEnv:      platform.DesktopEnv,
		GoVersion:       runtime.Version(),
		DevExVersion:    devexVersion,
		CPUCount:        runtime.NumCPU(),
		Timestamp:       time.Now(),
		PackageManagers: make(map[string]string),
	}

	// Get user information
	if homeDir := os.Getenv("HOME"); homeDir != "" {
		info.HomeDir = homeDir
	} else if currentUser, err := user.Current(); err == nil {
		info.HomeDir = currentUser.HomeDir
	}

	if username := os.Getenv("USER"); username != "" {
		info.Username = username
	} else if currentUser, err := user.Current(); err == nil {
		info.Username = currentUser.Username
	}

	// Get shell information
	if shell := os.Getenv("SHELL"); shell != "" {
		info.Shell = shell
	}

	// Get kernel version (Linux only)
	if platform.OS == "linux" {
		if kernel, err := exec.CommandContext(ctx, "uname", "-r").Output(); err == nil {
			info.KernelVersion = strings.TrimSpace(string(kernel))
		}
	}

	// Get environment variables
	info.Path = os.Getenv("PATH")

	if pwd, err := os.Getwd(); err == nil {
		info.CurrentDir = pwd
	}

	// Detect package managers and their versions
	info.detectPackageManagers(ctx)

	return info
}

// detectPackageManagers detects available package managers and their versions
func (info *SystemInfo) detectPackageManagers(ctx context.Context) {
	packageManagers := []struct {
		name    string
		command string
		args    []string
	}{
		{"apt", "apt", []string{"--version"}},
		{"dnf", "dnf", []string{"--version"}},
		{"yum", "yum", []string{"--version"}},
		{"pacman", "pacman", []string{"--version"}},
		{"zypper", "zypper", []string{"--version"}},
		{"emerge", "emerge", []string{"--version"}},
		{"apk", "apk", []string{"--version"}},
		{"xbps-install", "xbps-install", []string{"--version"}},
		{"eopkg", "eopkg", []string{"--version"}},
		{"flatpak", "flatpak", []string{"--version"}},
		{"snap", "snap", []string{"--version"}},
		{"brew", "brew", []string{"--version"}},
		{"pip", "pip", []string{"--version"}},
		{"pip3", "pip3", []string{"--version"}},
		{"docker", "docker", []string{"--version"}},
		{"mise", "mise", []string{"--version"}},
		{"git", "git", []string{"--version"}},
		{"curl", "curl", []string{"--version"}},
		{"wget", "wget", []string{"--version"}},
		{"nix", "nix", []string{"--version"}},
		{"npm", "npm", []string{"--version"}},
		{"pnpm", "pnpm", []string{"--version"}},
		{"yarn", "yarn", []string{"--version"}},
		{"go", "go", []string{"version"}},
		{"python3", "python3", []string{"--version"}},
		{"node", "node", []string{"--version"}},
		{"rust", "rustc", []string{"--version"}},
		{"java", "java", []string{"--version"}},
	}

	for _, pm := range packageManagers {
		// Try to get version with timeout
		timeoutCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel() // SECURITY: Ensure context is always cancelled

		if output, err := exec.CommandContext(timeoutCtx, pm.command, pm.args...).Output(); err == nil {
			version := strings.TrimSpace(string(output))
			// Clean up version output - take first line and limit length
			lines := strings.Split(version, "\n")
			if len(lines) > 0 {
				version = lines[0]
				if len(version) > 100 {
					version = version[:100] + "..."
				}
				info.PackageManagers[pm.name] = version
			}
		}
	}
}

// GetDetailedSystemInfoString returns comprehensive system information as a formatted string for log headers
func (info *SystemInfo) GetDetailedSystemInfoString() string {
	var sb strings.Builder

	sb.WriteString("=== DEVEX SYSTEM INFORMATION ===\n")
	sb.WriteString(fmt.Sprintf("Timestamp: %s\n", info.Timestamp.Format(time.RFC3339)))
	sb.WriteString(fmt.Sprintf("DevEx Version: %s\n", info.DevExVersion))

	sb.WriteString("=== PLATFORM ===\n")
	sb.WriteString(fmt.Sprintf("Operating System: %s\n", info.OS))
	if info.Distribution != "" {
		sb.WriteString(fmt.Sprintf("Distribution: %s\n", info.Distribution))
	}
	if info.Version != "" {
		sb.WriteString(fmt.Sprintf("OS Version: %s\n", info.Version))
	}
	sb.WriteString(fmt.Sprintf("Architecture: %s\n", info.Architecture))
	if info.DesktopEnv != "" && info.DesktopEnv != "unknown" {
		sb.WriteString(fmt.Sprintf("Desktop Environment: %s\n", info.DesktopEnv))
	}
	if info.KernelVersion != "" {
		sb.WriteString(fmt.Sprintf("Kernel Version: %s\n", info.KernelVersion))
	}

	sb.WriteString("=== RUNTIME ===\n")
	sb.WriteString(fmt.Sprintf("Go Version: %s\n", info.GoVersion))
	sb.WriteString(fmt.Sprintf("CPU Count: %d\n", info.CPUCount))
	if info.Username != "" {
		sb.WriteString(fmt.Sprintf("Username: %s\n", info.Username))
	}
	if info.Shell != "" {
		sb.WriteString(fmt.Sprintf("Shell: %s\n", info.Shell))
	}
	if info.HomeDir != "" {
		sb.WriteString(fmt.Sprintf("Home Directory: %s\n", info.HomeDir))
	}
	if info.CurrentDir != "" {
		sb.WriteString(fmt.Sprintf("Working Directory: %s\n", info.CurrentDir))
	}

	sb.WriteString("=== PACKAGE MANAGERS ===\n")
	if len(info.PackageManagers) == 0 {
		sb.WriteString("No package managers detected\n")
	} else {
		// Group by category for better organization
		systemPMs := []string{"apt", "dnf", "yum", "pacman", "zypper", "emerge", "apk", "xbps-install", "eopkg"}
		universalPMs := []string{"flatpak", "snap", "brew", "nix"}
		languagePMs := []string{"pip", "pip3", "npm", "pnpm", "yarn", "mise"}
		developmentTools := []string{"git", "docker", "go", "python3", "node", "rust", "java"}
		utilities := []string{"curl", "wget"}

		addPMCategory := func(category string, managers []string) {
			found := false
			for _, pm := range managers {
				if version, exists := info.PackageManagers[pm]; exists {
					if !found {
						sb.WriteString(fmt.Sprintf("=== %s ===\n", category))
						found = true
					}
					sb.WriteString(fmt.Sprintf("%s: %s\n", pm, version))
				}
			}
		}

		addPMCategory("SYSTEM PACKAGE MANAGERS", systemPMs)
		addPMCategory("UNIVERSAL PACKAGE MANAGERS", universalPMs)
		addPMCategory("LANGUAGE PACKAGE MANAGERS", languagePMs)
		addPMCategory("DEVELOPMENT TOOLS", developmentTools)
		addPMCategory("UTILITIES", utilities)
	}

	sb.WriteString("=== ENVIRONMENT ===\n")
	if info.Path != "" {
		// Truncate PATH if too long for readability
		path := info.Path
		if len(path) > 500 {
			path = path[:500] + "... [truncated]"
		}
		sb.WriteString(fmt.Sprintf("PATH: %s\n", path))
	}

	sb.WriteString("=== END SYSTEM INFORMATION ===\n")
	return sb.String()
}

// GetSystemInfoString returns system information as a formatted string
func (info *SystemInfo) GetSystemInfoString() string {
	var sb strings.Builder

	sb.WriteString("=== DEVEX SYSTEM INFORMATION ===\n")
	sb.WriteString(fmt.Sprintf("Timestamp: %s\n", info.Timestamp.Format(time.RFC3339)))
	sb.WriteString(fmt.Sprintf("DevEx Version: %s\n", info.DevExVersion))
	sb.WriteString(fmt.Sprintf("OS: %s", info.OS))

	if info.Distribution != "" {
		sb.WriteString(fmt.Sprintf(" (%s)", info.Distribution))
	}
	if info.Version != "" {
		sb.WriteString(fmt.Sprintf(" %s", info.Version))
	}
	sb.WriteString(fmt.Sprintf(" %s\n", info.Architecture))

	if info.DesktopEnv != "" && info.DesktopEnv != "unknown" {
		sb.WriteString(fmt.Sprintf("Desktop: %s\n", info.DesktopEnv))
	}
	if info.KernelVersion != "" {
		sb.WriteString(fmt.Sprintf("Kernel: %s\n", info.KernelVersion))
	}

	sb.WriteString(fmt.Sprintf("Go: %s | CPUs: %d\n", info.GoVersion, info.CPUCount))

	if info.Username != "" {
		sb.WriteString(fmt.Sprintf("User: %s", info.Username))
		if info.Shell != "" {
			sb.WriteString(fmt.Sprintf(" | Shell: %s", info.Shell))
		}
		sb.WriteString("\n")
	}

	if len(info.PackageManagers) > 0 {
		sb.WriteString("Package Managers: ")
		var pms []string
		for pm, version := range info.PackageManagers {
			// Show just name and major version for brevity
			versionShort := strings.Fields(version)[0]
			if len(versionShort) > 20 {
				versionShort = versionShort[:20] + "..."
			}
			pms = append(pms, fmt.Sprintf("%s(%s)", pm, versionShort))
		}
		sb.WriteString(strings.Join(pms, ", "))
		sb.WriteString("\n")
	}

	sb.WriteString("=== END SYSTEM INFORMATION ===\n")
	return sb.String()
}
