package mocks

import (
	"context"
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

	// Handle dpkg-query commands for checking installation status
	if strings.Contains(command, "dpkg-query -W -f='${Status}'") {
		// For edge cases, return that package is not installed so validation continues
		if strings.Contains(command, ".") || strings.Contains(command, "-") || strings.Contains(command, " ") || strings.Contains(command, "\t") || strings.Contains(command, "\n") {
			return "", fmt.Errorf("dpkg-query failed for invalid package name")
		}
		// For normal packages, return not installed status
		return "deinstall ok config-files", nil
	}

	if strings.Contains(command, "which") {
		// Check if this specific which command should fail first
		if command == m.FailingCommand || m.FailingCommands[command] {
			return "", fmt.Errorf("mock shell command failed: %s", command)
		}
		// Handle specific package manager detection
		if strings.Contains(command, "which dnf") {
			return "/usr/bin/dnf", nil
		}
		if strings.Contains(command, "which yum") {
			return "/usr/bin/yum", nil
		}
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

	// Handle Docker ps with filters for IsInstalled checks
	if strings.Contains(command, "docker ps") && strings.Contains(command, "--filter") {
		// Extract container name from filter
		var targetContainer string
		parts := strings.Fields(command)
		for i, part := range parts {
			if part == "--filter" && i+1 < len(parts) {
				filterValue := parts[i+1]
				if strings.HasPrefix(filterValue, "name=") {
					targetContainer = strings.TrimPrefix(filterValue, "name=")
					break
				}
			}
		}

		// Check if the target container is "installed" (running)
		if targetContainer != "" && m.InstallationState[targetContainer] {
			return targetContainer, nil
		}
		return "", nil // Container not running
	}

	// Handle sudo docker ps commands with filters
	if strings.Contains(command, "sudo docker ps") && strings.Contains(command, "--filter") {
		// Extract container name from filter
		var targetContainer string
		parts := strings.Fields(command)
		for i, part := range parts {
			if part == "--filter" && i+1 < len(parts) {
				filterValue := parts[i+1]
				if strings.HasPrefix(filterValue, "name=") {
					targetContainer = strings.TrimPrefix(filterValue, "name=")
					break
				}
			}
		}

		// Check if the target container is "installed" (running)
		if targetContainer != "" && m.InstallationState[targetContainer] {
			return targetContainer, nil
		}
		return "", nil // Container not running
	}

	// Handle Pacman-specific commands
	if strings.Contains(command, "pacman --version") {
		return "Pacman v6.0.2 - libalpm v13.0.2", nil
	}

	// Handle pacman -Q (check if package is installed)
	if strings.Contains(command, "pacman -Q") && !strings.Contains(command, "pacman -Qi") && !strings.Contains(command, "pacman -Qs") {
		parts := strings.Fields(command)
		if len(parts) >= 3 {
			packageName := parts[2] // "pacman -Q package-name"
			if m.InstallationState[packageName] {
				return fmt.Sprintf("%s 1.0.0-1", packageName), nil
			}
		}
		// For packages not in installation state, return error (package not found)
		return "error: package 'test-package' was not found", fmt.Errorf("package was not found")
	}

	// Handle pacman -Si (package info from repositories)
	if strings.Contains(command, "pacman -Si") {
		parts := strings.Fields(command)
		if len(parts) >= 3 {
			packageName := parts[2]
			if strings.Contains(packageName, "failing-package") || strings.Contains(packageName, "nonexistent") {
				return "error: package 'failing-package' was not found", fmt.Errorf("package was not found")
			}
			// Return mock package info
			return fmt.Sprintf("Repository      : core\nName            : %s\nVersion         : 1.0.0-1\nDescription     : Test package", packageName), nil
		}
	}

	// Handle pacman install commands - mark packages as installed
	if strings.Contains(command, "sudo pacman -S --noconfirm") {
		// Extract package names from command
		parts := strings.Fields(command)
		if len(parts) >= 4 {
			// All arguments after "sudo pacman -S --noconfirm" are package names
			for i := 4; i < len(parts); i++ {
				packageName := parts[i]
				m.InstallationState[packageName] = true
			}
		}
		return "resolving dependencies...\npackage installed successfully", nil
	}

	// Handle pacman remove commands - mark packages as uninstalled
	if strings.Contains(command, "sudo pacman -Rs --noconfirm") {
		parts := strings.Fields(command)
		if len(parts) >= 4 {
			packageName := parts[4] // "sudo pacman -Rs --noconfirm package-name"
			m.InstallationState[packageName] = false
		}
		return "checking dependencies...\npackage removed successfully", nil
	}

	// Handle pacman database update
	if strings.Contains(command, "sudo pacman -Sy") {
		return "synchronizing package databases...", nil
	}

	// Handle pacman system upgrade
	if strings.Contains(command, "sudo pacman -Syu --noconfirm") {
		return "upgrading system...\nupgrade complete", nil
	}

	// Handle pacman cache clean
	if strings.Contains(command, "sudo pacman -Sc --noconfirm") {
		return "removing old packages from cache...", nil
	}

	// Handle pacman list installed packages
	if command == "pacman -Q" {
		var packages []string
		for pkg, installed := range m.InstallationState {
			if installed {
				packages = append(packages, fmt.Sprintf("%s 1.0.0-1", pkg))
			}
		}
		return strings.Join(packages, "\n"), nil
	}

	// Handle pacman search
	if strings.Contains(command, "pacman -Ss") {
		parts := strings.Fields(command)
		if len(parts) >= 3 {
			query := parts[2]
			return fmt.Sprintf("core/%s 1.0.0-1\n    Test package matching %s", query, query), nil
		}
	}

	// Handle YAY commands (AUR helper)
	if strings.Contains(command, "yay -Si") {
		parts := strings.Fields(command)
		if len(parts) >= 3 {
			packageName := parts[2]
			if strings.Contains(packageName, "failing-package") {
				return "error: package 'failing-package' was not found", fmt.Errorf("package was not found")
			}
			// Return mock AUR package info
			return fmt.Sprintf("Repository      : aur\nName            : %s\nVersion         : 1.0.0-1\nDescription     : AUR package", packageName), nil
		}
	}

	// Handle YAY version check
	if strings.Contains(command, "yay --version") {
		return "yay v12.1.3 - libalpm v13.0.2", nil
	}

	if strings.Contains(command, "yay -S --noconfirm") {
		parts := strings.Fields(command)
		if len(parts) >= 4 {
			packageName := parts[3] // "yay -S --noconfirm package-name"
			m.InstallationState[packageName] = true
		}
		return "building package from AUR...\npackage installed successfully", nil
	}

	if strings.Contains(command, "yay -Ss") {
		parts := strings.Fields(command)
		if len(parts) >= 3 {
			query := parts[2]
			return fmt.Sprintf("aur/%s-git 1.0.0-1\n    AUR package matching %s", query, query), nil
		}
	}

	// Handle git clone for YAY installation
	if strings.Contains(command, "git clone https://aur.archlinux.org/yay.git") {
		return "Cloning into 'yay'...\nclone complete", nil
	}

	// Handle makepkg for building YAY
	if strings.Contains(command, "makepkg -si --noconfirm") {
		m.InstallationState["yay"] = true
		return "building package...\npackage built and installed successfully", nil
	}

	// Handle git status checks in YAY build directory
	if strings.Contains(command, "git status") && strings.Contains(command, "yay") {
		return "On branch master\nnothing to commit, working tree clean", nil
	}

	// Handle git pull in YAY build directory
	if strings.Contains(command, "git pull") && strings.Contains(command, "yay") {
		return "Already up to date.", nil
	}

	// Handle base-devel group installation
	if strings.Contains(command, "sudo pacman -S --noconfirm base-devel") {
		m.InstallationState["base-devel"] = true
		return "installing base-devel group...\ninstallation complete", nil
	}

	// Handle git package installation check
	if strings.Contains(command, "pacman -Q git") {
		if m.InstallationState["git"] {
			return "git 2.40.1-1", nil
		}
		return "error: package 'git' was not found", fmt.Errorf("package was not found")
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

	// Handle network validation commands
	if strings.Contains(command, "nslookup") {
		// Mock successful DNS resolution
		return "Server: 8.8.8.8\nAddress: 8.8.8.8#53\nNon-authoritative answer:\nName: google.com", nil
	}

	if strings.Contains(command, "ping -c 1 -W 3") {
		// Mock successful ping
		return "PING google.com (172.217.164.142) 56(84) bytes of data.\n64 bytes from lga25s57-in-f14.1e100.net (172.217.164.142): icmp_seq=1 ttl=117 time=25.4 ms", nil
	}

	// Handle permission check commands (touch and rm)
	if strings.Contains(command, "touch /tmp/devex-permission-test-") && strings.Contains(command, "&& rm -f") {
		// Mock successful permission check
		return "", nil
	}

	// Handle Docker daemon startup commands - should generally fail in test environment
	if strings.Contains(command, "sudo service docker start 2>/dev/null || sudo systemctl start docker 2>/dev/null || sudo dockerd") {
		return "", fmt.Errorf("mock daemon startup failed")
	}

	// Handle Flatpak commands
	if strings.Contains(command, "flatpak list") {
		// Check if this command should fail
		if m.FailingCommands[command] || m.FailingCommands["flatpak list --columns=application"] {
			return "", fmt.Errorf("mock flatpak list command failed")
		}

		// Handle grep pattern if present
		if strings.Contains(command, "grep -q") {
			// Extract the app ID from the grep pattern
			// Command format: "flatpak list --columns=application | grep -q '^appID$'"
			// or "flatpak list --user --columns=application | grep -q '^appID$'"
			parts := strings.Split(command, "grep -q")
			if len(parts) >= 2 {
				grepPattern := strings.TrimSpace(parts[1])
				// Remove quotes and regex anchors
				grepPattern = strings.Trim(grepPattern, " '\"")
				grepPattern = strings.TrimPrefix(grepPattern, "^")
				grepPattern = strings.TrimSuffix(grepPattern, "$")

				// Check if this app is installed
				if m.InstallationState[grepPattern] {
					return grepPattern, nil // grep found the pattern
				}
				// App not found - grep returns error
				return "", fmt.Errorf("pattern not found")
			}
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
		var appID string
		switch {
		case len(parts) >= 5 && !strings.Contains(parts[3], "."):
			// Format: "flatpak install -y remote appID"
			appID = parts[4] // The actual app ID after the remote
		case len(parts) >= 4:
			// Format: "flatpak install -y appID" or "flatpak install -y flathub firefox"
			if len(parts) == 5 {
				// Two arguments after -y means remote + appID
				appID = parts[4]
			} else {
				// Single argument after -y means just appID
				appID = parts[3]
			}
			// Handle empty app ID case
			if strings.TrimSpace(appID) == "" {
				return "", fmt.Errorf("mock flatpak install failed: empty application ID")
			}
		case len(parts) == 3 && strings.HasSuffix(command, "flatpak install -y"):
			// Handle case where command ends with "flatpak install -y" with no app ID
			return "", fmt.Errorf("mock flatpak install failed: no application ID provided")
		default:
			// Malformed command case
			return "", fmt.Errorf("mock flatpak install failed: malformed command")
		}

		// Store the app as installed
		if appID != "" {
			m.InstallationState[appID] = true
		}

		return "Installing application... Done.", nil
	}

	// Handle Flatpak uninstall commands - mark app as uninstalled
	if strings.Contains(command, "flatpak uninstall -y") {
		// Check if this command should fail
		if m.FailingCommands[command] {
			return "", fmt.Errorf("mock flatpak uninstall command failed")
		}

		// Extract app ID from flatpak uninstall command
		parts := strings.Fields(command)
		if len(parts) >= 4 {
			appID := parts[3]                  // "flatpak uninstall -y appID"
			delete(m.InstallationState, appID) // Remove from installed state
		}
		return "Uninstalling application... Done.", nil
	}

	// Handle Flatpak search commands
	if strings.Contains(command, "flatpak search") {
		// Check if this command should fail
		if m.FailingCommands[command] {
			return "", fmt.Errorf("mock flatpak search command failed")
		}
		// Return some mock search results
		parts := strings.Fields(command)
		if len(parts) >= 3 {
			query := parts[len(parts)-1]
			return fmt.Sprintf("Application Name  %s  Description", query), nil
		}
		return "No results found", nil
	}

	// Handle Flatpak remote-ls commands
	if strings.Contains(command, "flatpak remote-ls") {
		// Check if this command should fail (especially with grep)
		if strings.Contains(command, "grep") {
			// Extract the grep pattern
			parts := strings.Split(command, "grep -i")
			if len(parts) >= 2 {
				pattern := strings.TrimSpace(parts[1])
				if m.FailingCommands[command] {
					return "", fmt.Errorf("pattern not found")
				}
				return fmt.Sprintf("%s\tApplication", pattern), nil
			}
		}
		return "org.mozilla.firefox\tFirefox\norg.videolan.VLC\tVLC", nil
	}

	// Handle Flatpak remotes command
	if command == "flatpak remotes" {
		if m.FailingCommands[command] {
			return "", fmt.Errorf("mock flatpak remotes command failed")
		}
		return "flathub\tsystem", nil
	}

	// Handle Flatpak version command
	if strings.Contains(command, "flatpak --version") {
		if m.FailingCommands[command] {
			return "", fmt.Errorf("mock flatpak --version command failed")
		}
		return "Flatpak 1.14.4", nil
	}

	// Handle Flatpak update commands
	if strings.Contains(command, "flatpak update") {
		if m.FailingCommands[command] {
			return "", fmt.Errorf("mock flatpak update command failed")
		}
		return "Updating metadata...", nil
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
		// Extract group name (it may or may not be in quotes)
		var groupName string
		start := strings.Index(command, "'")
		end := strings.LastIndex(command, "'")
		if start != -1 && end != -1 && start != end {
			// Group name is in single quotes
			groupName = command[start+1 : end]
		} else {
			// Group name is not quoted, extract from end of command
			parts := strings.Fields(command)
			if len(parts) >= 5 {
				groupName = parts[len(parts)-1]
			}
		}
		if groupName != "" {
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

	// Handle DNF uninstall commands - mark package as uninstalled
	if strings.Contains(command, "sudo dnf remove -y") || strings.Contains(command, "sudo yum remove -y") {
		parts := strings.Fields(command)
		if len(parts) >= 4 {
			packageName := parts[len(parts)-1] // Last argument is the package name
			m.InstallationState[packageName] = false
		}
		return "Package removed successfully", nil
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
			switch {
			case strings.Contains(command, "-t pattern"):
				// Pattern installation: sudo zypper install --non-interactive -t pattern patternname
				for i, part := range parts {
					if part == "pattern" && i+1 < len(parts) {
						patternName := parts[i+1]
						m.InstallationState[patternName] = true
						break
					}
				}
			case strings.Contains(command, "-t product"):
				// Product installation: sudo zypper install --non-interactive -t product productname
				for i, part := range parts {
					if part == "product" && i+1 < len(parts) {
						productName := parts[i+1]
						m.InstallationState[productName] = true
						break
					}
				}
			default:
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

	// Handle Homebrew commands
	if strings.Contains(command, "brew --version") {
		if m.FailingCommands[command] {
			return "", fmt.Errorf("mock brew --version command failed")
		}
		return "Homebrew 4.0.10\nHomebrew/homebrew-core (git revision 123abc; last commit 2023-01-01)", nil
	}

	// Handle brew list commands for installation verification
	if strings.Contains(command, "brew list") {
		// Check if this command should fail
		if m.FailingCommands[command] {
			return "", fmt.Errorf("mock brew list command failed")
		}

		// Extract package name from brew list command
		parts := strings.Fields(command)
		if len(parts) >= 3 {
			packageName := parts[2] // "brew list package-name"
			if m.InstallationState[packageName] {
				return fmt.Sprintf("%s: version info", packageName), nil
			}
		}
		// For packages not in installation state, return error (package not found)
		return "", fmt.Errorf("no such keg: /usr/local/Cellar/package")
	}

	// Handle brew install commands - mark packages as installed
	if strings.Contains(command, "brew install") {
		// Check if this command should fail
		if m.FailingCommands[command] {
			return "", fmt.Errorf("mock brew install command failed")
		}

		// Extract package names from command
		parts := strings.Fields(command)
		if len(parts) >= 3 {
			// All arguments after "brew install" are package names
			for i := 2; i < len(parts); i++ {
				packageName := parts[i]
				m.InstallationState[packageName] = true
			}
		}
		return "Installing packages... Done.", nil
	}

	// Handle brew uninstall commands - mark packages as uninstalled
	if strings.Contains(command, "brew uninstall") {
		// Check if this command should fail
		if m.FailingCommands[command] {
			return "", fmt.Errorf("mock brew uninstall command failed")
		}

		// Extract package name from brew uninstall command
		parts := strings.Fields(command)
		if len(parts) >= 3 {
			packageName := parts[2]                  // "brew uninstall package-name"
			delete(m.InstallationState, packageName) // Remove from installed state
		}
		return "Uninstalling package... Done.", nil
	}

	// Handle brew search commands
	if strings.Contains(command, "brew search") {
		// Check if this command should fail
		if m.FailingCommands[command] {
			return "", fmt.Errorf("mock brew search command failed")
		}

		// Extract search query
		parts := strings.Fields(command)
		if len(parts) >= 3 {
			query := parts[2]
			if strings.Contains(query, "nonexistent") || strings.Contains(query, "failing") {
				return "No formula or cask found for \"" + query + "\"", nil
			}
			return fmt.Sprintf("%s  %s-dev  %s-tools", query, query, query), nil
		}
		return "No results found", nil
	}

	// Handle brew update commands
	if strings.Contains(command, "brew update") {
		if m.FailingCommands[command] {
			return "", fmt.Errorf("mock brew update command failed")
		}
		return "Updating Homebrew...", nil
	}

	// Handle brew cleanup commands
	if strings.Contains(command, "brew cleanup") {
		if m.FailingCommands[command] {
			return "", fmt.Errorf("mock brew cleanup command failed")
		}
		return "Cleaning up packages...", nil
	}

	// Handle brew --prefix commands
	if strings.Contains(command, "brew --prefix") {
		if m.FailingCommands[command] {
			return "", fmt.Errorf("mock brew --prefix command failed")
		}
		return "/usr/local", nil
	}

	return "mock output", nil
}

func (m *MockCommandExecutor) RunCommand(ctx context.Context, name string, args ...string) (string, error) {
	command := fmt.Sprintf("%s %s", name, strings.Join(args, " "))
	m.Commands = append(m.Commands, command)

	// Handle Docker ps with filters for IsInstalled checks (needed in RunCommand)
	if strings.Contains(command, "docker ps") && strings.Contains(command, "--filter") {
		// Extract container name from filter
		var targetContainer string
		parts := strings.Fields(command)
		for i, part := range parts {
			if part == "--filter" && i+1 < len(parts) {
				filterValue := parts[i+1]
				if strings.HasPrefix(filterValue, "name=") {
					targetContainer = strings.TrimPrefix(filterValue, "name=")
					break
				}
			}
		}

		// Check if the target container is "installed" (running)
		if targetContainer != "" && m.InstallationState[targetContainer] {
			return targetContainer, nil
		}
		return "", nil // Container not running
	}

	// Check for exact command match first
	if command == m.FailingCommand {
		return "", fmt.Errorf("mock command failed: %s", command)
	}

	// Check multiple failing commands
	if m.FailingCommands[command] {
		return "", fmt.Errorf("mock command failed: %s", command)
	}

	// Handle specific command patterns for realistic mock responses
	if name == "apt-cache" && len(args) >= 2 && args[0] == "policy" {
		packageName := args[1]

		// Handle edge cases that should fail early
		if packageName == "." || packageName == ".." || packageName == "-" || packageName == "--" ||
			strings.TrimSpace(packageName) == "" || strings.ContainsAny(packageName, "\n\t") {
			return "", fmt.Errorf("apt-cache policy command failed for invalid package name")
		}

		if packageName == "failing-package" {
			// Return output indicating package is not available
			return `N: Unable to locate package failing-package`, nil
		}
		// Return mock apt-cache policy output that indicates package is available
		return fmt.Sprintf(`%s:
  Installed: (none)
  Candidate: 1.0.0
  Version table:
     1.0.0 500
        500 http://archive.ubuntu.com/ubuntu focal/main amd64 Packages`, packageName), nil
	}

	// Handle sudo apt install commands
	if name == "sudo" && len(args) >= 3 && (args[0] == "apt" || args[0] == "apt-get") && args[1] == "install" {
		packageName := args[len(args)-1] // Last argument is the package name
		if packageName == "failing-package" {
			return "", fmt.Errorf("mock install failed: package not found")
		}
		// Mark package as installed for mock tracking
		m.InstallationState[packageName] = true
		return fmt.Sprintf("Reading package lists...\nBuilding dependency tree...\nReading state information...\nThe following NEW packages will be installed:\n  %s\n0 upgraded, 1 newly installed, 0 to remove and 0 not upgraded.\nProcessing triggers for systemd (245.4-4ubuntu3.18) ...", packageName), nil
	}

	// Handle sudo apt remove commands
	if name == "sudo" && len(args) >= 3 && (args[0] == "apt" || args[0] == "apt-get") && args[1] == "remove" {
		packageName := args[len(args)-1] // Last argument is the package name
		// Mark package as uninstalled for mock tracking
		m.InstallationState[packageName] = false
		return fmt.Sprintf("Reading package lists...\nBuilding dependency tree...\nReading state information...\nThe following packages will be REMOVED:\n  %s\n0 upgraded, 0 newly installed, 1 to remove and 0 not upgraded.\nProcessing triggers for systemd (245.4-4ubuntu3.18) ...", packageName), nil
	}

	// Handle which commands for package manager detection
	if name == "which" && len(args) >= 1 {
		commandName := args[0]
		// Check if this specific which command should fail
		if m.FailingCommand == command || m.FailingCommands[command] {
			return "", fmt.Errorf("mock which command failed: %s", commandName)
		}
		// Return path for available commands
		return fmt.Sprintf("/usr/bin/%s", commandName), nil
	}

	// Handle sudo dnf/yum install commands
	if name == "sudo" && len(args) >= 4 && (args[0] == "dnf" || args[0] == "yum") && args[1] == "install" && args[2] == "-y" {
		packageName := args[len(args)-1] // Last argument is the package name
		if packageName == "failing-package" {
			return "", fmt.Errorf("mock install failed: package not found")
		}
		// Mark package as installed for mock tracking
		m.InstallationState[packageName] = true
		return fmt.Sprintf("Installing package %s...\nComplete!", packageName), nil
	}

	// Handle sudo dnf/yum remove commands
	if name == "sudo" && len(args) >= 4 && (args[0] == "dnf" || args[0] == "yum") && args[1] == "remove" && args[2] == "-y" {
		packageName := args[len(args)-1] // Last argument is the package name
		// Mark package as uninstalled for mock tracking
		m.InstallationState[packageName] = false
		return fmt.Sprintf("Removing package %s...\nComplete!", packageName), nil
	}

	// Handle sudo dnf/yum group install commands
	if name == "sudo" && len(args) >= 5 && (args[0] == "dnf" || args[0] == "yum") {
		if (args[0] == "dnf" && args[1] == "group" && args[2] == "install" && args[3] == "-y") ||
			(args[0] == "yum" && args[1] == "groupinstall" && args[2] == "-y") {
			groupName := args[len(args)-1] // Last argument is the group name
			// Remove quotes if present for consistent tracking
			cleanGroupName := strings.Trim(groupName, "'\"")
			// Mark group as installed for mock tracking
			m.InstallationState[cleanGroupName] = true
			return fmt.Sprintf("Installing group %s...\nComplete!", groupName), nil
		}
	}

	// Handle sudo dnf/yum install for EPEL
	if name == "sudo" && len(args) >= 4 && (args[0] == "dnf" || args[0] == "yum") && args[1] == "install" && args[2] == "-y" && args[3] == "epel-release" {
		m.InstallationState["epel-release"] = true
		return "EPEL repository enabled", nil
	}

	// Handle rpm -q commands for DNF/YUM
	if name == "rpm" && len(args) >= 2 && args[0] == "-q" {
		packageName := args[1]
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
		// For packages not in installation state, return error (package not installed)
		return "package not installed", fmt.Errorf("package not installed")
	}

	// Handle rpm --version
	if name == "rpm" && len(args) >= 1 && args[0] == "--version" {
		return "RPM version 4.16.0", nil
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
