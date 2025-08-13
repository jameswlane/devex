package utils

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"sync"

	"github.com/jameswlane/devex/pkg/log"
	"github.com/jameswlane/devex/pkg/platform"
	"github.com/jameswlane/devex/pkg/types"
)

type Dependency struct {
	Name    string
	Command string
}

// validPackageNameRegex validates package names to prevent injection attacks
var validPackageNameRegex = regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9\-\+\.\_]*$`)

// validatePackageName validates a package name for security
func validatePackageName(packageName string) error {
	if packageName == "" {
		return fmt.Errorf("package name cannot be empty")
	}
	if len(packageName) > 255 {
		return fmt.Errorf("package name too long: %d characters (max 255)", len(packageName))
	}
	if !validPackageNameRegex.MatchString(packageName) {
		return fmt.Errorf("invalid package name: %s (contains invalid characters)", packageName)
	}
	return nil
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
func CheckDependencies(ctx context.Context, dependencies []Dependency) error {
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

	// Validate all package names for security
	for _, dep := range platformDeps {
		if err := validatePackageName(dep); err != nil {
			return fmt.Errorf("invalid dependency package name: %w", err)
		}
	}

	// Check which dependencies are missing - use parallel checking for performance
	missingDeps := dc.checkDependenciesParallel(ctx, platformDeps)

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

// checkDependenciesParallel checks multiple dependencies in parallel for better performance
func (dc *DependencyChecker) checkDependenciesParallel(ctx context.Context, dependencies []string) []string {
	if len(dependencies) == 0 {
		return nil
	}

	type depResult struct {
		dependency string
		missing    bool
	}

	resultsChan := make(chan depResult, len(dependencies))
	var wg sync.WaitGroup

	// Start parallel checking
	for _, dep := range dependencies {
		wg.Add(1)
		go func(dependency string) {
			defer wg.Done()

			// Check if dependency is available
			err := exec.CommandContext(ctx, "which", dependency).Run()
			isMissing := err != nil

			if isMissing {
				log.Info("Missing platform dependency", "dependency", dependency)
			} else {
				log.Info("Platform dependency available", "dependency", dependency)
			}

			select {
			case resultsChan <- depResult{dependency: dependency, missing: isMissing}:
			case <-ctx.Done():
				return
			}
		}(dep)
	}

	// Close channel when all goroutines complete
	go func() {
		wg.Wait()
		close(resultsChan)
	}()

	// Collect results
	var missingDeps []string
	for result := range resultsChan {
		if result.missing {
			missingDeps = append(missingDeps, result.dependency)
		}
	}

	return missingDeps
}
