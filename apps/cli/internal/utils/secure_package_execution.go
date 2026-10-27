package utils

import (
	"context"
	"fmt"
	"os/exec"
	"time"

	"github.com/jameswlane/devex/apps/cli/internal/log"
	"github.com/jameswlane/devex/apps/cli/internal/metrics"
)

// SecurePackageManagerType represents different package managers
type SecurePackageManagerType string

const (
	APT     SecurePackageManagerType = "apt"
	DNF     SecurePackageManagerType = "dnf"
	YUM     SecurePackageManagerType = "yum"
	Pacman  SecurePackageManagerType = "pacman"
	Zypper  SecurePackageManagerType = "zypper"
	Flatpak SecurePackageManagerType = "flatpak"
	Snap    SecurePackageManagerType = "snap"
	Brew    SecurePackageManagerType = "brew"
	APK     SecurePackageManagerType = "apk"
	Emerge  SecurePackageManagerType = "emerge"
	XBPS    SecurePackageManagerType = "xbps"
	EOPKG   SecurePackageManagerType = "eopkg"
	YAY     SecurePackageManagerType = "yay"
)

// SecurePackageInstaller provides secure installation methods for all package managers
type SecurePackageInstaller struct {
	defaultTimeout time.Duration
}

// NewSecurePackageInstaller creates a new secure package installer
func NewSecurePackageInstaller() *SecurePackageInstaller {
	return &SecurePackageInstaller{
		defaultTimeout: 10 * time.Minute,
	}
}

// WithTimeout sets a custom timeout for operations
func (s *SecurePackageInstaller) WithTimeout(timeout time.Duration) *SecurePackageInstaller {
	s.defaultTimeout = timeout
	return s
}

// InstallPackage securely installs a package using the appropriate package manager
func (s *SecurePackageInstaller) InstallPackage(ctx context.Context, pm SecurePackageManagerType, packageNames ...string) error {
	// Validate all package names first
	for _, pkg := range packageNames {
		if err := ValidatePackageName(pkg); err != nil {
			metrics.RecordError(metrics.MetricSecurityValidationFailed, err, map[string]string{
				"package_manager": string(pm),
				"package_name":    pkg,
			})
			return fmt.Errorf("package validation failed for %s: %w", pkg, err)
		}
	}

	// Create context with timeout
	timeoutCtx, cancel := context.WithTimeout(ctx, s.defaultTimeout)
	defer cancel()

	// Build command for the specific package manager
	cmdArgs, err := s.buildInstallCommand(pm, packageNames...)
	if err != nil {
		return fmt.Errorf("failed to build install command: %w", err)
	}

	// Start timing for metrics
	timer := metrics.StartInstallation(string(pm), fmt.Sprintf("%v", packageNames))

	// Execute the secure command
	if err := s.executeCommand(timeoutCtx, cmdArgs); err != nil {
		timer.Failure(err)
		return fmt.Errorf("failed to install packages %v: %w", packageNames, err)
	}

	timer.Success()
	log.Info("Packages installed successfully", "pm", pm, "packages", packageNames)
	return nil
}

// UninstallPackage securely uninstalls a package using the appropriate package manager
func (s *SecurePackageInstaller) UninstallPackage(ctx context.Context, pm SecurePackageManagerType, packageNames ...string) error {
	// Validate all package names first
	for _, pkg := range packageNames {
		if err := ValidatePackageName(pkg); err != nil {
			return fmt.Errorf("package validation failed for %s: %w", pkg, err)
		}
	}

	// Create context with timeout
	timeoutCtx, cancel := context.WithTimeout(ctx, s.defaultTimeout)
	defer cancel()

	// Build command for the specific package manager
	cmdArgs, err := s.buildUninstallCommand(pm, packageNames...)
	if err != nil {
		return fmt.Errorf("failed to build uninstall command: %w", err)
	}

	// Execute the secure command
	if err := s.executeCommand(timeoutCtx, cmdArgs); err != nil {
		return fmt.Errorf("failed to uninstall packages %v: %w", packageNames, err)
	}

	log.Info("Packages uninstalled successfully", "pm", pm, "packages", packageNames)
	return nil
}

// IsPackageManagerAvailable checks if a package manager is available on the system
func (s *SecurePackageInstaller) IsPackageManagerAvailable(ctx context.Context, pm SecurePackageManagerType) bool {
	return IsCommandAvailable(ctx, string(pm))
}

func (s *SecurePackageInstaller) CheckPackageInstalled(ctx context.Context, pm SecurePackageManagerType, packageName string) (bool, error) {
	if err := ValidatePackageName(packageName); err != nil {
		return false, err
	}

	// Create context with timeout
	timeoutCtx, cancel := context.WithTimeout(ctx, s.defaultTimeout)
	defer cancel()

	var cmd *exec.Cmd
	switch pm {
	case APT:
		cmd = exec.CommandContext(timeoutCtx, "dpkg-query", "-W", "-f=${Status}", packageName)
	case DNF, YUM:
		cmd = exec.CommandContext(timeoutCtx, "rpm", "-q", packageName)
	case Pacman:
		cmd = exec.CommandContext(timeoutCtx, "pacman", "-Qs", packageName)
	case Zypper:
		cmd = exec.CommandContext(timeoutCtx, "rpm", "-q", packageName)
	case Flatpak:
		cmd = exec.CommandContext(timeoutCtx, "flatpak", "list", "--app", "--columns=application", packageName)
	case Snap:
		cmd = exec.CommandContext(timeoutCtx, "snap", "list", packageName)
	case Brew:
		cmd = exec.CommandContext(timeoutCtx, "brew", "list", packageName)
	default:
		return false, fmt.Errorf("unsupported package manager: %s", pm)
	}

	output, err := cmd.Output()
	return err == nil && len(output) > 0, nil
}

func (s *SecurePackageInstaller) buildInstallCommand(pm SecurePackageManagerType, packageNames ...string) ([]string, error) {
	var base []string
	switch pm {
	case APT:
		base = []string{"sudo", "apt-get", "install", "-y"}
	case DNF:
		base = []string{"sudo", "dnf", "install", "-y"}
	case YUM:
		base = []string{"sudo", "yum", "install", "-y"}
	case Pacman:
		base = []string{"sudo", "pacman", "-S", "--noconfirm"}
	case Zypper:
		base = []string{"sudo", "zypper", "install", "-y"}
	case Flatpak:
		base = []string{"flatpak", "install", "-y"}
	case Snap:
		base = []string{"sudo", "snap", "install"}
	case Brew:
		base = []string{"brew", "install"}
	case APK:
		base = []string{"sudo", "apk", "add"}
	case Emerge:
		base = []string{"sudo", "emerge"}
	case XBPS:
		base = []string{"sudo", "xbps-install", "-S"}
	case EOPKG:
		base = []string{"sudo", "eopkg", "install"}
	case YAY:
		base = []string{"yay", "-S", "--noconfirm"}
	default:
		return nil, fmt.Errorf("unsupported package manager: %s", pm)
	}
	return commandWithArgs(base, packageNames), nil
}

func (s *SecurePackageInstaller) buildUninstallCommand(pm SecurePackageManagerType, packageNames ...string) ([]string, error) {
	var base []string
	switch pm {
	case APT:
		base = []string{"sudo", "apt-get", "remove", "-y"}
	case DNF:
		base = []string{"sudo", "dnf", "remove", "-y"}
	case YUM:
		base = []string{"sudo", "yum", "remove", "-y"}
	case Pacman:
		base = []string{"sudo", "pacman", "-R", "--noconfirm"}
	case Zypper:
		base = []string{"sudo", "zypper", "remove", "-y"}
	case Flatpak:
		base = []string{"flatpak", "uninstall", "-y"}
	case Snap:
		base = []string{"sudo", "snap", "remove"}
	case Brew:
		base = []string{"brew", "uninstall"}
	case APK:
		base = []string{"sudo", "apk", "del"}
	case Emerge:
		base = []string{"sudo", "emerge", "--unmerge"}
	case XBPS:
		base = []string{"sudo", "xbps-remove", "-R"}
	case EOPKG:
		base = []string{"sudo", "eopkg", "remove"}
	case YAY:
		base = []string{"yay", "-R", "--noconfirm"}
	default:
		return nil, fmt.Errorf("unsupported package manager: %s", pm)
	}
	return commandWithArgs(base, packageNames), nil
}

// commandWithArgs returns base followed by args in a newly allocated slice
func commandWithArgs(base, args []string) []string {
	cmd := make([]string, 0, len(base)+len(args))
	cmd = append(cmd, base...)
	return append(cmd, args...)
}

func (s *SecurePackageInstaller) UpdatePackageCache(ctx context.Context, pm SecurePackageManagerType) error {
	timeoutCtx, cancel := context.WithTimeout(ctx, s.defaultTimeout)
	defer cancel()

	var cmd *exec.Cmd
	switch pm {
	case APT:
		cmd = exec.CommandContext(timeoutCtx, "sudo", "apt-get", "update")
	case DNF:
		cmd = exec.CommandContext(timeoutCtx, "sudo", "dnf", "makecache")
	case YUM:
		cmd = exec.CommandContext(timeoutCtx, "sudo", "yum", "makecache")
	case Pacman:
		cmd = exec.CommandContext(timeoutCtx, "sudo", "pacman", "-Sy")
	case Zypper:
		cmd = exec.CommandContext(timeoutCtx, "sudo", "zypper", "refresh")
	case Flatpak:
		cmd = exec.CommandContext(timeoutCtx, "flatpak", "update")
	case APK:
		cmd = exec.CommandContext(timeoutCtx, "sudo", "apk", "update")
	case XBPS:
		cmd = exec.CommandContext(timeoutCtx, "sudo", "xbps-install", "-S")
	case EOPKG:
		cmd = exec.CommandContext(timeoutCtx, "sudo", "eopkg", "update-repo")
	default:
		return fmt.Errorf("package cache update not supported for: %s", pm)
	}

	return cmd.Run()
}

// GetAvailablePackageManagers returns a list of available package managers on the system
func (s *SecurePackageInstaller) GetAvailablePackageManagers(ctx context.Context) []SecurePackageManagerType {
	allPMs := []SecurePackageManagerType{
		APT, DNF, YUM, Pacman, Zypper, Flatpak, Snap, Brew, APK, Emerge, XBPS, EOPKG, YAY,
	}

	var available []SecurePackageManagerType
	for _, pm := range allPMs {
		if s.IsPackageManagerAvailable(ctx, pm) {
			available = append(available, pm)
		}
	}

	return available
}

// executeCommand executes a command securely without shell interpretation
func (s *SecurePackageInstaller) executeCommand(ctx context.Context, cmdArgs []string) error {
	if len(cmdArgs) == 0 {
		return fmt.Errorf("empty command")
	}

	cmd := exec.CommandContext(ctx, cmdArgs[0], cmdArgs[1:]...)
	return cmd.Run()
}

// Global instance for convenience
var GlobalSecureInstaller = NewSecurePackageInstaller()

// Convenience functions using the global instance
func SecureInstall(ctx context.Context, pm SecurePackageManagerType, packages ...string) error {
	return GlobalSecureInstaller.InstallPackage(ctx, pm, packages...)
}

func SecureUninstall(ctx context.Context, pm SecurePackageManagerType, packages ...string) error {
	return GlobalSecureInstaller.UninstallPackage(ctx, pm, packages...)
}

func IsAvailable(ctx context.Context, pm SecurePackageManagerType) bool {
	return GlobalSecureInstaller.IsPackageManagerAvailable(ctx, pm)
}

func UpdateCache(ctx context.Context, pm SecurePackageManagerType) error {
	return GlobalSecureInstaller.UpdatePackageCache(ctx, pm)
}
