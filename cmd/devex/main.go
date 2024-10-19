package main

import (
	"fmt"
	"github.com/charmbracelet/huh"
	"github.com/charmbracelet/log"
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

// Entry point
func main() {
	// Setup configuration using Viper
	setupConfig()

	// Execute the root command
	if err := rootCmd.Execute(); err != nil {
		log.Error(oops.In("root command").With("context", "root command execution failed").Wrap(err))
		os.Exit(1)
	}
}

// Setup configuration with Viper
func setupConfig() {
	viper.SetConfigName("config") // name of config file (without extension)
	viper.SetConfigType("yaml")   // specify the config file type

	// Check in ~/.devex first for a custom config
	homeDir, err := os.UserHomeDir()
	if err == nil {
		viper.AddConfigPath(filepath.Join(homeDir, ".devex"))
		viper.AddConfigPath(filepath.Join(homeDir, ".devex/config"))
	}

	// Fallback to /config
	viper.AddConfigPath("./config")

	// Read in environment variables that match
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
	// Adding subcommand: install
	var installCmd = &cobra.Command{
		Use:   "install",
		Short: "Install development environment",
		Long:  "Install all necessary tools, programming languages, and databases for your development environment.",
		Run: func(cmd *cobra.Command, args []string) {
			// Load custom configs for databases and programming languages
			homeDir, err := os.UserHomeDir()
			if err == nil {
				loadCustomConfig(filepath.Join(homeDir, ".devex/config/databases.yaml"))
				loadCustomConfig(filepath.Join(homeDir, ".devex/config/programming_languages.yaml"))
			} else {
				log.Error(oops.In("user directory").With("context", "failed to get user home directory").Wrap(err))
				os.Exit(1)
			}

			// Fallback to default config paths if custom configs are not found
			loadCustomConfig("./config/databases.yaml")
			loadCustomConfig("./config/programming_languages.yaml")

			// Check if DEVEX_NONINTERACTIVE is set
			if viper.GetBool("DEVEX_NONINTERACTIVE") {
				log.Info("Running in non-interactive mode, using default settings")
				// Use defaults from the loaded configuration files
				defaultLanguages := getDefaultsFromConfig("programming_languages")
				defaultDatabases := getDefaultsFromConfig("databases")
				log.Info("Selected programming languages: ", "languages", defaultLanguages)
				log.Info("Selected databases: ", "databases", defaultDatabases)
			} else {
				// Use huh form to ask for user input
				var selectedLanguages, selectedDatabases []string

				// Populate options from the configuration
				var languageOptions []huh.Option[string]
				if languages, ok := viper.Get("programming_languages").([]interface{}); ok {
					for _, lang := range languages {
						if langMap, ok := lang.(map[string]interface{}); ok {
							option := huh.NewOption(langMap["name"].(string), langMap["name"].(string))
							if defaultFlag, ok := langMap["default"].(bool); ok && defaultFlag {
								option = option.Selected(true)
							}
							languageOptions = append(languageOptions, option)
						}
					}
				}

				var databaseOptions []huh.Option[string]
				if databases, ok := viper.Get("databases").([]interface{}); ok {
					for _, db := range databases {
						if dbMap, ok := db.(map[string]interface{}); ok {
							option := huh.NewOption(dbMap["name"].(string), dbMap["name"].(string))
							if defaultFlag, ok := dbMap["default"].(bool); ok && defaultFlag {
								option = option.Selected(true)
							}
							databaseOptions = append(databaseOptions, option)
						}
					}
				}

				// Create form groups for selecting programming languages and databases
				form := huh.NewForm(
					huh.NewGroup(
						huh.NewMultiSelect[string]().
							Title("Select Programming Languages to Install").
							Options(languageOptions...).
							Value(&selectedLanguages),
					),
					huh.NewGroup(
						huh.NewMultiSelect[string]().
							Title("Select Databases to Install").
							Options(databaseOptions...).
							Value(&selectedDatabases),
					),
				)

				if err := form.Run(); err != nil {
					log.Error(oops.In("form execution").With("context", "running form failed").Wrap(err))
					os.Exit(1)
				}

				log.Info("User selected programming languages: ", "languages", selectedLanguages)
				log.Info("User selected databases: ", "databases", selectedDatabases)
			}

			// Placeholder for the actual install logic
			fmt.Println("Running installation...")
		},
	}

	// Adding flags to the install command (placeholder)
	installCmd.Flags().Bool("dry-run", false, "Run in dry-run mode without making any changes")
	viper.BindPFlag("dry-run", installCmd.Flags().Lookup("dry-run"))

	// Adding global debug flag
	rootCmd.PersistentFlags().BoolVar(&debug, "debug", false, "Enable debug mode with verbose logging")
	viper.BindPFlag("debug", rootCmd.PersistentFlags().Lookup("debug"))

	rootCmd.AddCommand(installCmd)
}

// Helper function to get defaults from configuration
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
