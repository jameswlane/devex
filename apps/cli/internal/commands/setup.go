package commands

import (
	"context"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/jameswlane/devex/apps/cli/internal/commands/setup"
	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/log"
	"github.com/jameswlane/devex/apps/cli/internal/platform"
	"github.com/jameswlane/devex/apps/cli/internal/types"
)

func init() {
	Register(NewSetupCmd)
}

func NewSetupCmd(repo types.Repository, settings config.CrossPlatformSettings) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "setup",
		Short: "Interactive guided setup for your development environment",
		Long: `The setup command provides an interactive, guided installation experience.

Choose from popular programming languages, databases, and applications to create
a customized development environment tailored to your needs.

The setup process includes:
  • Programming language selection (Node.js, Python, Go, Ruby, etc.)
  • Database installation (PostgreSQL, MySQL, MongoDB, etc.)
  • Essential development tools and applications
  • Shell and theme customization
  • Automated plugin installation

Run 'devex setup' to begin the guided installation process.`,
		Example: `  # Start interactive setup
  devex setup

  # Run automated setup (non-interactive mode)
  DEVEX_NONINTERACTIVE=1 devex setup

  # Setup with verbose logging
  devex setup --verbose`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}

			verbose := viper.GetBool("verbose")
			if verbose {
				log.Info("Starting DevEx setup in verbose mode")
			}

			// Detect current platform
			detectedPlatform := platform.DetectPlatform()

			log.Info("Platform detected",
				"os", detectedPlatform.OS,
				"distribution", detectedPlatform.Distribution,
				"desktop", detectedPlatform.DesktopEnv,
			)

			// Check if running in non-interactive mode
			if !setup.IsInteractiveMode() {
				log.Info("Running in non-interactive automated mode")
				return setup.RunAutomatedSetup(ctx, repo, settings)
			}

			// Run interactive setup using Bubble Tea
			log.Info("Starting interactive setup")
			model := setup.NewSetupModel(repo, settings, detectedPlatform)

			p := tea.NewProgram(model, tea.WithAltScreen())
			finalModel, err := p.Run()
			if err != nil {
				return fmt.Errorf("setup failed: %w", err)
			}

			// Check if setup was successful
			if m, ok := finalModel.(*setup.SetupModel); ok {
				if m.HasErrors() {
					log.Warn("Setup completed with errors")
					os.Exit(1)
				}
			}

			log.Info("Setup completed successfully")
			return nil
		},
	}

	cmd.Flags().BoolP("verbose", "v", false, "Enable verbose output")
	cmd.Flags().Bool("non-interactive", false, "Run in non-interactive mode (automated)")

	_ = viper.BindPFlag("verbose", cmd.Flags().Lookup("verbose"))
	_ = viper.BindPFlag("non-interactive", cmd.Flags().Lookup("non-interactive"))

	return cmd
}
