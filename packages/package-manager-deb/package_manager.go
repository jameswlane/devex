package main

import (
	"context"
	"fmt"
	"strings"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
)

// DebInstaller implements DEB package installer functionality
type DebInstaller struct {
	*sdk.PackageManagerPlugin
	logger sdk.Logger
}

// Execute handles command execution
func (d *DebInstaller) Execute(command string, args []string) error {
	ctx := context.Background()

	// Ensure dpkg is available
	d.EnsureAvailable()

	switch command {
	case "install":
		return d.handleInstall(ctx, args)
	case "remove":
		return d.handleRemove(ctx, args)
	case "info":
		return d.handleInfo(ctx, args)
	case "list-files":
		return d.handleListFiles(ctx, args)
	case "verify":
		return d.handleVerify(ctx, args)
	case "is-installed":
		return d.handleIsInstalled(ctx, args)
	case "extract":
		return d.handleExtract(ctx, args)
	default:
		return fmt.Errorf("unknown command: %s", command)
	}
}

// handleInstall installs .deb packages with dependency resolution
func (d *DebInstaller) handleInstall(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no packages specified")
	}

	var debFiles []string
	tempFiles := make(map[string]bool)

	// Process each argument - could be a file path or URL
	for _, arg := range args {
		if strings.HasPrefix(arg, "http://") || strings.HasPrefix(arg, "https://") {
			// Download the .deb file
			d.logger.Printf("Downloading package from: %s\n", arg)
			localPath, err := d.downloadDebFile(ctx, arg)
			if err != nil {
				// Clean up any temp files
				for f := range tempFiles {
					d.cleanupTempFile(f)
				}
				return fmt.Errorf("failed to download %s: %w", arg, err)
			}
			debFiles = append(debFiles, localPath)
			tempFiles[localPath] = true
		} else {
			// Local file
			if err := d.validateFilePath(arg); err != nil {
				return fmt.Errorf("invalid file path %s: %w", arg, err)
			}
			debFiles = append(debFiles, arg)
		}
	}

	// Verify all packages before installation
	d.logger.Println("Verifying packages...")
	missingDeps := make(map[string]bool)
	for _, debFile := range debFiles {
		deps, err := d.getMissingDependencies(debFile)
		if err != nil {
			d.logger.Warning("Failed to check dependencies for %s: %v", debFile, err)
			continue
		}
		for _, dep := range deps {
			missingDeps[dep] = true
		}
	}

	// Install missing dependencies if any
	if len(missingDeps) > 0 {
		d.logger.Printf("Installing missing dependencies: %v\n", mapKeys(missingDeps))
		depList := mapKeys(missingDeps)
		if err := d.installDependencies(ctx, depList); err != nil {
			d.logger.Warning("Failed to install some dependencies: %v", err)
			// Continue anyway, dpkg will handle it
		}
	}

	// Install the .deb packages
	for _, debFile := range debFiles {
		d.logger.Printf("Installing package: %s\n", debFile)
		if err := sdk.ExecCommandWithContext(ctx, true, "dpkg", "-i", debFile); err != nil {
			// Try to fix broken dependencies
			d.logger.Warning("Installation failed, attempting to fix dependencies...")
			if fixErr := sdk.ExecCommandWithContext(ctx, true, "apt-get", "install", "-f", "-y"); fixErr != nil {
				d.logger.ErrorMsg("Failed to fix dependencies: %v", fixErr)
			}
			return fmt.Errorf("failed to install %s: %w", debFile, err)
		}
	}

	// Clean up temporary files
	for f := range tempFiles {
		d.cleanupTempFile(f)
	}

	d.logger.Success("Successfully installed %d package(s)", len(debFiles))
	return nil
}

// handleRemove removes installed packages
func (d *DebInstaller) handleRemove(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no packages specified")
	}

	d.logger.Printf("Removing packages: %s\n", strings.Join(args, ", "))

	for _, pkg := range args {
		if err := d.validatePackageName(pkg); err != nil {
			return fmt.Errorf("invalid package name '%s': %w", pkg, err)
		}

		// Check if the package is installed
		if installed, _ := d.isPackageInstalled(pkg); !installed {
			d.logger.Printf("Package %s is not installed, skipping\n", pkg)
			continue
		}

		// Remove the package
		if err := sdk.ExecCommandWithContext(ctx, true, "dpkg", "-r", pkg); err != nil {
			return fmt.Errorf("failed to remove package %s: %w", pkg, err)
		}
	}

	d.logger.Success("Successfully removed packages")
	return nil
}

// handleInfo shows information about a .deb package
func (d *DebInstaller) handleInfo(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no package file specified")
	}

	for _, debFile := range args {
		d.logger.Printf("Package information for: %s\n", debFile)

		// Check if it's a file or installed package
		if d.isLocalFile(debFile) {
			if err := sdk.ExecCommandWithContext(ctx, false, "dpkg-deb", "-I", debFile); err != nil {
				return fmt.Errorf("failed to get info for %s: %w", debFile, err)
			}
		} else {
			// Try as an installed package
			if err := sdk.ExecCommandWithContext(ctx, false, "dpkg", "-s", debFile); err != nil {
				return fmt.Errorf("package %s not found (neither as file nor installed package)", debFile)
			}
		}

		if len(args) > 1 {
			fmt.Println("---")
		}
	}

	return nil
}

// handleListFiles lists files in a .deb package
func (d *DebInstaller) handleListFiles(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no package file specified")
	}

	for _, debFile := range args {
		d.logger.Printf("Files in package: %s\n", debFile)

		// Check if it's a file or installed package
		if d.isLocalFile(debFile) {
			if err := sdk.ExecCommandWithContext(ctx, false, "dpkg-deb", "-c", debFile); err != nil {
				return fmt.Errorf("failed to list files for %s: %w", debFile, err)
			}
		} else {
			// Try as an installed package
			if err := sdk.ExecCommandWithContext(ctx, false, "dpkg", "-L", debFile); err != nil {
				return fmt.Errorf("package %s not found (neither as file nor installed package)", debFile)
			}
		}

		if len(args) > 1 {
			fmt.Println("---")
		}
	}

	return nil
}

// handleVerify verifies package integrity and dependencies
func (d *DebInstaller) handleVerify(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no package file specified")
	}

	for _, debFile := range args {
		if err := d.validateFilePath(debFile); err != nil {
			return fmt.Errorf("package file validation failed for %s: %w", debFile, err)
		}

		d.logger.Printf("Verifying package: %s\n", debFile)

		// Check package integrity
		if err := sdk.ExecCommandWithContext(ctx, false, "dpkg-deb", "--info", debFile); err != nil {
			d.logger.ErrorMsg("Package integrity check failed for %s", debFile)
			return err
		}

		// Check dependencies
		deps, err := d.getMissingDependencies(debFile)
		if err != nil {
			d.logger.Warning("Failed to check dependencies: %v", err)
		} else if len(deps) > 0 {
			d.logger.Warning("Missing dependencies: %s", strings.Join(deps, ", "))
		} else {
			d.logger.Success("All dependencies are satisfied")
		}
	}

	return nil
}

// handleIsInstalled checks if packages are installed
func (d *DebInstaller) handleIsInstalled(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no packages specified")
	}

	allInstalled := true
	for _, pkg := range args {
		if err := d.validatePackageName(pkg); err != nil {
			return fmt.Errorf("invalid package name '%s': %w", pkg, err)
		}

		installed, err := d.isPackageInstalled(pkg)
		if err != nil {
			return fmt.Errorf("failed to check installation status of %s: %w", pkg, err)
		}

		if installed {
			d.logger.Success("Package %s is installed", pkg)
		} else {
			d.logger.ErrorMsg("Package %s is not installed", pkg)
			allInstalled = false
		}
	}

	if !allInstalled {
		return fmt.Errorf("one or more packages are not installed")
	}
	return nil
}

// handleExtract extracts .deb package contents without installing
func (d *DebInstaller) handleExtract(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no package file specified")
	}

	debFile := args[0]
	if err := d.validateFilePath(debFile); err != nil {
		return fmt.Errorf("package file validation failed: %w", err)
	}

	// Determine target directory
	targetDir := "."
	if len(args) > 1 {
		targetDir = args[1]
		if err := d.validateFilePath(targetDir); err != nil {
			return fmt.Errorf("target directory validation failed: %w", err)
		}
	}

	return d.extractPackage(ctx, debFile, targetDir)
}

// isPackageInstalled checks if a package is installed
func (d *DebInstaller) isPackageInstalled(packageName string) (bool, error) {
	// Use dpkg-query to check if the package is installed
	ctx := context.Background()
	output, err := sdk.ExecCommandOutputWithContext(ctx, "dpkg-query", "-W", "-f=${Status}", packageName)
	if err != nil {
		// Package not found
		return false, nil
	}

	// Check if package is properly installed
	return strings.Contains(output, "install ok installed"), nil
}

// mapKeys returns the keys of a map as a slice
func mapKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
