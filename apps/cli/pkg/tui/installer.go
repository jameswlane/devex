package tui

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os/exec"
	"regexp"
	"strings"
	"sync"
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

// SecureString represents a string that should be scrubbed from memory
type SecureString struct {
	data []byte
}

// NewSecureString creates a new secure string
func NewSecureString(s string) *SecureString {
	data := make([]byte, len(s))
	copy(data, s)
	return &SecureString{data: data}
}

// String returns the string value (use sparingly)
func (ss *SecureString) String() string {
	return string(ss.data)
}

// Clear scrubs the secure string from memory
func (ss *SecureString) Clear() {
	for i := range ss.data {
		ss.data[i] = 0
	}
	ss.data = nil
}

var (
	// allowedCommands defines safe commands that can be executed
	allowedCommands = map[string]bool{
		"apt":     true,
		"apt-get": true,
		"dpkg":    true,
		"curl":    true,
		"wget":    true,
		"git":     true,
		"docker":  true,
		"npm":     true,
		"pip":     true,
		"pip3":    true,
		"go":      true,
		"cargo":   true,
		"flatpak": true,
		"snap":    true,
		"dnf":     true,
		"yum":     true,
		"pacman":  true,
		"zypper":  true,
		"brew":    true,
		"mise":    true,
		"mkdir":   true,
		"cp":      true,
		"mv":      true,
		"chmod":   true,
		"chown":   true,
		"ln":      true,
		"tar":     true,
		"unzip":   true,
		"gunzip":  true,
		"echo":    true,
		"cat":     true,
		"which":   true,
		"whereis": true,
		"id":      true,
		"whoami":  true,
	}

	// dangerousPatterns are regex patterns for potentially dangerous command constructs
	dangerousPatterns = []*regexp.Regexp{
		regexp.MustCompile(`[;&]`),                                  // Command separators (but allow pipes for package managers)
		regexp.MustCompile(`` + "`" + `[^` + "`" + `]*` + "`" + ``), // Command substitution
		regexp.MustCompile(`\$\([^)]*\)`),                           // Command substitution
		regexp.MustCompile(`\$\{[^}]*\}`),                           // Variable expansion (except safe ones)
		regexp.MustCompile(`\.\./`),                                 // Directory traversal
		regexp.MustCompile(`/etc/passwd`),                           // Sensitive files
		regexp.MustCompile(`/etc/shadow`),                           // Sensitive files
		regexp.MustCompile(`rm\s+-rf\s+/[^a-zA-Z]`),                 // Dangerous rm commands on root
		regexp.MustCompile(`dd\s+if=/dev`),                          // Dangerous dd commands
		regexp.MustCompile(`:\(\)\{`),                               // Fork bombs
		regexp.MustCompile(`>\s*/etc/`),                             // Writing to system directories
		regexp.MustCompile(`>\s*/dev/`),                             // Writing to device files
	}
)

// NewStreamingInstaller creates a new streaming installer
func NewStreamingInstaller(program *tea.Program, repo types.Repository) *StreamingInstaller {
	return &StreamingInstaller{
		program: program,
		repo:    repo,
	}
}

// InstallApps installs multiple applications with streaming output
func (si *StreamingInstaller) InstallApps(apps []types.CrossPlatformApp, settings config.CrossPlatformSettings) error {
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
func (si *StreamingInstaller) InstallApp(app types.CrossPlatformApp, settings config.CrossPlatformSettings) error {
	si.sendLog("INFO", fmt.Sprintf("Starting installation of %s", app.Name))

	// Get platform-specific configuration
	osConfig := app.GetOSConfig()
	if osConfig.InstallMethod == "" {
		return fmt.Errorf("no configuration available for current platform")
	}

	// Validate app
	if err := app.Validate(); err != nil {
		return fmt.Errorf("app validation failed: %w", err)
	}

	// Handle pre-install commands
	if len(osConfig.PreInstall) > 0 {
		si.sendLog("INFO", "Executing pre-install commands...")
		if err := si.executeCommands(osConfig.PreInstall); err != nil {
			return fmt.Errorf("pre-install failed: %w", err)
		}
	}

	// Execute main installation command
	si.sendLog("INFO", fmt.Sprintf("Installing %s using %s...", app.Name, osConfig.InstallMethod))
	if err := si.executeInstallCommand(app, &osConfig); err != nil {
		return fmt.Errorf("installation failed: %w", err)
	}

	// Handle post-install commands
	if len(osConfig.PostInstall) > 0 {
		si.sendLog("INFO", "Executing post-install commands...")
		if err := si.executeCommands(osConfig.PostInstall); err != nil {
			return fmt.Errorf("post-install failed: %w", err)
		}
	}

	// Save to repository with better error handling
	if err := si.repo.AddApp(app.Name); err != nil {
		si.sendLog("ERROR", fmt.Sprintf("Failed to save app to database: %v", err))
		// Don't fail the installation for database errors, but log them prominently
		si.sendLog("WARN", "Installation succeeded but app tracking may be inconsistent")
	} else {
		si.sendLog("INFO", fmt.Sprintf("App %s registered in database", app.Name))
	}

	si.sendLog("INFO", fmt.Sprintf("Successfully installed %s", app.Name))
	return nil
}

// executeInstallCommand executes the main installation command
func (si *StreamingInstaller) executeInstallCommand(app types.CrossPlatformApp, osConfig *types.OSConfig) error {
	switch osConfig.InstallMethod {
	case "apt":
		return si.executeAptInstall(app, osConfig)
	case "curlpipe":
		return si.executeCurlPipeInstall(app, osConfig)
	case "docker":
		return si.executeDockerInstall(app, osConfig)
	default:
		// Generic command execution
		return si.executeCommandStream(osConfig.InstallCommand)
	}
}

// executeAptInstall handles APT package installation
func (si *StreamingInstaller) executeAptInstall(app types.CrossPlatformApp, osConfig *types.OSConfig) error {
	// Add APT sources if needed
	for _, source := range osConfig.AptSources {
		si.sendLog("INFO", fmt.Sprintf("Adding APT source: %s", source.SourceName))
		// This would integrate with your existing APT source handling
	}

	// Update package lists
	si.sendLog("INFO", "Updating package lists...")
	if err := si.executeCommandStream("sudo apt update"); err != nil {
		return err
	}

	// Install package
	return si.executeCommandStream(osConfig.InstallCommand)
}

// executeCurlPipeInstall handles curl pipe installations
func (si *StreamingInstaller) executeCurlPipeInstall(app types.CrossPlatformApp, osConfig *types.OSConfig) error {
	si.sendLog("INFO", fmt.Sprintf("Downloading from %s", osConfig.DownloadURL))
	command := fmt.Sprintf("curl -fsSL %s | bash", osConfig.DownloadURL)
	return si.executeCommandStream(command)
}

// executeDockerInstall handles Docker container installations
func (si *StreamingInstaller) executeDockerInstall(app types.CrossPlatformApp, osConfig *types.OSConfig) error {
	si.sendLog("INFO", fmt.Sprintf("Starting Docker installation for %s", app.Name))
	return si.executeCommandStream(osConfig.InstallCommand)
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
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	// Validate and parse command safely
	if strings.TrimSpace(command) == "" {
		return fmt.Errorf("empty command")
	}

	// Use shell for complex commands but validate first
	if err := si.validateCommand(command); err != nil {
		return fmt.Errorf("command validation failed: %w", err)
	}

	// Execute via shell with proper escaping for complex commands
	cmd := exec.CommandContext(ctx, "/bin/bash", "-c", command)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}

	// Create pipes for stdout and stderr
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return err
	}
	defer stdout.Close()

	stderr, err := cmd.StderrPipe()
	if err != nil {
		return err
	}
	defer stderr.Close()

	// Create pipe for stdin (for password prompts)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}
	defer stdin.Close()

	// Start command
	if err := cmd.Start(); err != nil {
		return err
	}

	// Use WaitGroup to ensure all goroutines complete
	var wg sync.WaitGroup

	// Stream output with goroutine tracking
	wg.Add(2)
	go func() {
		defer wg.Done()
		si.streamOutput(stdout, "STDOUT")
	}()
	go func() {
		defer wg.Done()
		si.streamOutput(stderr, "STDERR")
	}()

	// Monitor for password prompts with goroutine tracking
	wg.Add(1)
	go func() {
		defer wg.Done()
		si.monitorForInput(stderr, stdin)
	}()

	// Wait for command completion
	cmdErr := cmd.Wait()

	// Wait for all goroutines to finish
	wg.Wait()

	return cmdErr
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

// validateCommand validates a command for security issues
func (si *StreamingInstaller) validateCommand(command string) error {
	// Check for dangerous patterns
	for _, pattern := range dangerousPatterns {
		if pattern.MatchString(command) {
			return fmt.Errorf("command contains potentially dangerous pattern: %s", pattern.String())
		}
	}

	// Parse command and validate first word
	parts := strings.Fields(command)
	if len(parts) == 0 {
		return fmt.Errorf("empty command")
	}

	// Handle sudo commands specially
	if parts[0] == "sudo" {
		if len(parts) < 2 {
			return fmt.Errorf("sudo command missing actual command")
		}
		// Validate the command after sudo
		actualCommand := parts[1]
		if !allowedCommands[actualCommand] {
			return fmt.Errorf("sudo command '%s' is not in allowed list", actualCommand)
		}
	} else if !allowedCommands[parts[0]] {
		// Validate regular commands
		return fmt.Errorf("command '%s' is not in allowed list", parts[0])
	}

	return nil
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
			response := make(chan *SecureString, 1)
			si.program.Send(InputRequestMsg{
				Prompt:   line,
				Response: response,
			})

			// Wait for user response
			select {
			case secureInput := <-response:
				if secureInput != nil {
					// Write password and immediately scrub from memory
					if _, err := stdin.Write([]byte(secureInput.String() + "\n")); err != nil {
						si.sendLog("ERROR", fmt.Sprintf("Failed to write input: %v", err))
					}
					secureInput.Clear() // Scrub password from memory
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
func StartInstallation(apps []types.CrossPlatformApp, repo types.Repository, settings config.CrossPlatformSettings) error {
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
