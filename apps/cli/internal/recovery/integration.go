package recovery

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// AutoRecoveryHelper provides automatic recovery suggestions when commands fail
type AutoRecoveryHelper struct {
	manager *RecoveryManager
}

// NewAutoRecoveryHelper creates a new auto-recovery helper
func NewAutoRecoveryHelper(baseDir string) *AutoRecoveryHelper {
	return &AutoRecoveryHelper{
		manager: NewRecoveryManager(baseDir),
	}
}

// SuggestRecovery automatically suggests recovery options when a command fails
func (arh *AutoRecoveryHelper) SuggestRecovery(operation, command string, args []string, err error) {
	// Only suggest recovery for actual errors, not help or usage messages
	if err == nil || isHelpError(err) {
		return
	}

	ctx := RecoveryContext{
		Operation:  operation,
		Error:      err.Error(),
		Timestamp:  time.Now(),
		Command:    command,
		Args:       args,
		WorkingDir: getCurrentDir(),
	}

	// Analyze error and get recovery options
	options, analyzeErr := arh.manager.AnalyzeError(ctx)
	if analyzeErr != nil || len(options) == 0 {
		return // Silently skip if we can't analyze or no options available
	}

	// Display recovery suggestions
	arh.displayAutoRecoverySuggestions(ctx, options)
}

// displayAutoRecoverySuggestions displays recovery suggestions in a compact format
func (arh *AutoRecoveryHelper) displayAutoRecoverySuggestions(ctx RecoveryContext, options []RecoveryOption) {
	fmt.Fprintf(os.Stderr, "\n💡 Recovery Suggestions:\n")

	// Show top 2-3 most relevant options
	maxOptions := 3
	if len(options) < maxOptions {
		maxOptions = len(options)
	}

	for i := 0; i < maxOptions; i++ {
		option := options[i]
		icon := getCompactPriorityIcon(option.Priority)

		fmt.Fprintf(os.Stderr, "   %s %s\n", icon, option.Title)

		// For critical/recommended options, show the command to execute them
		if option.Priority == PriorityCritical || option.Priority == PriorityRecommended {
			if option.Automated {
				fmt.Fprintf(os.Stderr, "      → devex recover --execute %s\n", option.ID)
			} else {
				fmt.Fprintf(os.Stderr, "      → %s (manual)\n", option.Description)
			}
		}
	}

	if len(options) > maxOptions {
		fmt.Fprintf(os.Stderr, "   ... and %d more options\n", len(options)-maxOptions)
	}

	fmt.Fprintf(os.Stderr, "\n   Use 'devex recover --help' for more recovery options\n")
}

// getCompactPriorityIcon returns a compact icon for priority levels
func getCompactPriorityIcon(priority RecoveryPriority) string {
	switch priority {
	case PriorityCritical:
		return "🚨"
	case PriorityRecommended:
		return "✅"
	case PriorityOptional:
		return "💡"
	default:
		return "🔧"
	}
}

// isHelpError checks if an error is just a help/usage message
func isHelpError(err error) bool {
	errMsg := strings.ToLower(err.Error())

	// Common patterns for help/usage errors
	helpPatterns := []string{
		"usage:",
		"help for",
		"unknown command",
		"flag provided but not defined",
		"requires at least",
		"accepts at most",
	}

	for _, pattern := range helpPatterns {
		if strings.Contains(errMsg, pattern) {
			return true
		}
	}

	return false
}

// getCurrentDir gets the current working directory safely
func getCurrentDir() string {
	if dir, err := os.Getwd(); err == nil {
		return dir
	}
	return "unknown"
}

// RecoveryWrapper wraps command execution with automatic recovery suggestions
func RecoveryWrapper(baseDir, operation, command string, args []string, fn func() error) error {
	err := fn()

	if err != nil {
		helper := NewAutoRecoveryHelper(baseDir)
		helper.SuggestRecovery(operation, command, args, err)
	}

	return err
}

// QuickRecovery provides a simplified interface for common recovery scenarios
type QuickRecovery struct {
	manager *RecoveryManager
}

// NewQuickRecovery creates a quick recovery helper
func NewQuickRecovery(baseDir string) *QuickRecovery {
	return &QuickRecovery{
		manager: NewRecoveryManager(baseDir),
	}
}

// RecoverFromInstallError attempts quick recovery from installation errors
func (qr *QuickRecovery) RecoverFromInstallError(appName string, err error) error {
	ctx := RecoveryContext{
		Operation: "install",
		Error:     err.Error(),
		Timestamp: time.Now(),
		Command:   fmt.Sprintf("devex install %s", appName),
		Args:      []string{"install", appName},
	}

	options, analyzeErr := qr.manager.AnalyzeError(ctx)
	if analyzeErr != nil {
		return analyzeErr
	}

	// Try automated recovery options first
	for _, option := range options {
		if option.Automated && (option.Priority == PriorityCritical || option.Priority == PriorityRecommended) {
			fmt.Printf("🔄 Attempting automatic recovery: %s\n", option.Title)

			result, execErr := qr.manager.ExecuteRecovery(option, ctx)
			if execErr == nil && result.Success {
				fmt.Printf("✅ Recovery successful! You may retry the installation now.\n")
				return nil
			}
		}
	}

	return fmt.Errorf("automatic recovery failed, try 'devex recover --interactive'")
}

// RecoverFromConfigError attempts quick recovery from configuration errors
func (qr *QuickRecovery) RecoverFromConfigError(err error) error {
	ctx := RecoveryContext{
		Operation: "config",
		Error:     err.Error(),
		Timestamp: time.Now(),
		Command:   "devex config",
	}

	options, analyzeErr := qr.manager.AnalyzeError(ctx)
	if analyzeErr != nil {
		return analyzeErr
	}

	// For config errors, prioritize backup restoration
	for _, option := range options {
		if option.Type == RecoveryTypeBackupRestore && option.Automated {
			fmt.Printf("🔄 Attempting config recovery: %s\n", option.Title)

			result, execErr := qr.manager.ExecuteRecovery(option, ctx)
			if execErr == nil && result.Success {
				fmt.Printf("✅ Configuration recovered successfully!\n")
				return nil
			}
		}
	}

	return fmt.Errorf("config recovery failed, try 'devex recover --analyze-last'")
}

// Emergency recovery functions for critical situations

// EmergencyReset performs an emergency reset to get DevEx working again
func EmergencyReset(baseDir string) error {
	fmt.Println("🚨 Performing emergency DevEx reset...")

	manager := NewRecoveryManager(baseDir)

	// Create emergency backup first
	fmt.Println("📦 Creating emergency backup...")

	ctx := RecoveryContext{
		Operation: "emergency-reset",
		Error:     "emergency reset requested",
		Timestamp: time.Now(),
	}

	resetOption := RecoveryOption{
		ID:          "emergency-reset",
		Type:        RecoveryTypeConfigReset,
		Title:       "Emergency Reset",
		Description: "Reset all configuration to defaults",
		Priority:    PriorityCritical,
		Automated:   true,
		Steps: []RecoveryStep{
			{
				ID:          "config-reset",
				Title:       "Reset Configuration",
				Description: "Resetting all configuration files",
			},
		},
	}

	result, err := manager.ExecuteRecovery(resetOption, ctx)
	if err != nil {
		return fmt.Errorf("emergency reset failed: %w", err)
	}

	if result.Success {
		fmt.Println("✅ Emergency reset completed successfully!")
		fmt.Println("💡 You can now run 'devex init' to set up your configuration again")
		return nil
	}

	return fmt.Errorf("emergency reset failed: %s", result.Error)
}
