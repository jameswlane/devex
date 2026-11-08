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
		return nil, fmt.Errorf("read Depends field: %w", err)
	}

	if strings.TrimSpace(output) == "" {
		return nil, nil // No dependencies
	}

	var missing []string
	for _, group := range strings.Split(output, ",") {
		var first string
		anyInstalled := false
		// Split alternatives before removing each member's version constraint.
		for _, alternative := range strings.Split(group, "|") {
			name := strings.TrimSpace(alternative)
			// This best-effort preflight ignores versions and architecture qualifiers;
			// dpkg -i remains the authoritative dependency check.
			if idx := strings.IndexAny(name, "( :\t\r\n"); idx >= 0 {
				name = name[:idx]
			}
			if name == "" {
				continue
			}
			if first == "" {
				first = name
			}
			installed, err := d.isPackageInstalled(name)
			if err != nil {
				return nil, err
			}
			if installed {
				anyInstalled = true
				break
			}
		}
		if !anyInstalled && first != "" {
			missing = append(missing, first)
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
		// Best-effort: apt-get install below fails if stale lists matter.
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
