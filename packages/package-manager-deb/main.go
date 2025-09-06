package main

// Build timestamp: 2025-09-06

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
)

var version = "dev" // Set by goreleaser

// DebInstaller implements DEB package installer functionality
type DebInstaller struct {
	*sdk.PackageManagerPlugin
	logger sdk.Logger
}

// NewDebPlugin creates a new DEB plugin
func NewDebPlugin() *DebInstaller {
	info := sdk.PluginInfo{
		Name:        "package-manager-deb",
		Version:     version,
		Description: "Debian package (.deb) installer for local and remote packages",
		Author:      "DevEx Team",
		Repository:  "https://github.com/jameswlane/devex",
		Tags:        []string{"package-manager", "deb", "debian", "ubuntu", "dpkg"},
		Commands: []sdk.PluginCommand{
			{
				Name:        "install",
				Description: "Install .deb packages from file or URL",
				Usage:       "Install one or more .deb packages with automatic dependency resolution",
				Flags: map[string]string{
					"force":         "Force installation even if dependencies are missing",
					"no-deps":       "Skip dependency installation",
					"download-only": "Download packages without installing",
					"target-dir":    "Directory to download packages to",
				},
			},
			{
				Name:        "remove",
				Description: "Remove installed packages",
				Usage:       "Remove packages installed via dpkg",
				Flags: map[string]string{
					"purge": "Remove packages and their configuration files",
				},
			},
			{
				Name:        "info",
				Description: "Show information about a .deb package",
				Usage:       "Display detailed information about a .deb file",
			},
			{
				Name:        "list-files",
				Description: "List files in a .deb package",
				Usage:       "Show all files that will be installed by a .deb package",
			},
			{
				Name:        "verify",
				Description: "Verify package integrity and dependencies",
				Usage:       "Check if a .deb package can be installed and list missing dependencies",
			},
			{
				Name:        "is-installed",
				Description: "Check if a package is installed",
				Usage:       "Returns exit code 0 if package is installed, 1 if not",
			},
			{
				Name:        "extract",
				Description: "Extract .deb package contents without installing",
				Usage:       "Extract package contents to a specified directory",
				Flags: map[string]string{
					"target-dir": "Directory to extract files to",
				},
			},
		},
	}

	return &DebInstaller{
		PackageManagerPlugin: sdk.NewPackageManagerPlugin(info, "dpkg"),
		logger:               sdk.NewDefaultLogger(false),
	}
}

// Execute handles command execution
func (d *DebInstaller) Execute(command string, args []string) error {
	// Ensure dpkg is available
	d.EnsureAvailable()

	switch command {
	case "install":
		return d.handleInstall(args)
	case "remove":
		return d.handleRemove(args)
	case "info":
		return d.handleInfo(args)
	case "list-files":
		return d.handleListFiles(args)
	case "verify":
		return d.handleVerify(args)
	case "is-installed":
		return d.handleIsInstalled(args)
	case "extract":
		return d.handleExtract(args)
	default:
		return fmt.Errorf("unknown command: %s", command)
	}
}

// handleInstall installs .deb packages with dependency resolution
func (d *DebInstaller) handleInstall(args []string) error {
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
			localPath, err := d.downloadDebFile(arg)
			if err != nil {
				// Clean up any temp files
				for f := range tempFiles {
					os.Remove(f)
				}
				return fmt.Errorf("failed to download %s: %w", arg, err)
			}
			debFiles = append(debFiles, localPath)
			tempFiles[localPath] = true
		} else {
			// Local file
			if _, err := os.Stat(arg); err != nil {
				return fmt.Errorf("package file not found: %s", arg)
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
		if err := d.installDependencies(depList); err != nil {
			d.logger.Warning("Failed to install some dependencies: %v", err)
			// Continue anyway, dpkg will handle it
		}
	}

	// Install the .deb packages
	for _, debFile := range debFiles {
		d.logger.Printf("Installing package: %s\n", debFile)
		if err := sdk.ExecCommand(true, "dpkg", "-i", debFile); err != nil {
			// Try to fix broken dependencies
			d.logger.Warning("Installation failed, attempting to fix dependencies...")
			if fixErr := sdk.ExecCommand(true, "apt-get", "install", "-f", "-y"); fixErr != nil {
				d.logger.ErrorMsg("Failed to fix dependencies: %v", fixErr)
			}
			return fmt.Errorf("failed to install %s: %w", debFile, err)
		}
	}

	// Clean up temporary files
	for f := range tempFiles {
		if err := os.Remove(f); err != nil {
			d.logger.Warning("Failed to remove temporary file %s: %v", f, err)
		}
	}

	d.logger.Success("Successfully installed %d package(s)", len(debFiles))
	return nil
}

// downloadDebFile downloads a .deb file from a URL
func (d *DebInstaller) downloadDebFile(url string) (string, error) {
	// Create a temporary file
	tmpFile, err := os.CreateTemp("", "*.deb")
	if err != nil {
		return "", fmt.Errorf("failed to create temporary file: %w", err)
	}
	defer tmpFile.Close()

	// Download the file
	resp, err := http.Get(url)
	if err != nil {
		os.Remove(tmpFile.Name())
		return "", fmt.Errorf("failed to download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		os.Remove(tmpFile.Name())
		return "", fmt.Errorf("download failed with status: %s", resp.Status)
	}

	// Copy the response body to the file
	_, err = io.Copy(tmpFile, resp.Body)
	if err != nil {
		os.Remove(tmpFile.Name())
		return "", fmt.Errorf("failed to save file: %w", err)
	}

	return tmpFile.Name(), nil
}

// getMissingDependencies checks for missing dependencies of a .deb package
func (d *DebInstaller) getMissingDependencies(debFile string) ([]string, error) {
	// Get package dependencies
	output, err := sdk.ExecCommandOutput("dpkg-deb", "-f", debFile, "Depends")
	if err != nil {
		return nil, err
	}

	if strings.TrimSpace(output) == "" {
		return nil, nil // No dependencies
	}

	// Parse dependencies
	var missing []string
	deps := strings.Split(output, ",")
	for _, dep := range deps {
		// Clean up dependency string
		dep = strings.TrimSpace(dep)
		// Remove version constraints if present
		if idx := strings.IndexAny(dep, " ("); idx > 0 {
			dep = dep[:idx]
		}

		// Skip if it's an OR dependency (contains |)
		if strings.Contains(dep, "|") {
			parts := strings.Split(dep, "|")
			// Check if any of the alternatives is installed
			anyInstalled := false
			for _, part := range parts {
				part = strings.TrimSpace(part)
				if installed, _ := d.isPackageInstalled(part); installed {
					anyInstalled = true
					break
				}
			}
			if !anyInstalled && len(parts) > 0 {
				// Use the first alternative
				missing = append(missing, strings.TrimSpace(parts[0]))
			}
			continue
		}

		// Check if dependency is installed
		if installed, _ := d.isPackageInstalled(dep); !installed && dep != "" {
			missing = append(missing, dep)
		}
	}

	return missing, nil
}

// installDependencies installs missing dependencies using apt
func (d *DebInstaller) installDependencies(deps []string) error {
	if len(deps) == 0 {
		return nil
	}

	// Update package lists first
	d.logger.Println("Updating package lists...")
	if err := sdk.ExecCommand(true, "apt-get", "update"); err != nil {
		d.logger.Warning("Failed to update package lists: %v", err)
	}

	// Install dependencies
	cmdArgs := append([]string{"install", "-y"}, deps...)
	return sdk.ExecCommand(true, "apt-get", cmdArgs...)
}

// isPackageInstalled checks if a package is installed
func (d *DebInstaller) isPackageInstalled(packageName string) (bool, error) {
	// Use dpkg-query to check if package is installed
	output, err := sdk.ExecCommandOutput("dpkg-query", "-W", "-f=${Status}", packageName)
	if err != nil {
		// Package not found
		return false, nil
	}

	// Check if package is properly installed
	return strings.Contains(output, "install ok installed"), nil
}

// handleRemove removes installed packages
func (d *DebInstaller) handleRemove(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no packages specified")
	}

	d.logger.Printf("Removing packages: %s\n", strings.Join(args, ", "))

	for _, pkg := range args {
		// Check if package is installed
		if installed, _ := d.isPackageInstalled(pkg); !installed {
			d.logger.Printf("Package %s is not installed, skipping\n", pkg)
			continue
		}

		// Remove the package
		if err := sdk.ExecCommand(true, "dpkg", "-r", pkg); err != nil {
			return fmt.Errorf("failed to remove package %s: %w", pkg, err)
		}
	}

	d.logger.Success("Successfully removed packages")
	return nil
}

// handleInfo shows information about a .deb package
func (d *DebInstaller) handleInfo(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no package file specified")
	}

	for _, debFile := range args {
		d.logger.Printf("Package information for: %s\n", debFile)

		// Check if it's a file or installed package
		if _, err := os.Stat(debFile); err == nil {
			// It's a file
			if err := sdk.ExecCommand(false, "dpkg-deb", "-I", debFile); err != nil {
				return fmt.Errorf("failed to get info for %s: %w", debFile, err)
			}
		} else {
			// Try as installed package
			if err := sdk.ExecCommand(false, "dpkg", "-s", debFile); err != nil {
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
func (d *DebInstaller) handleListFiles(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no package file specified")
	}

	for _, debFile := range args {
		d.logger.Printf("Files in package: %s\n", debFile)

		// Check if it's a file or installed package
		if _, err := os.Stat(debFile); err == nil {
			// It's a file
			if err := sdk.ExecCommand(false, "dpkg-deb", "-c", debFile); err != nil {
				return fmt.Errorf("failed to list files for %s: %w", debFile, err)
			}
		} else {
			// Try as installed package
			if err := sdk.ExecCommand(false, "dpkg", "-L", debFile); err != nil {
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
func (d *DebInstaller) handleVerify(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no package file specified")
	}

	for _, debFile := range args {
		if _, err := os.Stat(debFile); err != nil {
			return fmt.Errorf("package file not found: %s", debFile)
		}

		d.logger.Printf("Verifying package: %s\n", debFile)

		// Check package integrity
		if err := sdk.ExecCommand(false, "dpkg-deb", "--info", debFile); err != nil {
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
func (d *DebInstaller) handleIsInstalled(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no packages specified")
	}

	allInstalled := true
	for _, pkg := range args {
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
func (d *DebInstaller) handleExtract(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no package file specified")
	}

	debFile := args[0]
	if _, err := os.Stat(debFile); err != nil {
		return fmt.Errorf("package file not found: %s", debFile)
	}

	// Determine target directory
	targetDir := "."
	if len(args) > 1 {
		targetDir = args[1]
	}

	// Create target directory if it doesn't exist
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return fmt.Errorf("failed to create target directory: %w", err)
	}

	d.logger.Printf("Extracting %s to %s\n", debFile, targetDir)

	// Get absolute paths
	absDebFile, err := filepath.Abs(debFile)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	absTargetDir, err := filepath.Abs(targetDir)
	if err != nil {
		return fmt.Errorf("failed to get absolute path: %w", err)
	}

	// Extract the package
	if err := sdk.ExecCommand(false, "dpkg-deb", "-x", absDebFile, absTargetDir); err != nil {
		return fmt.Errorf("failed to extract package: %w", err)
	}

	d.logger.Success("Successfully extracted package to %s", targetDir)
	return nil
}

// mapKeys returns the keys of a map as a slice
func mapKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

func main() {
	plugin := NewDebPlugin()

	// Handle args with panic recovery
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(os.Stderr, "Plugin panic recovered: %v\n", r)
			os.Exit(1)
		}
	}()

	sdk.HandleArgs(plugin, os.Args[1:])
}
