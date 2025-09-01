package utilities

import (
	"fmt"
	"strings"

	"github.com/jameswlane/devex/apps/cli/internal/log"
	"github.com/jameswlane/devex/apps/cli/internal/utils"
)

// PostInstallHandler defines a function that performs post-installation setup
type PostInstallHandler func() error

// HandlerRegistry manages post-installation handlers for different packages
type HandlerRegistry struct {
	handlers map[string]PostInstallHandler
}

// NewHandlerRegistry creates a new handler registry with default handlers
func NewHandlerRegistry() *HandlerRegistry {
	registry := &HandlerRegistry{
		handlers: make(map[string]PostInstallHandler),
	}

	// Register default handlers
	registry.RegisterDockerHandlers()
	registry.RegisterWebServerHandlers()

	return registry
}

// Register adds a post-install handler for the specified package
func (hr *HandlerRegistry) Register(packageName string, handler PostInstallHandler) {
	hr.handlers[packageName] = handler
}

// Execute runs the post-install handler for the specified package if one exists
func (hr *HandlerRegistry) Execute(packageName string) error {
	if handler, exists := hr.handlers[packageName]; exists {
		log.Debug("Running post-install handler", "package", packageName)
		return handler()
	}
	return nil // No handler needed
}

// HasHandler returns true if a handler is registered for the package
func (hr *HandlerRegistry) HasHandler(packageName string) bool {
	_, exists := hr.handlers[packageName]
	return exists
}

// RegisterDockerHandlers registers handlers for Docker-related packages
func (hr *HandlerRegistry) RegisterDockerHandlers() {
	hr.Register("docker", setupDockerService)
	hr.Register("docker-ce", setupDockerService)
	hr.Register("docker.io", setupDockerService) // Ubuntu package name
}

// RegisterWebServerHandlers registers handlers for web server packages
func (hr *HandlerRegistry) RegisterWebServerHandlers() {
	hr.Register("nginx", func() error {
		return setupWebServerService("nginx")
	})

	hr.Register("httpd", func() error {
		return setupWebServerService("httpd")
	})

	hr.Register("apache2", func() error {
		return setupWebServerService("apache2")
	})
}

// setupDockerService configures Docker service and user permissions
func setupDockerService() error {
	log.Debug("Configuring Docker service and permissions")

	// Enable Docker service to start on boot
	if _, err := utils.CommandExec.RunShellCommand("sudo systemctl enable docker"); err != nil {
		log.Warn("Failed to enable Docker service", "error", err)
		// Don't fail the installation for service management issues
	} else {
		log.Info("Docker service enabled for automatic startup")
	}

	// Start Docker service
	if _, err := utils.CommandExec.RunShellCommand("sudo systemctl start docker"); err != nil {
		log.Warn("Failed to start Docker service", "error", err)
		// Don't fail the installation for service management issues
	} else {
		log.Info("Docker service started")
	}

	// Add current user to docker group if not running as root
	currentUser := GetCurrentUser()
	if currentUser != "" && currentUser != "root" {
		addUserCmd := fmt.Sprintf("sudo usermod -aG docker %s", currentUser)
		if _, err := utils.CommandExec.RunShellCommand(addUserCmd); err != nil {
			log.Warn("Failed to add user to docker group", "user", currentUser, "error", err)
			log.Info("You may need to manually add your user to the docker group: sudo usermod -aG docker $USER")
		} else {
			log.Info("User added to docker group (may require logout/login)", "user", currentUser)
		}
	}

	return nil
}

// setupWebServerService configures web server services
func setupWebServerService(serviceName string) error {
	log.Debug("Configuring web server service", "service", serviceName)

	// Enable service to start on boot
	enableCmd := fmt.Sprintf("sudo systemctl enable %s", serviceName)
	if _, err := utils.CommandExec.RunShellCommand(enableCmd); err != nil {
		log.Warn("Failed to enable web server service", "service", serviceName, "error", err)
		return err
	}

	log.Info("Web server service enabled for automatic startup", "service", serviceName)

	// Note: We don't automatically start web servers as they may need configuration
	log.Info("Web server installed but not started - configure and start manually", "service", serviceName)
	log.Info("To start: sudo systemctl start "+serviceName, "service", serviceName)

	return nil
}

// Global registry instance
var DefaultRegistry = NewHandlerRegistry()

// RegisterHandler is a convenience function to register a handler with the default registry
func RegisterHandler(packageName string, handler PostInstallHandler) {
	DefaultRegistry.Register(packageName, handler)
}

// ExecutePostInstallHandler is a convenience function to execute a handler with the default registry
func ExecutePostInstallHandler(packageName string) error {
	// Try exact match first
	if err := DefaultRegistry.Execute(packageName); err != nil {
		return err
	}

	// Try with common variations/aliases
	variations := getPackageVariations(packageName)
	for _, variation := range variations {
		if DefaultRegistry.HasHandler(variation) {
			return DefaultRegistry.Execute(variation)
		}
	}

	return nil
}

// getPackageVariations returns common variations of a package name
func getPackageVariations(packageName string) []string {
	variations := []string{packageName}

	// Add common variations
	if strings.HasSuffix(packageName, "-ce") {
		variations = append(variations, strings.TrimSuffix(packageName, "-ce"))
	} else {
		variations = append(variations, packageName+"-ce")
	}

	// Docker variations
	switch packageName {
	case "docker":
		variations = append(variations, "docker.io", "docker-ce")
	case "docker.io":
		variations = append(variations, "docker", "docker-ce")
	}

	// Apache variations
	switch packageName {
	case "httpd":
		variations = append(variations, "apache2")
	case "apache2":
		variations = append(variations, "httpd")
	}

	return variations
}
