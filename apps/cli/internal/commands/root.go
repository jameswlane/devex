package commands

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/jameswlane/devex/apps/cli/internal/bootstrap"
	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/log"
	"github.com/jameswlane/devex/apps/cli/internal/types"
)

var (
	pluginBootstrap    *bootstrap.PluginBootstrap
	skipPluginDownload bool
	offlineMode        bool
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
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if err := initializeConfig(cmd); err != nil {
				return err
			}

			// Skip plugin system initialization for setup command to prevent premature downloads
			// Plugins will be downloaded during the setup flow itself when user confirms
			cmdName := cmd.Name()
			if cmdName == "setup" || (cmd.Parent() != nil && cmd.Parent().Name() == "setup") {
				log.Debug("Skipping plugin system initialization for setup command")
				return nil
			}

			return initializePluginSystem(cmd.Context())
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			// Don't auto-run setup during tests
			if isRunningInTest() {
				return cmd.Usage()
			}

			// If no subcommand is provided, run the guided setup
			setupCmd := NewSetupCmd(repo, settings)
			setupCmd.SetArgs(args)
			setupCmd.SetContext(ctx)
			return setupCmd.Execute()
		},
		SilenceUsage: true, // Prevent usage spam on runtime errors
	}

	// Register other commands
	cmd.AddCommand(NewSetupCmd(repo, settings))
	cmd.AddCommand(NewInstallCmd(repo, settings))
	cmd.AddCommand(NewUninstallCmd(repo, settings))
	cmd.AddCommand(NewRollbackCmd(repo, settings))
	cmd.AddCommand(NewStatusCmd(repo, settings))
	cmd.AddCommand(NewInitCmd(repo, settings))
	cmd.AddCommand(NewAddCmd(repo, settings))
	cmd.AddCommand(NewRemoveCmd(repo, settings))
	cmd.AddCommand(NewConfigCmd(repo, settings))
	cmd.AddCommand(NewUndoCmd(repo, settings))
	cmd.AddCommand(NewRecoveryCmd(repo, settings))
	cmd.AddCommand(NewTemplateCmd(repo, settings))
	cmd.AddCommand(NewCacheCmd(repo, settings))
	cmd.AddCommand(NewDetectCmd(repo, settings))
	cmd.AddCommand(NewListCmd(repo, settings))
	cmd.AddCommand(NewShellCmd(repo, settings))
	cmd.AddCommand(NewSystemCmd(settings))
	cmd.AddCommand(NewCompletionCmd())
	cmd.AddCommand(NewHelpCmd(repo, settings))

	// Define persistent flags (available to all subcommands)
	cmd.PersistentFlags().BoolP("verbose", "v", false, "Enable verbose output")
	cmd.PersistentFlags().Bool("dry-run", false, "Show what would be done without executing")
	cmd.PersistentFlags().String("config", "", "Config file (default: ~/.devex/config.yaml)")
	cmd.PersistentFlags().String("log-level", "info", "Log level (debug, info, warn, error)")
	cmd.PersistentFlags().BoolVar(&skipPluginDownload, "skip-plugin-download", false, "Skip automatic plugin downloads")
	cmd.PersistentFlags().BoolVar(&offlineMode, "offline", false, "Run in offline mode (no plugin downloads)")

	// Bind persistent flags to viper for global access
	_ = viper.BindPFlag("verbose", cmd.PersistentFlags().Lookup("verbose"))
	_ = viper.BindPFlag("dry-run", cmd.PersistentFlags().Lookup("dry-run"))
	_ = viper.BindPFlag("config", cmd.PersistentFlags().Lookup("config"))
	_ = viper.BindPFlag("log-level", cmd.PersistentFlags().Lookup("log-level"))

	return cmd
}

// initializeConfig implements 12-Factor configuration hierarchy:
// 1. Command-line flags (highest priority)
// 2. Environment variables
// 3. Configuration files
// 4. Default values (lowest priority)
func initializeConfig(cmd *cobra.Command) error {
	// 1. Set configuration defaults
	setConfigurationDefaults()

	// 2. Environment variables with DEVEX prefix
	viper.SetEnvPrefix("DEVEX")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_", "-", "_"))
	viper.AutomaticEnv()

	// 3. Configuration file paths (in order of precedence)
	cfgFile := viper.GetString("config")
	if cfgFile != "" {
		// Use config file from the flag
		viper.SetConfigFile(cfgFile)
	} else {
		// Look for config in standard locations
		home, err := os.UserHomeDir()
		if err != nil {
			return err
		}

		// Search config in home directory with name "config" (without extension)
		viper.AddConfigPath(".")                                              // Current directory
		viper.AddConfigPath(filepath.Join(home, ".devex"))                    // User override
		viper.AddConfigPath(filepath.Join(home, ".local/share/devex/config")) // Default location
		viper.SetConfigName("config")
		viper.SetConfigType("yaml")
	}

	// 4. Read config file (ignore if not found)
	if err := viper.ReadInConfig(); err != nil {
		var configFileNotFoundError viper.ConfigFileNotFoundError
		if !errors.As(err, &configFileNotFoundError) {
			return err
		}
		// Config file not found is acceptable - we'll use defaults
		log.Debug("Config file not found, using defaults")
	} else {
		log.Debug("Using config file", "path", viper.ConfigFileUsed())
	}

	// 5. Bind all command flags to Viper for hierarchical configuration
	if err := viper.BindPFlags(cmd.Flags()); err != nil {
		return err
	}

	// 6. Validate configuration after all sources are loaded
	return validateViperConfiguration()
}

// setConfigurationDefaults sets default values for configuration options
func setConfigurationDefaults() {
	// Core application defaults
	viper.SetDefault("verbose", false)
	viper.SetDefault("dry-run", false)
	viper.SetDefault("log-level", "info")

	// Configuration file defaults
	viper.SetDefault("config", "")

	// Command-specific defaults
	viper.SetDefault("categories", []string{})
	viper.SetDefault("non-interactive", false)

	log.Debug("Configuration defaults set")
}

// validateViperConfiguration validates the loaded configuration for consistency and correctness
func validateViperConfiguration() error {
	// Validate log level
	logLevel := viper.GetString("log-level")
	validLogLevels := []string{"debug", "info", "warn", "error"}
	if !contains(validLogLevels, logLevel) {
		return fmt.Errorf("invalid log-level '%s' - must be one of: %s",
			logLevel, strings.Join(validLogLevels, ", "))
	}

	// Validate categories if provided
	categories := viper.GetStringSlice("categories")
	if len(categories) > 0 {
		validCategories := []string{"development", "databases", "desktop", "terminal", "optional"}
		for _, category := range categories {
			if !contains(validCategories, category) {
				return fmt.Errorf("invalid category '%s' - must be one of: %s",
					category, strings.Join(validCategories, ", "))
			}
		}
	}

	log.Debug("Configuration validation passed")
	return nil
}

// contains checks if a slice contains a specific string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
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

// initializePluginSystem initializes the plugin system
func initializePluginSystem(ctx context.Context) error {
	var err error

	// Skip plugin system in offline mode or when explicitly disabled
	shouldSkipDownload := skipPluginDownload || offlineMode

	pluginBootstrap, err = bootstrap.NewPluginBootstrap(shouldSkipDownload)
	if err != nil {
		log.Warn("Failed to initialize plugin system", "error", err)
		return nil // Don't fail the entire CLI
	}

	if err := pluginBootstrap.Initialize(ctx); err != nil {
		log.Warn("Failed to bootstrap plugins", "error", err)
		return nil // Don't fail the entire CLI
	}

	// TODO: Register plugin commands with root command
	// This would need to be done differently since we're inside NewRootCmd
	// For now, plugins will be accessed via the plugin subcommand

	// Show platform info in verbose mode
	if viper.GetBool("verbose") {
		platform := pluginBootstrap.GetPlatform()
		log.Info("Platform detected", "platform", platform.String())
		log.Info("Available package managers", "managers", platform.PackageManagers)
	}

	return nil
}

// GetPluginBootstrap returns the plugin bootstrap instance
func GetPluginBootstrap() *bootstrap.PluginBootstrap {
	return pluginBootstrap
}
