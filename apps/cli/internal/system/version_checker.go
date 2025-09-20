package system

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/jameswlane/devex/apps/cli/internal/log"
	"github.com/jameswlane/devex/apps/cli/internal/utils"
)

// VersionChecker provides methods to check installed versions of various tools
type VersionChecker struct{}

// NewVersionChecker creates a new version checker
func NewVersionChecker() *VersionChecker {
	return &VersionChecker{}
}

// CheckDockerVersion checks if Docker meets the minimum version requirement
func (vc *VersionChecker) CheckDockerVersion(requiredVersion string) (bool, string, error) {
	log.Debug("Checking Docker version", "required", requiredVersion)

	// First check if docker command exists
	if _, err := utils.CommandExec.RunShellCommand("which docker"); err != nil {
		return false, "", fmt.Errorf("docker command not found")
	}

	// Get Docker version
	output, err := utils.CommandExec.RunShellCommand("docker version --format '{{.Client.Version}}'")
	if err != nil {
		// Try alternative command
		output, err = utils.CommandExec.RunShellCommand("docker --version")
		if err != nil {
			return false, "", fmt.Errorf("failed to get docker version: %w", err)
		}
		// Parse from "Docker version 20.10.8, build 3967b7d"
		re := regexp.MustCompile(`Docker version ([0-9]+\.[0-9]+\.[0-9]+)`)
		matches := re.FindStringSubmatch(output)
		if len(matches) < 2 {
			return false, "", fmt.Errorf("could not parse docker version from: %s", output)
		}
		output = matches[1]
	}

	installedVersion := strings.TrimSpace(output)
	meets, err := vc.CompareVersions(installedVersion, requiredVersion)
	if err != nil {
		return false, installedVersion, err
	}

	log.Debug("Docker version check result", "installed", installedVersion, "required", requiredVersion, "meets", meets)
	return meets, installedVersion, nil
}

// CheckDockerComposeVersion checks if Docker Compose meets the minimum version requirement
func (vc *VersionChecker) CheckDockerComposeVersion(requiredVersion string) (bool, string, error) {
	log.Debug("Checking Docker Compose version", "required", requiredVersion)

	// Try docker compose (newer syntax)
	output, err := utils.CommandExec.RunShellCommand("docker compose version --short")
	if err != nil {
		// Try docker-compose (legacy syntax)
		output, err = utils.CommandExec.RunShellCommand("docker-compose --version")
		if err != nil {
			return false, "", fmt.Errorf("docker-compose command not found")
		}
		// Parse from "docker-compose version 1.29.2, build 5becea4c"
		re := regexp.MustCompile(`docker-compose version ([0-9]+\.[0-9]+\.[0-9]+)`)
		matches := re.FindStringSubmatch(output)
		if len(matches) < 2 {
			return false, "", fmt.Errorf("could not parse docker-compose version from: %s", output)
		}
		output = matches[1]
	}

	installedVersion := strings.TrimSpace(output)
	meets, err := vc.CompareVersions(installedVersion, requiredVersion)
	if err != nil {
		return false, installedVersion, err
	}

	log.Debug("Docker Compose version check result", "installed", installedVersion, "required", requiredVersion, "meets", meets)
	return meets, installedVersion, nil
}

// CheckGoVersion checks if Go meets the minimum version requirement
func (vc *VersionChecker) CheckGoVersion(requiredVersion string) (bool, string, error) {
	log.Debug("Checking Go version", "required", requiredVersion)

	output, err := utils.CommandExec.RunShellCommand("go version")
	if err != nil {
		return false, "", fmt.Errorf("go command not found")
	}

	// Parse from "go version go1.19.5 linux/amd64"
	re := regexp.MustCompile(`go version go([0-9]+\.[0-9]+(?:\.[0-9]+)?)`)
	matches := re.FindStringSubmatch(output)
	if len(matches) < 2 {
		return false, "", fmt.Errorf("could not parse go version from: %s", output)
	}

	installedVersion := matches[1]
	meets, err := vc.CompareVersions(installedVersion, requiredVersion)
	if err != nil {
		return false, installedVersion, err
	}

	log.Debug("Go version check result", "installed", installedVersion, "required", requiredVersion, "meets", meets)
	return meets, installedVersion, nil
}

// CheckNodeVersion checks if Node.js meets the minimum version requirement
func (vc *VersionChecker) CheckNodeVersion(requiredVersion string) (bool, string, error) {
	log.Debug("Checking Node.js version", "required", requiredVersion)

	output, err := utils.CommandExec.RunShellCommand("node --version")
	if err != nil {
		return false, "", fmt.Errorf("node command not found")
	}

	// Parse from "v18.17.0"
	installedVersion := strings.TrimSpace(strings.TrimPrefix(output, "v"))
	meets, err := vc.CompareVersions(installedVersion, requiredVersion)
	if err != nil {
		return false, installedVersion, err
	}

	log.Debug("Node.js version check result", "installed", installedVersion, "required", requiredVersion, "meets", meets)
	return meets, installedVersion, nil
}

// CheckPythonVersion checks if Python meets the minimum version requirement
func (vc *VersionChecker) CheckPythonVersion(requiredVersion string) (bool, string, error) {
	log.Debug("Checking Python version", "required", requiredVersion)

	// Try python3 first, then python
	output, err := utils.CommandExec.RunShellCommand("python3 --version")
	if err != nil {
		output, err = utils.CommandExec.RunShellCommand("python --version")
		if err != nil {
			return false, "", fmt.Errorf("python command not found")
		}
	}

	// Parse from "Python 3.9.16"
	re := regexp.MustCompile(`Python ([0-9]+\.[0-9]+(?:\.[0-9]+)?)`)
	matches := re.FindStringSubmatch(output)
	if len(matches) < 2 {
		return false, "", fmt.Errorf("could not parse python version from: %s", output)
	}

	installedVersion := matches[1]
	meets, err := vc.CompareVersions(installedVersion, requiredVersion)
	if err != nil {
		return false, installedVersion, err
	}

	log.Debug("Python version check result", "installed", installedVersion, "required", requiredVersion, "meets", meets)
	return meets, installedVersion, nil
}

// CheckGitVersion checks if Git meets the minimum version requirement
func (vc *VersionChecker) CheckGitVersion(requiredVersion string) (bool, string, error) {
	log.Debug("Checking Git version", "required", requiredVersion)

	output, err := utils.CommandExec.RunShellCommand("git --version")
	if err != nil {
		return false, "", fmt.Errorf("git command not found")
	}

	// Parse from "git version 2.34.1"
	re := regexp.MustCompile(`git version ([0-9]+\.[0-9]+(?:\.[0-9]+)?)`)
	matches := re.FindStringSubmatch(output)
	if len(matches) < 2 {
		return false, "", fmt.Errorf("could not parse git version from: %s", output)
	}

	installedVersion := matches[1]
	meets, err := vc.CompareVersions(installedVersion, requiredVersion)
	if err != nil {
		return false, installedVersion, err
	}

	log.Debug("Git version check result", "installed", installedVersion, "required", requiredVersion, "meets", meets)
	return meets, installedVersion, nil
}

// CompareVersions compares an installed version against a requirement
// Supports formats: "1.13+", ">=1.19", "^18.0.0", "~2.7.0", "1.2.3", "latest"
func (vc *VersionChecker) CompareVersions(installed, required string) (bool, error) {
	if required == "" || required == "latest" {
		return true, nil // No requirement or "latest" always passes
	}

	// Handle different requirement formats
	if strings.HasSuffix(required, "+") {
		// Format: "1.13+" means >= 1.13
		minVersion := strings.TrimSuffix(required, "+")
		return vc.compareSemanticVersions(installed, minVersion, ">=")
	}

	if strings.HasPrefix(required, ">=") {
		minVersion := strings.TrimPrefix(required, ">=")
		return vc.compareSemanticVersions(installed, minVersion, ">=")
	}

	if strings.HasPrefix(required, ">") {
		minVersion := strings.TrimPrefix(required, ">")
		return vc.compareSemanticVersions(installed, minVersion, ">")
	}

	if strings.HasPrefix(required, "<=") {
		maxVersion := strings.TrimPrefix(required, "<=")
		return vc.compareSemanticVersions(installed, maxVersion, "<=")
	}

	if strings.HasPrefix(required, "<") {
		maxVersion := strings.TrimPrefix(required, "<")
		return vc.compareSemanticVersions(installed, maxVersion, "<")
	}

	if strings.HasPrefix(required, "^") {
		// Caret range: ^1.2.3 := >=1.2.3 <2.0.0 (compatible within major version)
		baseVersion := strings.TrimPrefix(required, "^")
		return vc.checkCaretRange(installed, baseVersion)
	}

	if strings.HasPrefix(required, "~") {
		// Tilde range: ~1.2.3 := >=1.2.3 <1.3.0 (compatible within minor version)
		baseVersion := strings.TrimPrefix(required, "~")
		return vc.checkTildeRange(installed, baseVersion)
	}

	// Exact version match
	return vc.compareSemanticVersions(installed, required, "=")
}

// compareSemanticVersions compares two semantic versions
func (vc *VersionChecker) compareSemanticVersions(v1, v2, operator string) (bool, error) {
	version1, err := vc.parseVersion(v1)
	if err != nil {
		return false, fmt.Errorf("invalid version format '%s': %w", v1, err)
	}

	version2, err := vc.parseVersion(v2)
	if err != nil {
		return false, fmt.Errorf("invalid version format '%s': %w", v2, err)
	}

	comparison := vc.compareVersionStructs(version1, version2)

	switch operator {
	case "=":
		return comparison == 0, nil
	case ">":
		return comparison > 0, nil
	case ">=":
		return comparison >= 0, nil
	case "<":
		return comparison < 0, nil
	case "<=":
		return comparison <= 0, nil
	default:
		return false, fmt.Errorf("unknown operator: %s", operator)
	}
}

// Version represents a semantic version
type Version struct {
	Major int
	Minor int
	Patch int
}

// parseVersion parses a version string into a Version struct
func (vc *VersionChecker) parseVersion(version string) (Version, error) {
	parts := strings.Split(version, ".")
	if len(parts) < 2 {
		return Version{}, fmt.Errorf("version must have at least major.minor")
	}

	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return Version{}, fmt.Errorf("invalid major version: %s", parts[0])
	}

	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return Version{}, fmt.Errorf("invalid minor version: %s", parts[1])
	}

	patch := 0
	if len(parts) > 2 {
		patch, err = strconv.Atoi(parts[2])
		if err != nil {
			return Version{}, fmt.Errorf("invalid patch version: %s", parts[2])
		}
	}

	return Version{Major: major, Minor: minor, Patch: patch}, nil
}

// compareVersionStructs compares two Version structs
// Returns: -1 if v1 < v2, 0 if v1 == v2, 1 if v1 > v2
func (vc *VersionChecker) compareVersionStructs(v1, v2 Version) int {
	if v1.Major != v2.Major {
		if v1.Major > v2.Major {
			return 1
		}
		return -1
	}

	if v1.Minor != v2.Minor {
		if v1.Minor > v2.Minor {
			return 1
		}
		return -1
	}

	if v1.Patch != v2.Patch {
		if v1.Patch > v2.Patch {
			return 1
		}
		return -1
	}

	return 0
}

// checkCaretRange checks if installed version is in caret range
func (vc *VersionChecker) checkCaretRange(installed, base string) (bool, error) {
	baseVersion, err := vc.parseVersion(base)
	if err != nil {
		return false, err
	}

	// >= base version
	meets, err := vc.compareSemanticVersions(installed, base, ">=")
	if err != nil || !meets {
		return false, err
	}

	// < next major version
	nextMajor := fmt.Sprintf("%d.0.0", baseVersion.Major+1)
	return vc.compareSemanticVersions(installed, nextMajor, "<")
}

// checkTildeRange checks if installed version is in tilde range
func (vc *VersionChecker) checkTildeRange(installed, base string) (bool, error) {
	baseVersion, err := vc.parseVersion(base)
	if err != nil {
		return false, err
	}

	// >= base version
	meets, err := vc.compareSemanticVersions(installed, base, ">=")
	if err != nil || !meets {
		return false, err
	}

	// < next minor version
	nextMinor := fmt.Sprintf("%d.%d.0", baseVersion.Major, baseVersion.Minor+1)
	return vc.compareSemanticVersions(installed, nextMinor, "<")
}
