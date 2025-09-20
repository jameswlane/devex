package main

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

// PluginMetadata represents the structure of metadata.yaml
type PluginMetadata struct {
	Name        string                 `yaml:"name"`
	Type        string                 `yaml:"type"`
	Description string                 `yaml:"description"`
	Version     string                 `yaml:"version"`
	Author      string                 `yaml:"author"`
	License     string                 `yaml:"license"`
	Homepage    string                 `yaml:"homepage"`
	Repository  string                 `yaml:"repository"`
	Platforms   map[string]interface{} `yaml:"platforms"`
	SDKVersion  string                 `yaml:"sdk_version"`
	APIVersion  string                 `yaml:"api_version"`
	Priority    int                    `yaml:"priority"`
	Tags        []string               `yaml:"tags"`
}

func TestMetadataFileExists(t *testing.T) {
	metadataPath := filepath.Join(".", "metadata.yaml")
	if _, err := os.Stat(metadataPath); os.IsNotExist(err) {
		t.Fatal("metadata.yaml file does not exist")
	}
}

func TestMetadataValidYAML(t *testing.T) {
	metadataPath := filepath.Join(".", "metadata.yaml")
	content, err := os.ReadFile(metadataPath)
	if err != nil {
		t.Fatalf("Failed to read metadata.yaml: %v", err)
	}

	var metadata PluginMetadata
	if err := yaml.Unmarshal(content, &metadata); err != nil {
		t.Fatalf("metadata.yaml is not valid YAML: %v", err)
	}
}

func TestMetadataRequiredFields(t *testing.T) {
	metadataPath := filepath.Join(".", "metadata.yaml")
	content, err := os.ReadFile(metadataPath)
	if err != nil {
		t.Fatalf("Failed to read metadata.yaml: %v", err)
	}

	var metadata PluginMetadata
	if err := yaml.Unmarshal(content, &metadata); err != nil {
		t.Fatalf("Failed to parse metadata.yaml: %v", err)
	}

	// Test required fields
	if metadata.Name == "" {
		t.Error("name field is required")
	}
	if metadata.Type == "" {
		t.Error("type field is required")
	}
	if metadata.Description == "" {
		t.Error("description field is required")
	}
	if metadata.Version == "" {
		t.Error("version field is required")
	}
	if metadata.SDKVersion == "" {
		t.Error("sdk_version field is required")
	}
	if metadata.APIVersion == "" {
		t.Error("api_version field is required")
	}
}

func TestMetadataPluginNameConvention(t *testing.T) {
	metadataPath := filepath.Join(".", "metadata.yaml")
	content, err := os.ReadFile(metadataPath)
	if err != nil {
		t.Fatalf("Failed to read metadata.yaml: %v", err)
	}

	var metadata PluginMetadata
	if err := yaml.Unmarshal(content, &metadata); err != nil {
		t.Fatalf("Failed to parse metadata.yaml: %v", err)
	}

	// For package manager plugins, name should start with "package-manager-"
	if metadata.Type == "package-manager" {
		expectedPrefix := "package-manager-"
		if len(metadata.Name) < len(expectedPrefix) || metadata.Name[:len(expectedPrefix)] != expectedPrefix {
			t.Errorf("Package manager plugin name must start with '%s', got: %s", expectedPrefix, metadata.Name)
		}
	}
}

func TestMetadataPlatformSupport(t *testing.T) {
	metadataPath := filepath.Join(".", "metadata.yaml")
	content, err := os.ReadFile(metadataPath)
	if err != nil {
		t.Fatalf("Failed to read metadata.yaml: %v", err)
	}

	var metadata PluginMetadata
	if err := yaml.Unmarshal(content, &metadata); err != nil {
		t.Fatalf("Failed to parse metadata.yaml: %v", err)
	}

	// Platform support should be defined
	if len(metadata.Platforms) == 0 {
		t.Error("platforms field must define at least one platform")
	}

	// For APT package manager, should support Linux
	if metadata.Name == "package-manager-apt" {
		if _, hasLinux := metadata.Platforms["linux"]; !hasLinux {
			t.Error("APT package manager must support Linux platform")
		}
	}
}

func TestMetadataVersionFormat(t *testing.T) {
	metadataPath := filepath.Join(".", "metadata.yaml")
	content, err := os.ReadFile(metadataPath)
	if err != nil {
		t.Fatalf("Failed to read metadata.yaml: %v", err)
	}

	var metadata PluginMetadata
	if err := yaml.Unmarshal(content, &metadata); err != nil {
		t.Fatalf("Failed to parse metadata.yaml: %v", err)
	}

	// Check SDK version format (should be semantic version)
	if metadata.SDKVersion != "1.0.0" {
		t.Errorf("sdk_version should be '1.0.0' for compatibility, got: %s", metadata.SDKVersion)
	}

	// Check API version format
	if metadata.APIVersion != "v1" {
		t.Errorf("api_version should be 'v1' for current API, got: %s", metadata.APIVersion)
	}
}

func TestMetadataPriorityRange(t *testing.T) {
	metadataPath := filepath.Join(".", "metadata.yaml")
	content, err := os.ReadFile(metadataPath)
	if err != nil {
		t.Fatalf("Failed to read metadata.yaml: %v", err)
	}

	var metadata PluginMetadata
	if err := yaml.Unmarshal(content, &metadata); err != nil {
		t.Fatalf("Failed to parse metadata.yaml: %v", err)
	}

	// Priority should be reasonable (1-100)
	if metadata.Priority < 1 || metadata.Priority > 100 {
		t.Errorf("priority should be between 1-100, got: %d", metadata.Priority)
	}
}
