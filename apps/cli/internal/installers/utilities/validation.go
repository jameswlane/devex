package utilities

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/jameswlane/devex/apps/cli/internal/log"
	"github.com/jameswlane/devex/apps/cli/internal/utils"
)

// ValidationResult contains the result of a validation check
type ValidationResult struct {
	Check    string
	Success  bool
	Error    error
	Message  string
	Duration time.Duration
}

// ValidationSuite represents a collection of validation checks
type ValidationSuite struct {
	Name   string
	Checks []ValidationCheck
}

// ValidationCheck defines a single validation operation
type ValidationCheck struct {
	Name        string
	Description string
	Validator   func(ctx context.Context) error
	Timeout     time.Duration
	Critical    bool // If true, failure blocks installation
}

// BackgroundValidator manages concurrent validation operations
type BackgroundValidator struct {
	suites  []ValidationSuite
	results map[string][]ValidationResult
	mutex   sync.RWMutex
	timeout time.Duration
}

// NewBackgroundValidator creates a new background validator
func NewBackgroundValidator(timeout time.Duration) *BackgroundValidator {
	return &BackgroundValidator{
		suites:  make([]ValidationSuite, 0),
		results: make(map[string][]ValidationResult),
		timeout: timeout,
	}
}

// AddSuite adds a validation suite to the validator
func (bv *BackgroundValidator) AddSuite(suite ValidationSuite) {
	bv.mutex.Lock()
	defer bv.mutex.Unlock()
	bv.suites = append(bv.suites, suite)
}

// RunValidations executes all validation suites concurrently
func (bv *BackgroundValidator) RunValidations(ctx context.Context) error {
	bv.mutex.Lock()
	suites := make([]ValidationSuite, len(bv.suites))
	copy(suites, bv.suites)
	bv.mutex.Unlock()

	if len(suites) == 0 {
		return nil
	}

	// Create context with timeout
	validationCtx, cancel := context.WithTimeout(ctx, bv.timeout)
	defer cancel()

	var wg sync.WaitGroup
	resultsChan := make(chan suiteResult, len(suites))

	// Run each suite concurrently
	for _, suite := range suites {
		wg.Add(1)
		go func(s ValidationSuite) {
			defer wg.Done()
			results := bv.runSuite(validationCtx, s)
			resultsChan <- suiteResult{suite: s.Name, results: results}
		}(suite)
	}

	// Wait for all validations to complete
	go func() {
		wg.Wait()
		close(resultsChan)
	}()

	// Collect results
	var criticalErrors []error
	bv.mutex.Lock()
	for result := range resultsChan {
		bv.results[result.suite] = result.results

		// Check for critical failures (avoid nested locking by calling helper directly)
		for _, r := range result.results {
			if !r.Success && bv.isCriticalCheckUnsafe(result.suite, r.Check) {
				criticalErrors = append(criticalErrors,
					fmt.Errorf("critical validation failed: %s.%s: %w", result.suite, r.Check, r.Error))
			}
		}
	}
	bv.mutex.Unlock()

	// Return error if any critical validations failed
	if len(criticalErrors) > 0 {
		return NewInstallerError(ErrorTypeSystem, "validation", "", "",
			fmt.Errorf("critical validations failed: %v", criticalErrors))
	}

	return nil
}

// suiteResult holds the results for a validation suite
type suiteResult struct {
	suite   string
	results []ValidationResult
}

// runSuite executes all checks in a validation suite
func (bv *BackgroundValidator) runSuite(ctx context.Context, suite ValidationSuite) []ValidationResult {
	results := make([]ValidationResult, len(suite.Checks))

	var wg sync.WaitGroup
	for i, check := range suite.Checks {
		wg.Add(1)
		go func(idx int, c ValidationCheck) {
			defer wg.Done()
			results[idx] = bv.runCheck(ctx, c)
		}(i, check)
	}

	wg.Wait()
	return results
}

// runCheck executes a single validation check
func (bv *BackgroundValidator) runCheck(ctx context.Context, check ValidationCheck) ValidationResult {
	start := time.Now()
	result := ValidationResult{
		Check: check.Name,
	}

	// Set timeout for individual check
	checkTimeout := check.Timeout
	if checkTimeout == 0 {
		checkTimeout = 10 * time.Second // Default timeout
	}

	checkCtx, cancel := context.WithTimeout(ctx, checkTimeout)
	defer cancel()

	// Run validation in goroutine to handle timeout
	done := make(chan error, 1)
	go func() {
		done <- check.Validator(checkCtx)
	}()

	select {
	case err := <-done:
		if err != nil {
			result.Success = false
			result.Error = err
			result.Message = fmt.Sprintf("Check '%s' failed: %v", check.Name, err)
			log.Debug("Validation check failed", "check", check.Name, "error", err)
		} else {
			result.Success = true
			result.Message = fmt.Sprintf("Check '%s' passed", check.Name)
			log.Debug("Validation check passed", "check", check.Name)
		}
	case <-checkCtx.Done():
		result.Success = false
		result.Error = checkCtx.Err()
		result.Message = fmt.Sprintf("Check '%s' timed out", check.Name)
		log.Debug("Validation check timed out", "check", check.Name)
	}

	result.Duration = time.Since(start)
	return result
}

// isCriticalCheck determines if a check is marked as critical
func (bv *BackgroundValidator) isCriticalCheck(suiteName, checkName string) bool {
	bv.mutex.RLock()
	defer bv.mutex.RUnlock()
	return bv.isCriticalCheckUnsafe(suiteName, checkName)
}

// isCriticalCheckUnsafe determines if a check is marked as critical (assumes lock is already held)
func (bv *BackgroundValidator) isCriticalCheckUnsafe(suiteName, checkName string) bool {
	for _, suite := range bv.suites {
		if suite.Name == suiteName {
			for _, check := range suite.Checks {
				if check.Name == checkName {
					return check.Critical
				}
			}
		}
	}
	return false
}

// GetResults returns validation results for a specific suite
func (bv *BackgroundValidator) GetResults(suiteName string) ([]ValidationResult, bool) {
	bv.mutex.RLock()
	defer bv.mutex.RUnlock()
	results, exists := bv.results[suiteName]
	return results, exists
}

// GetAllResults returns all validation results
func (bv *BackgroundValidator) GetAllResults() map[string][]ValidationResult {
	bv.mutex.RLock()
	defer bv.mutex.RUnlock()

	// Create a copy to avoid race conditions
	results := make(map[string][]ValidationResult)
	for k, v := range bv.results {
		results[k] = make([]ValidationResult, len(v))
		copy(results[k], v)
	}

	return results
}

// HasCriticalFailures checks if there are any critical validation failures
func (bv *BackgroundValidator) HasCriticalFailures() bool {
	bv.mutex.RLock()
	defer bv.mutex.RUnlock()

	for suiteName, results := range bv.results {
		for _, result := range results {
			if !result.Success && bv.isCriticalCheck(suiteName, result.Check) {
				return true
			}
		}
	}
	return false
}

// CreateSystemValidationSuite creates common system validation checks for installers
func CreateSystemValidationSuite(installer string) ValidationSuite {
	checks := []ValidationCheck{
		{
			Name:        "package-manager-available",
			Description: fmt.Sprintf("Check if %s is available in PATH", installer),
			Validator:   createCommandAvailabilityCheck(installer),
			Timeout:     5 * time.Second,
			Critical:    true,
		},
		{
			Name:        "package-manager-functional",
			Description: fmt.Sprintf("Check if %s responds to version command", installer),
			Validator:   createVersionCheck(installer),
			Timeout:     10 * time.Second,
			Critical:    true,
		},
		{
			Name:        "system-permissions",
			Description: "Check if user has necessary permissions",
			Validator:   createPermissionCheck(),
			Timeout:     5 * time.Second,
			Critical:    false,
		},
		{
			Name:        "disk-space",
			Description: "Check available disk space",
			Validator:   createDiskSpaceCheck(),
			Timeout:     5 * time.Second,
			Critical:    false,
		},
	}

	// Add installer-specific checks
	if installer == "dnf" {
		checks = append(checks, ValidationCheck{
			Name:        "rpm-available",
			Description: "Check if rpm is available for package verification",
			Validator:   createCommandAvailabilityCheck("rpm"),
			Timeout:     5 * time.Second,
			Critical:    true,
		})
		checks = append(checks, ValidationCheck{
			Name:        "rpm-functional",
			Description: "Check if rpm responds to version command",
			Validator:   createVersionCheck("rpm"),
			Timeout:     5 * time.Second,
			Critical:    true,
		})
	}

	return ValidationSuite{
		Name:   fmt.Sprintf("%s-system", installer),
		Checks: checks,
	}
}

// createCommandAvailabilityCheck creates a validator that checks if a command is available
func createCommandAvailabilityCheck(command string) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		_, err := utils.CommandExec.RunShellCommand(fmt.Sprintf("which %s", command))
		if err != nil {
			return fmt.Errorf("command '%s' not found in PATH", command)
		}
		return nil
	}
}

// createVersionCheck creates a validator that checks if a command responds to --version
func createVersionCheck(command string) func(ctx context.Context) error {
	return func(ctx context.Context) error {
		versionCommands := []string{
			fmt.Sprintf("%s --version", command),
			fmt.Sprintf("%s -V", command),
			fmt.Sprintf("%s version", command),
		}

		// Special case for rpm which has different version command format
		if command == "rpm" {
			versionCommands = []string{
				fmt.Sprintf("%s --version", command),
			}
		}

		for _, cmd := range versionCommands {
			if _, err := utils.CommandExec.RunShellCommand(cmd); err == nil {
				return nil // At least one version command worked
			}
		}

		return fmt.Errorf("command '%s' does not respond to version commands", command)
	}
}

// createPermissionCheck creates a validator that checks basic system permissions
func createPermissionCheck() func(ctx context.Context) error {
	return func(ctx context.Context) error {
		// Test if we can write to /tmp (basic permission check)
		testFile := fmt.Sprintf("/tmp/devex-permission-test-%d", time.Now().UnixNano())
		cmd := fmt.Sprintf("touch %s && rm -f %s", testFile, testFile)

		if _, err := utils.CommandExec.RunShellCommand(cmd); err != nil {
			return fmt.Errorf("insufficient permissions for file operations")
		}

		return nil
	}
}

// createDiskSpaceCheck creates a validator that checks available disk space
func createDiskSpaceCheck() func(ctx context.Context) error {
	return func(ctx context.Context) error {
		// Check available space in /tmp and /usr/local (common install locations)
		locations := []string{"/tmp", "/usr/local", "/"}

		for _, location := range locations {
			output, err := utils.CommandExec.RunShellCommand(fmt.Sprintf("df -h %s", location))
			if err != nil {
				log.Debug("Could not check disk space", "location", location, "error", err)
				continue
			}

			// Basic check - if df command succeeds, assume space is available
			// More sophisticated parsing could be added here
			if len(output) > 0 {
				log.Debug("Disk space check passed", "location", location)
			}
		}

		return nil
	}
}

// CreateNetworkValidationSuite creates network connectivity validation checks
func CreateNetworkValidationSuite() ValidationSuite {
	return ValidationSuite{
		Name: "network",
		Checks: []ValidationCheck{
			{
				Name:        "internet-connectivity",
				Description: "Check basic internet connectivity",
				Validator:   createInternetConnectivityCheck(),
				Timeout:     10 * time.Second,
				Critical:    false,
			},
			{
				Name:        "dns-resolution",
				Description: "Check DNS resolution",
				Validator:   createDNSResolutionCheck(),
				Timeout:     10 * time.Second,
				Critical:    false,
			},
		},
	}
}

// createInternetConnectivityCheck creates a validator for internet connectivity
func createInternetConnectivityCheck() func(ctx context.Context) error {
	return func(ctx context.Context) error {
		// Try to ping a reliable host
		hosts := []string{"8.8.8.8", "1.1.1.1", "google.com"}

		for _, host := range hosts {
			cmd := fmt.Sprintf("ping -c 1 -W 3 %s", host)
			if _, err := utils.CommandExec.RunShellCommand(cmd); err == nil {
				return nil // At least one ping succeeded
			}
		}

		return fmt.Errorf("no internet connectivity detected")
	}
}

// createDNSResolutionCheck creates a validator for DNS resolution
func createDNSResolutionCheck() func(ctx context.Context) error {
	return func(ctx context.Context) error {
		hosts := []string{"google.com", "github.com", "ubuntu.com"}

		for _, host := range hosts {
			cmd := fmt.Sprintf("nslookup %s", host)
			if _, err := utils.CommandExec.RunShellCommand(cmd); err == nil {
				return nil // At least one DNS lookup succeeded
			}
		}

		return fmt.Errorf("DNS resolution failed")
	}
}
