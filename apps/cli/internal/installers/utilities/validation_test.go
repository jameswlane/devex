package utilities_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/jameswlane/devex/apps/cli/internal/installers/utilities"
)

func TestBackgroundValidator(t *testing.T) {
	t.Run("creates validator with timeout", func(t *testing.T) {
		validator := utilities.NewBackgroundValidator(30 * time.Second)
		if validator == nil {
			t.Error("Expected non-nil validator")
		}
	})

	t.Run("adds validation suite", func(t *testing.T) {
		validator := utilities.NewBackgroundValidator(30 * time.Second)

		suite := utilities.ValidationSuite{
			Name: "test-suite",
			Checks: []utilities.ValidationCheck{
				{
					Name:        "always-pass",
					Description: "A check that always passes",
					Validator: func(ctx context.Context) error {
						return nil
					},
					Critical: false,
				},
			},
		}

		validator.AddSuite(suite)

		// We can't directly check if suite was added due to private fields,
		// but we can run validations to verify it works
		ctx := context.Background()
		err := validator.RunValidations(ctx)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}
	})

	t.Run("runs validations successfully", func(t *testing.T) {
		validator := utilities.NewBackgroundValidator(30 * time.Second)

		suite := utilities.ValidationSuite{
			Name: "test-suite",
			Checks: []utilities.ValidationCheck{
				{
					Name:        "pass-check",
					Description: "A check that passes",
					Validator: func(ctx context.Context) error {
						return nil
					},
					Critical: false,
				},
				{
					Name:        "fail-check",
					Description: "A check that fails but is not critical",
					Validator: func(ctx context.Context) error {
						return errors.New("test failure")
					},
					Critical: false,
				},
			},
		}

		validator.AddSuite(suite)

		ctx := context.Background()
		err := validator.RunValidations(ctx)
		if err != nil {
			t.Errorf("Expected no error for non-critical failures, got %v", err)
		}

		// Check results
		results, exists := validator.GetResults("test-suite")
		if !exists {
			t.Error("Expected results to exist for test-suite")
		}

		if len(results) != 2 {
			t.Errorf("Expected 2 results, got %d", len(results))
		}

		// Find and check the passing result
		var passResult, failResult *utilities.ValidationResult
		for i := range results {
			switch results[i].Check {
			case "pass-check":
				passResult = &results[i]
			case "fail-check":
				failResult = &results[i]
			}
		}

		if passResult == nil || !passResult.Success {
			t.Error("Expected pass-check to succeed")
		}

		if failResult == nil || failResult.Success {
			t.Error("Expected fail-check to fail")
		}
	})

	t.Run("handles critical validation failures", func(t *testing.T) {
		validator := utilities.NewBackgroundValidator(30 * time.Second)

		suite := utilities.ValidationSuite{
			Name: "critical-suite",
			Checks: []utilities.ValidationCheck{
				{
					Name:        "critical-fail",
					Description: "A critical check that fails",
					Validator: func(ctx context.Context) error {
						return errors.New("critical failure")
					},
					Critical: true,
				},
			},
		}

		validator.AddSuite(suite)

		ctx := context.Background()
		err := validator.RunValidations(ctx)
		if err == nil {
			t.Error("Expected error for critical validation failure")
		}

		if !validator.HasCriticalFailures() {
			t.Error("Expected to have critical failures")
		}
	})

	t.Run("handles validation timeouts", func(t *testing.T) {
		validator := utilities.NewBackgroundValidator(30 * time.Second)

		suite := utilities.ValidationSuite{
			Name: "timeout-suite",
			Checks: []utilities.ValidationCheck{
				{
					Name:        "slow-check",
					Description: "A check that takes too long",
					Validator: func(ctx context.Context) error {
						select {
						case <-time.After(200 * time.Millisecond):
							return nil
						case <-ctx.Done():
							return ctx.Err()
						}
					},
					Timeout:  50 * time.Millisecond, // Shorter than the validator delay
					Critical: false,
				},
			},
		}

		validator.AddSuite(suite)

		ctx := context.Background()
		err := validator.RunValidations(ctx)
		if err != nil {
			t.Errorf("Expected no error for non-critical timeout, got %v", err)
		}

		results, exists := validator.GetResults("timeout-suite")
		if !exists || len(results) == 0 {
			t.Error("Expected results to exist")
		}

		result := results[0]
		if result.Success {
			t.Error("Expected timeout check to fail")
		}

		if result.Error == nil {
			t.Error("Expected timeout error to be recorded")
		}
	})

	t.Run("gets all results", func(t *testing.T) {
		validator := utilities.NewBackgroundValidator(30 * time.Second)

		suite1 := utilities.ValidationSuite{
			Name: "suite1",
			Checks: []utilities.ValidationCheck{
				{
					Name:      "check1",
					Validator: func(ctx context.Context) error { return nil },
				},
			},
		}

		suite2 := utilities.ValidationSuite{
			Name: "suite2",
			Checks: []utilities.ValidationCheck{
				{
					Name:      "check2",
					Validator: func(ctx context.Context) error { return nil },
				},
			},
		}

		validator.AddSuite(suite1)
		validator.AddSuite(suite2)

		ctx := context.Background()
		err := validator.RunValidations(ctx)
		if err != nil {
			t.Errorf("Expected no error, got %v", err)
		}

		allResults := validator.GetAllResults()
		if len(allResults) != 2 {
			t.Errorf("Expected 2 suite results, got %d", len(allResults))
		}

		if _, exists := allResults["suite1"]; !exists {
			t.Error("Expected suite1 results")
		}

		if _, exists := allResults["suite2"]; !exists {
			t.Error("Expected suite2 results")
		}
	})
}

func TestValidationSuiteCreation(t *testing.T) {
	t.Run("creates system validation suite", func(t *testing.T) {
		suite := utilities.CreateSystemValidationSuite("apt")

		if suite.Name != "apt-system" {
			t.Errorf("Expected suite name 'apt-system', got '%s'", suite.Name)
		}

		if len(suite.Checks) == 0 {
			t.Error("Expected validation checks in system suite")
		}

		// Check that at least some critical checks exist
		hasCritical := false
		for _, check := range suite.Checks {
			if check.Critical {
				hasCritical = true
				break
			}
		}

		if !hasCritical {
			t.Error("Expected at least one critical check in system validation suite")
		}
	})

	t.Run("creates network validation suite", func(t *testing.T) {
		suite := utilities.CreateNetworkValidationSuite()

		if suite.Name != "network" {
			t.Errorf("Expected suite name 'network', got '%s'", suite.Name)
		}

		if len(suite.Checks) == 0 {
			t.Error("Expected validation checks in network suite")
		}

		// Check that checks have reasonable timeouts
		for _, check := range suite.Checks {
			if check.Timeout == 0 {
				t.Errorf("Check '%s' should have a timeout", check.Name)
			}
		}
	})
}

func TestValidationIntegration(t *testing.T) {
	t.Run("runs system and network validations together", func(t *testing.T) {
		validator := utilities.NewBackgroundValidator(60 * time.Second)

		// Add both system and network validation suites
		systemSuite := utilities.CreateSystemValidationSuite("apt")
		networkSuite := utilities.CreateNetworkValidationSuite()

		validator.AddSuite(systemSuite)
		validator.AddSuite(networkSuite)

		ctx := context.Background()
		err := validator.RunValidations(ctx)

		// We expect this might fail in test environment, but shouldn't panic
		if err != nil {
			t.Logf("Validation failed as expected in test environment: %v", err)
		}

		// Check that we got results for both suites
		systemResults, systemExists := validator.GetResults("apt-system")
		networkResults, networkExists := validator.GetResults("network")

		if !systemExists {
			t.Error("Expected system validation results")
		}

		if !networkExists {
			t.Error("Expected network validation results")
		}

		if systemExists && len(systemResults) == 0 {
			t.Error("Expected non-empty system results")
		}

		if networkExists && len(networkResults) == 0 {
			t.Error("Expected non-empty network results")
		}

		// Validate that results have duration information
		for _, result := range systemResults {
			if result.Duration == 0 {
				t.Error("Expected non-zero duration for validation check")
			}
		}
	})
}
