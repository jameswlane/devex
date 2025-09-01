package commands

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/jameswlane/devex/apps/cli/internal/log"
	"github.com/jameswlane/devex/apps/cli/internal/platform"
	"github.com/jameswlane/devex/apps/cli/internal/types"
)

// DependencyManager handles dependency analysis and management for uninstall operations
type DependencyManager struct {
	packageManager string
	platform       *platform.DetectionResult
	repo           types.Repository
}

// NewDependencyManager creates a new dependency manager instance
func NewDependencyManager(repo types.Repository) *DependencyManager {
	p := platform.DetectPlatform()
	pm := detectPackageManager(&p)

	return &DependencyManager{
		packageManager: pm,
		platform:       &p,
		repo:           repo,
	}
}

// detectPackageManager detects the primary package manager for the system
func detectPackageManager(p *platform.DetectionResult) string {
	switch p.Distribution {
	case "ubuntu", "debian":
		return "apt"
	case "fedora", "rhel", "centos":
		return "dnf"
	case "arch", "manjaro":
		return "pacman"
	case "opensuse", "suse":
		return "zypper"
	case "alpine":
		return "apk"
	case "gentoo":
		return "emerge"
	default:
		// Try to detect by checking for commands
		if _, err := exec.LookPath("apt"); err == nil {
			return "apt"
		}
		if _, err := exec.LookPath("dnf"); err == nil {
			return "dnf"
		}
		if _, err := exec.LookPath("pacman"); err == nil {
			return "pacman"
		}
		if _, err := exec.LookPath("zypper"); err == nil {
			return "zypper"
		}
		if _, err := exec.LookPath("apk"); err == nil {
			return "apk"
		}
		if _, err := exec.LookPath("emerge"); err == nil {
			return "emerge"
		}
		return "unknown"
	}
}

// GetDependents returns packages that depend on the given package
func (dm *DependencyManager) GetDependents(packageName string) ([]string, error) {
	switch dm.packageManager {
	case "apt":
		return dm.getAptDependents(packageName)
	case "dnf":
		return dm.getDnfDependents(packageName)
	case "pacman":
		return dm.getPacmanDependents(packageName)
	case "zypper":
		return dm.getZypperDependents(packageName)
	case "apk":
		return dm.getApkDependents(packageName)
	case "emerge":
		return dm.getEmergeDependents(packageName)
	default:
		return nil, fmt.Errorf("unsupported package manager: %s", dm.packageManager)
	}
}

// getAptDependents gets dependents for APT
func (dm *DependencyManager) getAptDependents(packageName string) ([]string, error) {
	ctx := context.Background()
	cmd := exec.CommandContext(ctx, "apt-cache", "rdepends", "--installed", packageName)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get APT dependents: %w", err)
	}

	var dependents []string
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, packageName) && !strings.Contains(line, "Reverse Depends:") {
			dependents = append(dependents, line)
		}
	}

	return dependents, nil
}

// getDnfDependents gets dependents for DNF
func (dm *DependencyManager) getDnfDependents(packageName string) ([]string, error) {
	cmd := exec.CommandContext(context.Background(), "dnf", "repoquery", "--whatrequires", packageName)
	output, err := cmd.Output()
	if err != nil {
		// Try rpm as fallback
		return dm.getRpmDependents(packageName)
	}

	var dependents []string
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			dependents = append(dependents, line)
		}
	}

	return dependents, nil
}

// getRpmDependents uses rpm as fallback for RPM-based systems
func (dm *DependencyManager) getRpmDependents(packageName string) ([]string, error) {
	cmd := exec.CommandContext(context.Background(), "rpm", "-q", "--whatrequires", packageName)
	output, err := cmd.Output()
	if err != nil {
		if strings.Contains(string(output), "no package requires") {
			return []string{}, nil
		}
		return nil, fmt.Errorf("failed to get RPM dependents: %w", err)
	}

	var dependents []string
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" && !strings.Contains(line, "no package requires") {
			dependents = append(dependents, line)
		}
	}

	return dependents, nil
}

// getPacmanDependents gets dependents for Pacman
func (dm *DependencyManager) getPacmanDependents(packageName string) ([]string, error) {
	cmd := exec.CommandContext(context.Background(), "pacman", "-Qi", packageName)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get Pacman info: %w", err)
	}

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "Required By") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				deps := strings.TrimSpace(parts[1])
				if deps == "None" {
					return []string{}, nil
				}
				return strings.Fields(deps), nil
			}
		}
	}

	return []string{}, nil
}

// getZypperDependents gets dependents for Zypper
func (dm *DependencyManager) getZypperDependents(packageName string) ([]string, error) {
	cmd := exec.CommandContext(context.Background(), "zypper", "search", "--requires", packageName)
	output, err := cmd.Output()
	if err != nil {
		// Try rpm as fallback
		return dm.getRpmDependents(packageName)
	}

	var dependents []string
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "Loading") || strings.Contains(line, "---") {
			continue
		}

		parts := strings.Split(line, "|")
		if len(parts) >= 2 {
			pkgName := strings.TrimSpace(parts[1])
			if pkgName != "" && pkgName != packageName {
				dependents = append(dependents, pkgName)
			}
		}
	}

	return dependents, nil
}

// getApkDependents gets dependents for APK (Alpine)
func (dm *DependencyManager) getApkDependents(packageName string) ([]string, error) {
	cmd := exec.CommandContext(context.Background(), "apk", "info", "--rdepends", packageName)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get APK dependents: %w", err)
	}

	var dependents []string
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" && !strings.HasPrefix(line, packageName) {
			dependents = append(dependents, line)
		}
	}

	return dependents, nil
}

// getEmergeDependents gets dependents for Emerge (Gentoo)
func (dm *DependencyManager) getEmergeDependents(packageName string) ([]string, error) {
	cmd := exec.CommandContext(context.Background(), "equery", "depends", packageName)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to get Emerge dependents: %w", err)
	}

	var dependents []string
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" && strings.Contains(line, "/") {
			// Extract package name from category/package format
			parts := strings.Fields(line)
			if len(parts) > 0 {
				dependents = append(dependents, parts[0])
			}
		}
	}

	return dependents, nil
}

// FindOrphans finds orphaned packages in the system
func (dm *DependencyManager) FindOrphans() ([]string, error) {
	switch dm.packageManager {
	case "apt":
		return dm.findAptOrphans()
	case "dnf":
		return dm.findDnfOrphans()
	case "pacman":
		return dm.findPacmanOrphans()
	case "zypper":
		return dm.findZypperOrphans()
	case "apk":
		return dm.findApkOrphans()
	case "emerge":
		return dm.findEmergeOrphans()
	default:
		return nil, fmt.Errorf("unsupported package manager: %s", dm.packageManager)
	}
}

// findAptOrphans finds orphaned packages in APT
func (dm *DependencyManager) findAptOrphans() ([]string, error) {
	cmd := exec.CommandContext(context.Background(), "apt", "list", "--installed")
	_, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list APT packages: %w", err)
	}

	// This is simplified - in production you'd use deborphan or apt-mark showauto
	log.Debug("APT orphan detection would use deborphan in production")
	return []string{}, nil
}

// findDnfOrphans finds orphaned packages in DNF
func (dm *DependencyManager) findDnfOrphans() ([]string, error) {
	cmd := exec.CommandContext(context.Background(), "dnf", "leaves")
	output, err := cmd.Output()
	if err != nil {
		// Try package-cleanup as fallback
		cmd = exec.CommandContext(context.Background(), "package-cleanup", "--leaves", "--quiet")
		output, err = cmd.Output()
		if err != nil {
			return []string{}, nil
		}
	}

	var orphans []string
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			orphans = append(orphans, line)
		}
	}

	return orphans, nil
}

// findPacmanOrphans finds orphaned packages in Pacman
func (dm *DependencyManager) findPacmanOrphans() ([]string, error) {
	cmd := exec.CommandContext(context.Background(), "pacman", "-Qtdq")
	output, err := cmd.Output()
	if err != nil {
		// No orphans returns error code
		if strings.TrimSpace(string(output)) == "" {
			return []string{}, nil
		}
		return nil, fmt.Errorf("failed to find Pacman orphans: %w", err)
	}

	var orphans []string
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line != "" {
			orphans = append(orphans, line)
		}
	}

	return orphans, nil
}

// findZypperOrphans finds orphaned packages in Zypper
func (dm *DependencyManager) findZypperOrphans() ([]string, error) {
	cmd := exec.CommandContext(context.Background(), "zypper", "packages", "--unneeded")
	output, err := cmd.Output()
	if err != nil {
		return []string{}, nil
	}

	var orphans []string
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "Loading") || strings.Contains(line, "---") {
			continue
		}

		parts := strings.Split(line, "|")
		if len(parts) >= 3 {
			pkgName := strings.TrimSpace(parts[2])
			if pkgName != "" {
				orphans = append(orphans, pkgName)
			}
		}
	}

	return orphans, nil
}

// findApkOrphans finds orphaned packages in APK
func (dm *DependencyManager) findApkOrphans() ([]string, error) {
	// APK doesn't have a direct orphan detection command
	// This would require more complex logic in production
	log.Debug("APK orphan detection not fully implemented")
	return []string{}, nil
}

// findEmergeOrphans finds orphaned packages in Emerge
func (dm *DependencyManager) findEmergeOrphans() ([]string, error) {
	cmd := exec.CommandContext(context.Background(), "emerge", "--depclean", "--pretend")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to find Emerge orphans: %w", err)
	}

	var orphans []string
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.Contains(line, "selected: ") {
			parts := strings.Split(line, "selected: ")
			if len(parts) == 2 {
				orphans = append(orphans, strings.TrimSpace(parts[1]))
			}
		}
	}

	return orphans, nil
}

// IsSystemPackage checks if a package is a critical system package
func (dm *DependencyManager) IsSystemPackage(packageName string) bool {
	// List of critical system packages that should never be removed
	criticalPackages := []string{
		"kernel", "linux", "systemd", "init", "bash", "sh", "coreutils",
		"glibc", "libc", "gcc", "binutils", "make", "sudo", "openssh",
		"grub", "grub2", "bootloader", "udev", "dbus", "networkmanager",
		"systemd-sysv", "sysvinit", "util-linux", "procps", "findutils",
		"diffutils", "grep", "gawk", "sed", "tar", "gzip", "bzip2",
	}

	packageLower := strings.ToLower(packageName)
	for _, critical := range criticalPackages {
		if strings.Contains(packageLower, critical) {
			return true
		}
	}

	// Check package manager specific critical packages
	switch dm.packageManager {
	case "apt":
		if strings.HasPrefix(packageName, "lib") || strings.Contains(packageName, "base") {
			return true
		}
	case "pacman":
		if strings.Contains(packageName, "base") || strings.Contains(packageName, "linux") {
			return true
		}
	}

	return false
}

// RemoveOrphans removes all orphaned packages
func (dm *DependencyManager) RemoveOrphans() error {
	orphans, err := dm.FindOrphans()
	if err != nil {
		return fmt.Errorf("failed to find orphans: %w", err)
	}

	if len(orphans) == 0 {
		log.Info("No orphaned packages to remove")
		return nil
	}

	log.Info("Removing orphaned packages", "count", len(orphans), "packages", orphans)

	switch dm.packageManager {
	case "apt":
		cmd := exec.CommandContext(context.Background(), "sudo", "apt", "autoremove", "-y")
		return cmd.Run()
	case "dnf":
		cmd := exec.CommandContext(context.Background(), "sudo", "dnf", "autoremove", "-y")
		return cmd.Run()
	case "pacman":
		orphansStr := strings.Join(orphans, " ")
		cmd := exec.CommandContext(context.Background(), "sudo", "pacman", "-Rs", "--noconfirm", orphansStr)
		return cmd.Run()
	case "zypper":
		cmd := exec.CommandContext(context.Background(), "sudo", "zypper", "remove", "--clean-deps", "-y")
		return cmd.Run()
	case "apk":
		orphansStr := strings.Join(orphans, " ")
		cmd := exec.CommandContext(context.Background(), "sudo", "apk", "del", orphansStr)
		return cmd.Run()
	case "emerge":
		cmd := exec.CommandContext(context.Background(), "sudo", "emerge", "--depclean")
		return cmd.Run()
	default:
		return fmt.Errorf("unsupported package manager: %s", dm.packageManager)
	}
}
