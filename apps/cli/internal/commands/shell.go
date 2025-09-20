package commands

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/log"
	"github.com/jameswlane/devex/apps/cli/internal/types"
	"github.com/jameswlane/devex/apps/cli/internal/utils"
)

func init() {
	Register(NewShellCmd)
}

// NewShellCmd creates the shell command
func NewShellCmd(repo types.Repository, settings config.CrossPlatformSettings) *cobra.Command {
	return &cobra.Command{
		Use:   "shell [shell-name]",
		Short: "Switch your default shell",
		Long: `Switch your default shell interactively with password prompt.

This command changes your system's default shell using the 'chsh' command.
You'll be prompted for your password for security verification.

Supported shells:
  • bash - Bourne Again Shell
  • zsh - Z Shell (recommended)
  • fish - Friendly Interactive Shell

Examples:
  devex shell zsh      # Switch to zsh
  devex shell bash     # Switch to bash
  devex shell fish     # Switch to fish`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			targetShell := args[0]

			verbose := viper.GetBool("verbose")
			dryRun := viper.GetBool("dry-run")

			return executeShellSwitch(ctx, targetShell, verbose, dryRun)
		},
		SilenceUsage: true, // Prevent usage spam on runtime errors
	}
}

// executeShellSwitch implements the core shell switching logic with proper context handling
func executeShellSwitch(ctx context.Context, targetShell string, verbose, dryRun bool) error {
	log.Info("Starting shell switch process",
		"targetShell", targetShell,
		"verbose", verbose,
		"dry-run", dryRun,
	)

	// Validate shell name
	supportedShells := []string{"bash", "zsh", "fish"}
	if err := validateShell(targetShell, supportedShells); err != nil {
		return err
	}

	// Check if shell is available
	shellPath, err := exec.LookPath(targetShell)
	if err != nil {
		return fmt.Errorf("shell '%s' not found on system: %w", targetShell, err)
	}

	if dryRun {
		log.Info("Dry run - would switch shell", "target", targetShell, "path", shellPath)
		return nil
	}

	// Execute the shell switch
	return performShellSwitch(ctx, targetShell, shellPath, verbose)
}

// validateShell validates that the target shell is supported
func validateShell(targetShell string, supportedShells []string) error {
	for _, shell := range supportedShells {
		if shell == targetShell {
			return nil
		}
	}
	return fmt.Errorf("shell '%s' is not supported - available shells: %s",
		targetShell, strings.Join(supportedShells, ", "))
}

// performShellSwitch executes the actual shell change
func performShellSwitch(ctx context.Context, targetShell, shellPath string, verbose bool) error {
	// Get current user
	currentUser := os.Getenv("USER")
	if currentUser == "" {
		return fmt.Errorf("cannot determine current user - USER environment variable not set")
	}

	// Check current shell
	currentShell, err := utils.GetUserShell(currentUser)
	switch {
	case err != nil:
		log.Warn("Could not detect current shell", "error", err, "user", currentUser)
		log.Info("Proceeding with shell change anyway")
	case currentShell == shellPath:
		log.Info("Shell already set", "shell", targetShell, "path", shellPath, "user", currentUser)
		log.Info("Shell is already configured correctly")
		return nil
	default:
		log.Info("Current shell differs from target", "current", currentShell, "target", shellPath, "user", currentUser)
	}

	// Inform user about the process
	log.Info("Switching shell", "from", currentShell, "to", shellPath)

	// Use chsh interactively so user can enter password
	chshCmd := exec.CommandContext(ctx, "chsh", "-s", shellPath)

	// Connect stdin/stdout/stderr so user can interact with password prompt
	chshCmd.Stdin = os.Stdin
	chshCmd.Stdout = os.Stdout
	chshCmd.Stderr = os.Stderr

	log.Info("Executing interactive shell change", "shell", targetShell, "path", shellPath, "user", currentUser)

	// Execute the command
	if err := chshCmd.Run(); err != nil {
		return fmt.Errorf("failed to change shell to %s: %w", targetShell, err)
	}

	log.Info("Shell change completed successfully", "shell", targetShell, "path", shellPath)
	return nil
}
