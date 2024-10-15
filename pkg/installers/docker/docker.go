package docker

import (
	"fmt"
	"github.com/jameswlane/devex/pkg/datastore"
	"github.com/jameswlane/devex/pkg/installers/check_install"
	"github.com/jameswlane/devex/pkg/logger"
	"gopkg.in/yaml.v2"
	"os"
	"os/exec"
	"time"
)

type App struct {
	Name           string        `yaml:"name"`
	Description    string        `yaml:"description"`
	Category       string        `yaml:"category"`
	InstallMethod  string        `yaml:"install_method"`
	InstallCommand string        `yaml:"install_command"`
	DockerOptions  DockerOptions `yaml:"docker_options"`
}

type DockerOptions struct {
	Ports         []string `yaml:"ports"`
	ContainerName string   `yaml:"container_name"`
	Environment   []string `yaml:"environment,omitempty"`
	RestartPolicy string   `yaml:"restart_policy"`
}

// LoadApps loads the app configuration from a YAML file
func LoadApps(filename string) ([]App, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read apps YAML file: %v", err)
	}

	var apps []App
	err = yaml.Unmarshal(data, &apps)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal apps YAML: %v", err)
	}

	return apps, nil
}

// Install installs a Docker app based on the app configuration
func Install(app App, dryRun bool, db *datastore.DB, logger *logger.Logger) error {
	// Check if the container is already running using Docker ps
	isInstalledOnSystem, err := check_install.IsAppInstalled(app.DockerOptions.ContainerName)
	if err != nil {
		return fmt.Errorf("failed to check if Docker container is running: %v", err)
	}

	if isInstalledOnSystem {
		logger.LogInfo(fmt.Sprintf("Docker container %s is already running, skipping installation", app.DockerOptions.ContainerName))
		return nil
	}

	// Handle dry-run case
	if dryRun {
		logger.LogInfo(fmt.Sprintf("[Dry Run] Would run Docker command for container: %s", app.DockerOptions.ContainerName))
		time.Sleep(5 * time.Second)
		return nil
	}

	// Build the Docker run command
	cmdArgs := []string{"run", "-d"}

	// Add restart policy
	if app.DockerOptions.RestartPolicy != "" {
		cmdArgs = append(cmdArgs, "--restart", app.DockerOptions.RestartPolicy)
	}

	// Add ports
	for _, port := range app.DockerOptions.Ports {
		cmdArgs = append(cmdArgs, "-p", port)
	}

	// Add environment variables
	for _, env := range app.DockerOptions.Environment {
		cmdArgs = append(cmdArgs, "-e", env)
	}

	// Add container name and image
	cmdArgs = append(cmdArgs, "--name", app.DockerOptions.ContainerName, app.InstallCommand)

	// Execute the Docker run command
	cmd := exec.Command("sudo", append([]string{"docker"}, cmdArgs...)...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		logger.LogError(fmt.Sprintf("Failed to install Docker container: %s - %s", app.DockerOptions.ContainerName, string(output)), err)
		return err
	}

	// Add the installed container to the database
	err = datastore.AddInstalledApp(db, app.DockerOptions.ContainerName)
	if err != nil {
		return fmt.Errorf("failed to add Docker container %s to database: %v", app.DockerOptions.ContainerName, err)
	}

	logger.LogInfo(fmt.Sprintf("Docker container %s installed successfully", app.DockerOptions.ContainerName))
	return nil
}