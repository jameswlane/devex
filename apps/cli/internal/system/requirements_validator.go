package system

import (
	"context"
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/jameswlane/devex/apps/cli/internal/log"
	"github.com/jameswlane/devex/apps/cli/internal/types"
	"github.com/jameswlane/devex/apps/cli/internal/utils"
)

// RequirementsValidator validates system requirements
type RequirementsValidator struct {
	versionChecker *VersionChecker
}

// NewRequirementsValidator creates a new requirements validator
func NewRequirementsValidator() *RequirementsValidator {
	return &RequirementsValidator{
		versionChecker: NewVersionChecker(),
	}
}

// ValidationResult represents the result of a system requirement validation
type ValidationResult struct {
	Requirement string
	Status      ValidationStatus
	Message     string
	Suggestion  string
}

// ValidationStatus represents the status of a validation
type ValidationStatus int

const (
	ValidationPassed ValidationStatus = iota
	ValidationFailed
	ValidationWarning
	ValidationSkipped
)

// String returns the string representation of ValidationStatus
func (vs ValidationStatus) String() string {
	switch vs {
	case ValidationPassed:
		return "PASSED"
	case ValidationFailed:
		return "FAILED"
	case ValidationWarning:
		return "WARNING"
	case ValidationSkipped:
		return "SKIPPED"
	default:
		return "UNKNOWN"
	}
}

// ValidateRequirements validates all system requirements for an application
func (rv *RequirementsValidator) ValidateRequirements(appName string, requirements types.SystemRequirements) ([]ValidationResult, error) {
	log.Info("Validating system requirements", "app", appName)

	// Pre-allocate results slice with estimated capacity
	results := make([]ValidationResult, 0, 20)

	// Validate memory requirements
	if requirements.MinMemoryMB > 0 {
		result := rv.validateMemoryRequirement(requirements.MinMemoryMB)
		results = append(results, result)
	}

	// Validate disk space requirements
	if requirements.MinDiskSpaceMB > 0 {
		result := rv.validateDiskSpaceRequirement(requirements.MinDiskSpaceMB)
		results = append(results, result)
	}

	// Validate version requirements
	versionChecks := map[string]string{
		"Docker":         requirements.DockerVersion,
		"Docker Compose": requirements.DockerComposeVersion,
		"Go":             requirements.GoVersion,
		"Node.js":        requirements.NodeVersion,
		"Python":         requirements.PythonVersion,
		"Ruby":           requirements.RubyVersion,
		"Java":           requirements.JavaVersion,
		"Git":            requirements.GitVersion,
		"kubectl":        requirements.KubectlVersion,
	}

	for tool, requiredVersion := range versionChecks {
		if requiredVersion != "" {
			result := rv.validateVersionRequirement(tool, requiredVersion)
			results = append(results, result)
		}
	}

	// Validate required commands
	for _, command := range requirements.RequiredCommands {
		result := rv.validateCommandRequirement(command)
		results = append(results, result)
	}

	// Validate required services
	for _, service := range requirements.RequiredServices {
		result := rv.validateServiceRequirement(service)
		results = append(results, result)
	}

	// Validate required ports
	for _, port := range requirements.RequiredPorts {
		result := rv.validatePortRequirement(port)
		results = append(results, result)
	}

	// Validate required environment variables
	for _, envVar := range requirements.RequiredEnvVars {
		result := rv.validateEnvVarRequirement(envVar)
		results = append(results, result)
	}

	log.Info("System requirements validation completed", "app", appName, "total_checks", len(results))
	return results, nil
}

// validateMemoryRequirement checks if system has enough memory
func (rv *RequirementsValidator) validateMemoryRequirement(minMemoryMB int) ValidationResult {
	log.Debug("Validating memory requirement", "required_mb", minMemoryMB)

	// Read memory info from /proc/meminfo on Linux
	output, err := utils.CommandExec.RunShellCommand("grep '^MemTotal:' /proc/meminfo")
	if err != nil {
		return ValidationResult{
			Requirement: fmt.Sprintf("Memory >= %d MB", minMemoryMB),
			Status:      ValidationWarning,
			Message:     "Could not determine system memory",
			Suggestion:  "Manually verify system has sufficient memory",
		}
	}

	// Parse MemTotal: 16384000 kB
	parts := strings.Fields(output)
	if len(parts) < 2 {
		return ValidationResult{
			Requirement: fmt.Sprintf("Memory >= %d MB", minMemoryMB),
			Status:      ValidationWarning,
			Message:     "Could not parse memory information",
			Suggestion:  "Manually verify system has sufficient memory",
		}
	}

	memoryKB, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return ValidationResult{
			Requirement: fmt.Sprintf("Memory >= %d MB", minMemoryMB),
			Status:      ValidationWarning,
			Message:     "Could not parse memory value",
			Suggestion:  "Manually verify system has sufficient memory",
		}
	}

	memoryMB := int(memoryKB / 1024)

	if memoryMB >= minMemoryMB {
		return ValidationResult{
			Requirement: fmt.Sprintf("Memory >= %d MB", minMemoryMB),
			Status:      ValidationPassed,
			Message:     fmt.Sprintf("System has %d MB memory", memoryMB),
		}
	}

	return ValidationResult{
		Requirement: fmt.Sprintf("Memory >= %d MB", minMemoryMB),
		Status:      ValidationFailed,
		Message:     fmt.Sprintf("System has only %d MB memory, requires %d MB", memoryMB, minMemoryMB),
		Suggestion:  "Add more RAM to the system or consider using a lighter alternative",
	}
}

// validateDiskSpaceRequirement checks if system has enough disk space
func (rv *RequirementsValidator) validateDiskSpaceRequirement(minDiskSpaceMB int) ValidationResult {
	log.Debug("Validating disk space requirement", "required_mb", minDiskSpaceMB)

	// Get disk usage for current directory using platform-specific implementation
	availableMBFromSyscall, err := getDiskSpaceInfo(".")
	if err == nil {
		// Use syscall result if successful
		if availableMBFromSyscall >= minDiskSpaceMB {
			return ValidationResult{
				Requirement: fmt.Sprintf("Disk space >= %d MB", minDiskSpaceMB),
				Status:      ValidationPassed,
				Message:     fmt.Sprintf("Available disk space: %d MB", availableMBFromSyscall),
			}
		} else {
			return ValidationResult{
				Requirement: fmt.Sprintf("Disk space >= %d MB", minDiskSpaceMB),
				Status:      ValidationFailed,
				Message:     fmt.Sprintf("Insufficient disk space. Available: %d MB, Required: %d MB", availableMBFromSyscall, minDiskSpaceMB),
				Suggestion:  fmt.Sprintf("Free up at least %d MB of disk space", minDiskSpaceMB-availableMBFromSyscall),
			}
		}
	}

	// Calculate available space in MB - use df command for safer cross-platform approach
	output, err := utils.CommandExec.RunShellCommand("df -m . | tail -1 | awk '{print $4}'")
	if err != nil {
		return ValidationResult{
			Requirement: fmt.Sprintf("Disk space >= %d MB", minDiskSpaceMB),
			Status:      ValidationWarning,
			Message:     "Could not determine available disk space using df command",
			Suggestion:  "Manually verify sufficient disk space is available",
		}
	}

	availableMB, err := strconv.Atoi(strings.TrimSpace(output))
	if err != nil {
		return ValidationResult{
			Requirement: fmt.Sprintf("Disk space >= %d MB", minDiskSpaceMB),
			Status:      ValidationWarning,
			Message:     "Could not parse disk space information",
			Suggestion:  "Manually verify sufficient disk space is available",
		}
	}

	if availableMB >= minDiskSpaceMB {
		return ValidationResult{
			Requirement: fmt.Sprintf("Disk space >= %d MB", minDiskSpaceMB),
			Status:      ValidationPassed,
			Message:     fmt.Sprintf("System has %d MB available disk space", availableMB),
		}
	}

	return ValidationResult{
		Requirement: fmt.Sprintf("Disk space >= %d MB", minDiskSpaceMB),
		Status:      ValidationFailed,
		Message:     fmt.Sprintf("System has only %d MB available, requires %d MB", availableMB, minDiskSpaceMB),
		Suggestion:  "Free up disk space or install to a different location",
	}
}

// validateVersionRequirement checks if a tool meets version requirements
func (rv *RequirementsValidator) validateVersionRequirement(tool, requiredVersion string) ValidationResult {
	log.Debug("Validating version requirement", "tool", tool, "required", requiredVersion)

	var meets bool
	var installedVersion string
	var err error

	switch tool {
	case "Docker":
		meets, installedVersion, err = rv.versionChecker.CheckDockerVersion(requiredVersion)
	case "Docker Compose":
		meets, installedVersion, err = rv.versionChecker.CheckDockerComposeVersion(requiredVersion)
	case "Go":
		meets, installedVersion, err = rv.versionChecker.CheckGoVersion(requiredVersion)
	case "Node.js":
		meets, installedVersion, err = rv.versionChecker.CheckNodeVersion(requiredVersion)
	case "Python":
		meets, installedVersion, err = rv.versionChecker.CheckPythonVersion(requiredVersion)
	case "Git":
		meets, installedVersion, err = rv.versionChecker.CheckGitVersion(requiredVersion)
	default:
		return ValidationResult{
			Requirement: fmt.Sprintf("%s %s", tool, requiredVersion),
			Status:      ValidationSkipped,
			Message:     fmt.Sprintf("Version checking not implemented for %s", tool),
			Suggestion:  fmt.Sprintf("Manually verify %s version %s is installed", tool, requiredVersion),
		}
	}

	if err != nil {
		suggestion := fmt.Sprintf("Install %s version %s or newer", tool, requiredVersion)
		if strings.Contains(err.Error(), "not found") {
			return ValidationResult{
				Requirement: fmt.Sprintf("%s %s", tool, requiredVersion),
				Status:      ValidationFailed,
				Message:     fmt.Sprintf("%s is not installed", tool),
				Suggestion:  suggestion,
			}
		}

		return ValidationResult{
			Requirement: fmt.Sprintf("%s %s", tool, requiredVersion),
			Status:      ValidationWarning,
			Message:     fmt.Sprintf("Could not check %s version: %v", tool, err),
			Suggestion:  suggestion,
		}
	}

	if meets {
		return ValidationResult{
			Requirement: fmt.Sprintf("%s %s", tool, requiredVersion),
			Status:      ValidationPassed,
			Message:     fmt.Sprintf("%s version %s meets requirement", tool, installedVersion),
		}
	}

	return ValidationResult{
		Requirement: fmt.Sprintf("%s %s", tool, requiredVersion),
		Status:      ValidationFailed,
		Message:     fmt.Sprintf("%s version %s does not meet requirement %s", tool, installedVersion, requiredVersion),
		Suggestion:  fmt.Sprintf("Update %s to version %s or newer", tool, requiredVersion),
	}
}

// validateCommandRequirement checks if a command is available
func (rv *RequirementsValidator) validateCommandRequirement(command string) ValidationResult {
	log.Debug("Validating command requirement", "command", command)

	_, err := utils.CommandExec.RunShellCommand(fmt.Sprintf("which %s", command))
	if err != nil {
		return ValidationResult{
			Requirement: fmt.Sprintf("Command '%s' available", command),
			Status:      ValidationFailed,
			Message:     fmt.Sprintf("Command '%s' not found", command),
			Suggestion:  fmt.Sprintf("Install package containing '%s' command", command),
		}
	}

	return ValidationResult{
		Requirement: fmt.Sprintf("Command '%s' available", command),
		Status:      ValidationPassed,
		Message:     fmt.Sprintf("Command '%s' is available", command),
	}
}

// validateServiceRequirement checks if a service is running
func (rv *RequirementsValidator) validateServiceRequirement(service string) ValidationResult {
	log.Debug("Validating service requirement", "service", service)

	// Try systemctl first (systemd systems)
	_, err := utils.CommandExec.RunShellCommand(fmt.Sprintf("systemctl is-active %s", service))
	if err == nil {
		return ValidationResult{
			Requirement: fmt.Sprintf("Service '%s' running", service),
			Status:      ValidationPassed,
			Message:     fmt.Sprintf("Service '%s' is active", service),
		}
	}

	// Try service command (SysV systems)
	_, err = utils.CommandExec.RunShellCommand(fmt.Sprintf("service %s status", service))
	if err == nil {
		return ValidationResult{
			Requirement: fmt.Sprintf("Service '%s' running", service),
			Status:      ValidationPassed,
			Message:     fmt.Sprintf("Service '%s' is running", service),
		}
	}

	return ValidationResult{
		Requirement: fmt.Sprintf("Service '%s' running", service),
		Status:      ValidationFailed,
		Message:     fmt.Sprintf("Service '%s' is not running", service),
		Suggestion:  fmt.Sprintf("Start service with: sudo systemctl start %s", service),
	}
}

// validatePortRequirement checks if a port is available
func (rv *RequirementsValidator) validatePortRequirement(port int) ValidationResult {
	log.Debug("Validating port requirement", "port", port)

	// Create a context with timeout for the network operation
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Use ListenConfig for proper context support
	lc := &net.ListenConfig{}
	listener, err := lc.Listen(ctx, "tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return ValidationResult{
			Requirement: fmt.Sprintf("Port %d available", port),
			Status:      ValidationFailed,
			Message:     fmt.Sprintf("Port %d is in use or not accessible", port),
			Suggestion:  fmt.Sprintf("Stop service using port %d or choose a different port", port),
		}
	}

	// Close the listener and handle potential error
	if closeErr := listener.Close(); closeErr != nil {
		log.Warn("Failed to close port test listener", "port", port, "error", closeErr)
	}

	return ValidationResult{
		Requirement: fmt.Sprintf("Port %d available", port),
		Status:      ValidationPassed,
		Message:     fmt.Sprintf("Port %d is available", port),
	}
}

// validateEnvVarRequirement checks if an environment variable is set
func (rv *RequirementsValidator) validateEnvVarRequirement(envVar string) ValidationResult {
	log.Debug("Validating environment variable requirement", "env_var", envVar)

	value := os.Getenv(envVar)
	if value == "" {
		return ValidationResult{
			Requirement: fmt.Sprintf("Environment variable '%s' set", envVar),
			Status:      ValidationFailed,
			Message:     fmt.Sprintf("Environment variable '%s' is not set", envVar),
			Suggestion:  fmt.Sprintf("Set environment variable: export %s=<value>", envVar),
		}
	}

	return ValidationResult{
		Requirement: fmt.Sprintf("Environment variable '%s' set", envVar),
		Status:      ValidationPassed,
		Message:     fmt.Sprintf("Environment variable '%s' is set", envVar),
	}
}

// HasFailures checks if any validation results have failures
func (rv *RequirementsValidator) HasFailures(results []ValidationResult) bool {
	for _, result := range results {
		if result.Status == ValidationFailed {
			return true
		}
	}
	return false
}

// GetFailures returns only the failed validation results
func (rv *RequirementsValidator) GetFailures(results []ValidationResult) []ValidationResult {
	var failures []ValidationResult
	for _, result := range results {
		if result.Status == ValidationFailed {
			failures = append(failures, result)
		}
	}
	return failures
}

// GetWarnings returns only the warning validation results
func (rv *RequirementsValidator) GetWarnings(results []ValidationResult) []ValidationResult {
	var warnings []ValidationResult
	for _, result := range results {
		if result.Status == ValidationWarning {
			warnings = append(warnings, result)
		}
	}
	return warnings
}
