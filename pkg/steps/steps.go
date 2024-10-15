package steps

import (
	"fmt"
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
	for i := range stepsList {
		step := &stepsList[i]

		// Log the beginning of the step
		logger.LogInfo(fmt.Sprintf("Starting step: %s", step.Name))

		// Call the InstallApp function
		err := installers.InstallApp(step.App, dryRun, db, logger)
		if err != nil {
			step.Status = "Error"
			logger.LogError(fmt.Sprintf("Failed to complete step: %s", step.Name), err)
		} else {
			step.Status = "Completed"
			logger.LogInfo(fmt.Sprintf("Completed step: %s", step.Name))
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