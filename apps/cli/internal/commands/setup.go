package commands

import (
	"context"
	"fmt"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/jameswlane/devex/apps/cli/internal/bootstrap"
	"github.com/jameswlane/devex/apps/cli/internal/commands/setup"
	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/installers"
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

  # Choose a shell for automated setup
  devex setup --non-interactive --shell=fish

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
				if viper.IsSet("applications") {
					return executeInstallWithOutput(ctx, nil, nil, verbose, viper.GetBool("dry-run"), repo, settings, cmd.OutOrStdout(), nil)
				}
				return setup.RunAutomatedSetup(ctx, repo, settings)
			}

			// The default model initializes plugins at its plugin step and reuses them
			// for finalization. Custom flows need their bootstrap before constructing actions.
			log.Info("Starting interactive setup")
			var model tea.Model
			if setupConfig != nil {
				pluginBootstrap, err := bootstrap.NewPluginBootstrap(viper.GetBool("skip-plugin-download") || viper.GetBool("offline"))
				if err != nil {
					return fmt.Errorf("failed to initialize plugin bootstrap: %w", err)
				}
				if err := pluginBootstrap.Initialize(ctx); err != nil {
					log.Warn("Failed to initialize some plugins", "error", err)
				} else {
					installers.InitializeWithPluginBootstrap(pluginBootstrap)
				}
				// Use dynamic model with custom config
				model = setup.NewDynamicSetupModel(setupConfig, repo, settings, detectedPlatform, pluginBootstrap)
			} else {
				// Use default setup model (fallback to old behavior for now)
				model = setup.NewSetupModel(repo, settings, detectedPlatform)
			}

			if setupModel, ok := model.(*setup.SetupModel); ok {
				setupModel.SetContext(ctx)
			}
			p := tea.NewProgram(model, tea.WithAltScreen(), tea.WithContext(ctx), tea.WithInput(cmd.InOrStdin()), tea.WithOutput(cmd.OutOrStdout()))
			finalModel, err := p.Run()
			if err != nil {
				return fmt.Errorf("setup failed: %w", err)
			}

			// Check if setup was successful
			if m, ok := finalModel.(*setup.SetupModel); ok {
				if err := m.CompletionError(); err != nil {
					return err
				}
			}

			log.Info("Setup completed successfully")
			return nil
		},
	}

	cmd.Flags().BoolP("verbose", "v", false, "Enable verbose output")
	cmd.Flags().Bool("non-interactive", false, "Run in non-interactive mode (automated)")
	cmd.Flags().Bool("no-defaults", false, "Skip the default tools; install only what you select")
	cmd.Flags().String("shell", "", "Shell for automated setup (bash, zsh, fish); config key: setup.shell; defaults to the current shell")
	cmd.Flags().StringP("config", "c", "", "Path to custom setup configuration file (YAML)")

	_ = viper.BindPFlag("verbose", cmd.Flags().Lookup("verbose"))
	_ = viper.BindPFlag("non-interactive", cmd.Flags().Lookup("non-interactive"))
	_ = viper.BindPFlag("no-defaults", cmd.Flags().Lookup("no-defaults"))
	_ = viper.BindPFlag("setup.shell", cmd.Flags().Lookup("shell"))
	_ = viper.BindPFlag("config", cmd.Flags().Lookup("config"))

	return cmd
}
