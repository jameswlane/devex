package main

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

	// Certificate validation strategy for Docker GPG key download security
	DockerGPGKeyDomain = "download.docker.com"

	// Certificate rotation strategy: Support multiple valid fingerprints
	// Primary certificate fingerprint (current)
	DockerPrimaryCertFingerprint = "B8:36:5E:7F:0C:7B:13:0A:F2:B8:96:CD:B0:E1:47:C5:03:54:49:44:2D:2B:FC:A9:E4:AB:CB:C0:93:77:D4:91"

	// Legacy single fingerprint for backward compatibility
	DockerCertFingerprint = DockerPrimaryCertFingerprint

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

// Known good certificate fingerprints for rotation support
var DockerValidCertFingerprints = []string{
	DockerPrimaryCertFingerprint,
	"C4:A7:B1:A4:7B:2C:71:FA:DB:E1:4B:90:75:FF:C4:15:60:85:89:10:A3:5C:8A:D2:2E:98:8A:48:1A:52:BC:87", // Backup cert 1
	// Add new certificates here during rotation
}

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
	"build":   true, // Added for building images
	"tag":     true, // Added for tagging images
	"push":    true, // Added for pushing images to registry
	"commit":  true, // Added for creating images from containers
	"history": true, // Added for image history
	"version": true, // Added for version checking
	"info":    true, // Added for system info
	"rm":      true,
	"rmi":     true,
}

// Suspicious patterns for Docker command validation
var SuspiciousPatterns = []string{
	"rm -rf",
	"sudo",
	"--privileged",
	"/dev",
	"/proc",
	"/sys",
	"&&",
	"||",
	";",
	"|",
	"`",
	"$(",
}
