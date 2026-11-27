package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
)

// useGlobalConfig reports whether tool mutations should target the global mise
// config (--global) rather than the local project config.
func (m *MisePlugin) useGlobalConfig() bool {
	miseLocal := os.Getenv("MISE_LOCAL")
	if miseLocal == "" {
		return true
	}
	// Validate environment variable value to prevent manipulation
	if miseLocal != "1" && miseLocal != "true" && miseLocal != "0" && miseLocal != "false" {
		m.logger.Warning("Invalid MISE_LOCAL value, using default --global")
		return true
	}
	return miseLocal != "1" && miseLocal != "true"
}

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

	// `mise install` only downloads a version and has no --global flag; `mise
	// use` installs the tool and records it in the global config (or, with
	// MISE_LOCAL, the local one) so it is active on PATH.
	useArgs := []string{"use"}
	if m.useGlobalConfig() {
		useArgs = append(useArgs, "--global")
	}

	// Backend tools (npm:, pipx:, ...) have install dependencies such as node
	// or uv that mise only resolves when they are installed in the same call,
	// so a separate per-tool `mise install` fails before they are active.
	hasBackendTool := false
	for _, tool := range args {
		if strings.Contains(tool, ":") {
			hasBackendTool = true
		}
	}

	// Use parallel installer for multiple tools
	if len(args) > 1 && !hasBackendTool {
		parallelInstaller := NewParallelInstaller(m.logger)

		// Progress callback
		progressCallback := func(completed, failed, total int) {
			m.logger.Info("Installation progress", "completed", completed, "failed", failed, "total", total)
		}

		results, err := parallelInstaller.InstallToolsWithProgress(ctx, args, progressCallback)

		// Report results
		for _, result := range results {
			if result.Success {
				m.logger.Success("Downloaded tool: %s (took %v)", result.Tool, result.Duration)
			} else {
				m.logger.Error("Failed to install tool", result.Error, "tool", result.Tool)
			}
		}
		if err != nil {
			return err
		}

		// Activate every tool in one write to the mise config.
		if err := sdk.ExecCommandWithContext(ctx, false, "mise", append(useArgs, args...)...); err != nil {
			return fmt.Errorf("failed to activate tools %s: %w", strings.Join(args, ", "), err)
		}
		m.logger.Success("Installed tools: %s", strings.Join(args, ", "))
		return nil
	}

	// One `mise use` call installs and activates every tool in dependency order.
	tool := strings.Join(args, " ")
	if err := sdk.ExecCommandWithContext(ctx, false, "mise", append(useArgs, args...)...); err != nil {
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

	// `mise unuse --global` (equivalent to `mise unuse -g`) edits the global
	// config (~/.config/mise/config.toml) that HandleInstall wrote, leaving a
	// project's own mise.toml pin alone (or, with MISE_LOCAL, edits the local
	// config). `mise uninstall` would delete the files but leave the config
	// entry, so `mise current` would still report the tool. One call edits
	// the config once, so tools never race on the file.
	unuseArgs := []string{"unuse"}
	if m.useGlobalConfig() {
		unuseArgs = append(unuseArgs, "--global")
	}
	if err := sdk.ExecCommandWithContext(ctx, false, "mise", append(unuseArgs, args...)...); err != nil {
		m.logger.Warning("Failed to remove tools %s: %v", strings.Join(args, ", "), err)
		return err
	}

	// unuse can keep the install and its shim, which leaves the tool on
	// PATH. prune removes only versions no tracked config still uses, so a
	// project that pins the tool keeps it.
	pruneArgs := []string{"prune", "--yes"}
	for _, tool := range args {
		pruneArgs = append(pruneArgs, toolName(tool))
	}
	if err := sdk.ExecCommandWithContext(ctx, false, "mise", pruneArgs...); err != nil {
		return fmt.Errorf("failed to prune removed tools %s: %w", strings.Join(args, ", "), err)
	}

	m.logger.Success("Removed tools: %s", strings.Join(args, ", "))
	return nil
}

// HandleUpdate updates Mise plugins and tool versions. When tool specs are
// provided (e.g. node@24), it upgrades only those tools to the newest release
// matching each spec while keeping the recorded version prefix.
func (m *MisePlugin) HandleUpdate(ctx context.Context, args []string) error {
	if len(args) > 0 {
		for _, tool := range args {
			if err := m.ValidateToolSpec(tool); err != nil {
				return fmt.Errorf("invalid tool specification '%s': %w", tool, err)
			}
		}

		m.logger.Printf("Updating tools with Mise: %s\n", strings.Join(args, ", "))
		if err := sdk.ExecCommandWithContext(ctx, false, "mise", append([]string{"upgrade"}, args...)...); err != nil {
			return fmt.Errorf("failed to update tools %s: %w", strings.Join(args, ", "), err)
		}

		m.logger.Success("Updated tools: %s", strings.Join(args, ", "))
		return nil
	}

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

// HandleIsInstalled checks that every given tool is installed. A catalog entry
// can name a runtime with its tool (node@lts npm:...), and a runtime left in
// place after the tool's removal must not report the tool as installed.
func (m *MisePlugin) HandleIsInstalled(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no tool specified")
	}
	for _, tool := range args {
		if err := m.ValidateToolSpec(tool); err != nil {
			return fmt.Errorf("invalid tool specification '%s': %w", tool, err)
		}
	}
	var missing []string
	for _, tool := range args {
		output, err := sdk.ExecCommandOutputWithContext(ctx, "mise", "where", tool)
		if err != nil {
			// mise also exits 1 for errors such as an invalid config file, so
			// only its "missing version" messages mean the tool is absent.
			var exitErr *exec.ExitError
			if errors.As(err, &exitErr) {
				stderr := string(exitErr.Stderr)
				if strings.Contains(stderr, "not installed") || strings.Contains(stderr, "no versions found") {
					missing = append(missing, tool)
					continue
				}
			}
			return fmt.Errorf("failed to check whether tool '%s' is installed: %w", tool, err)
		}
		installPath := strings.TrimSpace(output)
		if installPath == "" {
			missing = append(missing, tool)
			continue
		}
		m.logger.Success("Tool %s is installed: %s", tool, installPath)
	}
	if len(missing) > 0 {
		return &sdk.NotInstalledError{Packages: missing}
	}
	return nil
}

// toolName drops a trailing @version from a tool spec while keeping an npm
// scope: npm:@google/gemini-cli@latest becomes npm:@google/gemini-cli.
func toolName(spec string) string {
	i := strings.LastIndex(spec, "@")
	if i <= 0 || spec[i-1] == ':' {
		return spec
	}
	return spec[:i]
}
