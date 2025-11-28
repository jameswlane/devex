package commands

import (
	"context"
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/jameswlane/devex/apps/cli/internal/bootstrap"
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
  devex setup --verbose

  # Use a custom setup configuration
  devex setup --config=~/documents/my-setup.yaml

  # Use a predefined setup template
  devex setup --config=minimal
  devex setup --config=full-stack`,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}

			verbose := viper.GetBool("verbose")
			configPath := viper.GetString("config")

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

			// Load setup configuration (local or remote)
			var setupConfig *types.SetupConfig
			var err error
			if configPath != "" {
				log.Info("Loading setup configuration", "path", configPath)
				setupConfig, err = config.LoadSetupConfig(configPath)
				if err != nil {
					return fmt.Errorf("failed to load setup config: %w", err)
				}

				if verbose {
					log.Info("Loaded setup configuration",
						"name", setupConfig.Metadata.Name,
						"version", setupConfig.Metadata.Version,
						"steps", len(setupConfig.Steps),
					)
				}
			}

			// Check if running in non-interactive mode
			if !setup.IsInteractiveMode() {
				log.Info("Running in non-interactive automated mode")
				return setup.RunAutomatedSetup(ctx, repo, settings)
			}

			// Initialize plugin bootstrap for setup
			pluginBootstrap, err := bootstrap.NewPluginBootstrap(false)
			if err != nil {
				return fmt.Errorf("failed to initialize plugin bootstrap: %w", err)
			}

			// Initialize plugins
			if err := pluginBootstrap.Initialize(ctx); err != nil {
				log.Warn("Failed to initialize some plugins", "error", err)
				// Continue anyway - plugins might be installed during setup
			}

			// Run interactive setup using dynamic model
			log.Info("Starting interactive setup")
			var model tea.Model
			if setupConfig != nil {
				// Use dynamic model with custom config
				model = setup.NewDynamicSetupModel(setupConfig, repo, settings, detectedPlatform, pluginBootstrap)
			} else {
				// Use default setup model (fallback to old behavior for now)
				model = setup.NewSetupModel(repo, settings, detectedPlatform)
			}

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
	cmd.Flags().StringP("config", "c", "", "Path to custom setup configuration file (YAML)")

	_ = viper.BindPFlag("verbose", cmd.Flags().Lookup("verbose"))
	_ = viper.BindPFlag("non-interactive", cmd.Flags().Lookup("non-interactive"))
	_ = viper.BindPFlag("config", cmd.Flags().Lookup("config"))

	return cmd
}
