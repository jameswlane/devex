package config

import (
	"fmt"
	"io/ioutil"
	"os"
)

// LoadCustomOrDefault loads a custom config file if it exists, otherwise loads the default config
func LoadCustomOrDefault(defaultPath, customPath string) ([]byte, error) {
	// Check if custom config exists
	if _, err := os.Stat(customPath); err == nil {
		fmt.Printf("Using custom config: %s\n", customPath)
		return ioutil.ReadFile(customPath)
	}

	// Otherwise, fallback to the default config
	fmt.Printf("Using default config: %s\n", defaultPath)
	return ioutil.ReadFile(defaultPath)
}
