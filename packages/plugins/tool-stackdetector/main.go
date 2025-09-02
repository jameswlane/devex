package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	sdk "github.com/jameswlane/devex/packages/shared/plugin-sdk"
)

var version = "dev" // Set by goreleaser

// StackDetectorPlugin implements the Stack detection plugin
type StackDetectorPlugin struct {
	*sdk.BasePlugin
}

// NewStackDetectorPlugin creates a new StackDetector plugin
func NewStackDetectorPlugin() *StackDetectorPlugin {
	info := sdk.PluginInfo{
		Name:        "tool-stackdetector",
		Version:     version,
		Description: "Development stack detection and analysis",
		Author:      "DevEx Team",
		Repository:  "https://github.com/jameswlane/devex",
		Tags:        []string{"development", "stack", "detection"},
		Commands: []sdk.PluginCommand{
			{
				Name:        "detect",
				Description: "Detect development stack",
				Usage:       "Automatically detect technology stack in current directory",
			},
			{
				Name:        "analyze",
				Description: "Analyze project dependencies",
				Usage:       "Deep analysis of project structure and dependencies",
			},
			{
				Name:        "report",
				Description: "Generate stack report",
				Usage:       "Generate detailed report of detected technologies",
			},
		},
	}

	return &StackDetectorPlugin{
		BasePlugin: sdk.NewBasePlugin(info),
	}
}

// Execute handles command execution
func (p *StackDetectorPlugin) Execute(command string, args []string) error {
	switch command {
	case "detect":
		return p.handleDetect(args)
	case "analyze":
		return p.handleAnalyze(args)
	case "report":
		return p.handleReport(args)
	default:
		return fmt.Errorf("unknown command: %s", command)
	}
}

func (p *StackDetectorPlugin) handleDetect(args []string) error {
	dir := "."
	if len(args) > 0 {
		dir = args[0]
	}
	
	fmt.Printf("Detecting development stack in: %s\n", dir)
	
	// Basic file-based detection
	stack := p.detectStack(dir)
	if len(stack) == 0 {
		fmt.Println("No recognizable technology stack detected")
		return nil
	}
	
	fmt.Println("Detected technologies:")
	for _, tech := range stack {
		fmt.Printf("  - %s\n", tech)
	}
	
	return nil
}

func (p *StackDetectorPlugin) handleAnalyze(args []string) error {
	dir := "."
	if len(args) > 0 {
		dir = args[0]
	}
	
	fmt.Printf("Analyzing project structure in: %s\n", dir)
	
	// TODO: Implement deep project analysis
	return fmt.Errorf("project analysis not yet implemented in plugin")
}

func (p *StackDetectorPlugin) handleReport(args []string) error {
	dir := "."
	if len(args) > 0 {
		dir = args[0]
	}
	
	fmt.Printf("Generating stack report for: %s\n", dir)
	
	// TODO: Implement detailed report generation
	return fmt.Errorf("report generation not yet implemented in plugin")
}

func (p *StackDetectorPlugin) detectStack(dir string) []string {
	var detected []string
	
	// Common configuration files and their associated technologies
	detectors := map[string]string{
		"package.json":     "Node.js",
		"requirements.txt": "Python",
		"Pipfile":          "Python (Pipenv)",
		"pyproject.toml":   "Python (Poetry)",
		"Cargo.toml":       "Rust",
		"go.mod":           "Go",
		"composer.json":    "PHP",
		"pom.xml":          "Java (Maven)",
		"build.gradle":     "Java/Kotlin (Gradle)",
		"Gemfile":          "Ruby",
		"mix.exs":          "Elixir",
		"pubspec.yaml":     "Dart/Flutter",
		"Dockerfile":       "Docker",
		"docker-compose.yml": "Docker Compose",
		".terraform":       "Terraform",
		"yarn.lock":        "Node.js (Yarn)",
		"package-lock.json": "Node.js (npm)",
		"tsconfig.json":    "TypeScript",
	}
	
	// Check for files
	for file, tech := range detectors {
		path := filepath.Join(dir, file)
		if _, err := os.Stat(path); err == nil {
			detected = append(detected, tech)
		}
	}
	
	// Check for directories
	dirDetectors := map[string]string{
		"node_modules": "Node.js",
		".git":         "Git VCS",
		"venv":         "Python (venv)",
		"env":          "Python (virtualenv)",
		"target":       "Rust/Java/Scala",
		"dist":         "Build artifacts",
		".next":        "Next.js",
		".nuxt":        "Nuxt.js",
	}
	
	for dirName, tech := range dirDetectors {
		path := filepath.Join(dir, dirName)
		if info, err := os.Stat(path); err == nil && info.IsDir() {
			// Avoid duplicates
			if !contains(detected, tech) {
				detected = append(detected, tech)
			}
		}
	}
	
	return detected
}

func contains(slice []string, item string) bool {
	for _, s := range slice {
		if strings.Contains(s, item) || strings.Contains(item, s) {
			return true
		}
	}
	return false
}

func main() {
	plugin := NewStackDetectorPlugin()
	sdk.HandleArgs(plugin, os.Args[1:])
}
