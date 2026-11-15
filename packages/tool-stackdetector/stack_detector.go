package main

import (
	"fmt"
	"sort"
	"strings"
)

// Execute handles command execution
func (p *StackDetectorPlugin) Execute(command string, args []string) error {
	switch command {
	case "detect", "analyze", "report":
		if wantsHelp(args) {
			p.printCommandHelp(command)
			return nil
		}
	}

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

// wantsHelp reports whether args ask for command help.
func wantsHelp(args []string) bool {
	for _, arg := range args {
		if arg == "--help" || arg == "-h" {
			return true
		}
	}
	return false
}

// printCommandHelp prints usage for a command from the plugin metadata.
func (p *StackDetectorPlugin) printCommandHelp(command string) {
	info := p.Info()
	fmt.Printf("Usage: %s %s [flags]\n", info.Name, command)
	for _, cmd := range info.Commands {
		if cmd.Name != command {
			continue
		}
		if cmd.Description != "" {
			fmt.Printf("\n%s\n", cmd.Description)
		}
		if cmd.Usage != "" {
			fmt.Printf("%s\n", cmd.Usage)
		}
		if len(cmd.Flags) > 0 {
			names := make([]string, 0, len(cmd.Flags))
			for name := range cmd.Flags {
				names = append(names, name)
			}
			sort.Strings(names)
			fmt.Println("\nFlags:")
			for _, name := range names {
				fmt.Printf("  --%s\t%s\n", name, cmd.Flags[name])
			}
		}
	}
	fmt.Println("  -h, --help\tShow this help")
}

// parseDetectArgs returns the directory to analyze for the detect command.
// It accepts --path <dir>, --path=<dir>, or a positional directory.
func parseDetectArgs(args []string) (string, error) {
	dir := "."
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--path":
			if i+1 >= len(args) {
				return "", fmt.Errorf("flag --path requires a directory")
			}
			dir = args[i+1]
			i++
		case strings.HasPrefix(arg, "--path="):
			dir = strings.TrimPrefix(arg, "--path=")
		case strings.HasPrefix(arg, "-"):
			return "", fmt.Errorf("unknown flag %q for detect (see 'detect --help')", arg)
		default:
			dir = arg
		}
	}
	return dir, nil
}
