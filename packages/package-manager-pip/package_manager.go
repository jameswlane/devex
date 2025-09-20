package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
)

// Execute handles command execution
func (p *PipPlugin) Execute(command string, args []string) error {
	ctx := context.Background()

	switch command {
	case "is-installed":
		return p.handleIsInstalled(ctx, args)
	case "create-venv":
		return p.handleCreateVenv(ctx, args)
	case "freeze":
		return p.handleFreeze(ctx, args)
	case "install":
		p.EnsureAvailable()
		return p.handleInstall(ctx, args)
	case "remove":
		p.EnsureAvailable()
		return p.handleRemove(ctx, args)
	case "update":
		p.EnsureAvailable()
		return p.handleUpdate(ctx, args)
	case "search":
		p.EnsureAvailable()
		return p.handleSearch(ctx, args)
	case "list":
		p.EnsureAvailable()
		return p.handleList(ctx, args)
	default:
		return fmt.Errorf("unknown command: %s", command)
	}
}

// handleInstall installs Python packages
func (p *PipPlugin) handleInstall(ctx context.Context, args []string) error {
	// Check for requirements.txt installation
	if len(args) > 0 && (args[0] == "-r" || args[0] == "--requirement") {
		return p.installFromRequirements(ctx, args[1:])
	}

	// Check if requirements.txt exists in current directory
	if len(args) == 0 {
		if _, err := os.Stat("requirements.txt"); err == nil {
			p.logger.Printf("Found requirements.txt, installing from file...\n")
			return p.installFromRequirements(ctx, []string{"requirements.txt"})
		}
		return fmt.Errorf("no packages specified and no requirements.txt found")
	}

	// Validate package names
	for _, pkg := range args {
		if err := p.validatePackageName(pkg); err != nil {
			return fmt.Errorf("invalid package name '%s': %w", pkg, err)
		}
	}

	// Detect virtual environment
	venvActive := p.isVirtualEnvActive(ctx)
	if venvActive {
		p.logger.Printf("📦 Installing in virtual environment\n")
	} else {
		p.logger.Printf("⚠️  Installing in system Python (consider using virtual environment)\n")
	}

	p.logger.Printf("Installing packages: %s\n", strings.Join(args, ", "))

	// Build install command with appropriate flags
	cmdArgs := []string{"install"}

	// Process flags
	processedArgs := []string{}
	for _, arg := range args {
		switch arg {
		case "--user":
			cmdArgs = append(cmdArgs, "--user")
		case "--upgrade":
			cmdArgs = append(cmdArgs, "--upgrade")
		default:
			processedArgs = append(processedArgs, arg)
		}
	}

	cmdArgs = append(cmdArgs, processedArgs...)
	return sdk.ExecCommandWithContext(ctx, true, "pip", cmdArgs...)
}

// handleRemove removes Python packages
func (p *PipPlugin) handleRemove(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no packages specified")
	}

	// Validate package names
	for _, pkg := range args {
		if err := p.validatePackageName(pkg); err != nil {
			return fmt.Errorf("invalid package name '%s': %w", pkg, err)
		}
	}

	// Detect virtual environment
	venvActive := p.isVirtualEnvActive(ctx)
	if venvActive {
		p.logger.Printf("📦 Removing from virtual environment\n")
	} else {
		p.logger.Printf("⚠️  Removing from system Python\n")
	}

	p.logger.Printf("Removing packages: %s\n", strings.Join(args, ", "))

	// Build uninstall command
	cmdArgs := []string{"uninstall"}

	// Process flags
	processedArgs := []string{}
	for _, arg := range args {
		switch arg {
		case "--yes", "-y":
			cmdArgs = append(cmdArgs, "-y")
		default:
			processedArgs = append(processedArgs, arg)
		}
	}

	cmdArgs = append(cmdArgs, processedArgs...)
	return sdk.ExecCommandWithContext(ctx, true, "pip", cmdArgs...)
}

// handleUpdate updates Python packages
func (p *PipPlugin) handleUpdate(ctx context.Context, args []string) error {
	// First update pip itself
	p.logger.Printf("Updating pip...\n")
	if err := sdk.ExecCommandWithContext(ctx, true, "pip", "install", "--upgrade", "pip"); err != nil {
		p.logger.Warning("Failed to update pip: %v", err)
	}

	// Check for --all flag to update all packages
	for _, arg := range args {
		if arg == "--all" {
			p.logger.Printf("Updating all installed packages...\n")
			return p.updateAllPackages(ctx)
		}
	}

	// If specific packages provided, update them
	if len(args) > 0 {
		// Validate package names
		processedArgs := []string{}
		for _, arg := range args {
			if arg != "--all" {
				if err := p.validatePackageName(arg); err != nil {
					return fmt.Errorf("invalid package name '%s': %w", arg, err)
				}
				processedArgs = append(processedArgs, arg)
			}
		}

		if len(processedArgs) > 0 {
			p.logger.Printf("Updating packages: %s\n", strings.Join(processedArgs, ", "))
			cmdArgs := append([]string{"install", "--upgrade"}, processedArgs...)
			return sdk.ExecCommandWithContext(ctx, true, "pip", cmdArgs...)
		}
	}

	p.logger.Printf("Pip updated. Use --all flag to update all installed packages.\n")
	return nil
}

// handleSearch searches for Python packages
func (p *PipPlugin) handleSearch(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no search term specified")
	}

	searchTerm := strings.Join(args, " ")
	if err := p.validateSearchTerm(searchTerm); err != nil {
		return fmt.Errorf("invalid search term: %w", err)
	}

	p.logger.Printf("Searching for: %s\n", searchTerm)

	// Note: pip search was disabled by PyPI, so we provide alternative
	p.logger.Printf("Note: pip search is no longer available. Try searching at https://pypi.org/search/?q=%s\n", strings.ReplaceAll(searchTerm, " ", "+"))

	// Try to use pip index if available (pip >= 21.2)
	if err := sdk.ExecCommandWithContext(ctx, false, "pip", "index", "versions", searchTerm); err != nil {
		// Fallback: just show the PyPI URL
		p.logger.Printf("Search online at: https://pypi.org/search/?q=%s\n", strings.ReplaceAll(searchTerm, " ", "+"))
		return nil
	}

	return nil
}

// handleList lists installed Python packages
func (p *PipPlugin) handleList(ctx context.Context, args []string) error {
	cmdArgs := []string{"list"}

	// Process flags
	for _, arg := range args {
		switch arg {
		case "--outdated":
			cmdArgs = append(cmdArgs, "--outdated")
		case "--format":
			// Next argument should be the format
			continue
		case "columns", "freeze", "json":
			cmdArgs = append(cmdArgs, "--format", arg)
		}
	}

	// Show virtual environment status
	if p.isVirtualEnvActive(ctx) {
		p.logger.Printf("📦 Virtual environment active\n")
	} else {
		p.logger.Printf("🐍 System Python packages\n")
	}

	return sdk.ExecCommandWithContext(ctx, false, "pip", cmdArgs...)
}

// handleIsInstalled checks if a package is installed
func (p *PipPlugin) handleIsInstalled(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no package specified")
	}

	packageName := args[0]
	if err := p.validatePackageName(packageName); err != nil {
		return fmt.Errorf("invalid package name: %w", err)
	}

	if err := sdk.ExecCommandWithContext(ctx, false, "pip", "show", packageName); err != nil {
		p.logger.Printf("Package %s is not installed\n", packageName)
		os.Exit(1)
	} else {
		p.logger.Printf("Package %s is installed\n", packageName)
		os.Exit(0)
	}

	return nil
}

// updateAllPackages updates all installed packages
func (p *PipPlugin) updateAllPackages(ctx context.Context) error {
	// Get list of installed packages
	output, err := sdk.ExecCommandOutputWithContext(ctx, "pip", "list", "--outdated", "--format=freeze")
	if err != nil {
		return fmt.Errorf("failed to get outdated packages: %w", err)
	}

	if strings.TrimSpace(output) == "" {
		p.logger.Printf("All packages are up to date\n")
		return nil
	}

	lines := strings.Split(output, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Extract package name (format: package==version)
		if parts := strings.Split(line, "=="); len(parts) >= 1 {
			packageName := parts[0]
			p.logger.Printf("Updating %s...\n", packageName)
			if err := sdk.ExecCommandWithContext(ctx, true, "pip", "install", "--upgrade", packageName); err != nil {
				p.logger.Warning("Failed to update %s: %v", packageName, err)
			}
		}
	}

	p.logger.Success("Package updates completed")
	return nil
}
