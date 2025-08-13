package utils

import (
	"context"
	"fmt"
	"os/exec"

	"github.com/jameswlane/devex/pkg/log"
	"github.com/jameswlane/devex/pkg/platform"
	"github.com/jameswlane/devex/pkg/types"
)

type Dependency struct {
	Name    string
	Command string
}

// PackageManager interface for installing dependencies
type PackageManager interface {
	InstallPackages(ctx context.Context, packages []string, dryRun bool) error
	IsAvailable(ctx context.Context) bool
	GetName() string
}

// DependencyChecker handles dependency validation and installation
type DependencyChecker struct {
	packageManager PackageManager
	platform       platform.Platform
}

// NewDependencyChecker creates a new dependency checker with platform detection
func NewDependencyChecker(pm PackageManager, plat platform.Platform) *DependencyChecker {
	return &DependencyChecker{
		packageManager: pm,
		platform:       plat,
	}
}

// CheckDependencies verifies the availability of required dependencies.
func CheckDependencies(dependencies []Dependency) error {
	ctx := context.Background()
	for _, dep := range dependencies {
		if err := exec.CommandContext(ctx, "which", dep.Command).Run(); err != nil {
			log.Error("Missing dependency", err, "name", dep.Name, "command", dep.Command)
			return fmt.Errorf("missing dependency: %s (command: %s)", dep.Name, dep.Command)
		}
		log.Info("Dependency available", "name", dep.Name, "command", dep.Command)
	}
	return nil
}

// CheckAndInstallPlatformDependencies checks platform-specific dependencies and installs missing ones
func (dc *DependencyChecker) CheckAndInstallPlatformDependencies(ctx context.Context, osConfig types.OSConfig, dryRun bool) error {
	log.Info("Starting platform dependency validation", "platform_os", dc.platform.OS, "platform_distribution", dc.platform.Distribution)

	// Find platform requirements for current OS
	var platformDeps []string
	var matchedOS string
	for _, req := range osConfig.PlatformRequirements {
		if req.OS == dc.platform.Distribution || req.OS == dc.platform.OS {
			platformDeps = req.PlatformDependencies
			matchedOS = req.OS
			log.Info("Found matching platform requirements", "matched_os", matchedOS, "dependencies", platformDeps, "total_requirements", len(osConfig.PlatformRequirements))
			break
		}
	}

	if len(platformDeps) == 0 {
		log.Info("No platform-specific dependencies required", "checked_requirements", len(osConfig.PlatformRequirements))
		return nil
	}

	log.Info("Platform dependencies identified", "count", len(platformDeps), "dependencies", platformDeps)

	// Check which dependencies are missing
	missingDeps := []string{}
	for _, dep := range platformDeps {
		if err := exec.CommandContext(ctx, "which", dep).Run(); err != nil {
			log.Info("Missing platform dependency", "dependency", dep)
			missingDeps = append(missingDeps, dep)
		} else {
			log.Info("Platform dependency available", "dependency", dep)
		}
	}

	// Install missing dependencies if any
	if len(missingDeps) > 0 {
		if dryRun {
			log.Info("DRY RUN: Would install missing dependencies", "dependencies", missingDeps, "packageManager", dc.packageManager.GetName())
			return nil
		}

		log.Info("Installing missing platform dependencies", "dependencies", missingDeps, "packageManager", dc.packageManager.GetName())
		if err := dc.packageManager.InstallPackages(ctx, missingDeps, dryRun); err != nil {
			return fmt.Errorf("failed to install platform dependencies %v: %w", missingDeps, err)
		}
		log.Info("Successfully installed platform dependencies", "dependencies", missingDeps)

		// Verify installation
		for _, dep := range missingDeps {
			if err := exec.CommandContext(ctx, "which", dep).Run(); err != nil {
				return fmt.Errorf("dependency %s still not available after installation", dep)
			}
		}
		log.Info("All platform dependencies verified")
	}

	return nil
}
