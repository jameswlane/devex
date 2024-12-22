package install

import (
	"log"

	"github.com/jameswlane/devex/pkg/types"

	"github.com/spf13/cobra"

	"github.com/jameswlane/devex/pkg/config"
	"github.com/jameswlane/devex/pkg/datastore"
	"github.com/jameswlane/devex/pkg/datastore/repository"
	"github.com/jameswlane/devex/pkg/installers"
)

// CreateInstallCommand creates the `install` subcommand.
func CreateInstallCommand(homeDir string) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "install",
		Short: "Install development environment",
		Long:  "Install all necessary tools, programming languages, and databases for your development environment.",
		Run: func(cmd *cobra.Command, args []string) {
			runInstall(homeDir, cmd.Flags().Changed("dry-run"))
		},
	}
	cmd.Flags().Bool("dry-run", false, "Simulate installation without applying changes")
	return cmd
}

func runInstall(homeDir string, dryRun bool) {
	log.Println("Initializing database...")
	db, err := datastore.InitDB(homeDir + "/.devex/installed_apps.db")
	if err != nil {
		log.Fatalf("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// Use the correct type for repository initialization
	repo := repository.NewRepository(db.GetDB())

	log.Println("Loading configurations...")
	config.SetupConfig(homeDir) // Removed erroneous value usage

	log.Println("Installing components...")
	installComponents(repo, dryRun)

	log.Println("Installation process completed!")
}

func installComponents(repo repository.Repository, dryRun bool) {
	configNames := []string{"apps", "programming_languages", "databases"}
	for _, configName := range configNames {
		items, err := config.GetDefaults(configName)
		if err != nil {
			log.Printf("Failed to load configuration for %s: %v", configName, err)
			continue
		}

		log.Printf("Installing %s...", configName)
		for _, itemName := range items {
			app := types.AppConfig{
				Name: itemName,
				// Populate additional fields if needed
			}
			if err := installers.InstallApp(app, dryRun, repo); err != nil {
				log.Printf("Failed to install app %s: %v", app.Name, err)
			}
		}
	}
}
