package steps

import (
	"fmt"
	"github.com/charmbracelet/log"
	"github.com/jameswlane/devex/pkg/config"
	"github.com/jameswlane/devex/pkg/datastore"
	"github.com/jameswlane/devex/pkg/installers"
	"github.com/jameswlane/devex/pkg/logger"
)

// Step structure (ensure it exists in your steps package)
type Step struct {
	Name   string
	Status string
	App    installers.App // Tie each step to an App
}

// ExecuteSteps runs the installation process for each step and updates the status
func ExecuteSteps(stepsList []Step, dryRun bool, db *datastore.DB, logger *logger.Logger) {
	for i := 0; i < len(stepsList); i++ {
		step := &stepsList[i]

		// Check if the step is already completed
		if step.Status == "Completed" {
			log.Info(fmt.Sprintf("Skipping step: %s (already completed)", step.Name))
			continue
		}

		// Log the beginning of the step
		log.Info(fmt.Sprintf("Starting step: %s", step.Name))

		// Call the InstallApp function
		log.Info(fmt.Sprintf("Dry run mode: %t", dryRun))
		err := installers.InstallApp(step.App, dryRun, db, logger)
		if err != nil {
			step.Status = "Error"
			log.Error(fmt.Sprintf("Failed to complete step: %s", step.Name), "error", err)
		} else {
			step.Status = "Completed"
			log.Info(fmt.Sprintf("Completed step: %s", step.Name))
		}
	}
}

func GenerateSteps() ([]Step, error) {
	var stepsList []Step

	// Load the apps configuration
	appsConfig, err := config.LoadAppsConfig("config/apps.yaml")
	if err != nil {
		return nil, fmt.Errorf("failed to load apps config: %v", err)
	}
	log.Info("Loaded apps configuration")
	// Convert each app to a step
	for _, app := range appsConfig.Apps {
		step := Step{
			Name:   app.Name,
			Status: "Pending", // Default to "Pending"
			App:    installers.App(app),
		}
		stepsList = append(stepsList, step)
	}

	return stepsList, nil
}
