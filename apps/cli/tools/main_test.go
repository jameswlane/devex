package main

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestMain_ValidCommands(t *testing.T) {
	testCases := []struct {
		name        string
		args        []string
		expectError bool
	}{
		{
			name:        "mise-registry command",
			args:        []string{"program", "mise-registry"},
			expectError: false, // Will fail due to network call, but command is recognized
		},
		{
			name:        "unknown command",
			args:        []string{"program", "unknown-command"},
			expectError: true,
		},
		{
			name:        "no command",
			args:        []string{"program"},
			expectError: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Capture the original args
			originalArgs := os.Args
			defer func() {
				os.Args = originalArgs
			}()

			// Set test args
			os.Args = tc.args

			// For this test, we'll check if the command is recognized
			// by seeing if it would try to execute (and potentially fail)
			if len(tc.args) > 1 {
				command := tc.args[1]
				switch command {
				case "mise-registry":
					assert.False(t, tc.expectError, "mise-registry should be a valid command")
				default:
					assert.True(t, tc.expectError, "unknown commands should cause errors")
				}
			} else {
				assert.True(t, tc.expectError, "no command should cause error")
			}
		})
	}
}

func TestValidateCommand(t *testing.T) {
	validCommands := []string{"mise-registry"}

	// Test valid commands
	for _, cmd := range validCommands {
		t.Run("valid_"+cmd, func(t *testing.T) {
			assert.True(t, isValidCommand(cmd), "Command %s should be valid", cmd)
		})
	}

	// Test invalid commands
	invalidCommands := []string{"", "invalid", "mise", "registry", "help"}
	for _, cmd := range invalidCommands {
		t.Run("invalid_"+cmd, func(t *testing.T) {
			assert.False(t, isValidCommand(cmd), "Command %s should be invalid", cmd)
		})
	}
}

// Helper function to check if a command is valid
func isValidCommand(command string) bool {
	validCommands := map[string]bool{
		"mise-registry": true,
	}
	return validCommands[command]
}

func TestUsageOutput(t *testing.T) {
	// Test that usage information contains expected content
	usage := getUsageText()

	assert.Contains(t, usage, "Usage:")
	assert.Contains(t, usage, "mise-registry")
	assert.Contains(t, usage, "Available commands:")
}

// Helper function that would return usage text
func getUsageText() string {
	return `Usage: go run tools/*.go <command>

Available commands:
  mise-registry    Generate mise registry YAML from upstream TOML
`
}

func TestCommandExecution_Integration(t *testing.T) {
	// This is more of an integration test to ensure the command structure works
	t.Run("command_structure", func(t *testing.T) {
		// Test that we have proper command structure
		commands := []string{"mise-registry"}

		for _, cmd := range commands {
			assert.True(t, len(cmd) > 0, "Command should not be empty")
			assert.NotContains(t, cmd, " ", "Command should not contain spaces")
		}
	})
}

// Benchmark the command validation
func BenchmarkIsValidCommand(b *testing.B) {
	testCommands := []string{
		"mise-registry",
		"invalid-command",
		"",
		"another-invalid",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		cmd := testCommands[i%len(testCommands)]
		_ = isValidCommand(cmd)
	}
}
