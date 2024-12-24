package install

import (
	"log"

	"github.com/spf13/cobra"

	"github.com/jameswlane/devex/pkg/config"
	"github.com/jameswlane/devex/pkg/datastore"
	"github.com/jameswlane/devex/pkg/datastore/repository"
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
	_, err = config.LoadSettings(homeDir)
	if err != nil {
		return
	} // Removed erroneous value usage

	log.Println("Installing components...")
	installComponents(repo, dryRun)

	log.Println("Installation process completed!")
}

func installComponents(repo repository.Repository, dryRun bool) {
	// Log repo and dryRun values
	log.Printf("Repository: %v\n", repo)
	log.Printf("Dry Run: %v\n", dryRun)
}
