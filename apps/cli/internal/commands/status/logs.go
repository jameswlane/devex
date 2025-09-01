package status

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/jameswlane/devex/apps/cli/internal/types"
)

// AnalyzeApplicationLogs parses recent application logs for errors and warnings
func AnalyzeApplicationLogs(ctx context.Context, app *types.AppConfig) []string {
	var issues []string

	switch strings.ToLower(app.Name) {
	case "docker":
		// Analyze Docker daemon logs
		if dockerLogs := analyzeDockerLogs(ctx); len(dockerLogs) > 0 {
			issues = append(issues, dockerLogs...)
		}

	case "mysql":
		// Analyze MySQL error logs
		if mysqlLogs := analyzeMySQLLogs(ctx); len(mysqlLogs) > 0 {
			issues = append(issues, mysqlLogs...)
		}

	case "postgresql", "postgres":
		// Analyze PostgreSQL logs
		if pgLogs := analyzePostgreSQLLogs(ctx); len(pgLogs) > 0 {
			issues = append(issues, pgLogs...)
		}

	case "nginx":
		// Analyze Nginx error logs
		if nginxLogs := analyzeNginxLogs(ctx); len(nginxLogs) > 0 {
			issues = append(issues, nginxLogs...)
		}

	case "apache2", "httpd":
		// Analyze Apache error logs
		if apacheLogs := analyzeApacheLogs(ctx); len(apacheLogs) > 0 {
			issues = append(issues, apacheLogs...)
		}

	case "redis":
		// Analyze Redis logs
		if redisLogs := analyzeRedisLogs(ctx); len(redisLogs) > 0 {
			issues = append(issues, redisLogs...)
		}

	case "jenkins":
		// Analyze Jenkins logs
		if jenkinsLogs := analyzeJenkinsLogs(ctx); len(jenkinsLogs) > 0 {
			issues = append(issues, jenkinsLogs...)
		}
	}

	return issues
}

// AnalyzeSystemdLogs checks systemd service logs for a specific service
func AnalyzeSystemdLogs(ctx context.Context, serviceName string) []string {
	var issues []string

	cmd := exec.CommandContext(ctx, "journalctl", "-u", serviceName, "--since", "1 hour ago", "-p", "err", "--no-pager", "-q")
	output, err := cmd.Output()
	if err != nil {
		return issues
	}

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			issues = append(issues, fmt.Sprintf("Service %s: %s", serviceName, extractLogMessage(line)))
		}
	}

	return issues
}

// analyzeDockerLogs checks Docker daemon logs for recent errors
func analyzeDockerLogs(ctx context.Context) []string {
	var issues []string

	// Check Docker daemon logs via journalctl
	cmd := exec.CommandContext(ctx, "journalctl", "-u", "docker.service", "--since", "1 hour ago", "-p", "err", "--no-pager", "-q")
	output, err := cmd.Output()
	if err != nil {
		return issues
	}

	lines := strings.Split(string(output), "\n")
	errorCount := 0
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			errorCount++
			if errorCount <= 3 { // Limit to first 3 errors
				if strings.Contains(line, "failed") || strings.Contains(line, "error") {
					issues = append(issues, fmt.Sprintf("Docker error: %s", extractLogMessage(line)))
				}
			}
		}
	}

	if errorCount > 3 {
		issues = append(issues, fmt.Sprintf("Docker has %d additional errors in the last hour", errorCount-3))
	}

	return issues
}

// analyzeMySQLLogs checks MySQL error logs for recent issues
func analyzeMySQLLogs(ctx context.Context) []string {
	var issues []string

	// Common MySQL error log locations
	errorLogPaths := []string{
		"/var/log/mysql/error.log",
		"/var/log/mysqld.log",
		"/var/lib/mysql/mysql-error.log",
	}

	for _, logPath := range errorLogPaths {
		if _, err := os.Stat(logPath); err == nil {
			// Check recent entries (last 100 lines)
			cmd := exec.CommandContext(ctx, "tail", "-100", logPath)
			output, err := cmd.Output()
			if err != nil {
				continue
			}

			lines := strings.Split(string(output), "\n")
			for _, line := range lines {
				if strings.Contains(line, "[ERROR]") || strings.Contains(line, "[Warning]") {
					// Extract timestamp and check if it's recent (last hour)
					if isRecentLogEntry(line) {
						issues = append(issues, fmt.Sprintf("MySQL: %s", extractLogMessage(line)))
					}
				}
			}
			break // Use first available log file
		}
	}

	return issues
}

// analyzePostgreSQLLogs checks PostgreSQL logs for recent issues
func analyzePostgreSQLLogs(ctx context.Context) []string {
	var issues []string

	// Check PostgreSQL via journalctl
	cmd := exec.CommandContext(ctx, "journalctl", "-u", "postgresql.service", "--since", "1 hour ago", "-p", "warning", "--no-pager", "-q")
	output, err := cmd.Output()
	if err != nil {
		return issues
	}

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			if strings.Contains(line, "ERROR") || strings.Contains(line, "FATAL") {
				issues = append(issues, fmt.Sprintf("PostgreSQL: %s", extractLogMessage(line)))
			}
		}
	}

	return issues
}

// analyzeNginxLogs checks Nginx error logs
func analyzeNginxLogs(ctx context.Context) []string {
	var issues []string

	// Check Nginx error log
	errorLogPath := "/var/log/nginx/error.log"
	if _, err := os.Stat(errorLogPath); err == nil {
		cmd := exec.CommandContext(ctx, "tail", "-50", errorLogPath)
		output, err := cmd.Output()
		if err != nil {
			return issues
		}

		lines := strings.Split(string(output), "\n")
		for _, line := range lines {
			if strings.Contains(line, "[error]") || strings.Contains(line, "[crit]") {
				if isRecentLogEntry(line) {
					issues = append(issues, fmt.Sprintf("Nginx: %s", extractLogMessage(line)))
				}
			}
		}
	}

	return issues
}

// analyzeApacheLogs checks Apache error logs
func analyzeApacheLogs(ctx context.Context) []string {
	var issues []string

	// Common Apache error log locations
	errorLogPaths := []string{
		"/var/log/apache2/error.log",
		"/var/log/httpd/error_log",
	}

	for _, logPath := range errorLogPaths {
		if _, err := os.Stat(logPath); err == nil {
			cmd := exec.CommandContext(ctx, "tail", "-50", logPath)
			output, err := cmd.Output()
			if err != nil {
				continue
			}

			lines := strings.Split(string(output), "\n")
			for _, line := range lines {
				if strings.Contains(line, "[error]") || strings.Contains(line, "[crit]") {
					if isRecentLogEntry(line) {
						issues = append(issues, fmt.Sprintf("Apache: %s", extractLogMessage(line)))
					}
				}
			}
			break
		}
	}

	return issues
}

// analyzeRedisLogs checks Redis logs for issues
func analyzeRedisLogs(ctx context.Context) []string {
	var issues []string

	// Check Redis via journalctl
	cmd := exec.CommandContext(ctx, "journalctl", "-u", "redis.service", "--since", "1 hour ago", "-p", "warning", "--no-pager", "-q")
	output, err := cmd.Output()
	if err != nil {
		// Try alternative service names
		cmd = exec.CommandContext(ctx, "journalctl", "-u", "redis-server.service", "--since", "1 hour ago", "-p", "warning", "--no-pager", "-q")
		output, err = cmd.Output()
		if err != nil {
			return issues
		}
	}

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			if strings.Contains(line, "WARNING") || strings.Contains(line, "ERROR") {
				issues = append(issues, fmt.Sprintf("Redis: %s", extractLogMessage(line)))
			}
		}
	}

	return issues
}

// analyzeJenkinsLogs checks Jenkins logs for issues
func analyzeJenkinsLogs(ctx context.Context) []string {
	var issues []string

	// Check Jenkins via journalctl
	cmd := exec.CommandContext(ctx, "journalctl", "-u", "jenkins.service", "--since", "1 hour ago", "-p", "warning", "--no-pager", "-q")
	output, err := cmd.Output()
	if err != nil {
		return issues
	}

	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		if strings.TrimSpace(line) != "" {
			if strings.Contains(line, "WARNING") || strings.Contains(line, "ERROR") || strings.Contains(line, "SEVERE") {
				issues = append(issues, fmt.Sprintf("Jenkins: %s", extractLogMessage(line)))
			}
		}
	}

	return issues
}

// extractLogMessage extracts the meaningful part of a log line
func extractLogMessage(logLine string) string {
	// Remove timestamp and common prefixes
	line := strings.TrimSpace(logLine)

	// Remove systemd journal prefix
	if idx := strings.Index(line, "]: "); idx != -1 {
		line = line[idx+3:]
	}

	// Limit message length
	if len(line) > 100 {
		line = line[:97] + "..."
	}

	return line
}

// isRecentLogEntry checks if a log entry is from the last hour (simplified)
func isRecentLogEntry(logLine string) bool {
	// This is a simplified check - in production you'd parse actual timestamps
	// For now, we assume tail gives us recent entries
	return true
}
