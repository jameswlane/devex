package utilities

import (
	"os"
	"path/filepath"
)

// SystemPaths contains configurable system paths for different package managers
type SystemPaths struct {
	// YUM/DNF repository directory
	YumReposDir string
	// APT sources directory
	AptSourcesDir string
	// Pacman configuration directory
	PacmanConfDir string
	// Zypper repository directory
	ZypperReposDir string
	// Flatpak system installation directory
	FlatpakSystemDir string
	// Snap system directory
	SnapSystemDir string
}

// GetSystemPaths returns the system paths with defaults that can be overridden by environment variables
func GetSystemPaths() *SystemPaths {
	return &SystemPaths{
		YumReposDir:      getEnvWithDefault("DEVEX_YUM_REPOS_DIR", "/etc/yum.repos.d"),
		AptSourcesDir:    getEnvWithDefault("DEVEX_APT_SOURCES_DIR", "/etc/apt/sources.list.d"),
		PacmanConfDir:    getEnvWithDefault("DEVEX_PACMAN_CONF_DIR", "/etc/pacman.d"),
		ZypperReposDir:   getEnvWithDefault("DEVEX_ZYPPER_REPOS_DIR", "/etc/zypp/repos.d"),
		FlatpakSystemDir: getEnvWithDefault("DEVEX_FLATPAK_SYSTEM_DIR", "/var/lib/flatpak"),
		SnapSystemDir:    getEnvWithDefault("DEVEX_SNAP_SYSTEM_DIR", "/var/lib/snapd"),
	}
}

// GetRepositoryFilePath returns the full path for a repository file
func (sp *SystemPaths) GetRepositoryFilePath(repoType, repoName string) string {
	switch repoType {
	case "yum", "dnf":
		return filepath.Join(sp.YumReposDir, repoName+".repo")
	case "apt":
		return filepath.Join(sp.AptSourcesDir, repoName+".list")
	case "zypper":
		return filepath.Join(sp.ZypperReposDir, repoName+".repo")
	default:
		return ""
	}
}

// getEnvWithDefault returns the value of the environment variable or the default if not set
func getEnvWithDefault(envVar, defaultValue string) string {
	if value := os.Getenv(envVar); value != "" {
		return value
	}
	return defaultValue
}
