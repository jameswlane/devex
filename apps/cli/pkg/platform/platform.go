package platform

import (
	"os"
	"runtime"
	"strings"
)

// Platform represents the operating system and desktop environment
type Platform struct {
	OS           string
	DesktopEnv   string
	Distribution string
	Version      string
	Architecture string
}

// DetectPlatform detects the current platform information
func DetectPlatform() Platform {
	return Platform{
		OS:           runtime.GOOS,
		DesktopEnv:   detectDesktopEnvironment(),
		Distribution: detectDistribution(),
		Version:      detectVersion(),
		Architecture: runtime.GOARCH,
	}
}

// detectDesktopEnvironment detects the desktop environment on Linux
func detectDesktopEnvironment() string {
	if runtime.GOOS != "linux" {
		return runtime.GOOS // "darwin" or "windows"
	}

	// Check environment variables for desktop environment
	if os.Getenv("GNOME_DESKTOP_SESSION_ID") != "" ||
		strings.Contains(os.Getenv("XDG_CURRENT_DESKTOP"), "GNOME") {
		return "gnome"
	}

	if os.Getenv("KDE_FULL_SESSION") != "" ||
		strings.Contains(os.Getenv("XDG_CURRENT_DESKTOP"), "KDE") {
		return "kde"
	}

	if strings.Contains(os.Getenv("XDG_CURRENT_DESKTOP"), "XFCE") {
		return "xfce"
	}

	if strings.Contains(os.Getenv("XDG_CURRENT_DESKTOP"), "Unity") {
		return "unity"
	}

	if os.Getenv("DESKTOP_SESSION") == "cinnamon" {
		return "cinnamon"
	}

	return "unknown"
}

// detectDistribution detects the Linux distribution
func detectDistribution() string {
	if runtime.GOOS != "linux" {
		return ""
	}

	// Try to read /etc/os-release
	if data, err := os.ReadFile("/etc/os-release"); err == nil {
		content := string(data)
		for _, line := range strings.Split(content, "\n") {
			if strings.HasPrefix(line, "ID=") {
				return strings.Trim(strings.TrimPrefix(line, "ID="), "\"")
			}
		}
	}

	// Fallback to checking specific files
	distributions := map[string]string{
		"/etc/ubuntu-release":  "ubuntu",
		"/etc/debian_version":  "debian",
		"/etc/redhat-release":  "rhel",
		"/etc/centos-release":  "centos",
		"/etc/fedora-release":  "fedora",
		"/etc/arch-release":    "arch",
		"/etc/manjaro-release": "manjaro",
	}

	for file, distro := range distributions {
		if _, err := os.Stat(file); err == nil {
			return distro
		}
	}

	return "unknown"
}

// detectVersion detects the OS version
func detectVersion() string {
	if runtime.GOOS != "linux" {
		return ""
	}

	// Try to read version from /etc/os-release
	if data, err := os.ReadFile("/etc/os-release"); err == nil {
		content := string(data)
		for _, line := range strings.Split(content, "\n") {
			if strings.HasPrefix(line, "VERSION_ID=") {
				return strings.Trim(strings.TrimPrefix(line, "VERSION_ID="), "\"")
			}
		}
	}

	return "unknown"
}

// IsSupportedPlatform checks if the current platform is supported
func IsSupportedPlatform() bool {
	switch runtime.GOOS {
	case "linux", "darwin", "windows":
		return true
	default:
		return false
	}
}

// GetInstallerPriority returns the preferred installer order for the current platform
func GetInstallerPriority() []string {
	switch runtime.GOOS {
	case "linux":
		// Check distribution for package manager preference
		distro := detectDistribution()
		switch distro {
		case "ubuntu", "debian":
			return []string{"apt", "flatpak", "snap", "mise", "curlpipe", "download"}
		case "fedora", "rhel", "centos":
			return []string{"dnf", "rpm", "flatpak", "mise", "curlpipe", "download"}
		case "arch", "manjaro":
			return []string{"pacman", "yay", "flatpak", "mise", "curlpipe", "download"}
		default:
			return []string{"flatpak", "mise", "curlpipe", "download"}
		}
	case "darwin":
		return []string{"brew", "mas", "mise", "curlpipe", "download"}
	case "windows":
		return []string{"winget", "chocolatey", "scoop", "mise", "download"}
	default:
		return []string{"mise", "curlpipe", "download"}
	}
}

// GetSystemPackageManager returns the default system package manager
func GetSystemPackageManager() string {
	switch runtime.GOOS {
	case "linux":
		distro := detectDistribution()
		switch distro {
		case "ubuntu", "debian":
			return "apt"
		case "fedora", "rhel", "centos":
			return "dnf"
		case "arch", "manjaro":
			return "pacman"
		default:
			return "unknown"
		}
	case "darwin":
		return "brew"
	case "windows":
		return "winget"
	default:
		return "unknown"
	}
}
