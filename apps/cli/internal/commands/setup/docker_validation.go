package setup

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
)

// Security validation patterns
var (
	// validContainerName ensures container names only contain safe characters
	validContainerName = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9_.-]*$`)
	// validDockerImage ensures image names follow Docker naming conventions
	validDockerImage = regexp.MustCompile(`^[a-z0-9]([a-z0-9._/-]*[a-z0-9])?:?[a-zA-Z0-9._-]*$`)
	// validPortMapping ensures port mappings follow expected format
	validPortMapping = regexp.MustCompile(`^[0-9]+:[0-9]+$`)
	// validEnvVar ensures environment variables are safe but allows realistic values
	// Allows: letters, numbers, underscores, hyphens, dots, @, spaces, and common special chars
	// Blocks: shell metacharacters, quotes, backticks, and command substitution
	validEnvVar = regexp.MustCompile(`^[A-Z_][A-Z0-9_]*=[a-zA-Z0-9_@#%+,:=/\.\s-]*$`)
	// validShellPath ensures shell paths are absolute and contain safe characters
	validShellPath = regexp.MustCompile(`^/[a-zA-Z0-9/_.-]+$`)
	// validUsername ensures usernames contain only safe characters
	validUsername = regexp.MustCompile(`^[a-zA-Z0-9_.-]+$`)
)

// ValidateDockerConfig validates Docker configuration parameters for security
func ValidateDockerConfig(containerName, image, portMapping, envVar string) error {
	if !validContainerName.MatchString(containerName) {
		return fmt.Errorf("invalid container name: %s", containerName)
	}
	if !validDockerImage.MatchString(image) {
		return fmt.Errorf("invalid Docker image: %s", image)
	}
	if !validPortMapping.MatchString(portMapping) {
		return fmt.Errorf("invalid port mapping: %s", portMapping)
	}
	if envVar != "" && !validEnvVar.MatchString(envVar) {
		return fmt.Errorf("invalid environment variable: %s", envVar)
	}
	return nil
}

// ValidateShellCommand validates parameters for shell command execution
func ValidateShellCommand(shellPath, username string) error {
	if !validShellPath.MatchString(shellPath) {
		return fmt.Errorf("invalid shell path: %s", shellPath)
	}
	if !validUsername.MatchString(username) {
		return fmt.Errorf("invalid username: %s", username)
	}
	return nil
}

// ExecuteSecureShellChange executes chsh command with proper validation and argument separation
func ExecuteSecureShellChange(ctx context.Context, shellPath, username string) error {
	// Validate inputs
	if err := ValidateShellCommand(shellPath, username); err != nil {
		return err
	}

	// Use exec.Command with separate arguments instead of shell string
	cmd := exec.CommandContext(ctx, "chsh", "-s", shellPath, username)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("failed to change shell: %w (output: %s)", err, string(output))
	}

	return nil
}
