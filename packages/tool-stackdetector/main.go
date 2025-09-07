package main

// Build timestamp: 2025-09-06

import (
	"os"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
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
		Description: "Development stack detection and analysis for modern development workflows",
		Author:      "DevEx Team",
		Repository:  "https://github.com/jameswlane/devex",
		Tags:        []string{"development", "stack", "detection", "analysis", "project"},
		Commands: []sdk.PluginCommand{
			{
				Name:        "detect",
				Description: "Detect development stack",
				Usage:       "Automatically detect technology stack and frameworks in project directory",
				Flags: map[string]string{
					"path": "Directory path to analyze (default: current directory)",
				},
			},
			{
				Name:        "analyze",
				Description: "Analyze project dependencies",
				Usage:       "Deep analysis of project structure, dependencies, and configurations",
				Flags: map[string]string{
					"verbose": "Show detailed analysis information",
				},
			},
			{
				Name:        "report",
				Description: "Generate stack report",
				Usage:       "Generate comprehensive report of detected technologies and recommendations",
				Flags: map[string]string{
					"format": "Output format (text, json, yaml)",
					"output": "Output file path (default: stdout)",
				},
			},
		},
	}

	return &StackDetectorPlugin{
		BasePlugin: sdk.NewBasePlugin(info),
	}
}

func main() {
	plugin := NewStackDetectorPlugin()
	sdk.HandleArgs(plugin, os.Args[1:])
}
