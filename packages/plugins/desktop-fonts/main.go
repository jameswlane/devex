package main

import (
	"fmt"
	"os"
	"strings"

	sdk "github.com/jameswlane/devex/packages/shared/plugin-sdk"
)

var version = "dev" // Set by goreleaser

// FontsPlugin implements the desktop fonts plugin
type FontsPlugin struct {
	*sdk.BasePlugin
}

// NewFontsPlugin creates a new fonts plugin
func NewFontsPlugin() *FontsPlugin {
	info := sdk.PluginInfo{
		Name:        "desktop-fonts",
		Version:     version,
		Description: "Desktop font management and installation",
		Author:      "DevEx Team",
		Repository:  "https://github.com/jameswlane/devex",
		Tags:        []string{"desktop", "fonts", "customization"},
		Commands: []sdk.PluginCommand{
			{
				Name:        "install",
				Description: "Install fonts from configuration",
				Usage:       "Install fonts from YAML configuration file",
			},
			{
				Name:        "list",
				Description: "List available fonts",
				Usage:       "List fonts available for installation",
			},
			{
				Name:        "cache-refresh",
				Description: "Refresh font cache",
				Usage:       "Refresh system font cache using fc-cache",
			},
		},
	}

	return &FontsPlugin{
		BasePlugin: sdk.NewBasePlugin(info),
	}
}

// Execute handles command execution
func (p *FontsPlugin) Execute(command string, args []string) error {
	switch command {
	case "install":
		return p.handleInstall(args)
	case "list":
		return p.handleList(args)
	case "cache-refresh":
		return p.handleCacheRefresh(args)
	default:
		return fmt.Errorf("unknown command: %s", command)
	}
}

func (p *FontsPlugin) handleInstall(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no font configuration file specified")
	}

	configFile := args[0]
	fmt.Printf("Installing fonts from configuration: %s\n", configFile)

	// TODO: Implement font installation from moved font package
	return fmt.Errorf("font installation not yet implemented in plugin")
}

func (p *FontsPlugin) handleList(args []string) error {
	fmt.Println("Available font management operations:")
	fmt.Println("  - install <config.yaml>  Install fonts from configuration")
	fmt.Println("  - cache-refresh          Refresh system font cache")
	return nil
}

func (p *FontsPlugin) handleCacheRefresh(args []string) error {
	fmt.Println("Refreshing font cache...")
	return sdk.ExecCommand(false, "fc-cache", "-f", "-v")
}

func main() {
	plugin := NewFontsPlugin()
	sdk.HandleArgs(plugin, os.Args[1:])
}
