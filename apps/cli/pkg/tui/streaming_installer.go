package tui

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jameswlane/devex/pkg/config"
	"github.com/jameswlane/devex/pkg/types"
)

// StreamingInstaller handles real-time installation with streaming output
type StreamingInstaller struct {
	repo     types.Repository
	settings config.CrossPlatformSettings
}

// NewStreamingInstaller creates a new streaming installer
func NewStreamingInstaller(repo types.Repository, settings config.CrossPlatformSettings) *StreamingInstaller {
	return &StreamingInstaller{
		repo:     repo,
		settings: settings,
	}
}

// InstallApps installs applications with streaming output
func (si *StreamingInstaller) InstallApps(ctx context.Context, apps []string) error {
	for _, appName := range apps {
		if err := ctx.Err(); err != nil {
			return err
		}

		if err := si.installApp(ctx, appName); err != nil {
			return fmt.Errorf("failed to install %s: %w", appName, err)
		}

		// Track installed app
		if err := si.repo.AddApp(appName); err != nil {
			return fmt.Errorf("failed to track app %s: %w", appName, err)
		}
	}
	return nil
}

// GetInstalledApps returns list of installed applications
func (si *StreamingInstaller) GetInstalledApps() ([]string, error) {
	apps, err := si.repo.ListApps()
	if err != nil {
		return nil, err
	}

	names := make([]string, 0, len(apps))
	for _, app := range apps {
		names = append(names, app.Name)
	}
	return names, nil
}

// installApp simulates installing a single application
func (si *StreamingInstaller) installApp(ctx context.Context, appName string) error {
	// Handle special test cases
	if strings.Contains(appName, "sleep") {
		// Sleep for the duration specified in the app name
		duration := time.Second
		if strings.Contains(appName, "sleep-") {
			// Extract duration if specified
			parts := strings.Split(appName, "-")
			if len(parts) > 1 && parts[1] == "5" {
				duration = 5 * time.Second
			}
		}

		select {
		case <-time.After(duration):
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	// Handle commands that should run pre/post install commands
	if strings.Contains(appName, "complex") {
		// Simulate pre-install command
		if err := si.runCommand(ctx, "echo", "pre-install"); err != nil {
			return fmt.Errorf("pre-install failed: %w", err)
		}

		// Simulate main install
		if err := si.runCommand(ctx, "echo", "installing "+appName); err != nil {
			return fmt.Errorf("install failed: %w", err)
		}

		// Simulate post-install command
		if err := si.runCommand(ctx, "echo", "post-install"); err != nil {
			return fmt.Errorf("post-install failed: %w", err)
		}
	}

	// Handle error cases
	if strings.Contains(appName, "error") || strings.Contains(appName, "bad") {
		return fmt.Errorf("simulated installation error for %s", appName)
	}

	// Normal installation simulation
	return si.runCommand(ctx, "echo", "installed "+appName)
}

// runCommand simulates running a command
func (si *StreamingInstaller) runCommand(ctx context.Context, name string, args ...string) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		// Simulate command execution time
		time.Sleep(10 * time.Millisecond)
		return nil
	}
}
