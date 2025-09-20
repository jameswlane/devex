package main

import (
	"fmt"
)

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
