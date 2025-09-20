package main

import (
	"fmt"
	"regexp"
	"strings"
)

const (
	// MaxImageNameLength defines the maximum allowed image name length
	MaxImageNameLength = 255

	// MaxContainerNameLength defines the maximum allowed container name length
	MaxContainerNameLength = 100

	// MaxCommandLength defines the maximum allowed command length
	MaxCommandLength = 1000
)

// ValidateImageName validates Docker image names
func (d *DockerInstaller) ValidateImageName(imageName string) error {
	if imageName == "" {
		return fmt.Errorf("image name cannot be empty")
	}

	if len(imageName) > MaxImageNameLength {
		return fmt.Errorf("image name too long (max %d characters)", MaxImageNameLength)
	}

	// Check for null bytes and dangerous control characters
	for _, r := range imageName {
		if r == 0 || (r < 32 && r != 9 && r != 10 && r != 13) {
			return fmt.Errorf("image name contains invalid characters")
		}
	}

	// Docker image names should follow specific patterns
	// Format: [registry[:port]/]name[:tag]
	validImageRegex := regexp.MustCompile(`^[a-z0-9]([a-z0-9._-]*[a-z0-9])?(/[a-z0-9]([a-z0-9._-]*[a-z0-9])?)*?(:[a-zA-Z0-9]([a-zA-Z0-9._-]*[a-zA-Z0-9])?)?$`)
	if !validImageRegex.MatchString(imageName) {
		return fmt.Errorf("invalid Docker image name format")
	}

	return nil
}

// ValidateContainerName validates Docker container names
func (d *DockerInstaller) ValidateContainerName(containerName string) error {
	if containerName == "" {
		return fmt.Errorf("container name cannot be empty")
	}

	if len(containerName) > MaxContainerNameLength {
		return fmt.Errorf("container name too long (max %d characters)", MaxContainerNameLength)
	}

	// Check for null bytes and dangerous control characters
	for _, r := range containerName {
		if r == 0 || (r < 32 && r != 9 && r != 10 && r != 13) {
			return fmt.Errorf("container name contains invalid characters")
		}
	}

	// Docker container names should contain only valid characters
	validNameRegex := regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]*$`)
	if !validNameRegex.MatchString(containerName) {
		return fmt.Errorf("invalid container name format: must start with alphanumeric and contain only letters, numbers, dots, hyphens, and underscores")
	}

	return nil
}

// ValidateCommand validates command arguments to prevent injection
func (d *DockerInstaller) ValidateCommand(args []string) error {
	for _, arg := range args {
		if err := d.ValidateCommandArg(arg); err != nil {
			return fmt.Errorf("invalid command argument '%s': %w", arg, err)
		}
	}
	return nil
}

// ValidateCommandArg validates individual command arguments
func (d *DockerInstaller) ValidateCommandArg(arg string) error {
	if arg == "" {
		return fmt.Errorf("argument cannot be empty")
	}

	if len(arg) > MaxCommandLength {
		return fmt.Errorf("argument too long (max %d characters)", MaxCommandLength)
	}

	// Check for null bytes and dangerous control characters
	for _, r := range arg {
		if r == 0 {
			return fmt.Errorf("argument contains null bytes")
		}
	}

	// Check for shell metacharacters that could be used for command injection
	dangerousChars := []string{";", "&", "|", "`", "$", "(", ")", "<", ">", "\\"}
	for _, char := range dangerousChars {
		if strings.Contains(arg, char) {
			return fmt.Errorf("argument contains potentially dangerous character: %s", char)
		}
	}

	return nil
}

// ValidatePortMapping validates Docker port mappings
func (d *DockerInstaller) ValidatePortMapping(portMapping string) error {
	if portMapping == "" {
		return fmt.Errorf("port mapping cannot be empty")
	}

	// Port mapping format: [hostIP:][hostPort:]containerPort[/protocol]
	portRegex := regexp.MustCompile(`^(?:(?:\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}:)?(?:\d{1,5}:))?\d{1,5}(?:\/(?:tcp|udp))?$`)
	if !portRegex.MatchString(portMapping) {
		return fmt.Errorf("invalid port mapping format")
	}

	return nil
}

// SanitizeOutput sanitizes command output for safe logging
func (d *DockerInstaller) SanitizeOutput(output string) string {
	// Remove null bytes
	output = strings.ReplaceAll(output, "\x00", "")

	// Limit output length for logging
	const maxLogLength = 1000
	if len(output) > maxLogLength {
		output = output[:maxLogLength] + "...[truncated]"
	}

	return output
}
