package tui

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"syscall"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jameswlane/devex/pkg/config"
	"github.com/jameswlane/devex/pkg/types"
)

// StreamingInstaller handles installation with real-time output and interaction
type StreamingInstaller struct {
	program *tea.Program
	repo    types.Repository
}

// NewStreamingInstaller creates a new streaming installer
func NewStreamingInstaller(program *tea.Program, repo types.Repository) *StreamingInstaller {
	return &StreamingInstaller{
		program: program,
		repo:    repo,
	}
}

// InstallApps installs multiple applications with streaming output
func (si *StreamingInstaller) InstallApps(apps []types.AppConfig, settings config.CrossPlatformSettings) error {
	for _, app := range apps {
		if err := si.InstallApp(app, settings); err != nil {
			si.sendLog("ERROR", fmt.Sprintf("Failed to install %s: %v", app.Name, err))
			si.program.Send(AppCompleteMsg{
				AppName: app.Name,
				Error:   err,
			})
			continue
		}

		si.program.Send(AppCompleteMsg{
			AppName: app.Name,
			Error:   nil,
		})
	}
	return nil
}

// InstallApp installs a single application with streaming output
func (si *StreamingInstaller) InstallApp(app types.AppConfig, settings config.CrossPlatformSettings) error {
	si.sendLog("INFO", fmt.Sprintf("Starting installation of %s", app.Name))

	// Validate app
	if err := app.Validate(); err != nil {
		return fmt.Errorf("app validation failed: %w", err)
	}

	// Handle pre-install commands
	if len(app.PreInstall) > 0 {
		si.sendLog("INFO", "Executing pre-install commands...")
		if err := si.executeCommands(app.PreInstall); err != nil {
			return fmt.Errorf("pre-install failed: %w", err)
		}
	}

	// Execute main installation command
	si.sendLog("INFO", fmt.Sprintf("Installing %s using %s...", app.Name, app.InstallMethod))
	if err := si.executeInstallCommand(app); err != nil {
		return fmt.Errorf("installation failed: %w", err)
	}

	// Handle post-install commands
	if len(app.PostInstall) > 0 {
		si.sendLog("INFO", "Executing post-install commands...")
		if err := si.executeCommands(app.PostInstall); err != nil {
			return fmt.Errorf("post-install failed: %w", err)
		}
	}

	// Save to repository
	if err := si.repo.AddApp(app.Name); err != nil {
		si.sendLog("WARN", fmt.Sprintf("Failed to save app to database: %v", err))
	}

	si.sendLog("INFO", fmt.Sprintf("Successfully installed %s", app.Name))
	return nil
}

// executeInstallCommand executes the main installation command
func (si *StreamingInstaller) executeInstallCommand(app types.AppConfig) error {
	switch app.InstallMethod {
	case "apt":
		return si.executeAptInstall(app)
	case "curlpipe":
		return si.executeCurlPipeInstall(app)
	case "docker":
		return si.executeDockerInstall(app)
	default:
		// Generic command execution
		return si.executeCommandStream(app.InstallCommand)
	}
}

// executeAptInstall handles APT package installation
func (si *StreamingInstaller) executeAptInstall(app types.AppConfig) error {
	// Add APT sources if needed
	for _, source := range app.AptSources {
		si.sendLog("INFO", fmt.Sprintf("Adding APT source: %s", source.SourceName))
		// This would integrate with your existing APT source handling
	}

	// Update package lists
	si.sendLog("INFO", "Updating package lists...")
	if err := si.executeCommandStream("sudo apt update"); err != nil {
		return err
	}

	// Install package
	return si.executeCommandStream(app.InstallCommand)
}

// executeCurlPipeInstall handles curl pipe installations
func (si *StreamingInstaller) executeCurlPipeInstall(app types.AppConfig) error {
	si.sendLog("INFO", fmt.Sprintf("Downloading from %s", app.DownloadURL))
	command := fmt.Sprintf("curl -fsSL %s | bash", app.DownloadURL)
	return si.executeCommandStream(command)
}

// executeDockerInstall handles Docker container installations
func (si *StreamingInstaller) executeDockerInstall(app types.AppConfig) error {
	si.sendLog("INFO", fmt.Sprintf("Starting Docker container %s", app.DockerOptions.ContainerName))
	return si.executeCommandStream(app.InstallCommand)
}

// executeCommands executes a list of install commands
func (si *StreamingInstaller) executeCommands(commands []types.InstallCommand) error {
	for _, cmd := range commands {
		if cmd.Command != "" {
			si.sendLog("INFO", fmt.Sprintf("Executing: %s", cmd.Command))
			if err := si.executeCommandStream(cmd.Command); err != nil {
				return err
			}
		}

		if cmd.Shell != "" {
			si.sendLog("INFO", fmt.Sprintf("Executing shell: %s", cmd.Shell))
			if err := si.executeCommandStream(cmd.Shell); err != nil {
				return err
			}
		}

		if cmd.Copy != nil {
			si.sendLog("INFO", fmt.Sprintf("Copying %s to %s", cmd.Copy.Source, cmd.Copy.Destination))
			// Handle file copy
		}

		if cmd.Sleep > 0 {
			si.sendLog("INFO", fmt.Sprintf("Sleeping for %d seconds", cmd.Sleep))
			time.Sleep(time.Duration(cmd.Sleep) * time.Second)
		}
	}
	return nil
}

// executeCommandStream executes a command with streaming output
func (si *StreamingInstaller) executeCommandStream(command string) error {
	ctx := context.Background()

	// Parse command
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return fmt.Errorf("empty command")
	}

	cmd := exec.CommandContext(ctx, parts[0], parts[1:]...)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}

	// Create pipes for stdout and stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}

	// Create pipe for stdin (for password prompts)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}

	// Start command
	if err := cmd.Start(); err != nil {
		return err
	}

	// Stream output
	go si.streamOutput(stdout, "STDOUT")
	go si.streamOutput(stderr, "STDERR")

	// Monitor for password prompts
	go si.monitorForInput(stderr, stdin)

	// Wait for completion
	return cmd.Wait()
}

// streamOutput streams command output to the TUI
func (si *StreamingInstaller) streamOutput(reader io.Reader, source string) {
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.TrimSpace(line) != "" {
			si.sendLog(source, line)
		}
	}
}

// monitorForInput monitors stderr for password prompts and requests user input
func (si *StreamingInstaller) monitorForInput(stderr io.Reader, stdin io.WriteCloser) {
	scanner := bufio.NewScanner(stderr)
	for scanner.Scan() {
		line := scanner.Text()

		// Check for common password prompts
		lowerLine := strings.ToLower(line)
		if strings.Contains(lowerLine, "password") &&
			(strings.Contains(lowerLine, "sudo") ||
				strings.Contains(lowerLine, "enter") ||
				strings.Contains(lowerLine, ":")) {
			// Request input from user
			response := make(chan string, 1)
			si.program.Send(InputRequestMsg{
				Prompt:   line,
				Response: response,
			})

			// Wait for user response
			select {
			case userInput := <-response:
				if _, err := stdin.Write([]byte(userInput + "\n")); err != nil {
					si.sendLog("ERROR", fmt.Sprintf("Failed to write input: %v", err))
				}
			case <-time.After(30 * time.Second):
				si.sendLog("ERROR", "Input timeout - no response received")
				return
			}
		}
	}
}

// sendLog sends a log message to the TUI
func (si *StreamingInstaller) sendLog(level, message string) {
	si.program.Send(LogMsg{
		Message:   message,
		Timestamp: time.Now(),
		Level:     level,
	})
}

// StartInstallation starts the installation process in the TUI
func StartInstallation(apps []types.AppConfig, repo types.Repository, settings config.CrossPlatformSettings) error {
	// Create TUI model
	m := NewModel(apps)

	// Create program
	p := tea.NewProgram(m, tea.WithAltScreen())

	// Create streaming installer
	installer := NewStreamingInstaller(p, repo)

	// Start installation in background
	go func() {
		time.Sleep(500 * time.Millisecond) // Let TUI initialize
		if err := installer.InstallApps(apps, settings); err != nil {
			installer.sendLog("ERROR", fmt.Sprintf("Installation failed: %v", err))
		}
	}()

	// Start TUI
	_, err := p.Run()
	return err
}
