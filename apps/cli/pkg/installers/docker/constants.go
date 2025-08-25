package docker

import "time"

// Docker installation constants
const (
	// Docker GPG key fingerprint for verification (Docker's official key)
	DockerGPGKeyFingerprint = "9DC858229FC7DD38854AE2D88D81803C0EBFCD88"

	// Docker repository URLs - OS-specific
	DockerUbuntuGPGURL = "https://download.docker.com/linux/ubuntu/gpg"
	DockerDebianGPGURL = "https://download.docker.com/linux/debian/gpg"
	DockerCentOSGPGURL = "https://download.docker.com/linux/centos/gpg"
	DockerGPGKeyURL    = DockerUbuntuGPGURL // Default fallback

	DockerCentOSRepoURL = "https://download.docker.com/linux/centos/docker-ce.repo"

	// GPG key rotation support - backup fingerprints for seamless updates
	DockerBackupGPGFingerprints = "" // Space-separated list of backup fingerprints

	// Certificate pinning for Docker GPG key download security
	DockerGPGKeyDomain    = "download.docker.com"
	DockerCertFingerprint = "B8:36:5E:7F:0C:7B:13:0A:F2:B8:96:CD:B0:E1:47:C5:03:54:49:44:2D:2B:FC:A9:E4:AB:CB:C0:93:77:D4:91" // SHA-256 of Docker's cert

	// GPG operation timeouts
	GPGDownloadTimeout     = 30 * time.Second
	GPGVerificationTimeout = 15 * time.Second

	// Docker socket path
	DockerSocketPath = "/var/run/docker.sock"

	// Timeout constants
	DefaultServiceTimeout = 30 * time.Second
	DockerGroupTimeout    = 10 * time.Second
	UserDetectionTimeout  = 5 * time.Second
	DockerCommandTimeout  = 30 * time.Second
	ContainerCacheTimeout = 5 * time.Minute

	// Docker daemon configuration paths
	DockerConfigDir      = "/etc/docker"
	DockerDaemonConfig   = "/etc/docker/daemon.json"
	DockerSeccompProfile = "/etc/docker/seccomp.json"
	DockerKeyringsPath   = "/usr/share/keyrings/docker-archive-keyring.gpg"

	// Log configuration
	DefaultLogMaxSize  = "100m"
	DefaultLogMaxFiles = "5"

	// Port constants for database containers
	PostgreSQLPort = "5432"
	MySQLPort      = "3306"
	RedisPort      = "6379"
	MongoDBPort    = "27017"
	MariaDBPort    = "3307"

	// Container restart policy
	DefaultRestartPolicy = "unless-stopped"

	// Docker daemon configuration defaults
	DefaultStorageDriver = "overlay2"
	DefaultLogDriver     = "json-file"
)

// Docker package names for different package managers
var (
	DockerPackagesAPT = []string{
		"docker-ce",
		"docker-ce-cli",
		"containerd.io",
		"docker-buildx-plugin",
		"docker-compose-plugin",
		"docker-ce-rootless-extras",
	}

	DockerPackagesDNF = []string{
		"docker-ce",
		"docker-ce-cli",
		"containerd.io",
		"docker-buildx-plugin",
		"docker-compose-plugin",
	}

	DockerPackagesPacman = []string{
		"docker",
		"docker-compose",
	}

	DockerPackagesZypper = []string{
		"docker",
		"docker-compose",
	}
)

// Supported Docker subcommands for security validation
var AllowedDockerSubcommands = map[string]bool{
	"run":     true,
	"stop":    true,
	"start":   true,
	"restart": true,
	"ps":      true,
	"images":  true,
	"pull":    true,
	"inspect": true,
	"logs":    true,
	"exec":    true,
	"rm":      true,
	"rmi":     true,
}

// Suspicious patterns for Docker command validation
var SuspiciousPatterns = []string{
	"rm -rf",
	"sudo",
	"--privileged",
	"/dev/",
	"/proc/",
	"/sys/",
	"&&",
	"||",
	";",
	"|",
	"`",
	"$(",
}
