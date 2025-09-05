package security

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/jameswlane/devex/apps/cli/internal/log"
	"github.com/jameswlane/devex/apps/cli/internal/types"
)

// SecureCommandExecutor provides enhanced command execution with pattern-based security validation
type SecureCommandExecutor struct {
	validator *CommandValidator
	dryRun    bool
}

// NewSecureCommandExecutor creates a new secure command executor
func NewSecureCommandExecutor(level SecurityLevel) *SecureCommandExecutor {
	return &SecureCommandExecutor{
		validator: NewCommandValidator(level),
		dryRun:    false,
	}
}

// NewSecureCommandExecutorWithApps creates a secure executor that automatically whitelists commands from app configs
func NewSecureCommandExecutorWithApps(level SecurityLevel, apps []types.CrossPlatformApp) *SecureCommandExecutor {
	executor := NewSecureCommandExecutor(level)

	// Extract commands from app configurations and whitelist them
	for _, app := range apps {
		osConfig := app.GetOSConfig()

		// Whitelist commands from install_command
		if osConfig.InstallCommand != "" {
			executor.whitelistCommandFromString(osConfig.InstallCommand)
		}

		// Whitelist commands from pre_install
		for _, cmd := range osConfig.PreInstall {
			executor.whitelistCommandFromString(cmd.Command)
			if cmd.Shell != "" {
				executor.whitelistCommandFromString(cmd.Shell)
			}
		}

		// Whitelist commands from post_install
		for _, cmd := range osConfig.PostInstall {
			executor.whitelistCommandFromString(cmd.Command)
			if cmd.Shell != "" {
				executor.whitelistCommandFromString(cmd.Shell)
			}
		}
	}

	return executor
}

// whitelistCommandFromString extracts the base command from a command string and whitelists it
func (sce *SecureCommandExecutor) whitelistCommandFromString(commandString string) {
	parts := strings.Fields(commandString)
	if len(parts) > 0 {
		baseCmd := parts[0]
		// Handle sudo specially
		if baseCmd == "sudo" && len(parts) > 1 {
			baseCmd = parts[1]
		}

		log.Debug("Auto-whitelisting command from app config", "command", baseCmd)
		sce.validator.AddToWhitelist(baseCmd)
	}
}

// SetDryRun enables or disables dry run mode
func (sce *SecureCommandExecutor) SetDryRun(dryRun bool) {
	sce.dryRun = dryRun
}

// ExecuteCommand executes a command with enhanced security validation
func (sce *SecureCommandExecutor) ExecuteCommand(ctx context.Context, command string) (string, error) {
	if sce.dryRun {
		log.Info("DRY RUN: Would execute command", "command", command)
		return "DRY RUN: Command validation passed", nil
	}

	// Validate the command using pattern-based security
	if err := sce.validator.ValidateCommand(command); err != nil {
		log.Warn("Command validation failed", "command", command, "error", err)
		return "", fmt.Errorf("command validation failed: %w", err)
	}

	log.Debug("Command validated successfully", "command", command)

	// Execute the command
	cmd := exec.CommandContext(ctx, "bash", "-c", command)

	// Set platform-specific process attributes
	setPlatformSpecificAttrs(cmd)

	output, err := cmd.CombinedOutput()
	return string(output), err
}

// AddToWhitelist adds a command to the custom whitelist
func (sce *SecureCommandExecutor) AddToWhitelist(command string) {
	sce.validator.AddToWhitelist(command)
}

// AddToBlacklist adds a command to the custom blacklist
func (sce *SecureCommandExecutor) AddToBlacklist(command string) {
	sce.validator.AddToBlacklist(command)
}

// SetSecurityLevel changes the security level
func (sce *SecureCommandExecutor) SetSecurityLevel(level SecurityLevel) {
	sce.validator.SetSecurityLevel(level)
}

// GetSecurityLevel returns the current security level
func (sce *SecureCommandExecutor) GetSecurityLevel() SecurityLevel {
	return sce.validator.GetSecurityLevel()
}
