package status

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/jameswlane/devex/apps/cli/internal/types"
)

// CheckFileIntegrity verifies that critical files for an application exist and are accessible
func CheckFileIntegrity(ctx context.Context, app *types.AppConfig) []string {
	var issues []string

	switch strings.ToLower(app.Name) {
	case "docker":
		// Check critical Docker files
		criticalFiles := []string{
			"/var/run/docker.sock",
			"/etc/docker/daemon.json",
		}

		for _, file := range criticalFiles {
			if file == "/etc/docker/daemon.json" {
				// This file is optional, just check if it exists and is valid JSON if present
				if _, err := os.Stat(file); err == nil {
					if content, err := os.ReadFile(file); err == nil {
						var js json.RawMessage
						if json.Unmarshal(content, &js) != nil {
							issues = append(issues, fmt.Sprintf("Docker daemon.json is not valid JSON: %s", file))
						}
					}
				}
			} else {
				// Critical files must exist
				if _, err := os.Stat(file); err != nil {
					if os.IsNotExist(err) {
						issues = append(issues, fmt.Sprintf("Critical Docker file missing: %s", file))
					} else if os.IsPermission(err) {
						issues = append(issues, fmt.Sprintf("Permission denied accessing Docker file: %s", file))
					}
				}
			}
		}

	case "git":
		// Check Git configuration files
		homeDir, _ := os.UserHomeDir()
		gitConfig := filepath.Join(homeDir, ".gitconfig")
		if _, err := os.Stat(gitConfig); err != nil {
			issues = append(issues, "Git configuration file missing (~/.gitconfig)")
		}

		// Check global .gitignore if configured
		cmd := exec.CommandContext(ctx, "git", "config", "--global", "--get", "core.excludesfile")
		if output, err := cmd.Output(); err == nil {
			gitignorePath := strings.TrimSpace(string(output))
			if gitignorePath != "" {
				if _, err := os.Stat(gitignorePath); err != nil {
					issues = append(issues, fmt.Sprintf("Global gitignore file missing: %s", gitignorePath))
				}
			}
		}

	case "mysql":
		// Check MySQL configuration and data directory
		configFiles := []string{
			"/etc/mysql/mysql.conf.d/mysqld.cnf",
			"/etc/mysql/my.cnf",
			"/etc/my.cnf",
		}

		configFound := false
		for _, config := range configFiles {
			if _, err := os.Stat(config); err == nil {
				configFound = true
				break
			}
		}

		if !configFound {
			issues = append(issues, "MySQL configuration file not found")
		}

	case "postgresql", "postgres":
		// Check PostgreSQL configuration
		configDirs := []string{
			"/etc/postgresql",
			"/var/lib/postgresql",
		}

		for _, dir := range configDirs {
			if _, err := os.Stat(dir); err != nil {
				if os.IsNotExist(err) {
					issues = append(issues, fmt.Sprintf("PostgreSQL directory missing: %s", dir))
				}
			}
		}

	case "nginx":
		// Check Nginx configuration and log directories
		criticalPaths := []string{
			"/etc/nginx/nginx.conf",
			"/var/log/nginx",
			"/usr/share/nginx/html",
		}

		for _, path := range criticalPaths {
			if _, err := os.Stat(path); err != nil {
				if os.IsNotExist(err) {
					issues = append(issues, fmt.Sprintf("Nginx path missing: %s", path))
				}
			}
		}

		// Test nginx configuration syntax
		cmd := exec.CommandContext(ctx, "nginx", "-t")
		if err := cmd.Run(); err != nil {
			issues = append(issues, "Nginx configuration syntax error")
		}

	case "apache2", "httpd":
		// Check Apache configuration
		configFiles := []string{
			"/etc/apache2/apache2.conf",
			"/etc/httpd/conf/httpd.conf",
		}

		configFound := false
		for _, config := range configFiles {
			if _, err := os.Stat(config); err == nil {
				configFound = true
				// Test configuration syntax
				cmd := exec.CommandContext(ctx, "apache2ctl", "configtest")
				if cmd.Run() != nil {
					// Try httpd if apache2ctl fails
					cmd = exec.CommandContext(ctx, "httpd", "-t")
					if cmd.Run() != nil {
						issues = append(issues, "Apache configuration syntax error")
					}
				}
				break
			}
		}

		if !configFound {
			issues = append(issues, "Apache configuration file not found")
		}

	case "redis":
		// Check Redis configuration
		configFiles := []string{
			"/etc/redis/redis.conf",
			"/etc/redis.conf",
		}

		configFound := false
		for _, config := range configFiles {
			if _, err := os.Stat(config); err == nil {
				configFound = true
				break
			}
		}

		if !configFound {
			issues = append(issues, "Redis configuration file not found")
		}

	case "jenkins":
		// Check Jenkins home directory
		homeDir, _ := os.UserHomeDir()
		jenkinsHome := filepath.Join(homeDir, ".jenkins")
		if _, err := os.Stat(jenkinsHome); err != nil {
			issues = append(issues, "Jenkins home directory missing")
		} else {
			// Check critical Jenkins files
			criticalFiles := []string{
				filepath.Join(jenkinsHome, "config.xml"),
				filepath.Join(jenkinsHome, "secrets"),
				filepath.Join(jenkinsHome, "plugins"),
			}

			for _, file := range criticalFiles {
				if _, err := os.Stat(file); err != nil {
					if os.IsNotExist(err) {
						issues = append(issues, fmt.Sprintf("Jenkins file/directory missing: %s", filepath.Base(file)))
					}
				}
			}
		}

	case "node", "nodejs":
		// Check Node.js global modules directory
		homeDir, _ := os.UserHomeDir()
		nodeModules := filepath.Join(homeDir, ".npm")
		if _, err := os.Stat(nodeModules); err != nil {
			// Not critical, but worth noting
			issues = append(issues, "NPM cache directory not initialized")
		}

	case "python", "python3":
		// Check Python packages directory
		homeDir, _ := os.UserHomeDir()
		pipCache := filepath.Join(homeDir, ".cache/pip")
		if _, err := os.Stat(pipCache); err != nil {
			// Not critical, just informational
			issues = append(issues, "Python pip cache directory not initialized")
		}

	case "go":
		// Check Go workspace
		goPath := os.Getenv("GOPATH")
		if goPath == "" {
			homeDir, _ := os.UserHomeDir()
			goPath = filepath.Join(homeDir, "go")
		}

		if _, err := os.Stat(goPath); err != nil {
			issues = append(issues, "Go workspace directory missing")
		}
	}

	return issues
}
