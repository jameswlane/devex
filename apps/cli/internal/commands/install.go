package commands

import (
	"context"
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"

	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/help"
	"github.com/jameswlane/devex/apps/cli/internal/log"
	"github.com/jameswlane/devex/apps/cli/internal/tui"
	"github.com/jameswlane/devex/apps/cli/internal/types"
	"github.com/jameswlane/devex/apps/cli/internal/utils"
)

var tracer = otel.Tracer("devex/commands/install")

func init() {
	Register(NewInstallCmd)
}

func NewInstallCmd(repo types.Repository, settings config.CrossPlatformSettings) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "install [apps...]",
		Short: "Install development applications",
		Long: `Install applications with cross-platform package manager support.

The install command automates the installation of your development environment with:
  • Cross-platform package manager selection (APT, DNF, Pacman, Flatpak, Brew, etc.)
  • Dependency resolution and conflict handling
  • Post-installation configuration and shell setup
  • Theme application and desktop environment setup

Configuration hierarchy (highest to lowest priority):
  • Command-line flags
  • Environment variables (DEVEX_*)
  • Configuration files (~/.devex/config.yaml)
  • Default values

Configuration files are located in ~/.local/share/devex/config/:
  • applications.yaml - Development tools and applications
  • environment.yaml - Programming languages and fonts
  • desktop.yaml - Themes, extensions, and desktop settings
  • system.yaml - Git configuration and system settings`,
		Example: `  # Install default applications
  devex install

  # Install specific applications
  devex install docker git vscode

  # Install with categories
  devex install --categories development,databases

  # Dry run to preview changes
  devex install --dry-run

  # Install with verbose output
  devex install --verbose`,
		Args: cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			// Get configuration from Viper (respects hierarchy)
			verbose := viper.GetBool("verbose")
			dryRun := viper.GetBool("dry-run")
			categories := viper.GetStringSlice("categories")

			return executeInstall(ctx, args, categories, verbose, dryRun, repo, settings)
		},
		SilenceUsage: true, // Prevent usage spam on runtime errors
	}

	// Define command-specific flags
	cmd.Flags().StringSlice("categories", nil, "Install apps from specific categories")

	// Bind flags to Viper for hierarchical config
	_ = viper.BindPFlag("categories", cmd.Flags().Lookup("categories"))

	// Add contextual help integration
	AddContextualHelp(cmd, help.ContextCommand, "install")

	return cmd
}

// executeInstall implements the core installation logic with proper context handling
func executeInstall(ctx context.Context, apps []string, categories []string, verbose, dryRun bool, repo types.Repository, settings config.CrossPlatformSettings) error {
	_, span := tracer.Start(ctx, "install_command",
		trace.WithAttributes(
			attribute.StringSlice("apps", apps),
			attribute.StringSlice("categories", categories),
			attribute.Bool("verbose", verbose),
			attribute.Bool("dry_run", dryRun),
			attribute.String("platform", runtime.GOOS),
		),
	)
	defer span.End()

	// Note: Context will be properly propagated when TUI and other functions support it

	// Update settings with runtime configuration
	settings.Verbose = verbose

	log.Debug("Starting installation process",
		"verbose", verbose,
		"dry-run", dryRun,
		"apps", apps,
		"categories", categories,
	)

	// Validate inputs
	if err := validateInstallInputs(apps, categories); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Input validation failed")
		return fmt.Errorf("invalid inputs: %w", err)
	}

	// Determine what to install
	var appsToInstall []types.CrossPlatformApp
	switch {
	case len(apps) > 0:
		// Install specific apps requested - for now just use default apps as placeholder
		// TODO: Implement app lookup by name
		log.Info("Specific app installation requested", "apps", apps)
		log.Info("Currently installing default apps - specific app lookup will be implemented")
		appsToInstall = settings.GetDefaultApps()
	case len(categories) > 0:
		// Install apps from specific categories - for now just use default apps as placeholder
		// TODO: Implement category filtering
		log.Info("Category-based installation requested", "categories", categories)
		log.Info("Currently installing default apps - category filtering will be implemented")
		appsToInstall = settings.GetDefaultApps()
	default:
		// Install default apps
		appsToInstall = settings.GetDefaultApps()
	}

	// Handle dry run
	if dryRun {
		return previewInstallation(appsToInstall)
	}

	// Start TUI installation with cross-platform apps
	span.AddEvent("starting_installation", trace.WithAttributes(
		attribute.Int("app_count", len(appsToInstall)),
	))

	// Pass context to tui.StartInstallation for proper cancellation support
	if err := tui.StartInstallation(ctx, appsToInstall, repo, settings); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "Installation failed")
		return fmt.Errorf("installation failed: %w", err)
	}

	// Switch to zsh as the final step (after all installations are complete)
	log.Info("Switching to zsh shell as final step")
	// TODO: Update switchToZsh to accept context parameter
	if err := switchToZsh(); err != nil { //nolint:contextcheck
		log.Warn("Failed to switch to zsh shell", "error", err)
		log.Info("You can manually switch to zsh later with: chsh -s $(which zsh)")
	} else {
		log.Info("Successfully switched to zsh shell. Please restart your terminal or run 'exec zsh' to use the new shell.")
	}

	span.SetStatus(codes.Ok, "Installation completed successfully")
	return nil
}

// validateInstallInputs validates the installation inputs
func validateInstallInputs(apps []string, categories []string) error {
	// Apps and categories are mutually exclusive for clarity
	if len(apps) > 0 && len(categories) > 0 {
		return fmt.Errorf("cannot specify both specific apps and categories - choose one approach")
	}
	return nil
}

// previewInstallation shows what would be installed without executing
func previewInstallation(apps []types.CrossPlatformApp) error {
	log.Info("Dry run - showing what would be installed:")

	for _, app := range apps {
		log.Info("Would install", "app", app.Name, "category", app.Category)
	}

	log.Info("Total applications", "count", len(apps))
	log.Info("Use --verbose for detailed installation commands")

	return nil
}

// switchToZsh attempts to switch the user's shell to zsh
func switchToZsh() error {
	// Get the full path to zsh
	zshPath, err := utils.GetShellPath("zsh")
	if err != nil {
		return err
	}

	// Change the user's shell to zsh
	return utils.ChangeUserShell(zshPath)
}
