package commands

import (
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/jameswlane/devex/pkg/config"
	"github.com/jameswlane/devex/pkg/types"
)

func NewRootCmd(version string, repo types.Repository, settings config.CrossPlatformSettings) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "devex",
		Short: "DevEx CLI for setting up your development environment",
		Long: `DevEx is a CLI tool that helps you configure your development environment with minimal effort.

It automates the installation and configuration of:
  • Programming languages (Node.js, Python, Go, Ruby, etc.)
  • Development tools (Docker, Git, VS Code, etc.)
  • Databases (PostgreSQL, MySQL, Redis, etc.)
  • Desktop themes and GNOME extensions
  • Shell configurations and dotfiles

DevEx supports multiple platforms and package managers:
  • Linux: APT (Ubuntu/Debian), DNF (Fedora/RHEL), Pacman (Arch), Flatpak
  • macOS: Homebrew, Mac App Store
  • Windows: winget, Chocolatey, Scoop

All installations are configurable via YAML files in ~/.local/share/devex/config/`,
		Version: version,
		Run: func(cmd *cobra.Command, args []string) {
			// Don't auto-run setup during tests
			if isRunningInTest() {
				_ = cmd.Usage()
				return
			}
			// If no subcommand is provided, run the guided setup
			setupCmd := NewSetupCmd(repo, settings)
			setupCmd.SetArgs(args)
			_ = setupCmd.Execute()
		},
	}

	// Register other commands
	cmd.AddCommand(NewSetupCmd(repo, settings))
	cmd.AddCommand(NewInstallCmd(repo, settings))
	cmd.AddCommand(NewUninstallCmd(repo, settings))
	cmd.AddCommand(NewListCmd(repo, settings))
	cmd.AddCommand(NewShellCmd(repo, settings))
	cmd.AddCommand(NewSystemCmd())
	cmd.AddCommand(NewCompletionCmd())

	cmd.PersistentFlags().BoolP("verbose", "v", false, "Enable verbose output")

	// Bind flags to viper for global access
	_ = viper.BindPFlag("verbose", cmd.PersistentFlags().Lookup("verbose"))

	return cmd
}

// isRunningInTest detects if the code is running within a test environment
func isRunningInTest() bool {
	// Debug: Print args to understand what's happening during tests
	// fmt.Printf("DEBUG: os.Args = %v\n", os.Args)

	// Check for test-related command line arguments
	for _, arg := range os.Args {
		if strings.Contains(arg, "test") || strings.Contains(arg, ".test") || strings.Contains(arg, "ginkgo") {
			return true
		}
	}

	// Check for testing environment variables
	if os.Getenv("GO_TEST") == "1" || os.Getenv("TESTING") == "1" {
		return true
	}

	// More comprehensive test detection
	if len(os.Args) > 0 {
		executable := os.Args[0]
		if strings.HasSuffix(executable, ".test") || strings.Contains(executable, "test") {
			return true
		}
	}

	return false
}
