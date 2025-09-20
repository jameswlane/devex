package status

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/jameswlane/devex/apps/cli/internal/types"
)

// CheckRepositoryStatus verifies package manager registration and repository sources
func CheckRepositoryStatus(ctx context.Context, app *types.AppConfig) []string {
	var issues []string

	switch app.InstallMethod {
	case "apt":
		// Check if package is from official repositories
		packageName := app.InstallCommand
		if packageName == "" {
			packageName = app.Name
		}

		// Check package origin
		cmd := exec.CommandContext(ctx, "apt-cache", "policy", packageName)
		output, err := cmd.Output()
		if err != nil {
			issues = append(issues, fmt.Sprintf("Failed to check APT repository status for %s", packageName))
			return issues
		}

		outputStr := string(output)
		if strings.Contains(outputStr, "*** ") {
			// Package is installed, check if it's from official repos
			if !strings.Contains(outputStr, "ubuntu.com") &&
				!strings.Contains(outputStr, "debian.org") &&
				!strings.Contains(outputStr, "archive.ubuntu.com") {
				// Check if it's from a PPA or third-party repo
				if strings.Contains(outputStr, "ppa.launchpad.net") {
					issues = append(issues, fmt.Sprintf("Package %s installed from PPA (third-party repository)", packageName))
				} else {
					issues = append(issues, fmt.Sprintf("Package %s may be from non-official repository", packageName))
				}
			}
		}

		// Check if package needs updates
		cmd = exec.CommandContext(ctx, "apt", "list", "--upgradable", packageName)
		output, err = cmd.Output()
		if err == nil && len(strings.TrimSpace(string(output))) > 0 {
			lines := strings.Split(string(output), "\n")
			for _, line := range lines {
				if strings.Contains(line, packageName) && strings.Contains(line, "upgradable") {
					issues = append(issues, fmt.Sprintf("Package %s has available updates", packageName))
					break
				}
			}
		}

	case "dnf", "yum":
		// Check DNF/YUM repository status
		packageName := app.InstallCommand
		if packageName == "" {
			packageName = app.Name
		}

		// Check which repository the package came from
		var cmd *exec.Cmd
		if app.InstallMethod == "dnf" {
			cmd = exec.CommandContext(ctx, "dnf", "info", packageName)
		} else {
			cmd = exec.CommandContext(ctx, "yum", "info", packageName)
		}

		output, err := cmd.Output()
		if err != nil {
			issues = append(issues, fmt.Sprintf("Failed to check %s repository status for %s", strings.ToUpper(app.InstallMethod), packageName))
			return issues
		}

		outputStr := string(output)
		if strings.Contains(outputStr, "From repo") {
			// Check if it's from official repositories
			if !strings.Contains(outputStr, "fedora") &&
				!strings.Contains(outputStr, "updates") &&
				!strings.Contains(outputStr, "rhel") &&
				!strings.Contains(outputStr, "centos") &&
				!strings.Contains(outputStr, "base") {
				if strings.Contains(outputStr, "epel") {
					issues = append(issues, fmt.Sprintf("Package %s installed from EPEL (third-party repository)", packageName))
				} else {
					issues = append(issues, fmt.Sprintf("Package %s may be from non-official repository", packageName))
				}
			}
		}

	case "pacman":
		// Check Pacman repository status
		packageName := app.InstallCommand
		if packageName == "" {
			packageName = app.Name
		}

		// Check if package is from official repos or AUR
		cmd := exec.CommandContext(ctx, "pacman", "-Qi", packageName)
		output, err := cmd.Output()
		if err != nil {
			// Try checking if it's an AUR package
			cmd = exec.CommandContext(ctx, "yay", "-Qi", packageName)
			_, err = cmd.Output()
			if err == nil {
				issues = append(issues, fmt.Sprintf("Package %s installed from AUR (user repository)", packageName))
			} else {
				issues = append(issues, fmt.Sprintf("Failed to check repository status for %s", packageName))
			}
			return issues
		}

		outputStr := string(output)
		if strings.Contains(outputStr, "Repository") {
			// Check repository source
			if !strings.Contains(outputStr, "core") &&
				!strings.Contains(outputStr, "extra") &&
				!strings.Contains(outputStr, "community") &&
				!strings.Contains(outputStr, "multilib") {
				issues = append(issues, fmt.Sprintf("Package %s may be from non-official repository", packageName))
			}
		}

	case "zypper":
		// Check Zypper repository status
		packageName := app.InstallCommand
		if packageName == "" {
			packageName = app.Name
		}

		// Check package information
		cmd := exec.CommandContext(ctx, "zypper", "info", packageName)
		output, err := cmd.Output()
		if err != nil {
			issues = append(issues, fmt.Sprintf("Failed to check Zypper repository status for %s", packageName))
			return issues
		}

		outputStr := string(output)
		if strings.Contains(outputStr, "Repository") {
			// Check if it's from official openSUSE repositories
			if !strings.Contains(outputStr, "openSUSE") &&
				!strings.Contains(outputStr, "repo-oss") &&
				!strings.Contains(outputStr, "repo-non-oss") &&
				!strings.Contains(outputStr, "repo-update") {
				issues = append(issues, fmt.Sprintf("Package %s may be from non-official repository", packageName))
			}
		}

	case "brew":
		// Check Homebrew repository status
		packageName := app.InstallCommand
		if packageName == "" {
			packageName = app.Name
		}

		// Check if package is from official Homebrew repositories
		cmd := exec.CommandContext(ctx, "brew", "info", packageName)
		output, err := cmd.Output()
		if err != nil {
			issues = append(issues, fmt.Sprintf("Failed to check Homebrew repository status for %s", packageName))
			return issues
		}

		outputStr := string(output)
		if strings.Contains(outputStr, "From: ") {
			// Check if it's from a tap (third-party repository)
			if strings.Contains(outputStr, "/") && !strings.Contains(outputStr, "homebrew/core") && !strings.Contains(outputStr, "homebrew/cask") {
				issues = append(issues, fmt.Sprintf("Package %s installed from third-party tap", packageName))
			}
		}

	case "snap":
		// Check Snap repository status
		packageName := app.InstallCommand
		if packageName == "" {
			packageName = app.Name
		}

		// Check snap info
		cmd := exec.CommandContext(ctx, "snap", "info", packageName)
		output, err := cmd.Output()
		if err != nil {
			issues = append(issues, fmt.Sprintf("Failed to check Snap repository status for %s", packageName))
			return issues
		}

		outputStr := string(output)
		if strings.Contains(outputStr, "publisher:") {
			// Check if it's verified
			if !strings.Contains(outputStr, "✓") {
				issues = append(issues, fmt.Sprintf("Snap package %s from unverified publisher", packageName))
			}
		}

	case "flatpak":
		// Check Flatpak repository status
		packageName := app.InstallCommand
		if packageName == "" {
			packageName = app.Name
		}

		// Check flatpak info
		cmd := exec.CommandContext(ctx, "flatpak", "info", packageName)
		output, err := cmd.Output()
		if err != nil {
			issues = append(issues, fmt.Sprintf("Failed to check Flatpak repository status for %s", packageName))
			return issues
		}

		outputStr := string(output)
		if strings.Contains(outputStr, "Origin:") {
			// Check if it's from Flathub (official repository)
			if !strings.Contains(outputStr, "flathub") {
				issues = append(issues, fmt.Sprintf("Flatpak package %s not from official Flathub repository", packageName))
			}
		}

	default:
		// For other install methods (curlpipe, mise, etc.), skip repository checks
		// as they don't use traditional package managers
		return issues
	}

	return issues
}
