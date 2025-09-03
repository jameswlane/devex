package main

// Build timestamp: 2025-09-03 17:41:19

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
)

var version = "dev" // Set by goreleaser

// DockerPlugin implements the Docker package manager
type DockerPlugin struct {
	*sdk.PackageManagerPlugin
}

// NewDockerPlugin creates a new Docker plugin
func NewDockerPlugin() *DockerPlugin {
	info := sdk.PluginInfo{
		Name:        "package-manager-docker",
		Version:     version,
		Description: "Docker container management and Docker Engine installation",
		Author:      "DevEx Team",
		Repository:  "https://github.com/jameswlane/devex",
		Tags:        []string{"docker", "containers", "linux"},
		Commands: []sdk.PluginCommand{
			{
				Name:        "install",
				Description: "Install Docker containers or Docker Engine",
				Usage:       "Install Docker containers or Docker Engine",
			},
			{
				Name:        "remove",
				Description: "Remove Docker containers",
				Usage:       "Remove Docker containers from the system",
			},
			{
				Name:        "list",
				Description: "List running containers",
				Usage:       "List currently running containers",
			},
			{
				Name:        "status",
				Description: "Check Docker daemon status",
				Usage:       "Check if Docker daemon is running",
			},
		},
	}

	return &DockerPlugin{
		PackageManagerPlugin: sdk.NewPackageManagerPlugin(info, "docker"),
	}
}

// isDockerAvailable checks if Docker is available
func isDockerAvailable() bool {
	return sdk.CommandExists("docker")
}

// isDockerDaemonRunning checks if Docker daemon is running
func isDockerDaemonRunning() bool {
	cmd := exec.Command("docker", "info")
	return cmd.Run() == nil
}

// Execute handles command execution
func (p *DockerPlugin) Execute(command string, args []string) error {
	p.EnsureAvailable()

	switch command {
	case "install":
		return p.handleInstall(args)
	case "remove":
		return p.handleRemove(args)
	case "list":
		return p.handleList(args)
	case "status":
		return p.handleStatus(args)
	default:
		return fmt.Errorf("unknown command: %s", command)
	}
}

func (p *DockerPlugin) handleInstall(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no packages specified")
	}

	// Check if Docker is available
	if !isDockerAvailable() {
		return fmt.Errorf("docker is not installed or not in PATH")
	}

	// Check if trying to install Docker Engine
	for _, arg := range args {
		if strings.Contains(strings.ToLower(arg), "docker") || strings.Contains(strings.ToLower(arg), "engine") {
			return p.handleDockerEngineInstall()
		}
	}

	// Handle container installation
	fmt.Printf("Installing Docker containers: %s\n", strings.Join(args, ", "))

	for _, containerSpec := range args {
		if err := p.handleContainerInstall(containerSpec); err != nil {
			fmt.Printf("Warning: Failed to install container %s: %v\n", containerSpec, err)
		}
	}

	return nil
}

// handleDockerEngineInstall installs Docker Engine
func (p *DockerPlugin) handleDockerEngineInstall() error {
	fmt.Println("Installing Docker Engine...")

	// Simple Docker installation using the official convenience script
	fmt.Println("Downloading Docker installation script...")
	cmd := exec.Command("sh", "-c", "curl -fsSL https://get.docker.com -o get-docker.sh && sudo sh get-docker.sh")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to install Docker Engine: %w", err)
	}

	// Add current user to docker group
	fmt.Println("Adding user to docker group...")
	userCmd := exec.Command("sudo", "usermod", "-aG", "docker", os.Getenv("USER"))
	if err := userCmd.Run(); err != nil {
		fmt.Printf("Warning: Failed to add user to docker group: %v\n", err)
	}

	fmt.Println("Docker Engine installed successfully!")
	fmt.Println("Note: You may need to log out and back in for group changes to take effect.")
	return nil
}

// handleContainerInstall installs a Docker container
func (p *DockerPlugin) handleContainerInstall(containerSpec string) error {
	if !isDockerDaemonRunning() {
		return fmt.Errorf("docker daemon is not running")
	}

	// Simple container run - expects format like "nginx", "postgres:13", etc.
	fmt.Printf("Starting container: %s\n", containerSpec)

	// Extract container name for naming
	containerName := strings.Split(containerSpec, ":")[0]
	containerName = strings.ReplaceAll(containerName, "/", "_")

	// Run container in detached mode
	cmd := exec.Command("docker", "run", "-d", "--name", containerName, containerSpec)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to start container %s: %w", containerSpec, err)
	}

	fmt.Printf("Container %s started successfully\n", containerName)
	return nil
}

func (p *DockerPlugin) handleRemove(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no containers specified")
	}

	if !isDockerAvailable() {
		return fmt.Errorf("docker is not installed")
	}

	if !isDockerDaemonRunning() {
		return fmt.Errorf("docker daemon is not running")
	}

	fmt.Printf("Removing containers: %s\n", strings.Join(args, ", "))

	for _, containerName := range args {
		// Stop the container
		stopCmd := exec.Command("docker", "stop", containerName)
		_ = stopCmd.Run() // Ignore errors as container might already be stopped

		// Remove the container
		removeCmd := exec.Command("docker", "rm", containerName)
		if err := removeCmd.Run(); err != nil {
			fmt.Printf("Warning: Failed to remove container %s: %v\n", containerName, err)
		} else {
			fmt.Printf("Container %s removed successfully\n", containerName)
		}
	}

	return nil
}

func (p *DockerPlugin) handleList(args []string) error {
	if !isDockerAvailable() {
		return fmt.Errorf("docker is not installed")
	}

	if !isDockerDaemonRunning() {
		return fmt.Errorf("docker daemon is not running")
	}

	fmt.Println("Running containers:")
	return sdk.ExecCommand(false, "docker", "ps")
}

func (p *DockerPlugin) handleStatus(args []string) error {
	if !isDockerAvailable() {
		fmt.Println("❌ Docker is not installed")
		return nil
	}

	if !isDockerDaemonRunning() {
		fmt.Println("❌ Docker daemon is not running")
		return nil
	}

	fmt.Println("✅ Docker is available and daemon is running")
	return sdk.ExecCommand(false, "docker", "version")
}

func main() {
	plugin := NewDockerPlugin()
	sdk.HandleArgs(plugin, os.Args[1:])
}
