package utils

import (
	"os"
	"strings"

	"github.com/jameswlane/devex/apps/cli/internal/log"
)

// ReplacePlaceholders replaces placeholders in a string with values from environment variables and custom mappings.
func ReplacePlaceholders(input string, customPlaceholders map[string]string) string {
	log.Info("Replacing placeholders in string", "input", input)

	// Replace placeholders using environment variables
	output := os.Expand(input, func(key string) string {
		value, exists := os.LookupEnv(key)
		if exists {
			log.Info("Replacing environment variable placeholder", "placeholder", key, "value", value)
			return value
		}

		log.Warn("Environment variable placeholder not found", "placeholder", key)
		return ""
	})

	// Replace custom placeholders
	for placeholder, value := range customPlaceholders {
		log.Info("Replacing custom placeholder", "placeholder", placeholder, "value", value)
		output = strings.ReplaceAll(output, placeholder, value)
	}

	log.Info("Final string after placeholder replacement", "output", output)
	return output
}
