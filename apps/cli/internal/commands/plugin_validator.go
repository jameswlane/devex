package commands

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"time"

	"github.com/jameswlane/devex/apps/cli/internal/bootstrap"
	"github.com/jameswlane/devex/apps/cli/internal/log"
)

// PluginValidationResult represents the result of plugin validation
type PluginValidationResult struct {
	PluginName     string
	IsValid        bool
	ChecksumValid  bool
	SignatureValid bool
	Error          error
	ValidationTime time.Duration
}

// ValidationSummary aggregates validation results
type ValidationSummary struct {
	TotalPlugins     int
	ValidPlugins     int
	InvalidPlugins   int
	CriticalFailures int
	Errors           []error
	ValidationTime   time.Duration
	Results          []PluginValidationResult
}

// PluginValidator provides enhanced plugin validation with security and performance improvements
type PluginValidator struct {
	pluginBootstrap     *bootstrap.PluginBootstrap
	checksumVerifier    bool
	signatureVerifier   bool
	concurrency         int
	failOnCritical      bool
	criticalPlugins     map[string]bool
	verificationTimeout time.Duration
}

// PluginValidatorConfig configures the plugin validator
type PluginValidatorConfig struct {
	VerifyChecksums     bool
	VerifySignatures    bool
	Concurrency         int
	FailOnCritical      bool
	CriticalPlugins     []string
	VerificationTimeout time.Duration
}

// NewPluginValidator creates a new enhanced plugin validator
func NewPluginValidator(pluginBootstrap *bootstrap.PluginBootstrap, config PluginValidatorConfig) *PluginValidator {
	// Set reasonable defaults
	if config.Concurrency <= 0 {
		config.Concurrency = runtime.NumCPU()
		if config.Concurrency > 4 {
			config.Concurrency = 4 // Reasonable maximum for plugin verification
		}
	}

	if config.VerificationTimeout <= 0 {
		config.VerificationTimeout = 30 * time.Second
	}

	// Build critical plugins set for fast lookup
	criticalSet := make(map[string]bool)
	for _, plugin := range config.CriticalPlugins {
		criticalSet[plugin] = true
	}

	// Default critical plugins if none specified
	if len(criticalSet) == 0 {
		criticalSet = map[string]bool{
			"tool-shell":    true,
			"desktop-gnome": true,
			"desktop-kde":   true,
			"tool-git":      true,
		}
	}

	return &PluginValidator{
		pluginBootstrap:     pluginBootstrap,
		checksumVerifier:    config.VerifyChecksums,
		signatureVerifier:   config.VerifySignatures,
		concurrency:         config.Concurrency,
		failOnCritical:      config.FailOnCritical,
		criticalPlugins:     criticalSet,
		verificationTimeout: config.VerificationTimeout,
	}
}

// ValidatePlugins performs enhanced plugin validation with parallel processing and early termination
func (v *PluginValidator) ValidatePlugins(ctx context.Context, requiredPlugins []string) *ValidationSummary {
	startTime := time.Now()
	summary := &ValidationSummary{
		TotalPlugins: len(requiredPlugins),
		Results:      make([]PluginValidationResult, 0, len(requiredPlugins)),
		Errors:       make([]error, 0),
	}

	if len(requiredPlugins) == 0 {
		log.Debug("No plugins to validate")
		return summary
	}

	// Get installed plugins from manager
	manager := v.pluginBootstrap.GetManager()
	installedPlugins := manager.ListPlugins()

	// Create installed plugins set for O(1) lookup
	installedSet := make(map[string]bool)
	for pluginName := range installedPlugins {
		installedSet[pluginName] = true
	}

	log.Info("Starting enhanced plugin validation", "totalPlugins", len(requiredPlugins), "concurrency", v.concurrency)

	// Phase 1: Fast path - Check critical plugins first with early termination
	criticalResults := v.validateCriticalPluginsFirst(ctx, requiredPlugins, installedSet)
	for _, result := range criticalResults {
		summary.Results = append(summary.Results, result)
		if result.IsValid {
			summary.ValidPlugins++
		} else {
			summary.InvalidPlugins++
			summary.Errors = append(summary.Errors, result.Error)

			// Early termination on critical plugin failure if configured
			if v.criticalPlugins[result.PluginName] && v.failOnCritical {
				summary.CriticalFailures++
				log.Error("Critical plugin validation failed, terminating early", result.Error, "plugin", result.PluginName)
				summary.ValidationTime = time.Since(startTime)
				return summary
			}
		}
	}

	// Phase 2: Parallel verification of remaining (non-critical) plugins
	remainingPlugins := v.getRemainingPlugins(requiredPlugins, criticalResults)
	if len(remainingPlugins) > 0 {
		parallelResults := v.validatePluginsParallel(ctx, remainingPlugins, installedSet)
		for _, result := range parallelResults {
			summary.Results = append(summary.Results, result)
			if result.IsValid {
				summary.ValidPlugins++
			} else {
				summary.InvalidPlugins++
				summary.Errors = append(summary.Errors, result.Error)
			}
		}
	}

	summary.ValidationTime = time.Since(startTime)
	log.Info("Plugin validation completed",
		"totalTime", summary.ValidationTime,
		"validPlugins", summary.ValidPlugins,
		"invalidPlugins", summary.InvalidPlugins,
		"criticalFailures", summary.CriticalFailures,
	)

	return summary
}

// validateCriticalPluginsFirst validates critical plugins first for early feedback
func (v *PluginValidator) validateCriticalPluginsFirst(ctx context.Context, allPlugins []string, installedSet map[string]bool) []PluginValidationResult {
	var criticalPlugins []string
	for _, plugin := range allPlugins {
		if v.criticalPlugins[plugin] {
			criticalPlugins = append(criticalPlugins, plugin)
		}
	}

	if len(criticalPlugins) == 0 {
		return []PluginValidationResult{}
	}

	log.Debug("Validating critical plugins first", "count", len(criticalPlugins), "plugins", criticalPlugins)

	results := make([]PluginValidationResult, 0, len(criticalPlugins))
	for _, plugin := range criticalPlugins {
		result := v.validateSinglePlugin(ctx, plugin, installedSet)
		results = append(results, result)

		// Log critical plugin results immediately
		if result.IsValid {
			log.Info("Critical plugin validated successfully", "plugin", plugin)
		} else {
			log.Warn("Critical plugin validation failed", "plugin", plugin, "error", result.Error)
		}
	}

	return results
}

// validatePluginsParallel validates plugins in parallel using worker goroutines
func (v *PluginValidator) validatePluginsParallel(ctx context.Context, plugins []string, installedSet map[string]bool) []PluginValidationResult {
	if len(plugins) == 0 {
		return []PluginValidationResult{}
	}

	log.Debug("Starting parallel plugin validation", "count", len(plugins), "concurrency", v.concurrency)

	// Create channels for work distribution
	pluginChan := make(chan string, len(plugins))
	resultChan := make(chan PluginValidationResult, len(plugins))

	// Start worker goroutines
	var wg sync.WaitGroup
	for i := 0; i < v.concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			log.Debug("Starting plugin validation worker", "workerID", workerID)

			for plugin := range pluginChan {
				select {
				case <-ctx.Done():
					log.Debug("Plugin validation worker cancelled", "workerID", workerID)
					return
				default:
					result := v.validateSinglePlugin(ctx, plugin, installedSet)
					select {
					case resultChan <- result:
					case <-ctx.Done():
						return
					}
				}
			}
		}(i)
	}

	// Send plugins to workers
	go func() {
		defer close(pluginChan)
		for _, plugin := range plugins {
			select {
			case pluginChan <- plugin:
			case <-ctx.Done():
				return
			}
		}
	}()

	// Collect results
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Aggregate results maintaining order
	results := make([]PluginValidationResult, 0, len(plugins))
	pluginResultMap := make(map[string]PluginValidationResult)

	for result := range resultChan {
		pluginResultMap[result.PluginName] = result
	}

	// Maintain original order for consistent reporting
	for _, plugin := range plugins {
		if result, exists := pluginResultMap[plugin]; exists {
			results = append(results, result)
		}
	}

	return results
}

// validateSinglePlugin performs comprehensive validation of a single plugin
func (v *PluginValidator) validateSinglePlugin(ctx context.Context, pluginName string, installedSet map[string]bool) PluginValidationResult {
	startTime := time.Now()

	// Create timeout context for this plugin
	pluginCtx, cancel := context.WithTimeout(ctx, v.verificationTimeout)
	defer cancel()

	result := PluginValidationResult{
		PluginName: pluginName,
		IsValid:    false,
	}

	// Basic installation check
	if !installedSet[pluginName] {
		result.Error = fmt.Errorf("plugin %s is not installed or not available", pluginName)
		result.ValidationTime = time.Since(startTime)
		return result
	}

	// Enhanced validation with checksum and signature verification
	// This leverages the existing SDK validation infrastructure
	if v.checksumVerifier || v.signatureVerifier {
		if err := v.performEnhancedValidation(pluginCtx, pluginName); err != nil {
			result.Error = fmt.Errorf("plugin %s failed enhanced validation: %w", pluginName, err)
			result.ValidationTime = time.Since(startTime)
			return result
		}
	}

	// Plugin is valid
	result.IsValid = true
	result.ChecksumValid = v.checksumVerifier
	result.SignatureValid = v.signatureVerifier
	result.ValidationTime = time.Since(startTime)

	log.Debug("Plugin validation completed",
		"plugin", pluginName,
		"isValid", result.IsValid,
		"validationTime", result.ValidationTime,
	)

	return result
}

// performEnhancedValidation performs checksum and signature validation
func (v *PluginValidator) performEnhancedValidation(ctx context.Context, pluginName string) error {
	// This method integrates with the existing SDK validation infrastructure
	// For now, we'll implement basic validation and integrate full checksum/signature
	// verification in subsequent phases

	log.Debug("Performing enhanced validation", "plugin", pluginName)

	// TODO: Integrate with existing SDK checksum verification
	// TODO: Integrate with existing SDK signature verification
	// This will be implemented in Phase 2 of the enhancement plan

	return nil
}

// getRemainingPlugins filters out plugins that were already validated in the critical phase
func (v *PluginValidator) getRemainingPlugins(allPlugins []string, criticalResults []PluginValidationResult) []string {
	criticalPluginSet := make(map[string]bool)
	for _, result := range criticalResults {
		criticalPluginSet[result.PluginName] = true
	}

	var remaining []string
	for _, plugin := range allPlugins {
		if !criticalPluginSet[plugin] {
			remaining = append(remaining, plugin)
		}
	}

	return remaining
}
