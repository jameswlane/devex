package commands

import (
	"fmt"
	"strings"

	"github.com/jameswlane/devex/pkg/installers/docker"
	"github.com/jameswlane/devex/pkg/log"
	"github.com/jameswlane/devex/pkg/types"
)

// getSelectedDatabases returns the names of selected databases
func (m *SetupModel) getSelectedDatabases() []string {
	var selected []string
	for i, isSelected := range m.selectedDBs {
		if isSelected && i < len(m.databases) {
			selected = append(selected, m.databases[i])
		}
	}
	return selected
}

// getDockerApp returns a CrossPlatformApp for Docker Engine installation
func (m *SetupModel) getDockerApp() *types.CrossPlatformApp {
	return &types.CrossPlatformApp{
		Name:        "docker",
		Description: "Container platform and runtime for developing, shipping, and running applications",
		Linux: types.OSConfig{
			InstallMethod: "curlpipe",
			InstallCommand: `# Install Docker CE from official repository
set -euo pipefail  # Fail fast on any error

# Docker GPG key fingerprint for verification (Docker's official key)
DOCKER_GPG_KEY_FINGERPRINT="9DC858229FC7DD38854AE2D88D81803C0EBFCD88"

# Safely parse OS information without sourcing (prevents injection)
get_os_info() {
    if [ -f /etc/os-release ]; then
        # Parse ID safely without sourcing
        OS_ID=$(grep '^ID=' /etc/os-release | cut -d'=' -f2 | tr -d '"' | head -1)
        VERSION_CODENAME=$(grep '^VERSION_CODENAME=' /etc/os-release | cut -d'=' -f2 | tr -d '"' | head -1 || echo "")
        
        # Validate OS_ID contains only safe characters
        if ! echo "$OS_ID" | grep -qE '^[a-zA-Z0-9_-]+$'; then
            echo "Error: Invalid OS ID detected: $OS_ID"
            return 1
        fi
        
        export OS_ID VERSION_CODENAME
        return 0
    else
        echo "Error: /etc/os-release not found"
        return 1
    fi
}

# Download and verify GPG key with fingerprint check
setup_docker_gpg_key() {
    echo "Setting up Docker GPG key with fingerprint verification..."
    
    # Create keyring directory
    sudo mkdir -p /usr/share/keyrings
    
    # Download GPG key to temporary location
    TEMP_KEY=$(mktemp)
    if ! curl -fsSL "https://download.docker.com/linux/ubuntu/gpg" -o "$TEMP_KEY"; then
        echo "Error: Failed to download Docker GPG key"
        rm -f "$TEMP_KEY"
        return 1
    fi
    
    # Verify GPG key fingerprint
    KEY_FINGERPRINT=$(gpg --with-fingerprint --with-colons "$TEMP_KEY" 2>/dev/null | grep '^fpr:' | cut -d':' -f10 | head -1)
    if [ "$KEY_FINGERPRINT" != "$DOCKER_GPG_KEY_FINGERPRINT" ]; then
        echo "Error: GPG key fingerprint mismatch!"
        echo "Expected: $DOCKER_GPG_KEY_FINGERPRINT"
        echo "Got: $KEY_FINGERPRINT"
        rm -f "$TEMP_KEY"
        return 1
    fi
    
    # Import verified key
    sudo gpg --dearmor < "$TEMP_KEY" > /usr/share/keyrings/docker-archive-keyring.gpg
    sudo chmod 644 /usr/share/keyrings/docker-archive-keyring.gpg
    rm -f "$TEMP_KEY"
    
    echo "Docker GPG key verified and installed successfully"
    return 0
}

# Check if docker group exists, create if needed
ensure_docker_group() {
    if ! getent group docker >/dev/null 2>&1; then
        echo "Creating docker group..."
        sudo groupadd docker
    fi
}

# Main installation logic
main() {
    # Get OS information safely
    if ! get_os_info; then
        echo "Error: Could not detect OS information"
        return 1
    fi
    
    echo "Detected OS: $OS_ID"
    
    # Setup GPG key with verification
    if ! setup_docker_gpg_key; then
        echo "Error: Failed to setup Docker GPG key"
        return 1
    fi
    
    # Install Docker based on detected OS
    case "$OS_ID" in
        ubuntu|debian)
            if [ -z "$VERSION_CODENAME" ]; then
                VERSION_CODENAME=$(lsb_release -cs 2>/dev/null || echo "focal")
            fi
            echo "deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/docker-archive-keyring.gpg] https://download.docker.com/linux/$OS_ID $VERSION_CODENAME stable" | sudo tee /etc/apt/sources.list.d/docker.list > /dev/null
            sudo apt-get update
            sudo apt-get install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin docker-ce-rootless-extras
            ;;
        fedora|centos|rhel|rocky|almalinux)
            sudo dnf config-manager --add-repo https://download.docker.com/linux/centos/docker-ce.repo
            sudo dnf install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin
            ;;
        arch|manjaro)
            sudo pacman -S --noconfirm docker docker-compose
            ;;
        opensuse*|sles*)
            sudo zypper install -y docker docker-compose
            ;;
        *)
            echo "Error: Unsupported OS: $OS_ID"
            return 1
            ;;
    esac
    
    # Ensure docker group exists before adding user
    ensure_docker_group
    
    # Add current user to docker group with fallback methods
    username=""
    if [ -n "${USER:-}" ]; then
        username="$USER"
    elif [ -n "${USERNAME:-}" ]; then
        username="$USERNAME"
    elif command -v whoami >/dev/null 2>&1; then
        username=$(whoami)
    elif command -v id >/dev/null 2>&1; then
        username=$(id -un)
    fi
    
    # Validate username contains only safe characters
    if [ -n "$username" ] && echo "$username" | grep -qE '^[a-zA-Z0-9_-]+$'; then
        if [ "$username" != "root" ]; then
            sudo usermod -aG docker "$username"
            echo "Added user '$username' to docker group"
        fi
    else
        echo "Warning: Could not determine safe username for docker group setup"
    fi
    
    # Create secure Docker daemon configuration
    sudo mkdir -p /etc/docker
    sudo tee /etc/docker/daemon.json > /dev/null << 'EOF'
{
  "log-driver": "json-file",
  "log-opts": {
    "max-size": "100m",
    "max-file": "5"
  },
  "storage-driver": "overlay2",
  "live-restore": true,
  "userland-proxy": false,
  "no-new-privileges": true,
  "seccomp-profile": "/etc/docker/seccomp.json"
}
EOF
    
    # Set secure permissions on daemon config
    sudo chmod 644 /etc/docker/daemon.json
    sudo chown root:root /etc/docker/daemon.json
    
    # Enable and start Docker service
    sudo systemctl enable docker
    sudo systemctl start docker
    
    # Verify installation
    if sudo docker --version >/dev/null 2>&1; then
        echo "Docker installation completed successfully!"
        echo "Docker version: $(sudo docker --version)"
    else
        echo "Error: Docker installation may have failed"
        return 1
    fi
    
    echo "Note: You may need to log out and back in for Docker group permissions to take effect"
    echo "Or run: newgrp docker"
    
    return 0
}

# Execute main function
main`,
		},
		MacOS: types.OSConfig{
			InstallMethod:  "brew",
			InstallCommand: "docker",
		},
		Windows: types.OSConfig{
			InstallMethod:  "winget",
			InstallCommand: "Docker.DockerDesktop",
		},
	}
}

// getDatabaseApps creates pseudo-apps for database installations via Docker
func (m *SetupModel) getDatabaseApps() []types.CrossPlatformApp {
	var apps []types.CrossPlatformApp
	selectedDBs := m.getSelectedDatabases()

	dbConfigs := map[string]map[string]string{
		"PostgreSQL": {
			"image":     "postgres:16",
			"container": "postgres16",
			"port":      docker.PostgreSQLPort,
			"env":       "POSTGRES_HOST_AUTH_METHOD=trust",
		},
		"MySQL": {
			"image":     "mysql:8.4",
			"container": "mysql8",
			"port":      docker.MySQLPort,
			"env":       "MYSQL_ALLOW_EMPTY_PASSWORD=true",
		},
		"Redis": {
			"image":     "redis:7",
			"container": "redis",
			"port":      docker.RedisPort,
			"env":       "",
		},
	}

	for _, db := range selectedDBs {
		if dbConfig, exists := dbConfigs[db]; exists {
			// Build Docker command securely with validation
			dockerCmd, err := BuildSecureDockerCommand(
				dbConfig["container"],
				dbConfig["image"],
				dbConfig["port"],
				dbConfig["env"],
			)
			if err != nil {
				log.Error("Invalid Docker configuration for database", err, "database", db)
				continue // Skip this database if configuration is invalid
			}

			app := types.CrossPlatformApp{
				Name:        fmt.Sprintf("docker-%s", strings.ToLower(db)),
				Description: fmt.Sprintf("Install %s database via Docker", db),
				Linux: types.OSConfig{
					InstallMethod:  "docker",
					InstallCommand: dockerCmd,
				},
				MacOS: types.OSConfig{
					InstallMethod:  "docker",
					InstallCommand: dockerCmd,
				},
				Windows: types.OSConfig{
					InstallMethod:  "docker",
					InstallCommand: dockerCmd,
				},
			}
			apps = append(apps, app)
		}
	}

	return apps
}

// BuildSecureDockerCommand constructs a Docker command with proper validation and escaping
func BuildSecureDockerCommand(containerName, image, portMapping, envVar string) (string, error) {
	// Validate all inputs first
	if err := ValidateDockerConfig(containerName, image, portMapping, envVar); err != nil {
		return "", err
	}

	// Build command using safe string concatenation (not fmt.Sprintf with user input)
	var cmdParts []string
	cmdParts = append(cmdParts, "docker", "run", "-d")
	cmdParts = append(cmdParts, "--name", containerName)
	cmdParts = append(cmdParts, "--restart", "unless-stopped")
	cmdParts = append(cmdParts, "-p", "127.0.0.1:"+portMapping)

	if envVar != "" {
		cmdParts = append(cmdParts, "-e", envVar)
	}

	cmdParts = append(cmdParts, image)

	// Join with spaces - this is safe since all parts are validated
	return strings.Join(cmdParts, " "), nil
}
