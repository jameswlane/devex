package main

import (
	"fmt"
	"path/filepath"
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

// ValidateContainerArgs validates one or more container names.
func (d *DockerInstaller) ValidateContainerArgs(containers []string) error {
	for _, container := range containers {
		if err := d.ValidateContainerName(container); err != nil {
			return err
		}
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

// ValidateEnvVar validates Docker environment assignment flags.
func (d *DockerInstaller) ValidateEnvVar(env string) error {
	if env == "" {
		return fmt.Errorf("environment variable cannot be empty")
	}
	if len(env) > MaxCommandLength {
		return fmt.Errorf("environment variable too long (max %d characters)", MaxCommandLength)
	}
	if strings.ContainsRune(env, 0) {
		return fmt.Errorf("environment variable contains null bytes")
	}
	key, _, hasValue := strings.Cut(env, "=")
	if !hasValue || key == "" {
		return fmt.Errorf("environment variable must use KEY=value format")
	}
	validEnvKeyRegex := regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_]*$`)
	if !validEnvKeyRegex.MatchString(key) {
		return fmt.Errorf("invalid environment variable name")
	}
	return nil
}

// ValidateVolumeMount validates Docker volume mount flags.
func (d *DockerInstaller) ValidateVolumeMount(volume string) error {
	if volume == "" {
		return fmt.Errorf("volume mount cannot be empty")
	}
	if len(volume) > MaxCommandLength {
		return fmt.Errorf("volume mount too long (max %d characters)", MaxCommandLength)
	}
	if strings.ContainsRune(volume, 0) {
		return fmt.Errorf("volume mount contains null bytes")
	}
	if strings.Count(volume, ":") < 1 {
		return fmt.Errorf("volume mount must use source:target format")
	}
	dangerousChars := []string{";", "&", "|", "`", "$", "<", ">"}
	for _, char := range dangerousChars {
		if strings.Contains(volume, char) {
			return fmt.Errorf("volume mount contains potentially dangerous character: %s", char)
		}
	}
	return nil
}

// ValidateRestartPolicy validates Docker restart policy values.
func (d *DockerInstaller) ValidateRestartPolicy(policy string) error {
	if policy == "" {
		return fmt.Errorf("restart policy cannot be empty")
	}
	validRestartRegex := regexp.MustCompile(`^(no|always|unless-stopped|on-failure(?::[1-9][0-9]*)?)$`)
	if !validRestartRegex.MatchString(policy) {
		return fmt.Errorf("invalid restart policy")
	}
	return nil
}

// ValidateComposeFilePath validates Docker Compose file paths used by templates.
func (d *DockerInstaller) ValidateComposeFilePath(path string) error {
	if path == "" {
		return fmt.Errorf("invalid compose file path: path cannot be empty")
	}
	if len(path) > MaxCommandLength {
		return fmt.Errorf("invalid compose file path: path too long (max %d characters)", MaxCommandLength)
	}
	if filepath.IsAbs(path) {
		return fmt.Errorf("invalid compose file path: absolute paths are not allowed")
	}
	if strings.ContainsRune(path, 0) {
		return fmt.Errorf("invalid compose file path: path contains null bytes")
	}
	dangerousChars := []string{";", "&", "|", "`", "$", "<", ">", "\n", "\r"}
	for _, char := range dangerousChars {
		if strings.Contains(path, char) {
			return fmt.Errorf("invalid compose file path: path contains potentially dangerous character")
		}
	}
	cleaned := filepath.Clean(path)
	if cleaned == "." || cleaned == ".." {
		return fmt.Errorf("invalid compose file path")
	}
	for _, part := range strings.Split(cleaned, string(filepath.Separator)) {
		if part == ".." {
			return fmt.Errorf("invalid compose file path: path traversal is not allowed")
		}
	}
	return nil
}

func (d *DockerInstaller) ValidateDockerBuildPath(path string) error {
	if path == "" {
		return fmt.Errorf("invalid docker build path: path cannot be empty")
	}
	if len(path) > MaxCommandLength {
		return fmt.Errorf("invalid docker build path: path too long (max %d characters)", MaxCommandLength)
	}
	if strings.ContainsRune(path, 0) {
		return fmt.Errorf("invalid docker build path: path contains null bytes")
	}
	dangerousChars := []string{";", "&", "|", "`", "$", "<", ">", "\n", "\r"}
	for _, char := range dangerousChars {
		if strings.Contains(path, char) {
			return fmt.Errorf("invalid docker build path: path contains potentially dangerous character")
		}
	}
	cleaned := filepath.Clean(path)
	if cleaned == ".." {
		return fmt.Errorf("invalid docker build path")
	}
	for _, part := range strings.Split(cleaned, string(filepath.Separator)) {
		if part == ".." {
			return fmt.Errorf("invalid docker build path: path traversal is not allowed")
		}
	}
	return nil
}

func (d *DockerInstaller) ValidateDockerFilter(filter string) error {
	if filter == "" {
		return fmt.Errorf("docker filter cannot be empty")
	}
	if len(filter) > MaxCommandLength {
		return fmt.Errorf("docker filter too long (max %d characters)", MaxCommandLength)
	}
	if strings.ContainsRune(filter, 0) || strings.ContainsAny(filter, "\n\r") {
		return fmt.Errorf("docker filter contains unsafe control characters")
	}
	if strings.ContainsAny(filter, ";&|`$<>") {
		return fmt.Errorf("docker filter contains potentially dangerous character")
	}
	key, value, ok := strings.Cut(filter, "=")
	if !ok || key == "" || value == "" {
		return fmt.Errorf("docker filter must use key=value format")
	}
	validFilterKey := regexp.MustCompile(`^[A-Za-z0-9_.-]+$`)
	if !validFilterKey.MatchString(key) {
		return fmt.Errorf("invalid docker filter key")
	}
	return nil
}

func (d *DockerInstaller) ValidateDockerFormat(format string) error {
	if format == "" {
		return fmt.Errorf("docker format cannot be empty")
	}
	if len(format) > MaxCommandLength {
		return fmt.Errorf("docker format too long (max %d characters)", MaxCommandLength)
	}
	if strings.ContainsRune(format, 0) || strings.ContainsAny(format, "\n\r") {
		return fmt.Errorf("docker format contains unsafe control characters")
	}
	if strings.ContainsAny(format, ";&|`$<>") {
		return fmt.Errorf("docker format contains potentially dangerous character")
	}
	return nil
}

func (d *DockerInstaller) ValidateImageReferencePattern(pattern string) error {
	if pattern == "" {
		return fmt.Errorf("image reference pattern cannot be empty")
	}
	if len(pattern) > MaxImageNameLength {
		return fmt.Errorf("image reference pattern too long (max %d characters)", MaxImageNameLength)
	}
	if strings.ContainsRune(pattern, 0) || strings.ContainsAny(pattern, "\n\r") {
		return fmt.Errorf("image reference pattern contains unsafe control characters")
	}
	validPatternRegex := regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:/@*-]*$`)
	if !validPatternRegex.MatchString(pattern) {
		return fmt.Errorf("invalid image reference pattern")
	}
	if strings.Contains(pattern, "..") {
		return fmt.Errorf("image reference pattern must not contain traversal sequences")
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
