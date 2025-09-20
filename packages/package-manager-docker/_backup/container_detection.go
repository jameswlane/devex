package main

import (
	"os"
	"strings"

	"github.com/jameswlane/devex/apps/cli/internal/log"
)

// Container detection methods - each focused on a specific detection technique

// detectDockerEnvFile checks for the presence of /.dockerenv file
func detectDockerEnvFile() bool {
	if _, err := os.Stat("/.dockerenv"); err == nil {
		log.Debug("Container detected via .dockerenv file")
		return true
	}
	return false
}

// detectCgroupSignatures examines cgroup information for container references
func detectCgroupSignatures() bool {
	data, err := os.ReadFile("/proc/1/cgroup")
	if err != nil {
		return false
	}

	cgroup := string(data)
	containerIndicators := []string{
		"docker", "containerd", "kubepods", "lxc", "systemd:/docker",
		"/docker/", "/containerd/", "/kubelet", "garden",
	}

	for _, indicator := range containerIndicators {
		if strings.Contains(cgroup, indicator) {
			log.Debug("Container detected via cgroup", "indicator", indicator)
			return true
		}
	}
	return false
}

// detectContainerEnvironmentVars checks for container-specific environment variables
func detectContainerEnvironmentVars() bool {
	containerEnvVars := []string{
		"container", "CONTAINER_ID", "DOCKER_CONTAINER",
		"KUBERNETES_SERVICE_HOST", "K8S_POD_NAME", "NOMAD_ALLOC_ID",
		"HOSTNAME", "MESOS_TASK_ID", "MARATHON_APP_ID",
	}

	for _, envVar := range containerEnvVars {
		value := os.Getenv(envVar)
		if value == "" {
			continue
		}

		// Special handling for HOSTNAME to avoid false positives
		if envVar == "HOSTNAME" && !isContainerHostname(value) {
			continue
		}

		log.Debug("Container detected via environment variable", "var", envVar, "value", value)
		return true
	}
	return false
}

// isContainerHostname validates if hostname follows container patterns
func isContainerHostname(hostname string) bool {
	hostname = strings.TrimSpace(hostname)

	// Container hostnames are typically short hex strings (12 chars)
	if len(hostname) == 12 && isHexString(hostname) {
		log.Debug("Container detected via hostname pattern", "hostname", hostname)
		return true
	}

	// Kubernetes pod patterns
	if strings.Contains(hostname, "-") &&
		(strings.Contains(hostname, "pod") || strings.Contains(hostname, "deployment")) {
		log.Debug("Container detected via k8s hostname pattern", "hostname", hostname)
		return true
	}

	return false
}

// detectContainerInitProcess checks init process characteristics
func detectContainerInitProcess() bool {
	data, err := os.ReadFile("/proc/1/comm")
	if err != nil {
		return false
	}

	initProcess := strings.TrimSpace(string(data))
	containerInitProcesses := []string{
		"sh", "bash", "entrypoint", "docker-init", "tini", "dumb-init",
	}

	for _, containerInit := range containerInitProcesses {
		if initProcess == containerInit {
			log.Debug("Container detected via init process", "init", initProcess)
			return true
		}
	}
	return false
}

// detectContainerFilesystems examines filesystem mount information
func detectContainerFilesystems() bool {
	data, err := os.ReadFile("/proc/mounts")
	if err != nil {
		return false
	}

	mounts := string(data)

	// Check if root filesystem is overlay (strong indicator of container)
	lines := strings.Split(mounts, "\n")
	for _, line := range lines {
		fields := strings.Fields(line)
		if len(fields) >= 3 {
			// Check if root (/) is mounted as overlay
			if fields[1] == "/" && (fields[2] == "overlay" || fields[2] == "aufs") {
				log.Debug("Container detected: root filesystem is overlay")
				return true
			}
		}
	}

	// Check for container-specific mount patterns
	// These paths indicate we're inside a container, not just on a host with Docker
	containerIndicators := []string{
		"overlay / overlay",           // Root on overlay
		"aufs / aufs",                 // Root on aufs
		"/docker/containers/",         // Inside a container path
		"/var/lib/docker/containers/", // Container's own mount
	}

	for _, indicator := range containerIndicators {
		if strings.Contains(mounts, indicator) {
			log.Debug("Container detected via mount indicator", "indicator", indicator)
			return true
		}
	}

	return false
}

// detectVirtualizationIndicators checks CPU info for virtualization hints
func detectVirtualizationIndicators() bool {
	data, err := os.ReadFile("/proc/cpuinfo")
	if err != nil {
		return false
	}

	cpuInfo := string(data)
	if strings.Contains(cpuInfo, "container") || strings.Contains(cpuInfo, "docker") {
		log.Debug("Container detected via CPU info")
		return true
	}
	return false
}

// IsRunningInContainer detects if the current process is running inside a container.
// It uses multiple detection methods to reliably identify container environments.
// This prevents unsafe Docker-in-Docker scenarios and provides proper user guidance.
func IsRunningInContainer() bool {
	return isRunningInContainer()
}

// isRunningInContainer is the internal implementation
func isRunningInContainer() bool {
	// Run detection methods in order of reliability and performance
	detectionMethods := []struct {
		name   string
		detect func() bool
	}{
		{"dockerenv", detectDockerEnvFile},
		{"cgroup", detectCgroupSignatures},
		{"environment", detectContainerEnvironmentVars},
		{"initprocess", detectContainerInitProcess},
		{"filesystem", detectContainerFilesystems},
		{"virtualization", detectVirtualizationIndicators},
	}

	for _, method := range detectionMethods {
		if method.detect() {
			log.Debug("Container detected", "method", method.name)
			return true
		}
	}

	log.Debug("No container environment detected")
	return false
}

// isHexString checks if a string contains only hexadecimal characters
func isHexString(s string) bool {
	for _, char := range s {
		if (char < '0' || char > '9') && (char < 'a' || char > 'f') && (char < 'A' || char > 'F') {
			return false
		}
	}
	return true
}
