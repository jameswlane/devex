package commands

import (
	"fmt"
	"strings"

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
# Add Docker's official GPG key
curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo gpg --dearmor -o /usr/share/keyrings/docker-archive-keyring.gpg

# Add Docker repository based on detected OS
if [ -f /etc/os-release ]; then
    . /etc/os-release
    case "$ID" in
        ubuntu|debian)
            echo "deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/docker-archive-keyring.gpg] https://download.docker.com/linux/$ID $(lsb_release -cs) stable" | sudo tee /etc/apt/sources.list.d/docker.list > /dev/null
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
        opensuse*|sles)
            sudo zypper install -y docker docker-compose
            ;;
        *)
            echo "Unsupported OS: $ID"
            exit 1
            ;;
    esac
else
    echo "Cannot detect OS, defaulting to Ubuntu/Debian installation"
    echo "deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/docker-archive-keyring.gpg] https://download.docker.com/linux/ubuntu $(lsb_release -cs) stable" | sudo tee /etc/apt/sources.list.d/docker.list > /dev/null
    sudo apt-get update
    sudo apt-get install -y docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin docker-ce-rootless-extras
fi

# Add current user to docker group with fallback methods
username=""
if [ -n "$USER" ]; then
    username="$USER"
elif [ -n "$USERNAME" ]; then
    username="$USERNAME"
elif command -v whoami >/dev/null 2>&1; then
    username=$(whoami)
elif command -v id >/dev/null 2>&1; then
    username=$(id -un)
fi

if [ -n "$username" ] && [ "$username" != "root" ]; then
    sudo usermod -aG docker "$username"
    echo "Added user '$username' to docker group"
else
    echo "Could not determine username for docker group setup"
fi

# Create Docker daemon configuration
sudo mkdir -p /etc/docker
sudo tee /etc/docker/daemon.json > /dev/null << 'EOF'
{
  "log-driver": "json-file",
  "log-opts": {
    "max-size": "100m",
    "max-file": "5"
  },
  "storage-driver": "overlay2"
}
EOF

# Enable and start Docker service
sudo systemctl enable docker
sudo systemctl start docker

echo "Docker installation completed successfully!"
echo "Note: You may need to log out and back in for Docker group permissions to take effect"
echo "Or run: newgrp docker"`,
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
			"port":      PostgreSQLPort,
			"env":       "POSTGRES_HOST_AUTH_METHOD=trust",
		},
		"MySQL": {
			"image":     "mysql:8.4",
			"container": "mysql8",
			"port":      MySQLPort,
			"env":       "MYSQL_ALLOW_EMPTY_PASSWORD=true",
		},
		"Redis": {
			"image":     "redis:7",
			"container": "redis",
			"port":      RedisPort,
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
