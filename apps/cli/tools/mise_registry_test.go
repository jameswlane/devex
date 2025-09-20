package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/BurntSushi/toml"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

// Sample TOML registry data for testing
const sampleRegistryTOML = `[tools]
1password.aliases = ["1password-cli", "op"]
1password.description = "Password manager developed by AgileBits Inc"
1password.backends = [
    "aqua:1password/cli",
    "asdf:mise-plugins/mise-1password-cli"
]

act.description = "Run your GitHub Actions locally"
act.backends = ["aqua:nektos/act", "ubi:nektos/act", "asdf:gr1m0h/asdf-act"]
act.test = ["act --version", "act version {{version}}"]

clangd.backends = ["asdf:mise-plugins/mise-clangd"]
# No description for clangd

go.description = "Go programming language"
go.backends = ["asdf:kennyp/asdf-golang"]

python.description = "Python programming language"
python.backends = ["asdf:danhper/asdf-python"]
`

func TestGenerateMiseRegistryYAML(t *testing.T) {
	// Create a test server that serves our sample TOML
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte(sampleRegistryTOML))
		require.NoError(t, err)
	}))
	defer server.Close()

	// Create a temporary directory for output
	tempDir := t.TempDir()
	testOutputPath := filepath.Join(tempDir, "mise.yaml")

	// Use our testable function
	err := generateMiseRegistryYAMLWithURL(server.URL, testOutputPath)
	require.NoError(t, err)

	// Verify the output file was created
	assert.FileExists(t, testOutputPath)

	// Read and parse the generated YAML
	yamlData, err := os.ReadFile(testOutputPath)
	require.NoError(t, err)

	var tools []ToolEntry
	err = yaml.Unmarshal(yamlData, &tools)
	require.NoError(t, err)

	// Verify we got the expected tools (only those with descriptions)
	expectedTools := map[string]string{
		"1password": "Password manager developed by AgileBits Inc",
		"act":       "Run your GitHub Actions locally",
		"go":        "Go programming language",
		"python":    "Python programming language",
	}

	assert.Len(t, tools, len(expectedTools))

	// Check each tool
	toolMap := make(map[string]string)
	for _, tool := range tools {
		toolMap[tool.Tool] = tool.Description
	}

	for expectedTool, expectedDesc := range expectedTools {
		assert.Contains(t, toolMap, expectedTool)
		assert.Equal(t, expectedDesc, toolMap[expectedTool])
	}

	// Verify tools are sorted alphabetically
	for i := 1; i < len(tools); i++ {
		assert.True(t, tools[i-1].Tool < tools[i].Tool,
			"Tools should be sorted alphabetically: %s should come before %s",
			tools[i-1].Tool, tools[i].Tool)
	}
}

func TestGenerateMiseRegistryYAML_HTTPError(t *testing.T) {
	// Create a test server that returns an error
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer server.Close()

	tempDir := t.TempDir()
	testOutputPath := filepath.Join(tempDir, "mise.yaml")

	err := generateMiseRegistryYAMLWithURL(server.URL, testOutputPath)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "non-200 response")
}

func TestGenerateMiseRegistryYAML_InvalidTOML(t *testing.T) {
	// Create a test server that serves invalid TOML
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte("invalid toml [[["))
		require.NoError(t, err)
	}))
	defer server.Close()

	tempDir := t.TempDir()
	testOutputPath := filepath.Join(tempDir, "mise.yaml")

	err := generateMiseRegistryYAMLWithURL(server.URL, testOutputPath)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "failed to parse TOML")
}

func TestGenerateMiseRegistryYAML_EmptyRegistry(t *testing.T) {
	// Create a test server that serves empty TOML
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte("[tools]"))
		require.NoError(t, err)
	}))
	defer server.Close()

	tempDir := t.TempDir()
	testOutputPath := filepath.Join(tempDir, "mise.yaml")

	err := generateMiseRegistryYAMLWithURL(server.URL, testOutputPath)
	require.NoError(t, err)

	// Verify empty output
	yamlData, err := os.ReadFile(testOutputPath)
	require.NoError(t, err)

	var tools []ToolEntry
	err = yaml.Unmarshal(yamlData, &tools)
	require.NoError(t, err)

	assert.Len(t, tools, 0)
}

func TestGenerateMiseRegistryYAML_NoToolsSection(t *testing.T) {
	// Create a test server that serves TOML without tools section
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte("[other]\nkey = \"value\""))
		require.NoError(t, err)
	}))
	defer server.Close()

	tempDir := t.TempDir()
	testOutputPath := filepath.Join(tempDir, "mise.yaml")

	err := generateMiseRegistryYAMLWithURL(server.URL, testOutputPath)
	require.NoError(t, err)

	// Verify empty output
	yamlData, err := os.ReadFile(testOutputPath)
	require.NoError(t, err)

	var tools []ToolEntry
	err = yaml.Unmarshal(yamlData, &tools)
	require.NoError(t, err)

	assert.Len(t, tools, 0)
}

func TestGenerateMiseRegistryYAML_InvalidOutputPath(t *testing.T) {
	// Create a test server that serves valid TOML
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte(sampleRegistryTOML))
		require.NoError(t, err)
	}))
	defer server.Close()

	// Try to write to an invalid path (directory that doesn't exist and can't be created)
	invalidPath := "/invalid/nonexistent/path/mise.yaml"

	err := generateMiseRegistryYAMLWithURL(server.URL, invalidPath)
	assert.Error(t, err)
}

func TestToolEntry_YAMLSerialization(t *testing.T) {
	tools := []ToolEntry{
		{Tool: "go", Description: "Go programming language"},
		{Tool: "python", Description: "Python programming language"},
		{Tool: "node", Description: "Node.js runtime"},
	}

	// Serialize to YAML
	yamlData, err := yaml.Marshal(tools)
	require.NoError(t, err)

	// Deserialize back
	var deserializedTools []ToolEntry
	err = yaml.Unmarshal(yamlData, &deserializedTools)
	require.NoError(t, err)

	assert.Equal(t, tools, deserializedTools)

	// Verify YAML structure
	yamlStr := string(yamlData)
	assert.Contains(t, yamlStr, "tool: go")
	assert.Contains(t, yamlStr, "description: Go programming language")
	assert.Contains(t, yamlStr, "tool: python")
	assert.Contains(t, yamlStr, "description: Python programming language")
}

func TestGenerateMiseRegistryYAML_SpecialCharacters(t *testing.T) {
	specialTOML := `[tools]
special-tool.description = "Tool with special chars: !@#$%^&*()"
unicode-tool.description = "Tool with unicode: 🔧 测试 пример"
quotes-tool.description = "Tool with \"quotes\" and 'apostrophes'"
`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte(specialTOML))
		require.NoError(t, err)
	}))
	defer server.Close()

	tempDir := t.TempDir()
	testOutputPath := filepath.Join(tempDir, "mise.yaml")

	err := generateMiseRegistryYAMLWithURL(server.URL, testOutputPath)
	require.NoError(t, err)

	// Read and parse the generated YAML
	yamlData, err := os.ReadFile(testOutputPath)
	require.NoError(t, err)

	var tools []ToolEntry
	err = yaml.Unmarshal(yamlData, &tools)
	require.NoError(t, err)

	// Verify special characters are preserved
	toolMap := make(map[string]string)
	for _, tool := range tools {
		toolMap[tool.Tool] = tool.Description
	}

	assert.Equal(t, "Tool with special chars: !@#$%^&*()", toolMap["special-tool"])
	assert.Equal(t, "Tool with unicode: 🔧 测试 пример", toolMap["unicode-tool"])
	assert.Equal(t, "Tool with \"quotes\" and 'apostrophes'", toolMap["quotes-tool"])
}

// Helper function for testing with custom URL and output path
func generateMiseRegistryYAMLWithURL(url, outputPath string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
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

	// Sort tools alphabetically
	sort.Slice(tools, func(i, j int) bool {
		return tools[i].Tool < tools[j].Tool
	})

	// Ensure output directory exists
	if err := os.MkdirAll(filepath.Dir(outputPath), 0755); err != nil {
		return fmt.Errorf("failed to ensure output dir: %w", err)
	}

	// Create output file
	file, err := os.Create(outputPath)
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

	return nil
}

// Benchmark tests
func BenchmarkGenerateMiseRegistryYAML_SmallRegistry(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte(sampleRegistryTOML))
		require.NoError(b, err)
	}))
	defer server.Close()

	tempDir := b.TempDir()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		testOutputPath := filepath.Join(tempDir, fmt.Sprintf("mise_bench_%d.yaml", i))
		err := generateMiseRegistryYAMLWithURL(server.URL, testOutputPath)
		require.NoError(b, err)
		os.Remove(testOutputPath) // Clean up for next iteration
	}
}

func BenchmarkGenerateMiseRegistryYAML_LargeRegistry(b *testing.B) {
	// Generate a larger TOML for benchmark
	largeTOML := "[tools]\n"
	for i := 0; i < 1000; i++ {
		largeTOML += fmt.Sprintf("tool%d.description = \"Description for tool %d\"\n", i, i)
		largeTOML += fmt.Sprintf("tool%d.backends = [\"backend%d\"]\n", i, i)
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain")
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte(largeTOML))
		require.NoError(b, err)
	}))
	defer server.Close()

	tempDir := b.TempDir()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		testOutputPath := filepath.Join(tempDir, fmt.Sprintf("mise_large_bench_%d.yaml", i))
		err := generateMiseRegistryYAMLWithURL(server.URL, testOutputPath)
		require.NoError(b, err)
		os.Remove(testOutputPath) // Clean up for next iteration
	}
}
