package platform

import (
	"os"
	"runtime"
	"strings"
)

// OSInfo represents operating system information for installers
type OSInfo struct {
	Distribution string
	Version      string
	Codename     string
	Architecture string
}

// OSDetector interface for OS detection (enables mocking in tests)
type OSDetector interface {
	DetectOS() (*OSInfo, error)
}

// DefaultOSDetector implements OSDetector using the platform detection system
type DefaultOSDetector struct {
	detector *PlatformDetector
}

// NewOSDetector creates a new OS detector
func NewOSDetector() OSDetector {
	return &DefaultOSDetector{
		detector: NewPlatformDetector(),
	}
}

// DetectOS detects the operating system information
func (d *DefaultOSDetector) DetectOS() (*OSInfo, error) {
	platform := d.detector.Detect()

	return &OSInfo{
		Distribution: platform.Distribution,
		Version:      platform.Version,
		Codename:     d.detectCodename(),
		Architecture: platform.Architecture,
	}, nil
}

// detectCodename detects the version codename (Ubuntu/Debian)
func (d *DefaultOSDetector) detectCodename() string {
	if runtime.GOOS != "linux" {
		return ""
	}

	// Try to read codename from /etc/os-release
	if data, err := d.detector.fs.ReadFile("/etc/os-release"); err == nil {
		content := string(data)
		for _, line := range strings.Split(content, "\n") {
			if strings.HasPrefix(line, "VERSION_CODENAME=") {
				return strings.Trim(strings.TrimPrefix(line, "VERSION_CODENAME="), "\"")
			}
		}
	}

	return ""
}

// RuntimeProvider interface for runtime information
type RuntimeProvider interface {
	GOOS() string
	GOARCH() string
}

// DefaultRuntimeProvider implements RuntimeProvider using the actual runtime
type DefaultRuntimeProvider struct{}

func (d DefaultRuntimeProvider) GOOS() string   { return runtime.GOOS }
func (d DefaultRuntimeProvider) GOARCH() string { return runtime.GOARCH }

// FileSystemProvider interface for file system operations
type FileSystemProvider interface {
	ReadFile(filename string) ([]byte, error)
	Stat(name string) (os.FileInfo, error)
}

// DefaultFileSystemProvider implements FileSystemProvider using os package
type DefaultFileSystemProvider struct{}

func (d DefaultFileSystemProvider) ReadFile(filename string) ([]byte, error) {
	return os.ReadFile(filename)
}

func (d DefaultFileSystemProvider) Stat(name string) (os.FileInfo, error) {
	return os.Stat(name)
}

// DetectionResult represents the result of platform detection
type DetectionResult struct {
	OS           string
	DesktopEnv   string
	Distribution string
	Version      string
	Architecture string
}

// PlatformDetector handles platform detection with configurable dependencies
type PlatformDetector struct {
	runtime RuntimeProvider
	fs      FileSystemProvider
}

// NewPlatformDetector creates a new platform detector with default providers
func NewPlatformDetector() *PlatformDetector {
	return &PlatformDetector{
		runtime: DefaultRuntimeProvider{},
		fs:      DefaultFileSystemProvider{},
	}
}

// NewPlatformDetectorWithProviders creates a platform detector with custom providers
func NewPlatformDetectorWithProviders(runtime RuntimeProvider, fs FileSystemProvider) *PlatformDetector {
	return &PlatformDetector{
		runtime: runtime,
		fs:      fs,
	}
}

// DetectPlatform detects the current platform information
func DetectPlatform() DetectionResult {
	detector := NewPlatformDetector()
	return detector.Detect()
}

// Detect performs platform detection using the configured providers
func (pd *PlatformDetector) Detect() DetectionResult {
	return DetectionResult{
		OS:           pd.runtime.GOOS(),
		DesktopEnv:   pd.detectDesktopEnvironment(),
		Distribution: pd.detectDistribution(),
		Version:      pd.detectVersion(),
		Architecture: pd.runtime.GOARCH(),
	}
}

// detectDesktopEnvironment detects the desktop environment on Linux
func (pd *PlatformDetector) detectDesktopEnvironment() string {
	if pd.runtime.GOOS() != "linux" {
		return pd.runtime.GOOS() // "darwin" or "windows"
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
func (pd *PlatformDetector) detectDistribution() string {
	if pd.runtime.GOOS() != "linux" {
		return ""
	}

	// Try to read /etc/os-release
	if data, err := pd.fs.ReadFile("/etc/os-release"); err == nil {
		content := string(data)
		for _, line := range strings.Split(content, "\n") {
			if strings.HasPrefix(line, "ID=") {
				return strings.Trim(strings.TrimPrefix(line, "ID="), "\"")
			}
		}
	}

	// Fallback to checking specific files
	distributions := map[string]string{
		"/etc/ubuntu-release":      "ubuntu",
		"/etc/debian_version":      "debian",
		"/etc/redhat-release":      "rhel",
		"/etc/centos-release":      "centos",
		"/etc/fedora-release":      "fedora",
		"/etc/arch-release":        "arch",
		"/etc/manjaro-release":     "manjaro",
		"/etc/endeavouros-release": "endeavouros",
		"/etc/arcolinux-release":   "arcolinux",
		"/etc/garuda-release":      "garuda",
		"/etc/SUSE-release":        "opensuse",
		"/etc/SuSE-release":        "opensuse",
		"/usr/lib/os-release":      "opensuse", // Modern openSUSE systems
	}

	for file, distro := range distributions {
		if _, err := pd.fs.Stat(file); err == nil {
			return distro
		}
	}

	return "unknown"
}

// detectVersion detects the OS version
func (pd *PlatformDetector) detectVersion() string {
	if pd.runtime.GOOS() != "linux" {
		return ""
	}

	// Try to read version from /etc/os-release
	if data, err := pd.fs.ReadFile("/etc/os-release"); err == nil {
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
	detector := NewPlatformDetector()
	return detector.IsSupported()
}

// IsSupported checks if the platform is supported
func (pd *PlatformDetector) IsSupported() bool {
	switch pd.runtime.GOOS() {
	case "linux", "darwin", "windows":
		return true
	default:
		return false
	}
}

// GetInstallerPriority returns the preferred installer order for the current platform
func GetInstallerPriority() []string {
	detector := NewPlatformDetector()
	return detector.GetInstallerPriority()
}

// GetInstallerPriority returns the installer priority for this platform
func (pd *PlatformDetector) GetInstallerPriority() []string {
	switch pd.runtime.GOOS() {
	case "linux":
		// Check distribution for package manager preference
		distro := pd.detectDistribution()
		switch distro {
		case "ubuntu", "debian":
			return []string{"apt", "flatpak", "snap", "mise", "curlpipe", "download"}
		case "fedora", "rhel", "centos":
			return []string{"dnf", "rpm", "flatpak", "mise", "curlpipe", "download"}
		case "arch", "manjaro", "endeavouros", "arcolinux", "garuda":
			return []string{"pacman", "yay", "flatpak", "mise", "curlpipe", "download"}
		case "opensuse", "sles", "sled", "opensuse-leap", "opensuse-tumbleweed":
			return []string{"zypper", "flatpak", "mise", "curlpipe", "download"}
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
	detector := NewPlatformDetector()
	return detector.GetSystemPackageManager()
}

// GetSystemPackageManager returns the system package manager for this platform
func (pd *PlatformDetector) GetSystemPackageManager() string {
	switch pd.runtime.GOOS() {
	case "linux":
		distro := pd.detectDistribution()
		switch distro {
		case "ubuntu", "debian":
			return "apt"
		case "fedora", "rhel", "centos":
			return "dnf"
		case "arch", "manjaro", "endeavouros", "arcolinux", "garuda":
			return "pacman"
		case "opensuse", "sles", "sled", "opensuse-leap", "opensuse-tumbleweed":
			return "zypper"
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
