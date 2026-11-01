// Package agentapi holds typed, side-effect-free read and planning services
// shared by the Cobra commands and future agent adapters (for example an MCP
// server), so neither has to parse human-readable CLI output.
//
// Services here never write configuration or state, run hooks, or bootstrap
// or download plugins.
package agentapi

import (
	"context"
	"os"
	"path/filepath"
	"strings"

	"github.com/jameswlane/devex/apps/cli/internal/config"
)

// Severity of a diagnostic.
type Severity string

const (
	SeverityError   Severity = "error"
	SeverityWarning Severity = "warning"
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
	}
	r.Valid = r.Errors == 0
}

// ValidateConfig checks the canonical configuration directory (schema and
// per-file content) and the effective application set (install methods and
// dependency cycles). It only reads.
func ValidateConfig(ctx context.Context, settings config.CrossPlatformSettings, strict bool) (ValidationResult, error) {
	result := ValidationResult{Valid: true, Diagnostics: []Diagnostic{}}
	if err := ctx.Err(); err != nil {
		return result, err
	}

	if hasCanonicalConfig(settings.GetConfigDir()) {
		validator := config.NewConfigValidator(settings.HomeDir, strict)
		_ = validator.ValidateDirectoryStructure(settings.GetConfigDir())
		for _, e := range validator.GetErrors() {
			result.Add(fromValidationError(SeverityError, CategorySchema, e))
		}
		for _, w := range validator.GetWarnings() {
			result.Add(fromValidationError(SeverityWarning, CategorySchema, w))
		}
	}

	if err := ctx.Err(); err != nil {
		return result, err
	}
	for _, e := range config.ValidateApplicationGraph(settings.GetAllApps()) {
		category := CategoryInstallMethodUnknown
		if strings.Contains(e.Message, "dependency cycle") {
			category = CategoryDependencyCycle
		}
		result.Add(fromValidationError(SeverityError, category, e))
	}
	return result, nil
}

func fromValidationError(severity Severity, category Category, e config.ValidationError) Diagnostic {
	message := e.Message
	if e.Err != nil {
		message += ": " + e.Err.Error()
	}
	return Diagnostic{Severity: severity, Category: category, File: e.File, Field: e.Field, Message: message}
}

func hasCanonicalConfig(dir string) bool {
	for _, directory := range config.ConfigDirectories {
		if info, err := os.Stat(filepath.Join(dir, directory)); err == nil && info.IsDir() {
			return true
		}
	}
	return false
}
