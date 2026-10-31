package commands

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	devexpaths "github.com/jameswlane/devex/packages/plugin-sdk/paths"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"golang.org/x/term"

	"github.com/jameswlane/devex/apps/cli/internal/bootstrap"
	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/installers"
	"github.com/jameswlane/devex/apps/cli/internal/interaction"
	"github.com/jameswlane/devex/apps/cli/internal/log"
	"github.com/jameswlane/devex/apps/cli/internal/selfupdate"
	"github.com/jameswlane/devex/apps/cli/internal/state"
	"github.com/jameswlane/devex/apps/cli/internal/types"
)

type rootRuntime struct {
	pluginBootstrap *bootstrap.PluginBootstrap
}

type rootOptions struct {
	runSetup    func(*cobra.Command, []string) error
	hasTerminal func(*cobra.Command) bool
}

func NewRootCmd(version string, repo types.Repository, settings config.CrossPlatformSettings) *cobra.Command {
	return newRootCmd(version, repo, settings, rootOptions{})
}

func newRootCmd(version string, repo types.Repository, settings config.CrossPlatformSettings, options rootOptions) *cobra.Command {
	runtime := &rootRuntime{}
	setupCmd := NewSetupCmd(repo, settings)
	if options.runSetup == nil {
		options.runSetup = setupCmd.RunE
	}
	if options.hasTerminal == nil {
		options.hasTerminal = hasInteractiveTerminal
	}
	var notification *selfupdate.Notification
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

All installations are configurable via YAML files in $DEVEX_DATA_DIR/config/`,
		Version: version,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			if err := initializeConfig(cmd); err != nil {
				return err
			}
			// Even offline bootstrap can create plugin state or execute local plugins.
			// Preview commands resolve their plans without initializing the runtime.
			if viper.GetBool("dry-run") {
				return nil
			}

			notification = startUpdateNotification(cmd, version)

			// Install resolves applications before initializing plugins so invalid input
			// fails without downloads; bare devex and setup own their setup flow.
			cmdName := cmd.Name()
			for parent := cmd.Parent(); parent != nil; parent = parent.Parent() {
				if parent.Name() == "plugin" {
					return nil
				}
			}
			if cmd.Parent() == nil || (cmdName == "install" && cmd.Flags().Lookup("template").Value.String() == "") || cmdName == "setup" || (cmd.Parent() != nil && cmd.Parent().Name() == "setup") {
				log.Debug("Deferring plugin initialization to command", "command", cmdName)
				return nil
			}

			if cmdName == "self-update" || cmdName == "paths" || cmdName == "help" {
				return nil
			}
			return runtime.initializePluginSystem(cmd.Context())
		},
		PersistentPostRun: func(cmd *cobra.Command, args []string) {
			if message := notification.Finish(); message != "" {
				cmd.PrintErrln(message)
			}
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if !interaction.IsInteractiveMode() || !options.hasTerminal(cmd) {
				return printRootNextSteps(cmd)
			}

			// Existing tracked applications indicate an environment already managed by DevEx.
			apps, err := repo.ListApps()
			if err != nil {
				return fmt.Errorf("check setup state: %w; run 'devex status' to inspect your environment", err)
			}
			if len(apps) > 0 {
				return printRootNextSteps(cmd)
			}

			// Invoke the handler directly: Execute on a child restarts the root command.
			return options.runSetup(cmd, args)
		},
		SilenceUsage: true, // Prevent usage spam on runtime errors
	}

	cmd.AddCommand(NewSelfUpdateCmd(version, nil))
	cmd.AddCommand(newPathsCmd())

	// Register other commands
	cmd.AddCommand(setupCmd)
	cmd.AddCommand(NewInstallCmdWithPluginProvider(repo, settings, runtime.getPluginBootstrap))
	cmd.AddCommand(NewUninstallCmd(repo, settings))
	cmd.AddCommand(NewRollbackCmd(repo, settings))
	cmd.AddCommand(NewStatusCmdWithPluginProvider(repo, settings, runtime.getPluginBootstrap))
	cmd.AddCommand(NewMaintainCmdWithPluginProvider(repo, settings, runtime.getPluginBootstrap))
	cmd.AddCommand(NewInitCmd(repo, settings))
	cmd.AddCommand(NewAddCmd(repo, settings))
	cmd.AddCommand(NewRemoveCmdWithPluginProvider(repo, settings, runtime.getPluginBootstrap))
	cmd.AddCommand(NewConfigCmd(repo, settings))
	cmd.AddCommand(NewUndoCmd(repo, settings))
	cmd.AddCommand(NewRecoveryCmd(repo, settings))
	cmd.AddCommand(NewTemplateCmd(repo, settings, runtime.getPluginBootstrap))
	cmd.AddCommand(NewRunsCmd())
	cmd.AddCommand(NewCacheCmd(repo, settings))
	cmd.AddCommand(NewDetectCmd(repo, settings))
	cmd.AddCommand(NewListCmd(repo, settings))
	cmd.AddCommand(NewShellCmd(repo, settings))
	cmd.AddCommand(NewSystemCmd(settings))
	cmd.AddCommand(NewCompletionCmd())
	cmd.AddCommand(bootstrap.NewPluginManagementCmd())
	cmd.SetHelpCommand(NewHelpCmd(repo, settings))

	// Define persistent flags (available to all subcommands)
	cmd.PersistentFlags().BoolP("verbose", "v", false, "Enable verbose output")
	cmd.PersistentFlags().Bool("dry-run", false, "Show what would be done without executing")
	cmd.PersistentFlags().String("config", "", "Config file (default: $DEVEX_CONFIG_DIR/config.yaml)")
	cmd.PersistentFlags().String("log-level", "info", "Log level (debug, info, warn, error)")
	cmd.PersistentFlags().Bool("skip-plugin-download", false, "Skip automatic plugin downloads")
	cmd.PersistentFlags().Bool("non-interactive", false, "Run without interactive prompts")
	cmd.PersistentFlags().Bool("offline", false, "Run in offline mode (no plugin downloads)")

	// Bind persistent flags to viper for global access
	_ = viper.BindPFlag("verbose", cmd.PersistentFlags().Lookup("verbose"))
	_ = viper.BindPFlag("dry-run", cmd.PersistentFlags().Lookup("dry-run"))
	_ = viper.BindPFlag("config", cmd.PersistentFlags().Lookup("config"))
	_ = viper.BindPFlag("log-level", cmd.PersistentFlags().Lookup("log-level"))
	_ = viper.BindPFlag("skip-plugin-download", cmd.PersistentFlags().Lookup("skip-plugin-download"))
	_ = viper.BindPFlag("offline", cmd.PersistentFlags().Lookup("offline"))

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
	_ = viper.BindEnv("non-interactive", "DEVEX_NONINTERACTIVE", "DEVEX_NON_INTERACTIVE")
	_ = viper.BindEnv(state.DBPathConfigKey, state.DBPathEnvVar, state.DBPathAutoEnvVar)

	if err := loadCLIConfig(viper.GetString("config")); err != nil {
		return err
	}

	// 5. Bind all command flags to Viper for hierarchical configuration
	if err := viper.BindPFlags(cmd.Flags()); err != nil {
		return err
	}

	// 6. Validate configuration after all sources are loaded
	return validateViperConfiguration()
}

// loadCLIConfig keeps file loading separate from application resolution.
func loadCLIConfig(cfgFile string) error {
	// 3. Configuration file paths (in order of precedence)
	if cfgFile != "" {
		// Use config file from the flag
		viper.SetConfigFile(cfgFile)
	} else {
		// Look for config in standard locations
		home, err := devexHomeDir()
		if err != nil {
			return err
		}

		// Search config in home directory with name "config" (without extension)
		viper.AddConfigPath(".")                                                                                                            // Current directory
		viper.AddConfigPath(filepath.Dir(devexpaths.ForHome(home).ReadPath(filepath.Join(devexpaths.ForHome(home).Config, "config.yaml")))) // User override
		viper.AddConfigPath(devexpaths.ForHome(home).Defaults())                                                                            // Default location
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

	return nil
}

// setConfigurationDefaults sets default values for configuration options
func setConfigurationDefaults() {
	// Core application defaults
	viper.SetDefault("verbose", false)
	viper.SetDefault("dry-run", false)
	viper.SetDefault("log-level", "info")

	// Configuration file defaults
	viper.SetDefault("config", "")
	viper.SetDefault(state.DBPathConfigKey, "")

	// Command-specific defaults
	viper.SetDefault("categories", []string{})
	viper.SetDefault("non-interactive", false)
	viper.SetDefault("skip-plugin-download", false)
	viper.SetDefault("offline", false)
	viper.SetDefault("update_check", true)

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

// Require both streams to be terminals so redirected stdin or output never starts a TUI.
func hasInteractiveTerminal(cmd *cobra.Command) bool {
	input, inputOK := cmd.InOrStdin().(*os.File)
	output, outputOK := cmd.OutOrStdout().(*os.File)
	return inputOK && outputOK && term.IsTerminal(int(input.Fd())) && term.IsTerminal(int(output.Fd()))
}

func printRootNextSteps(cmd *cobra.Command) error {
	_, err := fmt.Fprintln(cmd.OutOrStdout(), `DevEx helps set up and manage your development environment.

Next steps:
  devex setup     Start guided setup in an interactive terminal
  devex install   Install configured applications
  devex status    Inspect your environment
  devex --help    Show all commands`)
	return err
}

func (r *rootRuntime) initializePluginSystem(ctx context.Context) error {
	var err error

	// Skip plugin system in offline mode or when explicitly disabled
	shouldSkipDownload := viper.GetBool("skip-plugin-download") || viper.GetBool("offline")

	r.pluginBootstrap, err = bootstrap.NewPluginBootstrap(shouldSkipDownload)
	if err != nil {
		log.Warn("Failed to initialize plugin system", "error", err)
		return nil // Don't fail the entire CLI
	}

	if err := r.pluginBootstrap.Initialize(ctx); err != nil {
		log.Warn("Failed to bootstrap plugins", "error", err)
		return nil // Don't fail the entire CLI
	}

	installers.InitializeWithPluginBootstrap(r.pluginBootstrap)

	// Show platform info in verbose mode
	if viper.GetBool("verbose") {
		platform := r.pluginBootstrap.GetPlatform()
		log.Info("Platform detected", "platform", platform.String())
		log.Info("Available package managers", "managers", platform.PackageManagers)
	}

	return nil
}

func (r *rootRuntime) getPluginBootstrap() *bootstrap.PluginBootstrap {
	return r.pluginBootstrap
}
