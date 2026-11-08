// Package agentapi provides typed read-only services for CLI and agent adapters.
package agentapi

import (
	"context"

	"github.com/jameswlane/devex/apps/cli/internal/config"
)

type Severity = config.Severity
type Category = config.Category
type Diagnostic = config.Diagnostic
type ValidationResult = config.ValidationResult

const (
	SeverityError                = config.SeverityError
	SeverityWarning              = config.SeverityWarning
	CategoryFileMissing          = config.CategoryFileMissing
	CategoryFileInvalidYAML      = config.CategoryFileInvalidYAML
	CategorySchema               = config.CategorySchema
	CategoryInstallMethodUnknown = config.CategoryInstallMethodUnknown
	CategoryDependencyCycle      = config.CategoryDependencyCycle
	CategoryDependencyNative     = config.CategoryDependencyNative
	CategoryInstallMethodPlanned = config.CategoryInstallMethodPlanned
)

// ValidateConfig delegates to the validator shared with configuration loading.
func ValidateConfig(ctx context.Context, settings config.CrossPlatformSettings, strict bool) (ValidationResult, error) {
	return config.ValidateConfig(ctx, settings, strict)
}
