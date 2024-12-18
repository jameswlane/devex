package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/log"
	"github.com/samber/oops"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/jameswlane/devex/pkg/datastore"
	"github.com/jameswlane/devex/pkg/installers"
	"github.com/jameswlane/devex/pkg/types"
)

var rootCmd = &cobra.Command{
	Use:   "devex",
	Short: "DevEx CLI for setting up your development environment",
	Long:  "DevEx is a CLI tool that helps you install and configure your development environment easily.",
}

var (
	debug   bool
	homeDir string
)

func main() {
	setupConfig()

	if err := rootCmd.Execute(); err != nil {
		handleError("root command", err)
	}
}

func setupConfig() {
	viper.SetConfigType("yaml")

	localConfigPath := filepath.Join(homeDir, ".devex/config/config.yaml")
	defaultConfigPath := filepath.Join(homeDir, ".local/share/devex/config/config.yaml")

	if err := loadFirstAvailableConfig(localConfigPath, defaultConfigPath); err != nil {
		log.Warn(oops.In("config setup").With("context", "reading config file failed").Wrap(err))
	}
	viper.AutomaticEnv()
}

func loadFirstAvailableConfig(paths ...string) error {
	for _, path := range paths {
		log.Debug("Attempting to load config file", "path", path)
		if _, err := os.Stat(path); err == nil {
			viper.SetConfigFile(path)
			if err := viper.ReadInConfig(); err == nil {
				log.Info("Successfully loaded config file", "path", path)
				return nil
			}
			log.Warn("Error reading config file, proceeding with next", "path", path, "error", err)
		}
	}
	return fmt.Errorf("no valid config file found")
}

func loadCustomConfig(filename string) {
	viper.SetConfigFile(filename)
	if err := viper.MergeInConfig(); err != nil {
		log.Warn(oops.In("custom config load").With("filename", filename).Errorf("Could not read %s, proceeding with defaults", filename))
	} else {
		log.Debug("Loaded custom config file", "file", filename)
	}
}

func getHomeDir() (string, error) {
	if sudoUser := os.Getenv("SUDO_USER"); sudoUser != "" {
		return filepath.Join("/home", sudoUser), nil
	}
	return os.UserHomeDir()
}

func init() {
	var err error
	homeDir, err = getHomeDir()
	if err != nil {
		handleError("user directory", err)
	}

	installCmd := &cobra.Command{
		Use:   "install",
		Short: "Install development environment",
		Long:  "Install all necessary tools, programming languages, and databases for your development environment.",
		Run: func(cmd *cobra.Command, args []string) {
			// Initialize the database here, ensuring it stays open during execution
			db, err := datastore.InitDB(filepath.Join(homeDir, ".devex/installed_apps.db"))
			if err != nil {
				handleError("database initialization", err)
			}
			defer db.Close() // Close the database only after everything finishes

			loadConfigs()
			log.Info("Starting installation process...")

			var selectedLanguages, selectedDatabases []string

			selectedApps := getDefaultsFromConfig("apps")
			log.Info("Selected apps:", "apps", selectedApps)
			installApps(selectedApps, "apps", db)

			if viper.GetBool("default") || viper.GetBool("DEVEX_NONINTERACTIVE") {
				log.Info("Running with default settings")
				selectedLanguages = getDefaultsFromConfig("programming_languages")
				selectedDatabases = getDefaultsFromConfig("databases")
			} else {
				selectedLanguages = getUserSelections("programming_languages")
				selectedDatabases = getUserSelections("databases")
			}

			log.Info("Selected programming languages:", "languages", selectedLanguages)
			log.Info("Selected databases:", "databases", selectedDatabases)

			installApps(selectedLanguages, "programming_languages", db)
			installApps(selectedDatabases, "databases", db)
		},
	}

	bindFlag(installCmd, "dry-run", "Run in dry-run mode without making any changes")
	bindFlag(installCmd, "default", "Use default programming languages and databases")
	bindFlag(installCmd, "debug-delay", "Set delay in seconds for debug mode")

	rootCmd.PersistentFlags().BoolVar(&debug, "debug", false, "Enable debug mode with verbose logging")
	if err := viper.BindPFlag("debug", rootCmd.PersistentFlags().Lookup("debug")); err != nil {
		handleError("flag binding", err)
	}

	rootCmd.AddCommand(installCmd)
}

func loadConfigs() {
	configFiles := []string{
		"config/apps.yaml",
		"config/databases.yaml",
		"config/dock.yaml",
		"config/fonts.yaml",
		"config/git_config.yaml",
		"config/gnome_extensions.yaml",
		"config/gnome_settings.yaml",
		"config/optional_apps.yaml",
		"config/programming_languages.yaml",
		"config/themes.yaml",
	}

	for _, configFile := range configFiles {
		localPath := filepath.Join(homeDir, ".devex", configFile)
		fallbackPath := filepath.Join(homeDir, ".local/share/devex", configFile)

		if _, err := os.Stat(localPath); err == nil {
			log.Debug("Loading custom config file:", "path", localPath)
			loadCustomConfig(localPath)
		} else {
			log.Debug("Custom config not found, loading fallback:", "fallback", fallbackPath)
			loadCustomConfig(fallbackPath)
		}
	}
}

func getUserSelections(category string) []string {
	var selectedItems []string
	var options []huh.Option[string]

	if items, ok := viper.Get(category).([]any); ok {
		for _, item := range items {
			if itemMap, ok := item.(map[string]any); ok {
				option := huh.NewOption(itemMap["name"].(string), itemMap["name"].(string))
				if defaultFlag, ok := itemMap["default"].(bool); ok && defaultFlag {
					option = option.Selected(true)
				}
				options = append(options, option)
			}
		}
	}

	form := huh.NewForm(
		huh.NewGroup(
			huh.NewMultiSelect[string]().
				Title(fmt.Sprintf("Select %s to Install", category)).
				Options(options...).
				Value(&selectedItems),
		),
	)

	if err := form.Run(); err != nil {
		handleError("form execution", err)
	}

	return selectedItems
}

func installApps(selectedItems []string, category string, db *datastore.DB) {
	var appConfigs []types.AppConfig
	if err := viper.UnmarshalKey(category, &appConfigs); err != nil {
		log.Error("Failed to unmarshal apps for category", "category", category, "error", err)
		return
	}

	for _, itemName := range selectedItems {
		for _, appConfig := range appConfigs {
			if appConfig.Name == itemName {
				log.Info(fmt.Sprintf("Installing %s using method %s", appConfig.Name, appConfig.InstallMethod))

				app := types.AppConfig{
					Name:           appConfig.Name,
					Description:    appConfig.Description,
					Category:       category,
					InstallMethod:  appConfig.InstallMethod,
					InstallCommand: appConfig.InstallCommand,
					DownloadUrl:    appConfig.DownloadUrl,
					Dependencies:   appConfig.Dependencies,
				}

				if err := installers.InstallApp(app, viper.GetBool("dry-run"), db); err != nil {
					log.Error(oops.With("context", fmt.Sprintf("failed to install %s", appConfig.Name)).Wrap(err))
				}
			}
		}
	}
}

func getDefaultsFromConfig(category string) []string {
	var apps []map[string]any
	if err := viper.UnmarshalKey(category, &apps); err != nil {
		log.Error("Failed to unmarshal apps configuration", "category", category, "error", err)
		return nil
	}

	var defaults []string
	for _, app := range apps {
		if defaultFlag, ok := app["default"].(bool); ok && defaultFlag {
			if name, ok := app["name"].(string); ok {
				defaults = append(defaults, name)
			} else {
				log.Debug("App name missing or invalid", "app", app)
			}
		}
	}

	log.Debug("Default apps selected", "defaults", defaults)
	return defaults
}

func handleError(context string, err error) {
	if err != nil {
		log.Error(oops.In(context).Wrap(err))
		os.Exit(1)
	}
}

func bindFlag(cmd *cobra.Command, flag string, description string) {
	cmd.Flags().Bool(flag, false, description)
	if err := viper.BindPFlag(flag, cmd.Flags().Lookup(flag)); err != nil {
		handleError("flag binding", err)
	}
}
