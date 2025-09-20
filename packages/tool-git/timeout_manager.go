package main

import (
	"os"
	"runtime"
	"time"
)

// TimeoutManager handles dynamic timeout scaling based on operation type and system characteristics
type TimeoutManager struct {
	baseTimeout time.Duration
	cpuCount    int
	isCI        bool
}

// NewTimeoutManager creates a new timeout manager with system-aware defaults
func NewTimeoutManager() *TimeoutManager {
	return &TimeoutManager{
		baseTimeout: 30 * time.Second,
		cpuCount:    runtime.NumCPU(),
		isCI:        isRunningInCI(),
	}
}

// GetTimeout returns an appropriate timeout for the given operation type
func (tm *TimeoutManager) GetTimeout(operationType string) time.Duration {
	base := tm.baseTimeout

	// Adjust base timeout if running in CI environment
	if tm.isCI {
		base = base * 2 // CI environments often have slower I/O
	}

	// Scale timeout based on operation type
	switch operationType {
	case "config-read":
		// Simple config reads should be fast
		return tm.scaleTimeout(5*time.Second, 0.5)
	case "config-write":
		// Config writes might trigger hooks
		return tm.scaleTimeout(10*time.Second, 0.7)
	case "status":
		// Status can be slow on large repos
		return tm.scaleTimeout(30*time.Second, 1.0)
	case "clone":
		// Cloning can take a long time for large repos
		return tm.scaleTimeout(5*time.Minute, 2.0)
	case "fetch", "pull":
		// Network operations need more time
		return tm.scaleTimeout(2*time.Minute, 1.5)
	case "push":
		// Push operations can be slow with large changes
		return tm.scaleTimeout(3*time.Minute, 1.5)
	case "shell":
		// Generic shell operations get default timeout
		return tm.scaleTimeout(base, 1.0)
	default:
		// Unknown operations get conservative timeout
		return tm.scaleTimeout(base, 1.2)
	}
}

// scaleTimeout adjusts timeout based on system characteristics
func (tm *TimeoutManager) scaleTimeout(base time.Duration, multiplier float64) time.Duration {
	scaled := time.Duration(float64(base) * multiplier)

	// Adjust for CPU count (slower systems get more time)
	if tm.cpuCount < 2 {
		scaled = time.Duration(float64(scaled) * 1.5)
	} else if tm.cpuCount < 4 {
		scaled = time.Duration(float64(scaled) * 1.2)
	}

	// Ensure minimum timeout
	if scaled < 1*time.Second {
		scaled = 1 * time.Second
	}

	// Cap maximum timeout
	if scaled > 10*time.Minute {
		scaled = 10 * time.Minute
	}

	return scaled
}

// GetTimeoutWithContext returns a timeout that respects context deadline
func (tm *TimeoutManager) GetTimeoutWithContext(operationType string, deadline time.Time) time.Duration {
	operationTimeout := tm.GetTimeout(operationType)

	if deadline.IsZero() {
		return operationTimeout
	}

	// Calculate time until deadline
	timeUntilDeadline := time.Until(deadline)

	// Use the shorter of operation timeout or time until deadline (with small buffer)
	buffer := 100 * time.Millisecond
	if timeUntilDeadline > buffer && timeUntilDeadline < operationTimeout {
		return timeUntilDeadline - buffer
	}

	return operationTimeout
}

// isRunningInCI detects if the code is running in a CI environment
func isRunningInCI() bool {
	// Check common CI environment variables
	ciVars := []string{
		"CI",
		"CONTINUOUS_INTEGRATION",
		"GITHUB_ACTIONS",
		"GITLAB_CI",
		"JENKINS_URL",
		"CIRCLECI",
		"TRAVIS",
		"BUILDKITE",
	}

	for _, env := range ciVars {
		if val, exists := os.LookupEnv(env); exists && val != "" && val != "0" && val != "false" {
			return true
		}
	}

	return false
}

// GetRetryStrategy returns retry configuration for the operation type
func (tm *TimeoutManager) GetRetryStrategy(operationType string) (maxRetries int, backoff time.Duration) {
	switch operationType {
	case "config-read", "config-write":
		// Config operations rarely need retries
		return 1, 100 * time.Millisecond
	case "status":
		// Status might fail due to lock contention
		return 3, 500 * time.Millisecond
	case "fetch", "pull", "push", "clone":
		// Network operations benefit from retries
		return 3, 2 * time.Second
	default:
		// Conservative defaults
		return 2, 1 * time.Second
	}
}
