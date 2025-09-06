package main

import (
	"fmt"
	"os"
	"strings"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
)

// handleInstall installs development tools using Mise
func (m *MisePlugin) handleInstall(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no tools specified")
	}

	m.logger.Printf("Installing tools with Mise: %s\n", strings.Join(args, ", "))

	for _, tool := range args {
		if err := m.validateMiseCommand(tool); err != nil {
			return fmt.Errorf("invalid tool specification '%s': %w", tool, err)
		}

		// Check if global flag is set
		globalFlag := "--global"
		if os.Getenv("MISE_LOCAL") == "1" {
			globalFlag = "--local"
		}

		// Install the tool
		if err := sdk.ExecCommand(false, "mise", "install", globalFlag, tool); err != nil {
			return fmt.Errorf("failed to install tool '%s': %w", tool, err)
		}

		m.logger.Success("Installed tool: %s", tool)
	}

	return nil
}

// handleRemove removes development tools using Mise
func (m *MisePlugin) handleRemove(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no tools specified")
	}

	m.logger.Printf("Removing tools with Mise: %s\n", strings.Join(args, ", "))

	for _, tool := range args {
		if err := m.validateMiseCommand(tool); err != nil {
			return fmt.Errorf("invalid tool specification '%s': %w", tool, err)
		}

		// Remove the tool
		if err := sdk.ExecCommand(false, "mise", "uninstall", tool); err != nil {
			m.logger.Warning("Failed to remove tool '%s': %v", tool, err)
			continue
		}

		m.logger.Success("Removed tool: %s", tool)
	}

	return nil
}

// handleUpdate updates Mise plugins and tool versions
func (m *MisePlugin) handleUpdate(args []string) error {
	m.logger.Println("Updating Mise plugins and tools...")

	// Update plugins
	if err := sdk.ExecCommand(false, "mise", "plugins", "update"); err != nil {
		m.logger.Warning("Failed to update plugins: %v", err)
	}

	// Update tools to latest versions
	if err := sdk.ExecCommand(false, "mise", "upgrade"); err != nil {
		return fmt.Errorf("failed to update tools: %w", err)
	}

	m.logger.Success("Mise plugins and tools updated successfully")
	return nil
}

// handleSearch searches for available tools
func (m *MisePlugin) handleSearch(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no search term specified")
	}

	searchTerm := strings.Join(args, " ")
	m.logger.Printf("Searching for tools: %s\n", searchTerm)

	// Search for plugins
	if err := sdk.ExecCommand(false, "mise", "plugins", "ls-remote", searchTerm); err != nil {
		return fmt.Errorf("failed to search for tools: %w", err)
	}

	return nil
}

// handleList lists installed tools
func (m *MisePlugin) handleList(args []string) error {
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
		return sdk.ExecCommand(false, "mise", "ls", "--all")
	} else if showCurrent {
		return sdk.ExecCommand(false, "mise", "current")
	} else if showOutdated {
		return sdk.ExecCommand(false, "mise", "outdated")
	}

	// Default: show installed tools
	return sdk.ExecCommand(false, "mise", "ls")
}

// handleIsInstalled checks if a tool is installed
func (m *MisePlugin) handleIsInstalled(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no tool specified")
	}

	tool := args[0]
	if err := m.validateMiseCommand(tool); err != nil {
		return fmt.Errorf("invalid tool specification '%s': %w", tool, err)
	}

	// Check if tool is installed
	output, err := sdk.ExecCommandOutput("mise", "current", tool)
	if err != nil || strings.TrimSpace(output) == "" {
		return fmt.Errorf("tool '%s' is not installed", tool)
	}

	m.logger.Success("Tool %s is installed: %s", tool, strings.TrimSpace(output))
	return nil
}
