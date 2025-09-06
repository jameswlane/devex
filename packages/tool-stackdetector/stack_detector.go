package main

import (
	"fmt"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
)

// StackDetectorPlugin implements the Stack detection plugin
type StackDetectorPlugin struct {
	*sdk.BasePlugin
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