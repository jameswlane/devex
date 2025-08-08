package mocks

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

type MockCommandExecutor struct {
	Commands          []string        // Stores commands executed
	FailingCommand    string          // Command that should fail
	FailingCommands   map[string]bool // Multiple commands that should fail
	InstallationState map[string]bool // Track package installation state
}

func NewMockCommandExecutor() *MockCommandExecutor {
	return &MockCommandExecutor{
		InstallationState: make(map[string]bool),
		FailingCommands:   make(map[string]bool),
	}
}

func (m *MockCommandExecutor) RunShellCommand(command string) (string, error) {
	m.Commands = append(m.Commands, command)

	// Check for exact command match first
	if command == m.FailingCommand {
		return "", fmt.Errorf("mock shell command failed: %s", command)
	}

	// Check multiple failing commands
	if m.FailingCommands[command] {
		return "", fmt.Errorf("mock shell command failed: %s", command)
	}

	// Special handling for sudo commands - only fail if explicitly marked to fail
	if m.FailingCommand != "" && strings.HasPrefix(command, "sudo ") {
		baseCommand := strings.TrimPrefix(command, "sudo ")
		// Check if the sudo version should also fail
		if m.FailingCommands[command] {
			return "", fmt.Errorf("mock shell command failed: %s", command)
		}
		// If base command fails but sudo doesn't, allow sudo to succeed
		if baseCommand == m.FailingCommand {
			return "mock output", nil
		}
	}

	// Check for partial command matches for non-sudo commands
	if m.FailingCommand != "" && !strings.HasPrefix(command, "sudo ") && strings.Contains(command, m.FailingCommand) {
		return "", fmt.Errorf("mock shell command failed: %s", command)
	}

	// Handle specific command patterns for realistic mock responses
	if strings.Contains(command, "apt-cache policy") {
		if strings.Contains(command, "failing-package") {
			// Return output indicating package is not available
			return `N: Unable to locate package failing-package`, nil
		}
		// Return mock apt-cache policy output that indicates package is available
		return `test-package:
  Installed: (none)
  Candidate: 1.0.0
  Version table:
     1.0.0 500
        500 http://archive.ubuntu.com/ubuntu focal/main amd64 Packages`, nil
	}

	if strings.Contains(command, "which") {
		// Most which commands should succeed
		return "/usr/bin/command", nil
	}

	if strings.Contains(command, "dpkg --version") {
		return "Debian dpkg package management program version 1.20.5", nil
	}

	if command == "whoami" {
		return "testuser", nil
	}

	if strings.Contains(command, "systemctl") {
		// Mock systemctl commands for Docker setup
		return "mock systemctl output", nil
	}

	if strings.Contains(command, "docker.io") && strings.Contains(command, "apt-cache policy") {
		// Return mock apt-cache policy output for docker.io package
		return `docker.io:
  Installed: (none)
  Candidate: 20.10.12-0ubuntu2~20.04.1
  Version table:
     20.10.12-0ubuntu2~20.04.1 500
        500 http://archive.ubuntu.com/ubuntu focal-updates/universe amd64 Packages`, nil
	}

	// Handle apt-get install commands - mark packages as installed
	if strings.Contains(command, "sudo apt-get install -y") {
		parts := strings.Fields(command)
		if len(parts) >= 4 {
			packageName := parts[len(parts)-1] // Last argument is the package name
			m.InstallationState[packageName] = true
		}
		return "Reading package lists...\nBuilding dependency tree...\nPackage installed successfully", nil
	}

	// Handle dpkg-query commands for installation verification
	if strings.Contains(command, "dpkg-query -W -f='${Status}'") {
		// Extract package name from command
		parts := strings.Fields(command)
		if len(parts) >= 3 {
			packageName := parts[len(parts)-1]
			if m.InstallationState[packageName] {
				return "install ok installed", nil
			}
		}
		// For other packages, return not installed
		return "", fmt.Errorf("dpkg-query: no packages found matching package")
	}

	// Handle dpkg -l commands (alternative installation check)
	if strings.Contains(command, "dpkg -l") {
		parts := strings.Fields(command)
		if len(parts) >= 3 {
			packageName := parts[len(parts)-1]
			if m.InstallationState[packageName] {
				return fmt.Sprintf("ii  %s    1.0.0    amd64    Test package description", packageName), nil
			}
		}
		// For other packages, return not found
		return "", fmt.Errorf("dpkg-query: no packages found matching package")
	}

	// Handle dpkg --print-architecture (used by APT source functions)
	if strings.Contains(command, "dpkg --print-architecture") {
		return "amd64", nil
	}

	// Handle lsb_release commands (used for codename detection)
	if strings.Contains(command, "lsb_release -cs") {
		return "focal", nil
	}

	// Handle Docker-specific commands
	if strings.Contains(command, "docker version") {
		return "24.0.2", nil
	}

	// Handle Docker container listing
	if strings.Contains(command, "docker ps -a --format {{.Names}}") {
		// Return container names based on installation state
		var containers []string
		for containerName, installed := range m.InstallationState {
			if installed {
				containers = append(containers, containerName)
			}
		}
		return strings.Join(containers, "\n"), nil
	}

	// Handle Docker run commands - mark container as installed
	if strings.Contains(command, "docker run") && strings.Contains(command, "--name") {
		// Extract container name from docker run command
		parts := strings.Fields(command)
		for i, part := range parts {
			if part == "--name" && i+1 < len(parts) {
				containerName := parts[i+1]
				m.InstallationState[containerName] = true
				break
			}
		}
		return "container started successfully", nil
	}

	// Handle file existence checks
	if strings.Contains(command, "test -S /var/run/docker.sock") {
		// Return success unless specifically set to fail
		if m.FailingCommand == "test -S /var/run/docker.sock" {
			return "", fmt.Errorf("socket not found")
		}
		return "", nil
	}

	// Handle container detection commands
	if strings.Contains(command, "cat /proc/1/cgroup") {
		return "false", nil // Not in container by default
	}

	if command == "hostname" {
		// Check if HOSTNAME env var is set to container-like value
		if hostname := os.Getenv("HOSTNAME"); hostname != "" && len(hostname) == 12 {
			return hostname, nil
		}
		return "test-host", nil // Normal hostname, not container-like
	}

	// Handle sleep commands
	if strings.Contains(command, "sleep") {
		return "", nil
	}

	// Handle Docker daemon startup commands - should generally fail in test environment
	if strings.Contains(command, "sudo service docker start 2>/dev/null || sudo systemctl start docker 2>/dev/null || sudo dockerd") {
		return "", fmt.Errorf("mock daemon startup failed")
	}

	// Handle Flatpak commands
	if strings.Contains(command, "flatpak list --columns=application") {
		// Check if this command should fail
		if m.FailingCommands[command] || m.FailingCommands["flatpak list --columns=application"] {
			return "", fmt.Errorf("mock flatpak list command failed")
		}
		// Return installed applications based on installation state
		var installedApps []string
		for appID, installed := range m.InstallationState {
			if installed {
				installedApps = append(installedApps, appID)
			}
		}
		return strings.Join(installedApps, "\n"), nil
	}

	// Handle Flatpak install commands - mark app as installed
	if strings.Contains(command, "flatpak install -y") {
		// Check if this command should fail
		if m.FailingCommands[command] {
			return "", fmt.Errorf("mock flatpak install command failed")
		}

		// Handle specific case of "flatpak install -y " (with trailing space but no app ID)
		if command == "flatpak install -y " {
			return "", fmt.Errorf("mock flatpak install failed: no application ID provided")
		}

		// Extract app ID from flatpak install command
		parts := strings.Fields(command)
		switch {
		case len(parts) >= 4:
			appID := strings.Join(parts[3:], " ") // Everything after "flatpak install -y"
			// Handle empty app ID case
			if strings.TrimSpace(appID) == "" {
				return "", fmt.Errorf("mock flatpak install failed: empty application ID")
			}
			m.InstallationState[appID] = true
		case len(parts) == 3 && strings.HasSuffix(command, "flatpak install -y"):
			// Handle case where command ends with "flatpak install -y" with no app ID
			return "", fmt.Errorf("mock flatpak install failed: no application ID provided")
		default:
			// Malformed command case
			return "", fmt.Errorf("mock flatpak install failed: malformed command")
		}
		return "Installing application... Done.", nil
	}

	// Handle Pip show commands for installation verification
	if strings.Contains(command, "pip show") {
		// Check if this command should fail
		if m.FailingCommands[command] {
			return "", fmt.Errorf("mock pip show command failed")
		}

		// Extract package name from pip show command
		parts := strings.Fields(command)
		if len(parts) >= 3 {
			packageName := parts[2] // "pip show package-name"
			if m.InstallationState[packageName] {
				return fmt.Sprintf("Name: %s\nVersion: 1.0.0\nSummary: Test package\n", packageName), nil
			}
		}
		// For packages not in installation state, return error (package not found)
		return "", fmt.Errorf("WARNING: Package(s) not found")
	}

	// Handle Pip install commands - mark package as installed
	if strings.Contains(command, "pip install") {
		// Check if this command should fail
		if m.FailingCommands[command] {
			return "", fmt.Errorf("mock pip install command failed")
		}

		// Handle specific case of "pip install " (with trailing space but no package)
		if command == "pip install " {
			return "", fmt.Errorf("mock pip install failed: no package name provided")
		}

		// Extract package name from pip install command
		parts := strings.Fields(command)
		switch {
		case len(parts) >= 3:
			packageName := parts[2] // "pip install package-name"
			// Handle empty package name case
			if strings.TrimSpace(packageName) == "" {
				return "", fmt.Errorf("mock pip install failed: empty package name")
			}
			m.InstallationState[packageName] = true
		case len(parts) == 2 && strings.HasSuffix(command, "pip install"):
			// Handle case where command ends with "pip install" with no package
			return "", fmt.Errorf("mock pip install failed: no package name provided")
		default:
			// Malformed command case
			return "", fmt.Errorf("mock pip install failed: malformed command")
		}
		return "Successfully installed package", nil
	}

	// Handle DNF/YUM commands and zypper rpm -q commands
	if strings.Contains(command, "rpm -q") {
		// Extract package name from rpm -q command
		parts := strings.Fields(command)
		if len(parts) >= 3 {
			packageName := parts[2] // "rpm -q package-name"

			// Handle patterns and products - strip prefixes for lookup
			lookupName := packageName
			if strings.HasPrefix(packageName, "pattern:") {
				lookupName = strings.TrimPrefix(packageName, "pattern:")
			} else if strings.HasPrefix(packageName, "product:") {
				lookupName = strings.TrimPrefix(packageName, "product:")
			}

			if m.InstallationState[lookupName] || m.InstallationState[packageName] {
				return fmt.Sprintf("%s-1.0-1.x86_64", packageName), nil
			}
		}
		// For packages not in installation state, return error (package not installed)
		return "package not installed", fmt.Errorf("package not installed")
	}

	// Handle DNF install commands - mark package as installed
	if strings.Contains(command, "sudo dnf install -y") || strings.Contains(command, "sudo yum install -y") {
		parts := strings.Fields(command)
		if len(parts) >= 4 {
			packageName := parts[len(parts)-1] // Last argument is the package name
			m.InstallationState[packageName] = true
		}
		return "Package installed successfully", nil
	}

	// Handle DNF group install commands
	if strings.Contains(command, "sudo dnf group install -y") || strings.Contains(command, "sudo yum groupinstall -y") {
		// Extract group name (it's in quotes, so we need to handle that)
		start := strings.Index(command, "'")
		end := strings.LastIndex(command, "'")
		if start != -1 && end != -1 && start != end {
			groupName := command[start+1 : end]
			m.InstallationState[groupName] = true
		}
		return "Group installed successfully", nil
	}

	// Handle DNF/YUM info commands
	if strings.Contains(command, "dnf info") || strings.Contains(command, "yum info") {
		parts := strings.Fields(command)
		if len(parts) >= 3 {
			packageName := parts[2]
			// For known packages, return package info
			if !strings.Contains(packageName, "nonexistent") && !strings.Contains(packageName, "failing") {
				return fmt.Sprintf("Name        : %s\nAvailable Packages", packageName), nil
			}
		}
		return "No matching packages to list", fmt.Errorf("no matching packages")
	}

	// Handle DNF/YUM check-update commands
	if strings.Contains(command, "sudo dnf check-update") || strings.Contains(command, "sudo yum check-update") {
		// Mock successful check-update
		return "Checking for updates...", nil
	}

	// Handle rpm --version
	if strings.Contains(command, "rpm --version") {
		return "RPM version 4.16.0", nil
	}

	// Handle repository commands
	if strings.Contains(command, "tee /etc/yum.repos.d/") {
		return "Repository configuration written", nil
	}

	// Handle EPEL install commands
	if strings.Contains(command, "epel-release") {
		m.InstallationState["epel-release"] = true
		return "EPEL repository enabled", nil
	}

	// Handle Zypper commands (SUSE package manager)
	if strings.Contains(command, "sudo zypper install --non-interactive") {
		// Handle zypper install commands - mark package as installed
		parts := strings.Fields(command)
		if len(parts) >= 4 {
			// Handle different package types
			if strings.Contains(command, "-t pattern") {
				// Pattern installation: sudo zypper install --non-interactive -t pattern patternname
				for i, part := range parts {
					if part == "pattern" && i+1 < len(parts) {
						patternName := parts[i+1]
						m.InstallationState[patternName] = true
						break
					}
				}
			} else if strings.Contains(command, "-t product") {
				// Product installation: sudo zypper install --non-interactive -t product productname
				for i, part := range parts {
					if part == "product" && i+1 < len(parts) {
						productName := parts[i+1]
						m.InstallationState[productName] = true
						break
					}
				}
			} else {
				// Regular package installation
				packageName := parts[len(parts)-1]
				m.InstallationState[packageName] = true
			}
		}
		return "Package installed successfully", nil
	}

	// Handle zypper remove commands - mark package as uninstalled
	if strings.Contains(command, "sudo zypper remove --non-interactive") {
		parts := strings.Fields(command)
		if len(parts) >= 4 {
			packageName := parts[len(parts)-1]
			m.InstallationState[packageName] = false
		}
		return "Package removed successfully", nil
	}

	// Handle zypper info commands for package availability checking
	if strings.Contains(command, "zypper info --non-interactive") {
		parts := strings.Fields(command)
		if len(parts) >= 3 {
			packageName := parts[len(parts)-1]
			// Simulate package not found for certain test packages
			if strings.Contains(packageName, "test-unavailable") || strings.Contains(packageName, "nonexistent") {
				return "package 'test-unavailable' not found", fmt.Errorf("package not found")
			}
			// For other packages, return mock package info
			return fmt.Sprintf("Information for package %s:\nRepository     : Main Repository\nName           : %s", packageName, packageName), nil
		}
	}

	// Handle zypper refresh commands
	if strings.Contains(command, "sudo zypper refresh --non-interactive") {
		return "Repository metadata refreshed", nil
	}

	// Handle zypper update/upgrade commands
	if strings.Contains(command, "sudo zypper update --non-interactive") {
		return "System updated successfully", nil
	}

	if strings.Contains(command, "sudo zypper dup --non-interactive") {
		return "Distribution upgrade completed", nil
	}

	// Handle zypper clean commands
	if strings.Contains(command, "sudo zypper clean --all") {
		return "All caches cleaned", nil
	}

	// Handle zypper search commands
	if strings.Contains(command, "zypper search --installed-only --type package") {
		// Return installed packages
		var installedPackages []string
		for pkg, installed := range m.InstallationState {
			if installed {
				installedPackages = append(installedPackages, fmt.Sprintf("i | %s | Test package", pkg))
			}
		}
		return strings.Join(installedPackages, "\n"), nil
	}

	if strings.Contains(command, "zypper search --type package") || strings.Contains(command, "zypper search --type pattern") {
		// Extract search query
		parts := strings.Fields(command)
		if len(parts) >= 4 {
			query := parts[len(parts)-1]
			if strings.Contains(command, "--type pattern") {
				return fmt.Sprintf("v | %s | Pattern description", query), nil
			}
			return fmt.Sprintf("v | %s | Package description", query), nil
		}
	}

	// Handle zypper repository management commands
	if strings.Contains(command, "sudo zypper addrepo --refresh") {
		return "Repository added successfully", nil
	}

	if strings.Contains(command, "sudo zypper removerepo") {
		return "Repository removed successfully", nil
	}

	// Handle zypper lock/unlock commands
	if strings.Contains(command, "sudo zypper addlock") {
		return "Package locked successfully", nil
	}

	if strings.Contains(command, "sudo zypper removelock") {
		return "Package unlocked successfully", nil
	}

	// Handle rpm --import for GPG keys
	if strings.Contains(command, "sudo rpm --import") {
		return "GPG key imported successfully", nil
	}

	return "mock output", nil
}

func (m *MockCommandExecutor) RunCommand(ctx context.Context, name string, args ...string) (string, error) {
	command := fmt.Sprintf("%s %s", name, args)
	m.Commands = append(m.Commands, command)
	if command == m.FailingCommand {
		return "", errors.New("mock command failed")
	}
	return "mock output", nil
}

func (m *MockCommandExecutor) DownloadFileWithContext(ctx context.Context, url, filepath string) error {
	// Simulate a successful or failing file download based on the URL
	if url == m.FailingCommand {
		return fmt.Errorf("mock download failed for url: %s", url)
	}
	m.Commands = append(m.Commands, fmt.Sprintf("download %s to %s", url, filepath))

	// For test URLs, simulate creating a mock GPG key file
	if strings.Contains(url, "example.com") || strings.Contains(url, "test") {
		// Create a mock GPG key content to satisfy file size checks
		_ = "-----BEGIN PGP PUBLIC KEY BLOCK-----\nMock GPG key content for testing\n-----END PGP PUBLIC KEY BLOCK-----"
		// We don't actually write to filesystem in tests but this simulates success
	}

	return nil
}

// ExecuteCommand implements CommandExecutor.ExecuteCommand
func (m *MockCommandExecutor) ExecuteCommand(ctx context.Context, command string) (*exec.Cmd, error) {
	m.Commands = append(m.Commands, command)

	// Check if this command should fail
	if command == m.FailingCommand || m.FailingCommands[command] {
		return nil, fmt.Errorf("mock command execution failed: %s", command)
	}

	// Return a mock command that will not actually execute
	// This is safe for testing as we're not running real commands
	cmd := exec.CommandContext(ctx, "echo", "mock-execution")
	return cmd, nil
}

// ValidateCommand implements CommandExecutor.ValidateCommand
func (m *MockCommandExecutor) ValidateCommand(command string) error {
	// Basic validation - reject empty commands
	if strings.TrimSpace(command) == "" {
		return fmt.Errorf("empty command")
	}

	// For testing purposes, allow most commands but reject ones marked as failing
	if command == m.FailingCommand || m.FailingCommands[command] {
		return fmt.Errorf("mock command validation failed: %s", command)
	}

	return nil
}
