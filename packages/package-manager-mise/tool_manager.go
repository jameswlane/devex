package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
)

// HandleInstall installs development tools using Mise
func (m *MisePlugin) HandleInstall(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no tools specified")
	}

	m.logger.Printf("Installing tools with Mise: %s\n", strings.Join(args, ", "))

	// Validate all tools first
	for _, tool := range args {
		if err := m.ValidateToolSpec(tool); err != nil {
			return fmt.Errorf("invalid tool specification '%s': %w", tool, err)
		}
	}

	// Check if global flag is set
	globalFlag := "--global"
	miseLocal := os.Getenv("MISE_LOCAL")
	if miseLocal != "" {
		// Validate environment variable value to prevent manipulation
		if miseLocal != "1" && miseLocal != "true" && miseLocal != "0" && miseLocal != "false" {
			m.logger.Warning("Invalid MISE_LOCAL value, using default --global")
		} else if miseLocal == "1" || miseLocal == "true" {
			globalFlag = "--local"
		}
	}

	// Use parallel installer for multiple tools
	if len(args) > 1 {
		parallelInstaller := NewParallelInstaller(m.logger)

		// Progress callback
		progressCallback := func(completed, failed, total int) {
			m.logger.Info("Installation progress", "completed", completed, "failed", failed, "total", total)
		}

		results, err := parallelInstaller.InstallToolsWithProgress(ctx, args, globalFlag, progressCallback)

		// Report results
		for _, result := range results {
			if result.Success {
				m.logger.Success("Installed tool: %s (took %v)", result.Tool, result.Duration)
			} else {
				m.logger.Error("Failed to install tool", result.Error, "tool", result.Tool)
			}
		}

		return err
	}

	// Single tool installation (sequential)
	tool := args[0]
	if err := sdk.ExecCommandWithContext(ctx, false, "mise", "install", globalFlag, tool); err != nil {
		return fmt.Errorf("failed to install tool '%s': %w", tool, err)
	}

	m.logger.Success("Installed tool: %s", tool)

	return nil
}

// HandleRemove removes development tools using Mise
func (m *MisePlugin) HandleRemove(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no tools specified")
	}

	m.logger.Printf("Removing tools with Mise: %s\n", strings.Join(args, ", "))

	// Validate all tools first
	for _, tool := range args {
		if err := m.ValidateToolSpec(tool); err != nil {
			return fmt.Errorf("invalid tool specification '%s': %w", tool, err)
		}
	}

	// Use parallel remover for multiple tools
	if len(args) > 1 {
		parallelInstaller := NewParallelInstaller(m.logger)
		results, err := parallelInstaller.RemoveTools(ctx, args)

		// Report results
		for _, result := range results {
			if result.Success {
				m.logger.Success("Removed tool: %s (took %v)", result.Tool, result.Duration)
			} else {
				m.logger.Warning("Failed to remove tool '%s': %v", result.Tool, result.Error)
			}
		}

		return err
	}

	// Single tool removal (sequential)
	tool := args[0]
	if err := sdk.ExecCommandWithContext(ctx, false, "mise", "uninstall", tool); err != nil {
		m.logger.Warning("Failed to remove tool '%s': %v", tool, err)
		return err
	}

	m.logger.Success("Removed tool: %s", tool)
	return nil
}

// HandleUpdate updates Mise plugins and tool versions
func (m *MisePlugin) HandleUpdate(ctx context.Context, args []string) error {
	m.logger.Println("Updating Mise plugins and tools...")

	// Update plugins
	if err := sdk.ExecCommandWithContext(ctx, false, "mise", "plugins", "update"); err != nil {
		m.logger.Warning("Failed to update plugins: %v", err)
	}

	// Update tools to latest versions
	if err := sdk.ExecCommandWithContext(ctx, false, "mise", "upgrade"); err != nil {
		return fmt.Errorf("failed to update tools: %w", err)
	}

	m.logger.Success("Mise plugins and tools updated successfully")
	return nil
}

// HandleSearch searches for available tools
func (m *MisePlugin) HandleSearch(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no search term specified")
	}

	searchTerm := strings.Join(args, " ")
	// Validate search term
	if err := m.ValidateCommandArg(searchTerm); err != nil {
		return fmt.Errorf("invalid search term: %w", err)
	}
	m.logger.Printf("Searching for tools: %s\n", searchTerm)

	// Search for plugins
	if err := sdk.ExecCommandWithContext(ctx, false, "mise", "plugins", "ls-remote", searchTerm); err != nil {
		return fmt.Errorf("failed to search for tools: %w", err)
	}

	return nil
}

// HandleList lists installed tools
func (m *MisePlugin) HandleList(ctx context.Context, args []string) error {
	m.logger.Println("Listing installed tools...")

	// Parse flags
	showAll := false
	showCurrent := false
	showOutdated := false

	for _, arg := range args {
		switch arg {
		case "--all":
			showAll = true
		case "--current":
			showCurrent = true
		case "--outdated":
			showOutdated = true
		}
	}

	if showAll {
		return sdk.ExecCommandWithContext(ctx, false, "mise", "ls", "--all")
	} else if showCurrent {
		return sdk.ExecCommandWithContext(ctx, false, "mise", "current")
	} else if showOutdated {
		return sdk.ExecCommandWithContext(ctx, false, "mise", "outdated")
	}

	// Default: show installed tools
	return sdk.ExecCommandWithContext(ctx, false, "mise", "ls")
}

// HandleIsInstalled checks if a tool is installed
func (m *MisePlugin) HandleIsInstalled(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no tool specified")
	}

	tool := args[0]
	if err := m.ValidateToolSpec(tool); err != nil {
		return fmt.Errorf("invalid tool specification '%s': %w", tool, err)
	}

	// Check if tool is installed
	output, err := sdk.ExecCommandOutputWithContext(ctx, "mise", "current", tool)
	if err != nil || strings.TrimSpace(output) == "" {
		return fmt.Errorf("tool '%s' is not installed", tool)
	}

	m.logger.Success("Tool %s is installed: %s", tool, strings.TrimSpace(output))
	return nil
}
