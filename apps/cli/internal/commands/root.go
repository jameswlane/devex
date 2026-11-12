package commands

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	devexpaths "github.com/jameswlane/devex/packages/plugin-sdk/paths"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"golang.org/x/term"

	"github.com/jameswlane/devex/apps/cli/internal/agentapi"
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

Package manager support:
  • Stable: APT (Debian/Ubuntu)
  • Experimental: DNF (Fedora/RHEL), Pacman (Arch), Flatpak, Snap, Homebrew
  • Planned (no plugin): Mac App Store, winget, Chocolatey, Scoop

Support matrix and limitations: https://docs.devex.sh/platforms

All installations are configurable via YAML files in $DEVEX_DATA_DIR/config/`,
		Version:     version,
		Annotations: map[string]string{catalogAnnotation: catalogRequired},
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			// Presentation routes must not initialize plugins or update checks.
			if cmd.Name() == "help" || cmd.Name() == "completion" || cmd.Name() == "__complete" || cmd.Name() == "__completeNoDesc" {
				return nil
			}
			if err := initializeConfig(cmd); err != nil {
				if f := cmd.Flags().Lookup("format"); f != nil && f.Value.String() == "json" {
					return writeAPIJSON(cmd, nil, agentapi.APIError(err))
				}
				return err
			}
			if viper.GetInt("install-concurrency") < 1 {
				return fmt.Errorf("install-concurrency must be at least 1")
			}
			if cmd.HasParent() && requiresCatalog(cmd) {
				if err := loadCommandCatalog(cmd, settings); err != nil {
					return err
				}
			}
			// Adding configuration never requires a plugin runtime. Keep failed
			// validation and cancelled previews free of plugin-state mutations.
			if cmd.Name() == "add" {
				return nil
			}
			// Even offline bootstrap can create plugin state or execute local plugins.
			// Preview commands resolve their plans without initializing the runtime.
			if viper.GetBool("dry-run") {
				return nil
			}
			if readAPICommand(cmd) {
				// JSON status probes plugins that are already installed; it never
				// downloads plugins or creates plugin state.
				if cmd.Name() == "status" {
					return runtime.discoverLocalPlugins(cmd.Context())
				}
				return nil
			}

			notification = startUpdateNotification(cmd, version)

			// Install resolves applications before initializing plugins so invalid input
			// fails without downloads; bare devex and setup own their setup flow.
			cmdName := cmd.Name()
			for parent := cmd.Parent(); parent != nil; parent = parent.Parent() {
				// Dotfiles deployment never runs plugins.
				if parent.Name() == "plugin" || parent.Name() == "runs" || parent.Name() == "dotfiles" {
					return nil
				}
				// Profiles manage configuration files only; they never need, download
				// or run plugins, even before invalid input is rejected.
				if parent.Name() == "profile" && parent.Parent() == cmd.Root() {
					return nil
				}
			}
			if cmd.Parent() == nil || (cmdName == "install" && cmd.Flags().Lookup("template").Value.String() == "") || cmdName == "update" || cmdName == "upgrade" || cmdName == "setup" || (cmd.Parent() != nil && cmd.Parent().Name() == "setup") {
				log.Debug("Deferring plugin initialization to command", "command", cmdName)
				return nil
			}

			if cmdName == "self-update" || cmdName == "paths" || cmdName == "help" {
				return nil
			}
			// Detection installs and runs only the stack detector plugin.
			if detectCommand(cmd) {
				return nil
			}
			// Generating a dev container never runs plugins.
			if cmd.Parent() != nil && cmd.Parent().Name() == "devcontainer" {
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

			// Only the first-run setup flow needs the root catalog opt-in.
			if requiresCatalog(cmd) {
				if err := loadCommandCatalog(cmd, settings); err != nil {
					return err
				}
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
	cmd.AddCommand(newPackageMaintenanceCmd("update", settings, runtime.getPluginBootstrap))
	cmd.AddCommand(newPackageMaintenanceCmd("upgrade", settings, runtime.getPluginBootstrap))
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
	runsCmd := NewRunsCmd()
	runsCmd.AddCommand(newRunsResumeCmd(repo, settings, runtime.getPluginBootstrap))
	cmd.AddCommand(runsCmd)
	cmd.AddCommand(NewCacheCmd(repo, settings))
	cmd.AddCommand(NewDetectCmd(repo, settings))
	cmd.AddCommand(NewListCmd(repo, settings))
	cmd.AddCommand(NewProfileCmd(repo, settings))
	cmd.AddCommand(NewDevcontainerCmd(repo, settings))
	cmd.AddCommand(NewDotfilesCmd(repo, settings))
	cmd.AddCommand(NewPlatformCmd(repo, settings))
	cmd.AddCommand(NewShellCmd(repo, settings))
	cmd.AddCommand(NewSystemCmd(settings))
	cmd.AddCommand(NewCompletionCmd())
	cmd.AddCommand(bootstrap.NewPluginManagementCmd())
	cmd.SetHelpCommand(NewHelpCmd(repo, settings))

	// Define persistent flags (available to all subcommands)
	cmd.PersistentFlags().Duration("apt-lists-max-age", 15*time.Minute, "Reuse populated APT lists younger than this duration (0 always refreshes)")
	_ = viper.BindPFlag("apt-lists-max-age", cmd.PersistentFlags().Lookup("apt-lists-max-age"))
	cmd.PersistentFlags().Int("install-concurrency", 1, "Maximum concurrent installations (at least 1)")
	_ = viper.BindPFlag("install-concurrency", cmd.PersistentFlags().Lookup("install-concurrency"))
	cmd.PersistentFlags().BoolP("verbose", "v", false, "Enable debug logging unless log-level is configured")
	cmd.PersistentFlags().BoolP("quiet", "q", false, "Suppress routine logs and notifications; preserve errors, results and consent prompts")
	cmd.PersistentFlags().Bool("dry-run", false, "Show what would be done without executing")
	cmd.PersistentFlags().String("config", "", "Config file (default: $DEVEX_CONFIG_DIR/config.yaml)")
	cmd.PersistentFlags().String("log-level", "info", "Log level (debug, info, warn, error)")
	cmd.PersistentFlags().Bool("skip-plugin-download", false, "Skip automatic plugin downloads")
	cmd.PersistentFlags().Bool("non-interactive", false, "Run without interactive prompts")
	cmd.PersistentFlags().Bool("offline", false, "Use installed plugins and cached pinned installation artifacts only")
	cmd.PersistentFlags().Bool("no-cache", false, "Bypass installation artifact and validation caches")
	_ = viper.BindPFlag("no-cache", cmd.PersistentFlags().Lookup("no-cache"))

	// Bind persistent flags to viper for global access
	_ = viper.BindPFlag("quiet", cmd.PersistentFlags().Lookup("quiet"))
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
	// Preserve legacy debug environment aliases below the supported verbose key.
	_ = viper.BindEnv("verbose", "DEVEX_VERBOSE", "DEVEX_DEBUG", "DEBUG")
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
	if err := validateViperConfiguration(); err != nil {
		return err
	}
	level := viper.GetString("log-level")
	// An explicitly configured level wins over the verbose shorthand.
	if !cmd.Flags().Changed("log-level") && !cmd.Root().PersistentFlags().Changed("log-level") && os.Getenv("DEVEX_LOG_LEVEL") == "" && !viper.InConfig("log-level") && viper.GetBool("verbose") {
		level = "debug"
	}
	log.Configure(level, viper.GetBool("quiet"))
	return nil
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
	viper.SetDefault("install-concurrency", 1)
	viper.SetDefault("verbose", false)
	viper.SetDefault("quiet", false)
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

// discoverLocalPlugins registers installers for plugins that are already
// installed, with downloads disabled, for read-only inspection commands.
func (r *rootRuntime) discoverLocalPlugins(ctx context.Context) error {
	pluginBootstrap, err := bootstrap.NewPluginBootstrap(true)
	if err != nil {
		log.Debug("Local plugin discovery unavailable", "error", err)
		return nil
	}
	if err := pluginBootstrap.Initialize(ctx); err != nil {
		log.Debug("Local plugin discovery failed", "error", err)
		return nil
	}
	r.pluginBootstrap = pluginBootstrap
	installers.InitializeWithPluginBootstrap(pluginBootstrap)
	return nil
}

func (r *rootRuntime) getPluginBootstrap() *bootstrap.PluginBootstrap {
	return r.pluginBootstrap
}

func readAPICommand(cmd *cobra.Command) bool {
	switch cmd.Name() {
	case "platform":
		return true
	case "list":
		// Interactive list keeps first-run bootstrap; JSON inspection does not.
		format, _ := cmd.Flags().GetString("format")
		return format == "json"
	case "status":
		// Interactive status bootstraps plugins to probe installed state; only
		// the JSON inspection contract reports unavailable plugins as unknown.
		fix, _ := cmd.Flags().GetBool("fix")
		template, _ := cmd.Flags().GetString("template")
		format, _ := cmd.Flags().GetString("format")
		return !fix && template == "" && format == "json"
	case "effective", "show", "validate":
		return cmd.Parent() != nil && cmd.Parent().Name() == "config"
	}
	return false
}
