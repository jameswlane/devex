package config

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/jameswlane/devex/apps/cli/internal/types"

	devexpaths "github.com/jameswlane/devex/packages/plugin-sdk/paths"
)

// Severity of a diagnostic.
type Severity string

const (
	SeverityError   Severity = "error"
	SeverityWarning Severity = "warning"
	// SeverityInfo marks expected, informational findings (for example
	// install methods planned for other platforms). Infos never affect
	// validity and are not counted as warnings.
	SeverityInfo Severity = "info"
)

// Category is a stable, machine-readable diagnostic kind. Values are part of
// the JSON contract: add new ones, don't rename existing ones.
type Category string

const (
	CategoryFileMissing          Category = "file_missing"
	CategoryFileInvalidYAML      Category = "file_invalid_yaml"
	CategorySchema               Category = "schema"
	CategoryInstallMethodUnknown Category = "install_method_unknown"
	CategoryDependencyCycle      Category = "dependency_cycle"
	CategoryApplicationConflict  Category = "application_conflict"
	CategoryDependencyNative     Category = "dependency_native"
	CategoryInstallMethodPlanned Category = "install_method_planned"
)

// Diagnostic is one validation finding.
type Diagnostic struct {
	Severity Severity `json:"severity"`
	Category Category `json:"category"`
	File     string   `json:"file,omitempty"`
	Field    string   `json:"field,omitempty"`
	Message  string   `json:"message"`
}

// ValidationResult is the outcome of validating a configuration.
type ValidationResult struct {
	Valid       bool         `json:"valid"`
	Errors      int          `json:"errors"`
	Warnings    int          `json:"warnings"`
	Infos       int          `json:"infos"`
	Diagnostics []Diagnostic `json:"diagnostics"`
}

// Add records a diagnostic and updates the counts.
func (r *ValidationResult) Add(d Diagnostic) {
	r.Diagnostics = append(r.Diagnostics, d)
	switch d.Severity {
	case SeverityError:
		r.Errors++
	case SeverityWarning:
		r.Warnings++
	case SeverityInfo:
		r.Infos++
	}
	r.Valid = r.Errors == 0
}

// EffectiveConfigDirs returns the runtime precedence order, including environment layers.
func EffectiveConfigDirs(settings CrossPlatformSettings) []string {
	d, t, u, e := settings.GetConfigDirsWithEnvironment()
	dirs := []string{d, e["default"], t, e["team"], u, e["user"]}
	for i := range dirs {
		dirs[i] = devexpaths.ForHome(settings.HomeDir).ReadPath(dirs[i])
	}
	return dirs
}

// ValidateConfig is the shared, read-only semantic validation entry point.
// strict promotes unknown schema keys to errors; malformed definitions always fail.
func ValidateConfig(ctx context.Context, settings CrossPlatformSettings, strict bool) (ValidationResult, error) {
	return ValidateConfigDirs(ctx, settings, EffectiveConfigDirs(settings), strict)
}

// ValidateConfigDirs validates an explicitly staged precedence stream.
func ValidateConfigDirs(ctx context.Context, settings CrossPlatformSettings, dirs []string, strict bool) (ValidationResult, error) {
	result := ValidationResult{Valid: true, Diagnostics: []Diagnostic{}}
	sources := map[string]string{}
	for _, dir := range dirs {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		validator := NewConfigValidator(settings.HomeDir, strict)
		if info, err := os.Stat(dir); err == nil && info.IsDir() {
			_ = validator.validateEffectiveDirectory(dir, sources)
		} else if err != nil && !os.IsNotExist(err) {
			validator.addError(dir, "", "Unable to read configuration directory", err)
		}
		for _, e := range validator.GetErrors() {
			result.Add(fromValidationError(SeverityError, CategorySchema, e))
		}
		for _, e := range validator.GetWarnings() {
			result.Add(fromValidationError(SeverityWarning, CategorySchema, e))
		}
	}
	if err := ctx.Err(); err != nil {
		return result, err
	}
	loaded, loadErr := loadConfigDirs(settings.HomeDir, dirs, FileApplicationLoader{})
	if loadErr != nil {
		if result.Valid {
			result.Add(Diagnostic{Severity: SeverityError, Category: CategorySchema, Message: loadErr.Error()})
		}
		return result, nil
	}
	apps := mergeApplications(loaded.GetAllApps(), settings.GetAllApps())
	for _, e := range ValidateApplicationGraph(apps) {
		category := CategoryInstallMethodUnknown
		if strings.Contains(e.Message, "dependency cycle") {
			category = CategoryDependencyCycle
		}
		if strings.Contains(e.Field, ".conflicts[") {
			category = CategoryApplicationConflict
		}
		if path, ok := sources[strings.ToLower(e.File)]; ok {
			e.File = path
		}
		result.Add(fromValidationError(SeverityError, category, e))
	}
	for _, d := range applicationCapabilityWarnings(apps, sources) {
		result.Add(d)
	}
	return result, nil
}

// RequireValidConfig stops a mutation before it creates backups, state or files.
func RequireValidConfig(ctx context.Context, settings CrossPlatformSettings) error {
	result, err := ValidateConfig(ctx, settings, false)
	if err != nil {
		return err
	}
	if !result.Valid {
		for _, d := range result.Diagnostics {
			if d.Severity == SeverityError {
				return fmt.Errorf("configuration validation failed: %s: %s: %s", d.File, d.Field, d.Message)
			}
		}
	}
	return nil
}

func fromValidationError(severity Severity, category Category, e ValidationError) Diagnostic {
	message := e.Message
	if e.Err != nil {
		message += ": " + e.Err.Error()
	}
	return Diagnostic{Severity: severity, Category: category, File: e.File, Field: e.Field, Message: message}
}

func applicationCapabilityWarnings(apps []types.CrossPlatformApp, sources map[string]string) []Diagnostic {
	known := map[string]bool{}
	for _, app := range apps {
		known[strings.ToLower(app.Name)] = true
		for _, osConfig := range platformConfigs(app) {
			if len(strings.Fields(osConfig.InstallCommand)) == 1 {
				known[strings.ToLower(osConfig.InstallCommand)] = true
			}
		}
	}
	var diagnostics []Diagnostic
	for _, app := range apps {
		file := sources[strings.ToLower(app.Name)]
		if file == "" {
			file = app.Name
		}
		var visit func(types.OSConfig, string)
		visit = func(osConfig types.OSConfig, field string) {
			if ClassifyInstallMethod(osConfig.InstallMethod) == MethodPlanned {
				diagnostics = append(diagnostics, Diagnostic{Severity: SeverityInfo, Category: CategoryInstallMethodPlanned, File: file, Field: field + ".install_method", Message: fmt.Sprintf("install method %q is planned; no plugin capability is available yet", osConfig.InstallMethod)})
			}
			for i, dependency := range osConfig.Dependencies {
				if !known[strings.ToLower(dependency)] {
					diagnostics = append(diagnostics, Diagnostic{Severity: SeverityInfo, Category: CategoryDependencyNative, File: file, Field: fmt.Sprintf("%s.dependencies[%d]", field, i), Message: fmt.Sprintf("%q is not a known app; treated as native package", dependency)})
				}
			}
			for i, alternative := range osConfig.Alternatives {
				visit(alternative, fmt.Sprintf("%s.alternatives[%d]", field, i))
			}
		}
		for _, platform := range sortedKeys(platformConfigs(app)) {
			visit(platformConfigs(app)[platform], platform)
		}
	}
	return diagnostics
}
