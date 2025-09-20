package main

// Build timestamp: 2025-09-06

import (
	"fmt"
	"os"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
)

var version = "dev" // Set by goreleaser

// NewDockerPlugin creates a new Docker plugin
func NewDockerPlugin() *DockerInstaller {
	info := sdk.PluginInfo{
		Name:        "package-manager-docker",
		Version:     version,
		Description: "Docker container management and Docker Engine installation",
		Author:      "DevEx Team",
		Repository:  "https://github.com/jameswlane/devex",
		Tags:        []string{"package-manager", "docker", "containers", "linux"},
		Commands: []sdk.PluginCommand{
			{
				Name:        "install",
				Description: "Install and run Docker containers",
				Usage:       "Install Docker containers or Docker Engine",
				Flags: map[string]string{
					"name":   "Name for the container",
					"port":   "Port mapping (e.g., 8080:80)",
					"env":    "Environment variables",
					"volume": "Volume mounts",
					"detach": "Run container in background",
					"engine": "Install Docker Engine instead of containers",
				},
			},
			{
				Name:        "remove",
				Description: "Remove Docker containers",
				Usage:       "Remove Docker containers from the system",
				Flags: map[string]string{
					"force":   "Force removal of running containers",
					"volumes": "Remove associated volumes",
				},
			},
			{
				Name:        "start",
				Description: "Start stopped containers",
				Usage:       "Start one or more stopped containers",
			},
			{
				Name:        "stop",
				Description: "Stop running containers",
				Usage:       "Stop one or more running containers",
			},
			{
				Name:        "restart",
				Description: "Restart containers",
				Usage:       "Restart one or more containers",
			},
			{
				Name:        "list",
				Description: "List containers",
				Usage:       "List running or all containers",
				Flags: map[string]string{
					"all": "Show all containers (including stopped)",
				},
			},
			{
				Name:        "logs",
				Description: "Show container logs",
				Usage:       "Display logs from a container",
				Flags: map[string]string{
					"follow": "Follow log output",
					"tail":   "Number of lines to show from the end",
				},
			},
			{
				Name:        "exec",
				Description: "Execute command in container",
				Usage:       "Run a command inside a running container",
				Flags: map[string]string{
					"interactive": "Keep stdin open",
					"tty":         "Allocate a pseudo-TTY",
				},
			},
			{
				Name:        "build",
				Description: "Build Docker image",
				Usage:       "Build Docker image from Dockerfile",
				Flags: map[string]string{
					"tag":      "Name and optionally tag for the image",
					"file":     "Path to Dockerfile",
					"context":  "Build context directory",
					"no-cache": "Don't use cache when building",
				},
			},
			{
				Name:        "pull",
				Description: "Pull Docker image",
				Usage:       "Pull an image from a registry",
			},
			{
				Name:        "push",
				Description: "Push Docker image",
				Usage:       "Push an image to a registry",
			},
			{
				Name:        "images",
				Description: "List Docker images",
				Usage:       "Show locally available images",
			},
			{
				Name:        "rmi",
				Description: "Remove Docker images",
				Usage:       "Remove one or more images",
				Flags: map[string]string{
					"force": "Force removal of images",
				},
			},
			{
				Name:        "status",
				Description: "Check Docker daemon status",
				Usage:       "Check if Docker daemon is running",
			},
			{
				Name:        "ensure-installed",
				Description: "Install Docker Engine if not present",
				Usage:       "Install Docker Engine and configure system",
				Flags: map[string]string{
					"add-user": "Add current user to docker group",
				},
			},
			{
				Name:        "compose",
				Description: "Docker Compose operations",
				Usage:       "Manage multi-container applications with Docker Compose",
				Flags: map[string]string{
					"file":    "Compose file path",
					"project": "Project name",
					"detach":  "Run in background",
				},
			},
		},
	}

	return &DockerInstaller{
		PackageManagerPlugin: sdk.NewPackageManagerPlugin(info, "docker"),
		logger:               sdk.NewDefaultLogger(false),
	}
}

func main() {
	plugin := NewDockerPlugin()

	// Handle args with panic recovery
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(os.Stderr, "Plugin panic recovered: %v\n", r)
			os.Exit(1)
		}
	}()

	sdk.HandleArgs(plugin, os.Args[1:])
}
