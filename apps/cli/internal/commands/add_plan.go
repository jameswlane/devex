package commands

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/types"
)

// planAddition resolves catalog references without changing configuration or history.
// Unknown dependencies remain native packages, matching the shared validator.
func planAddition(ctx context.Context, settings config.CrossPlatformSettings, name string) ([]types.CrossPlatformApp, error) {
	configured, err := config.LoadApplicationConfigs(settings.GetConfigDir())
	if err != nil {
		return nil, err
	}
	catalog := append(settings.GetAllApps(), configured...)
	index := map[string]types.CrossPlatformApp{}
	present := map[string]bool{}
	for _, app := range catalog {
		index[strings.ToLower(app.Name)] = app
	}
	for _, app := range catalog {
		for _, platform := range []types.OSConfig{app.AllPlatforms, app.Linux, app.MacOS, app.Windows} {
			if len(strings.Fields(platform.InstallCommand)) == 1 {
				key := strings.ToLower(platform.InstallCommand)
				if _, exists := index[key]; !exists {
					index[key] = app
				}
			}
		}
	}
	for _, app := range configured {
		present[strings.ToLower(app.Name)] = true
	}
	target, found := index[strings.ToLower(name)]
	if !found {
		return nil, fmt.Errorf("application %q not found in catalog", name)
	}
	if present[strings.ToLower(target.Name)] {
		return nil, fmt.Errorf("application %q is already in your configuration", name)
	}
	var plan []types.CrossPlatformApp
	visited := map[string]bool{}
	var visit func(types.CrossPlatformApp) error
	visit = func(app types.CrossPlatformApp) error {
		if err := ctx.Err(); err != nil {
			return err
		}
		key := strings.ToLower(app.Name)
		if visited[key] {
			return nil
		}
		visited[key] = true
		var dependencies func(types.OSConfig) error
		dependencies = func(platform types.OSConfig) error {
			for _, dep := range platform.Dependencies {
				if candidate, ok := index[strings.ToLower(dep)]; ok && !strings.EqualFold(candidate.Name, app.Name) {
					if err := visit(candidate); err != nil {
						return err
					}
				}
			}
			for _, alternative := range platform.Alternatives {
				if err := dependencies(alternative); err != nil {
					return err
				}
			}
			return nil
		}
		for _, platform := range []types.OSConfig{app.AllPlatforms, app.Linux, app.MacOS, app.Windows} {
			if err := dependencies(platform); err != nil {
				return err
			}
		}
		if !present[key] {
			plan = append(plan, app)
		}
		return nil
	}
	if err := visit(target); err != nil {
		return nil, err
	}
	if err := checkPlanCollisions(settings, configured, plan, nil); err != nil {
		return nil, err
	}
	if err := validatePlan(ctx, settings, configured, plan, false); err != nil {
		return nil, err
	}
	return plan, nil
}

// checkPlanCollisions preflights destination collisions before backup/undo state
// is created. Applications named in replacing may overwrite their own files.
func checkPlanCollisions(settings config.CrossPlatformSettings, configured, plan []types.CrossPlatformApp, replacing map[string]bool) error {
	for _, app := range plan {
		path, err := applicationConfigPath(settings.GetConfigDir(), "applications", app.Category, app.Name)
		if err != nil {
			return err
		}
		if _, err := os.Stat(path); err == nil {
			if replacing[strings.ToLower(app.Name)] {
				if owned, ownedErr := fileDefinesApplication(path, app.Name); ownedErr == nil && owned {
					continue
				}
			}
			return fmt.Errorf("configuration path %s already exists; rename the existing file or change the application's name or category, then retry", path)
		} else if !os.IsNotExist(err) {
			return err
		}
	}
	// Also check for collisions with legacy files that migration will materialize.
	for _, app := range configured {
		if replacing[strings.ToLower(app.Name)] {
			continue
		}
		oldPath, _ := applicationConfigPath(settings.GetConfigDir(), "applications", app.Category, app.Name)
		for _, added := range plan {
			newPath, _ := applicationConfigPath(settings.GetConfigDir(), "applications", added.Category, added.Name)
			if filepath.Clean(oldPath) == filepath.Clean(newPath) {
				return fmt.Errorf("configuration path %s already exists; rename the existing file before adding %q", newPath, added.Name)
			}
		}
	}
	return nil
}

// validatePlan serializes candidates so required-field checks use the same
// schema path as disk config, then validates their effective merge. Replacements
// take the highest precedence so the old definition cannot shadow the candidate.
func validatePlan(ctx context.Context, settings config.CrossPlatformSettings, configured, plan []types.CrossPlatformApp, replacing bool) error {
	staged, err := os.MkdirTemp("", "devex-add-validation-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(staged)
	for _, app := range append(append([]types.CrossPlatformApp{}, configured...), plan...) {
		path, err := applicationConfigPath(staged, "applications", app.Category, app.Name)
		if err != nil {
			return err
		}
		if err := writeNewApplicationConfig(app, path); err != nil {
			return err
		}
	}
	candidate := config.CrossPlatformSettings{HomeDir: settings.HomeDir, ApplicationOverrides: []types.CrossPlatformApp{}}
	dirs := config.EffectiveConfigDirs(settings)
	if replacing {
		dirs = append(dirs, staged)
	} else {
		dirs = append([]string{dirs[0], staged}, dirs[1:]...)
	}
	// Validate the serialized selection independently, then its effective merge.
	result, err := config.ValidateConfigDirs(ctx, candidate, []string{staged}, false)
	if err == nil && result.Valid {
		result, err = config.ValidateConfigDirs(ctx, candidate, dirs, false)
	}
	if err != nil {
		return err
	}
	if !result.Valid {
		var messages []string
		for _, diagnostic := range result.Diagnostics {
			if diagnostic.Severity == config.SeverityError {
				messages = append(messages, fmt.Sprintf("%s: %s: %s", diagnostic.File, diagnostic.Field, diagnostic.Message))
			}
		}
		return fmt.Errorf("configuration validation failed: %s", strings.Join(messages, "; "))
	}
	return nil
}
