package main

import (
	"context"
	"fmt"
	"strings"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
)

// getMissingDependencies checks for missing dependencies of a .deb package
func (d *DebInstaller) getMissingDependencies(debFile string) ([]string, error) {
	// Get package dependencies
	ctx := context.Background()
	output, err := sdk.ExecCommandOutputWithContext(ctx, "dpkg-deb", "-f", debFile, "Depends")
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
func (d *DebInstaller) installDependencies(ctx context.Context, deps []string) error {
	if len(deps) == 0 {
		return nil
	}

	// Validate all dependency names first
	for _, dep := range deps {
		if err := d.validatePackageName(dep); err != nil {
			return fmt.Errorf("invalid dependency name '%s': %w", dep, err)
		}
	}

	// Update package lists first
	d.logger.Println("Updating package lists...")
	if err := sdk.ExecCommandWithContext(ctx, true, "apt-get", "update"); err != nil {
		d.logger.Warning("Failed to update package lists: %v", err)
	}

	// Install dependencies
	cmdArgs := append([]string{"install", "-y"}, deps...)
	if err := sdk.ExecCommandWithContext(ctx, true, "apt-get", cmdArgs...); err != nil {
		return fmt.Errorf("failed to install dependencies: %w", err)
	}

	d.logger.Success("Successfully installed dependencies: %s", strings.Join(deps, ", "))
	return nil
}
