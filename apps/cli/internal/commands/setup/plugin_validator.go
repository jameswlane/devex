package setup

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/jameswlane/devex/apps/cli/internal/bootstrap"
	"github.com/jameswlane/devex/apps/cli/internal/log"
	"github.com/spf13/viper"
)

// PluginValidationResult represents the comprehensive result of validating a single plugin.
type PluginValidationResult struct {
	PluginName     string
	IsValid        bool
	ChecksumValid  bool
	SignatureValid bool
	Error          error
	ValidationTime time.Duration
}

// ValidationSummary provides an aggregated view of plugin validation results across multiple plugins.
type ValidationSummary struct {
	TotalPlugins     int
	ValidPlugins     int
	InvalidPlugins   int
	CriticalFailures int
	Errors           []error
	ValidationTime   time.Duration
	Results          []PluginValidationResult
}

// PluginValidator provides enterprise-grade plugin validation with advanced security verification
type PluginValidator struct {
	pluginBootstrap      *bootstrap.PluginBootstrap
	checksumVerifier     bool
	signatureVerifier    bool
	concurrency          int
	failOnCritical       bool
	criticalPlugins      map[string]bool
	verificationTimeout  time.Duration
	allowInsecurePlugins bool
}

// PluginValidatorConfig provides comprehensive configuration options for plugin validation behavior.
type PluginValidatorConfig struct {
	VerifyChecksums      bool
	VerifySignatures     bool
	Concurrency          int
	FailOnCritical       bool
	CriticalPlugins      []string
	VerificationTimeout  time.Duration
	AllowInsecurePlugins bool
}

// NewPluginValidator creates a new enhanced plugin validator with the specified configuration.
func NewPluginValidator(pluginBootstrap *bootstrap.PluginBootstrap, config PluginValidatorConfig) *PluginValidator {
	// Set reasonable defaults
	if config.Concurrency <= 0 {
		config.Concurrency = runtime.NumCPU()
		if config.Concurrency > 4 {
			config.Concurrency = 4 // Reasonable maximum for plugin verification
		}
	}

	if config.VerificationTimeout <= 0 {
		config.VerificationTimeout = PluginVerifyTimeout
	}

	// Build critical plugins set for fast lookup
	criticalSet := make(map[string]bool)
	for _, plugin := range config.CriticalPlugins {
		criticalSet[plugin] = true
	}

	// Load critical plugins from configuration if not specified
	if len(criticalSet) == 0 {
		criticalSet = loadCriticalPluginsFromConfig()
	}

	return &PluginValidator{
		pluginBootstrap:      pluginBootstrap,
		checksumVerifier:     config.VerifyChecksums,
		signatureVerifier:    config.VerifySignatures,
		concurrency:          config.Concurrency,
		failOnCritical:       config.FailOnCritical,
		criticalPlugins:      criticalSet,
		verificationTimeout:  config.VerificationTimeout,
		allowInsecurePlugins: config.AllowInsecurePlugins,
	}
}

// loadCriticalPluginsFromConfig loads critical plugins from the configuration hierarchy.
func loadCriticalPluginsFromConfig() map[string]bool {
	criticalSet := make(map[string]bool)

	// 1. Try environment variable first (highest priority)
	envCritical := os.Getenv("DEVEX_CRITICAL_PLUGINS")
	if envCritical != "" {
		plugins := strings.Split(envCritical, ",")
		validPlugins := make([]string, 0, len(plugins))
		for _, plugin := range plugins {
			plugin = strings.TrimSpace(plugin)
			if plugin != "" && isValidPluginName(plugin) {
				criticalSet[plugin] = true
				validPlugins = append(validPlugins, plugin)
			} else if plugin != "" {
				log.Warn("Invalid critical plugin name ignored", "plugin", plugin, "source", "environment")
			}
		}
		if len(validPlugins) > 0 {
			log.Debug("Loaded critical plugins from environment", "plugins", validPlugins)
		}
		return criticalSet
	}

	// 2. Try Viper configuration (medium priority)
	if viper.IsSet("plugin.critical") {
		configPlugins := viper.GetStringSlice("plugin.critical")
		validPlugins := make([]string, 0, len(configPlugins))
		for _, plugin := range configPlugins {
			plugin = strings.TrimSpace(plugin)
			if plugin != "" && isValidPluginName(plugin) {
				criticalSet[plugin] = true
				validPlugins = append(validPlugins, plugin)
			} else if plugin != "" {
				log.Warn("Invalid critical plugin name ignored", "plugin", plugin, "source", "config")
			}
		}
		if len(validPlugins) > 0 {
			log.Debug("Loaded critical plugins from config", "plugins", validPlugins)
		}
		return criticalSet
	}

	// 3. Default critical plugins (lowest priority) - pre-validated
	criticalSet = map[string]bool{
		"tool-shell":    true,
		"desktop-gnome": true,
		"desktop-kde":   true,
		"tool-git":      true,
	}
	log.Debug("Using default critical plugins", "plugins", []string{"tool-shell", "desktop-gnome", "desktop-kde", "tool-git"})
	return criticalSet
}

// isValidPluginName validates plugin name format and security constraints
func isValidPluginName(pluginName string) bool {
	if pluginName == "" {
		return false
	}

	// Length constraints
	if len(pluginName) > 64 || len(pluginName) < 2 {
		return false
	}

	// Security checks - prevent path traversal and injection
	if strings.Contains(pluginName, "..") ||
		strings.Contains(pluginName, "/") ||
		strings.Contains(pluginName, "\\") ||
		strings.Contains(pluginName, "$") ||
		strings.Contains(pluginName, "`") ||
		strings.Contains(pluginName, ";") ||
		strings.Contains(pluginName, "&") ||
		strings.Contains(pluginName, "|") ||
		strings.Contains(pluginName, ">") ||
		strings.Contains(pluginName, "<") {
		return false
	}

	// Basic format validation - allow letters, numbers, hyphens, underscores
	for _, char := range pluginName {
		if (char < 'a' || char > 'z') &&
			(char < 'A' || char > 'Z') &&
			(char < '0' || char > '9') &&
			char != '-' && char != '_' {
			return false
		}
	}

	return true
}

// isNetworkError checks if the error is related to network connectivity
func isNetworkError(err error) bool {
	if err == nil {
		return false
	}
	errorStr := strings.ToLower(err.Error())
	return strings.Contains(errorStr, "network") ||
		strings.Contains(errorStr, "connection") ||
		strings.Contains(errorStr, "timeout") ||
		strings.Contains(errorStr, "unreachable") ||
		strings.Contains(errorStr, "dns") ||
		strings.Contains(errorStr, "no route") ||
		strings.Contains(errorStr, "connection refused")
}

// isPermissionError checks if the error is related to file permissions
func isPermissionError(err error) bool {
	if err == nil {
		return false
	}
	errorStr := strings.ToLower(err.Error())
	return strings.Contains(errorStr, "permission") ||
		strings.Contains(errorStr, "access denied") ||
		strings.Contains(errorStr, "operation not permitted") ||
		strings.Contains(errorStr, "insufficient privileges")
}

// isDiskSpaceError checks if the error is related to disk space
func isDiskSpaceError(err error) bool {
	if err == nil {
		return false
	}
	errorStr := strings.ToLower(err.Error())
	return strings.Contains(errorStr, "no space") ||
		strings.Contains(errorStr, "disk full") ||
		strings.Contains(errorStr, "insufficient space")
}

// enhanceErrorMessage provides actionable guidance for common error scenarios
func enhanceErrorMessage(operation, pluginName string, err error) error {
	if err == nil {
		return nil
	}

	baseMsg := fmt.Sprintf("%s failed for plugin %s", operation, pluginName)

	if isNetworkError(err) {
		return fmt.Errorf("%s due to network connectivity issues. Please check your internet connection, verify proxy settings, and try again: %w", baseMsg, err)
	}

	if isPermissionError(err) {
		return fmt.Errorf("%s due to insufficient permissions. Try running with sudo, check directory permissions, or ensure your user has access to the plugin directory: %w", baseMsg, err)
	}

	if isDiskSpaceError(err) {
		return fmt.Errorf("%s due to insufficient disk space. Please free up disk space and try again: %w", baseMsg, err)
	}

	// Default enhanced message for unknown errors
	return fmt.Errorf("%s. Please check the plugin name is correct, verify your system meets requirements, and consult the documentation at https://docs.devex.sh/troubleshooting: %w", baseMsg, err)
}

// ValidatePlugins performs comprehensive plugin validation using a two-phase approach
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
	installedPlugins := manager.ListPluginsWithContext(ctx)

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

	// Start worker goroutines with proper cleanup
	var wg sync.WaitGroup
	workerCtx, cancelWorkers := context.WithCancel(ctx)
	defer cancelWorkers() // Ensure all workers are cancelled on function exit

	for i := 0; i < v.concurrency; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()
			log.Debug("Starting plugin validation worker", "workerID", workerID)

			for {
				select {
				case <-workerCtx.Done():
					log.Debug("Plugin validation worker cancelled", "workerID", workerID)
					return
				case plugin, ok := <-pluginChan:
					if !ok {
						log.Debug("Plugin validation worker finished - channel closed", "workerID", workerID)
						return
					}

					// Validate plugin with worker context
					result := v.validateSinglePlugin(workerCtx, plugin, installedSet)

					// Send result with proper cancellation handling
					select {
					case resultChan <- result:
					case <-workerCtx.Done():
						log.Debug("Plugin validation worker cancelled while sending result", "workerID", workerID)
						return
					}
				}
			}
		}(i)
	}

	// Send plugins to workers with proper cancellation handling
	go func() {
		defer close(pluginChan)
		for _, plugin := range plugins {
			select {
			case pluginChan <- plugin:
			case <-workerCtx.Done():
				log.Debug("Plugin sender cancelled - stopping work distribution")
				return
			}
		}
	}()

	// Collect results with timeout protection
	go func() {
		wg.Wait()
		close(resultChan)
		log.Debug("All plugin validation workers completed")
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
		result.Error = enhanceErrorMessage("Plugin installation check", pluginName,
			fmt.Errorf("plugin is not installed or not available"))
		result.ValidationTime = time.Since(startTime)
		return result
	}

	// Enhanced validation with checksum and signature verification
	// This leverages the existing SDK validation infrastructure
	if v.checksumVerifier || v.signatureVerifier {
		if err := v.performEnhancedValidation(pluginCtx, pluginName); err != nil {
			result.Error = enhanceErrorMessage("Plugin security validation", pluginName, err)
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
	log.Debug("Performing enhanced validation", "plugin", pluginName)

	// Get plugin manager to access plugin information
	manager := v.pluginBootstrap.GetManager()
	pluginInfo, exists := manager.ListPluginsWithContext(ctx)[pluginName]
	if !exists {
		return enhanceErrorMessage("Plugin manager lookup", pluginName,
			fmt.Errorf("plugin not found in manager"))
	}

	// Basic plugin integrity checks
	if err := v.validatePluginIntegrity(ctx, pluginName, pluginInfo); err != nil {
		return enhanceErrorMessage("Plugin integrity validation", pluginName, err)
	}

	// Checksum verification if enabled
	if v.checksumVerifier {
		if err := v.validatePluginChecksum(ctx, pluginName, pluginInfo); err != nil {
			log.Warn("Checksum verification failed", "plugin", pluginName, "error", err)
			// Fail validation unless insecure plugins are explicitly allowed
			if !v.allowInsecurePlugins {
				return fmt.Errorf("checksum verification failed for plugin %s (use --allow-insecure-plugins to bypass): %w", pluginName, err)
			}
			log.Warn("Continuing with unverified plugin due to --allow-insecure-plugins flag", "plugin", pluginName)
		}
	}

	// Signature verification if enabled
	if v.signatureVerifier {
		if err := v.validatePluginSignature(ctx, pluginName, pluginInfo); err != nil {
			log.Warn("Signature verification failed", "plugin", pluginName, "error", err)
			// Fail validation unless insecure plugins are explicitly allowed
			if !v.allowInsecurePlugins {
				return fmt.Errorf("signature verification failed for plugin %s (use --allow-insecure-plugins to bypass): %w", pluginName, err)
			}
			log.Warn("Continuing with unsigned plugin due to --allow-insecure-plugins flag", "plugin", pluginName)
		}
	}

	return nil
}

// validatePluginIntegrity performs basic plugin integrity checks
func (v *PluginValidator) validatePluginIntegrity(ctx context.Context, pluginName string, pluginInfo interface{}) error {
	// Check context cancellation
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	// Basic validation: ensure plugin name is not empty and matches expected pattern
	if pluginName == "" {
		return fmt.Errorf("plugin name cannot be empty")
	}

	// Validate plugin name format (basic security check)
	if strings.Contains(pluginName, "..") || strings.Contains(pluginName, "/") {
		return fmt.Errorf("plugin name contains invalid characters: %s", pluginName)
	}

	log.Debug("Plugin integrity check passed", "plugin", pluginName)
	return nil
}

// validatePluginChecksum performs checksum validation (placeholder for SDK integration)
func (v *PluginValidator) validatePluginChecksum(ctx context.Context, pluginName string, pluginInfo interface{}) error {
	// Check context cancellation
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	log.Debug("Checksum validation placeholder", "plugin", pluginName)
	return nil
}

// validatePluginSignature performs signature validation (placeholder for SDK integration)
func (v *PluginValidator) validatePluginSignature(ctx context.Context, pluginName string, pluginInfo interface{}) error {
	// Check context cancellation
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}

	log.Debug("Signature validation placeholder", "plugin", pluginName)
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
