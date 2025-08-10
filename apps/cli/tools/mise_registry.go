package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/BurntSushi/toml"
	"gopkg.in/yaml.v3"
)

const (
	miseRegistryURL = "https://raw.githubusercontent.com/jdx/mise/main/registry.toml"
	outputFilePath  = "config/mise.yaml"
)

// ToolEntry represents a tool with its description.
type ToolEntry struct {
	Tool        string `yaml:"tool"`
	Description string `yaml:"description"`
}

// GenerateMiseRegistryYAML fetches, parses, and writes the registry to YAML.
func GenerateMiseRegistryYAML() error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", miseRegistryURL, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to fetch registry TOML: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("non-200 response from GitHub: %s", resp.Status)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read registry: %w", err)
	}

	// Parse as generic map to see the structure
	raw := make(map[string]interface{})
	if err := toml.Unmarshal(body, &raw); err != nil {
		return fmt.Errorf("failed to parse TOML: %w", err)
	}

	var tools []ToolEntry

	// Try to extract tools
	if toolsSection, exists := raw["tools"]; exists {
		if toolsMap, ok := toolsSection.(map[string]interface{}); ok {
			log.Printf("🔍 Debug: Processing %d tools", len(toolsMap))

			// Let's examine the first tool to understand the structure
			count := 0
			for toolName, toolData := range toolsMap {
				if count == 0 {
					log.Printf("🔍 Debug: First tool name: %s", toolName)
					log.Printf("🔍 Debug: First tool data type: %T", toolData)
					if toolMap, ok := toolData.(map[string]interface{}); ok {
						log.Printf("🔍 Debug: First tool has %d properties", len(toolMap))
						for key, val := range toolMap {
							log.Printf("🔍 Debug: Property: %s = %v (type: %T)", key, val, val)
							if count > 5 { // Limit debug output
								break
							}
							count++
						}
					}
					break
				}
			}

			// Now extract tools with descriptions
			for toolName, toolData := range toolsMap {
				if toolMap, ok := toolData.(map[string]interface{}); ok {
					if desc, hasDesc := toolMap["description"]; hasDesc {
						if description, ok := desc.(string); ok {
							tools = append(tools, ToolEntry{
								Tool:        toolName,
								Description: strings.TrimSpace(description),
							})
						}
					}
				}
			}
		}
	}

	log.Printf("🔍 Found %d tools with descriptions", len(tools))

	// Sort tools alphabetically
	sort.Slice(tools, func(i, j int) bool {
		return tools[i].Tool < tools[j].Tool
	})

	// Ensure output directory exists
	if err := os.MkdirAll("apps/cli/config", 0750); err != nil {
		return fmt.Errorf("failed to ensure config dir: %w", err)
	}

	// Create output file
	file, err := os.Create(outputFilePath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer file.Close()

	// Write YAML
	enc := yaml.NewEncoder(file)
	enc.SetIndent(2)
	if err := enc.Encode(tools); err != nil {
		return fmt.Errorf("failed to encode YAML: %w", err)
	}

	log.Printf("✅ Wrote %d tools to %s", len(tools), outputFilePath)
	return nil
}
