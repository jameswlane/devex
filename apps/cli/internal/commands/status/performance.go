package status

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"github.com/jameswlane/devex/apps/cli/internal/types"
)

// CollectPerformanceMetrics gathers CPU, memory, and process information for applications
func CollectPerformanceMetrics(ctx context.Context, app *types.AppConfig) *PerformanceMetrics {
	metrics := &PerformanceMetrics{}

	// Get the process name for the application
	processName := getProcessName(app.Name)
	if processName == "" {
		return nil
	}

	// Find the process using pgrep
	cmd := exec.CommandContext(ctx, "pgrep", "-f", processName)
	output, err := cmd.Output()
	if err != nil {
		// Process not running
		return nil
	}

	pids := strings.Split(strings.TrimSpace(string(output)), "\n")
	if len(pids) == 0 || pids[0] == "" {
		return nil
	}

	// Use the first PID found
	pidStr := pids[0]

	// Convert PID to integer
	pid := 0
	if _, err := fmt.Sscanf(pidStr, "%d", &pid); err == nil {
		metrics.ProcessID = pid
	} else {
		return nil
	}

	// Get CPU and memory usage using ps
	cmd = exec.CommandContext(ctx, "ps", "-p", pidStr, "-o", "pid,pcpu,pmem,rss,etime", "--no-headers")
	output, err = cmd.Output()
	if err != nil {
		return metrics // Return what we have so far
	}

	// Parse ps output: PID %CPU %MEM RSS ELAPSED
	fields := strings.Fields(string(output))
	if len(fields) >= 5 {
		// Parse CPU usage
		if _, err := fmt.Sscanf(fields[1], "%f", &metrics.CPUUsage); err != nil {
			metrics.CPUUsage = 0
		}

		// Parse memory usage (RSS in KB, convert to bytes)
		var memoryKB int64
		if _, err := fmt.Sscanf(fields[3], "%d", &memoryKB); err == nil {
			metrics.MemoryUsage = memoryKB * 1024 // Convert KB to bytes
		}

		// Parse uptime
		if len(fields) >= 5 {
			metrics.Uptime = fields[4]
		}
	}

	// Additional metrics for specific applications
	switch strings.ToLower(app.Name) {
	case "docker":
		// Get Docker-specific metrics
		if dockerMetrics := getDockerMetrics(ctx); dockerMetrics != nil {
			// Merge Docker metrics if available
			if dockerMetrics.MemoryUsage > 0 {
				metrics.MemoryUsage = dockerMetrics.MemoryUsage
			}
			if dockerMetrics.CPUUsage > 0 {
				metrics.CPUUsage = dockerMetrics.CPUUsage
			}
		}

	case "mysql":
		// Get MySQL-specific metrics
		if mysqlMetrics := getMySQLMetrics(ctx); mysqlMetrics != nil {
			if mysqlMetrics.MemoryUsage > 0 {
				metrics.MemoryUsage = mysqlMetrics.MemoryUsage
			}
		}

	case "postgresql", "postgres":
		// Get PostgreSQL-specific metrics
		if pgMetrics := getPostgreSQLMetrics(ctx); pgMetrics != nil {
			if pgMetrics.MemoryUsage > 0 {
				metrics.MemoryUsage = pgMetrics.MemoryUsage
			}
		}

	case "redis":
		// Get Redis-specific metrics
		if redisMetrics := getRedisMetrics(ctx); redisMetrics != nil {
			if redisMetrics.MemoryUsage > 0 {
				metrics.MemoryUsage = redisMetrics.MemoryUsage
			}
		}

	case "nginx":
		// Get Nginx-specific metrics
		if nginxMetrics := getNginxMetrics(ctx); nginxMetrics != nil {
			if nginxMetrics.CPUUsage > 0 {
				metrics.CPUUsage = nginxMetrics.CPUUsage
			}
		}
	}

	return metrics
}

// getProcessName returns the process name to search for given an application name
func getProcessName(appName string) string {
	processMap := map[string]string{
		"docker":     "dockerd",
		"mysql":      "mysqld",
		"postgresql": "postgres",
		"postgres":   "postgres",
		"redis":      "redis-server",
		"nginx":      "nginx",
		"apache2":    "apache2",
		"httpd":      "httpd",
		"jenkins":    "jenkins",
		"node":       "node",
		"nodejs":     "node",
		"python":     "python",
		"python3":    "python3",
		"go":         "go",
		"git":        "", // Git doesn't run as a daemon
		"curl":       "", // Curl doesn't run as a daemon
	}

	if process, ok := processMap[strings.ToLower(appName)]; ok {
		return process
	}

	// Default to the app name itself
	return strings.ToLower(appName)
}

// getDockerMetrics collects Docker-specific performance metrics
func getDockerMetrics(ctx context.Context) *PerformanceMetrics {
	metrics := &PerformanceMetrics{}

	// Get Docker system information
	cmd := exec.CommandContext(ctx, "docker", "system", "df", "--format", "table")
	output, err := cmd.Output()
	if err != nil {
		return nil
	}

	// Parse docker system df output for memory usage approximation
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.Contains(line, "Images") || strings.Contains(line, "Containers") {
			// Extract size information - this is a simplified approach
			fields := strings.Fields(line)
			if len(fields) > 2 {
				// This is an approximation - Docker doesn't directly expose memory in this command
				// In a production system, you'd use docker stats or other APIs
				_ = fields
			}
		}
	}

	return metrics
}

// getMySQLMetrics collects MySQL-specific performance metrics
func getMySQLMetrics(ctx context.Context) *PerformanceMetrics {
	metrics := &PerformanceMetrics{}

	// Try to get MySQL memory usage from SHOW STATUS
	cmd := exec.CommandContext(ctx, "mysql", "-e", "SHOW STATUS LIKE 'Innodb_buffer_pool_bytes_data';")
	output, err := cmd.Output()
	if err != nil {
		return nil
	}

	// Parse output to extract memory usage
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.Contains(line, "Innodb_buffer_pool_bytes_data") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				var memBytes int64
				if _, err := fmt.Sscanf(fields[1], "%d", &memBytes); err == nil {
					metrics.MemoryUsage = memBytes
				}
			}
		}
	}

	return metrics
}

// getPostgreSQLMetrics collects PostgreSQL-specific performance metrics
func getPostgreSQLMetrics(ctx context.Context) *PerformanceMetrics {
	metrics := &PerformanceMetrics{}

	// Try to get PostgreSQL memory usage from pg_stat_database
	cmd := exec.CommandContext(ctx, "psql", "-c", "SELECT pg_size_pretty(pg_database_size(current_database()));", "-t")
	output, err := cmd.Output()
	if err != nil {
		return nil
	}

	// This gives database size, not memory usage, but it's informational
	sizeStr := strings.TrimSpace(string(output))
	if sizeStr != "" {
		// Convert size string to bytes (simplified)
		// In production, you'd parse units like MB, GB, etc.
		_ = sizeStr
	}

	return metrics
}

// getRedisMetrics collects Redis-specific performance metrics
func getRedisMetrics(ctx context.Context) *PerformanceMetrics {
	metrics := &PerformanceMetrics{}

	// Get Redis memory usage
	cmd := exec.CommandContext(ctx, "redis-cli", "INFO", "memory")
	output, err := cmd.Output()
	if err != nil {
		return nil
	}

	// Parse Redis INFO output
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.HasPrefix(line, "used_memory:") {
			parts := strings.Split(line, ":")
			if len(parts) == 2 {
				var memBytes int64
				if _, err := fmt.Sscanf(parts[1], "%d", &memBytes); err == nil {
					metrics.MemoryUsage = memBytes
				}
			}
		}
	}

	return metrics
}

// getNginxMetrics collects Nginx-specific performance metrics
func getNginxMetrics(ctx context.Context) *PerformanceMetrics {
	metrics := &PerformanceMetrics{}

	// Get Nginx status if status module is enabled
	cmd := exec.CommandContext(ctx, "curl", "-s", "http://localhost/nginx_status")
	output, err := cmd.Output()
	if err != nil {
		// Nginx status not available
		return nil
	}

	// Parse nginx status output
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.Contains(line, "Active connections:") {
			// Extract connection count - this could be used for load metrics
			_ = line
		}
	}

	return metrics
}
