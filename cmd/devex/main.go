package main

import (
	"fmt"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/log"
	"github.com/jameswlane/devex/pkg/datastore"
	"github.com/jameswlane/devex/pkg/installers"
	"github.com/jameswlane/devex/pkg/types"
	"github.com/samber/oops"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"os"
	"path/filepath"
)

var rootCmd = &cobra.Command{
	Use:   "devex",
	Short: "DevEx CLI for setting up your development environment",
	Long:  "DevEx is a CLI tool that helps you install and configure your development environment easily.",
}

var debug bool

func main() {
	setupConfig()

	if err := rootCmd.Execute(); err != nil {
		log.Error(oops.In("root command").With("context", "root command execution failed").Wrap(err))
		os.Exit(1)
	}
}

func setupConfig() {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")

	homeDir, err := os.UserHomeDir()
	if err == nil {
		viper.AddConfigPath(filepath.Join(homeDir, ".devex"))
		viper.AddConfigPath(filepath.Join(homeDir, ".devex/config"))
	}

	viper.AddConfigPath("./config")
	viper.AutomaticEnv()

	if err := viper.ReadInConfig(); err != nil {
		log.Warn(oops.In("config setup").With("context", "reading config file failed").Wrap(err))
	}
}

func loadCustomConfig(filename string) {
	if _, err := os.Stat(filename); err == nil {
		viper.SetConfigFile(filename)
		if err := viper.MergeInConfig(); err != nil {
			log.Warn(oops.In("custom config load").With("filename", filename).Errorf("Could not read %s, proceeding with defaults", filename))
		}
	} else if debug {
		log.Debug(fmt.Sprintf("Config file %s not found, skipping", filename))
	}
}

func init() {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		log.Error(oops.In("user directory").With("context", "failed to get user home directory").Wrap(err))
		os.Exit(1)
	}

	db, err := datastore.InitDB(filepath.Join(homeDir, ".devex/installed_apps.db"))
	if err != nil {
		log.Error(oops.In("database initialization").With("context", "failed to initialize database").Wrap(err))
		os.Exit(1)
	}
	defer db.Close()

	var installCmd = &cobra.Command{
		Use:   "install",
		Short: "Install development environment",
		Long:  "Install all necessary tools, programming languages, and databases for your development environment.",
		Run: func(cmd *cobra.Command, args []string) {
			loadConfigs()

			var selectedLanguages, selectedDatabases []string
			if viper.GetBool("default") {
				log.Info("Running with default settings")
				selectedLanguages = getDefaultsFromConfig("programming_languages")
				selectedDatabases = getDefaultsFromConfig("databases")
			} else if viper.GetBool("DEVEX_NONINTERACTIVE") {
				log.Info("Running in non-interactive mode, using default settings")
				selectedLanguages = getDefaultsFromConfig("programming_languages")
				selectedDatabases = getDefaultsFromConfig("databases")
			} else {
				selectedLanguages = getUserSelections("programming_languages")
				selectedDatabases = getUserSelections("databases")
			}

			log.Info("Selected programming languages: ", "languages", selectedLanguages)
			log.Info("Selected databases: ", "databases", selectedDatabases)

			installApps(selectedLanguages, "programming_languages", db)
			installApps(selectedDatabases, "databases", db)
		},
	}

	installCmd.Flags().Bool("dry-run", false, "Run in dry-run mode without making any changes")
	if err := viper.BindPFlag("dry-run", installCmd.Flags().Lookup("dry-run")); err != nil {
		log.Error(oops.In("flag binding").With("context", "failed to bind dry-run flag").Wrap(err))
		os.Exit(1)
	}

	installCmd.Flags().Bool("default", false, "Use default programming languages and databases")
	if err := viper.BindPFlag("default", installCmd.Flags().Lookup("default")); err != nil {
		log.Error(oops.In("flag binding").With("context", "failed to bind default flag").Wrap(err))
		os.Exit(1)
	}

	installCmd.Flags().Int("debug-delay", 0, "Set delay in seconds for debug mode")
	if err := viper.BindPFlag("debug-delay", installCmd.Flags().Lookup("debug-delay")); err != nil {
		log.Error(oops.In("flag binding").With("context", "failed to bind debug-delay flag").Wrap(err))
		os.Exit(1)
	}

	rootCmd.PersistentFlags().BoolVar(&debug, "debug", false, "Enable debug mode with verbose logging")
	if err := viper.BindPFlag("debug", rootCmd.PersistentFlags().Lookup("debug")); err != nil {
		log.Error(oops.In("flag binding").With("context", "failed to bind debug flag").Wrap(err))
		os.Exit(1)
	}

	rootCmd.AddCommand(installCmd)
}

func loadConfigs() {
	homeDir, err := os.UserHomeDir()
	if err == nil {
		loadCustomConfig(filepath.Join(homeDir, ".devex/config/databases.yaml"))
		loadCustomConfig(filepath.Join(homeDir, ".devex/config/programming_languages.yaml"))
	} else {
		log.Error(oops.In("user directory").With("context", "failed to get user home directory").Wrap(err))
		os.Exit(1)
	}

	loadCustomConfig("./config/databases.yaml")
	loadCustomConfig("./config/programming_languages.yaml")
}

func getUserSelections(category string) []string {
	var selectedItems []string
	var options []huh.Option[string]

	if items, ok := viper.Get(category).([]interface{}); ok {
		for _, item := range items {
			if itemMap, ok := item.(map[string]interface{}); ok {
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
		log.Error(oops.In("form execution").With("context", "running form failed").Wrap(err))
		os.Exit(1)
	}

	return selectedItems
}

func installApps(selectedItems []string, category string, db *datastore.DB) {
	var appConfigs []types.AppConfig
	if err := viper.UnmarshalKey(category, &appConfigs); err != nil {
		log.Error(fmt.Sprintf("Failed to unmarshal apps for category %s: %v", category, err))
		return
	}

	for _, itemName := range selectedItems {
		for _, appConfig := range appConfigs {
			if appConfig.Name == itemName {
				log.Info(fmt.Sprintf("Installing %s using method %s", appConfig.Name, appConfig.InstallMethod))

				app := installers.App{
					Name:           appConfig.Name,
					Description:    appConfig.Description,
					Category:       category,
					InstallMethod:  appConfig.InstallMethod,
					InstallCommand: appConfig.InstallCommand,
				}

				if err := installers.InstallApp(app, viper.GetBool("dry-run"), db); err != nil {
					log.Error(oops.With("context", fmt.Sprintf("failed to install %s", appConfig.Name)).Wrap(err))
				}
			}
		}
	}
}

func getDefaultsFromConfig(category string) []string {
	var defaults []string
	apps := viper.GetStringMap(fmt.Sprintf("%s.apps", category))
	for _, app := range apps {
		appMap := app.(map[string]interface{})
		if defaultFlag, ok := appMap["default"].(bool); ok && defaultFlag {
			if name, ok := appMap["name"].(string); ok {
				defaults = append(defaults, name)
			}
		}
	}
	return defaults
}
