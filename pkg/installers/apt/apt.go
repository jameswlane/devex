package apt

import (
	"fmt"
	"sync"
	"time"

	"github.com/charmbracelet/log"

	"github.com/jameswlane/devex/pkg/datastore/repository"
	"github.com/jameswlane/devex/pkg/installers/check_install"
	"github.com/jameswlane/devex/pkg/utils"
)

var (
	lastAptUpdateTime time.Time
	aptCacheLock      sync.RWMutex
)

// RunAptUpdate runs `sudo apt update` if it hasn't already been run in the past 24 hours.
func RunAptUpdate(forceUpdate bool, repo repository.Repository) error {
	log.Info("Starting RunAptUpdate", "forceUpdate", forceUpdate)

	// Step 1: Check in-memory cache for the last update time
	aptCacheLock.RLock()
	if !forceUpdate && !lastAptUpdateTime.IsZero() && time.Since(lastAptUpdateTime) < 24*time.Hour {
		log.Info("Skipping 'apt update' as it was recently run (cached)")
		aptCacheLock.RUnlock()
		return nil
	}
	aptCacheLock.RUnlock()

	// Step 2: Check repository for last update time if cache is empty
	if !forceUpdate {
		lastUpdate, err := repo.Get("last_apt_update")
		if err == nil && lastUpdate != "" {
			lastUpdateTime, _ := time.Parse(time.RFC3339, lastUpdate)
			if time.Since(lastUpdateTime) < 24*time.Hour {
				log.Info("Skipping 'apt update' as it was recently run (repository)")
				return nil
			}
		} else {
			log.Warn("Failed to get last apt update time from repository", "error", err)
		}
	}

	// Step 3: Execute `sudo apt update`
	log.Info("Running 'sudo apt-get update'")
	if err := utils.ExecAsUser("sudo apt-get update", false); err != nil {
		log.Error("Failed to run 'apt update'", "error", err)
		return fmt.Errorf("failed to run 'apt update': %v", err)
	}

	// Step 4: Cache the update time in memory and save to the repository
	currentTime := time.Now()
	aptCacheLock.Lock()
	lastAptUpdateTime = currentTime
	aptCacheLock.Unlock()

	if err := repo.Set("last_apt_update", currentTime.Format(time.RFC3339)); err != nil {
		log.Warn("Failed to store last apt update time in repository", "error", err)
	}

	log.Info("RunAptUpdate completed successfully")
	return nil
}

// Install installs a package using apt
func Install(packageName string, dryRun bool, repo repository.Repository) error {
	log.Info("Starting Install", "packageName", packageName, "dryRun", dryRun)

	// Step 1: Check if the app is already recorded in the repository
	exists, err := repo.GetApp(packageName)
	if err != nil {
		log.Error("Failed to check app existence in repository", "packageName", packageName, "error", err)
		return fmt.Errorf("failed to check app existence in repository: %v", err)
	}

	if exists {
		log.Info(fmt.Sprintf("%s is already recorded in the repository, skipping installation", packageName))
		return nil
	}

	// Step 2: Check if the app is installed on the system
	log.Info("Checking if app is installed on the system", "packageName", packageName)
	isInstalledOnSystem, err := check_install.IsAppInstalled(packageName)
	if err != nil {
		log.Error("Failed to check if app is installed on system", "packageName", packageName, "error", err)
		return fmt.Errorf("failed to check if app is installed on system: %v", err)
	}

	if isInstalledOnSystem {
		log.Info(fmt.Sprintf("%s is already installed on the system, adding to repository", packageName))
		if err := repo.AddApp(packageName); err != nil {
			log.Error("Failed to add app to repository", "packageName", packageName, "error", err)
			return fmt.Errorf("failed to add %s to repository: %v", packageName, err)
		}
		return nil
	}

	// Step 3: Run `apt update` if needed
	log.Info("Running apt update if needed", "packageName", packageName)
	if err := RunAptUpdate(false, repo); err != nil {
		log.Error("Failed to run apt update", "packageName", packageName, "error", err)
		return err
	}

	// Step 4: Execute or simulate the installation command
	command := fmt.Sprintf("sudo apt-get install -y %s", packageName)
	if dryRun {
		log.Info(fmt.Sprintf("[Dry Run] Would run command: %s", command))
		return nil
	}

	log.Info("Executing installation command", "command", command)
	if err := utils.ExecAsUser(command, false); err != nil {
		log.Error("Failed to install package", "packageName", packageName, "error", err)
		return fmt.Errorf("failed to install %s: %v", packageName, err)
	}

	// Step 5: Add the app to the repository after successful installation
	log.Info("Adding app to repository after successful installation", "packageName", packageName)
	if err := repo.AddApp(packageName); err != nil {
		log.Error("Failed to add app to repository", "packageName", packageName, "error", err)
		return fmt.Errorf("failed to add %s to repository: %v", packageName, err)
	}

	log.Info(fmt.Sprintf("%s installed successfully and added to the repository", packageName))
	return nil
}
