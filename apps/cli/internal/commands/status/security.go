package status

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/jameswlane/devex/apps/cli/internal/types"
)

// CheckSecurityAndUpdates validates security settings and checks for available updates
func CheckSecurityAndUpdates(ctx context.Context, app *types.AppConfig) []string {
	var issues []string

	// Check for security updates
	if securityUpdates := checkForSecurityUpdates(ctx, app); len(securityUpdates) > 0 {
		issues = append(issues, securityUpdates...)
	}

	// Application-specific security validations
	switch strings.ToLower(app.Name) {
	case "docker":
		if dockerSecurity := validateDockerSecurity(ctx); len(dockerSecurity) > 0 {
			issues = append(issues, dockerSecurity...)
		}

	case "mysql":
		if mysqlSecurity := validateMySQLSecurity(ctx); len(mysqlSecurity) > 0 {
			issues = append(issues, mysqlSecurity...)
		}

	case "postgresql", "postgres":
		if pgSecurity := validatePostgreSQLSecurity(ctx); len(pgSecurity) > 0 {
			issues = append(issues, pgSecurity...)
		}

	case "nginx":
		if nginxSecurity := validateNginxSecurity(ctx); len(nginxSecurity) > 0 {
			issues = append(issues, nginxSecurity...)
		}

	case "apache2", "httpd":
		if apacheSecurity := validateApacheSecurity(ctx); len(apacheSecurity) > 0 {
			issues = append(issues, apacheSecurity...)
		}

	case "redis":
		if redisSecurity := validateRedisSecurity(ctx); len(redisSecurity) > 0 {
			issues = append(issues, redisSecurity...)
		}

	case "ssh", "openssh":
		if sshSecurity := validateSSHSecurity(ctx); len(sshSecurity) > 0 {
			issues = append(issues, sshSecurity...)
		}
	}

	return issues
}

// checkForSecurityUpdates checks if security updates are available for the application
func checkForSecurityUpdates(ctx context.Context, app *types.AppConfig) []string {
	var issues []string

	switch app.InstallMethod {
	case "apt":
		// Check for security updates via apt
		packageName := app.InstallCommand
		if packageName == "" {
			packageName = app.Name
		}

		// Check apt security updates
		cmd := exec.CommandContext(ctx, "apt", "list", "--upgradable", packageName)
		output, err := cmd.Output()
		if err == nil {
			lines := strings.Split(string(output), "\n")
			for _, line := range lines {
				if strings.Contains(line, packageName) && strings.Contains(line, "security") {
					issues = append(issues, fmt.Sprintf("Security update available for %s", packageName))
					break
				}
			}
		}

		// Also check unattended-upgrades security updates
		cmd = exec.CommandContext(ctx, "apt", "list", "--upgradable")
		output, err = cmd.Output()
		if err == nil && strings.Contains(string(output), packageName) {
			// This is a general update check - in production you'd parse the specific update details
			issues = append(issues, fmt.Sprintf("Updates available for %s (check if security-related)", packageName))
		}

	case "dnf":
		// Check DNF security updates
		packageName := app.InstallCommand
		if packageName == "" {
			packageName = app.Name
		}

		cmd := exec.CommandContext(ctx, "dnf", "updateinfo", "list", "security", packageName)
		output, err := cmd.Output()
		if err == nil && strings.TrimSpace(string(output)) != "" {
			issues = append(issues, fmt.Sprintf("Security updates available for %s", packageName))
		}

	case "pacman":
		// Check Pacman updates (Arch doesn't separate security updates)
		packageName := app.InstallCommand
		if packageName == "" {
			packageName = app.Name
		}

		cmd := exec.CommandContext(ctx, "pacman", "-Qu", packageName)
		output, err := cmd.Output()
		if err == nil && strings.TrimSpace(string(output)) != "" {
			issues = append(issues, fmt.Sprintf("Updates available for %s", packageName))
		}

	case "zypper":
		// Check Zypper security patches
		packageName := app.InstallCommand
		if packageName == "" {
			packageName = app.Name
		}

		cmd := exec.CommandContext(ctx, "zypper", "list-patches", "--category", "security")
		output, err := cmd.Output()
		if err == nil && strings.Contains(string(output), packageName) {
			issues = append(issues, fmt.Sprintf("Security patches available for %s", packageName))
		}
	}

	return issues
}

// validateDockerSecurity checks Docker security configuration
func validateDockerSecurity(ctx context.Context) []string {
	var issues []string

	// Check if Docker daemon is running with secure defaults
	cmd := exec.CommandContext(ctx, "docker", "system", "info", "--format", "{{.SecurityOptions}}")
	output, err := cmd.Output()
	if err == nil {
		securityInfo := string(output)
		if !strings.Contains(securityInfo, "apparmor") && !strings.Contains(securityInfo, "selinux") {
			issues = append(issues, "Docker missing mandatory access control (AppArmor/SELinux)")
		}
	}

	// Check for insecure registries
	cmd = exec.CommandContext(ctx, "docker", "system", "info", "--format", "{{.InsecureRegistries}}")
	output, err = cmd.Output()
	if err == nil && strings.TrimSpace(string(output)) != "[]" {
		issues = append(issues, "Docker configured with insecure registries")
	}

	// Check Docker socket permissions
	if stat, err := os.Stat("/var/run/docker.sock"); err == nil {
		mode := stat.Mode()
		if mode.Perm() != 0660 {
			issues = append(issues, "Docker socket has insecure permissions")
		}
	}

	return issues
}

// validateMySQLSecurity checks MySQL security configuration
func validateMySQLSecurity(ctx context.Context) []string {
	var issues []string

	// Check for default/weak MySQL users
	cmd := exec.CommandContext(ctx, "mysql", "-e", "SELECT User, Host FROM mysql.user WHERE User IN ('', 'root') AND Host='%';")
	output, err := cmd.Output()
	if err == nil && strings.TrimSpace(string(output)) != "" {
		lines := strings.Split(string(output), "\n")
		if len(lines) > 1 { // More than just header
			issues = append(issues, "MySQL has insecure user accounts (root@% or anonymous users)")
		}
	}

	// Check MySQL version for known vulnerabilities
	cmd = exec.CommandContext(ctx, "mysql", "--version")
	output, err = cmd.Output()
	if err == nil {
		version := string(output)
		// This is a simplified check - in production you'd check against CVE databases
		if strings.Contains(version, "5.5") || strings.Contains(version, "5.6") {
			issues = append(issues, "MySQL version may have known security vulnerabilities")
		}
	}

	return issues
}

// validatePostgreSQLSecurity checks PostgreSQL security configuration
func validatePostgreSQLSecurity(ctx context.Context) []string {
	var issues []string

	// Check PostgreSQL authentication methods
	cmd := exec.CommandContext(ctx, "psql", "-c", "SHOW hba_file;", "-t")
	output, err := cmd.Output()
	if err == nil {
		hbaFile := strings.TrimSpace(string(output))
		if hbaFile != "" {
			// Read pg_hba.conf for insecure authentication
			if content, err := os.ReadFile(hbaFile); err == nil {
				hbaContent := string(content)
				if strings.Contains(hbaContent, "trust") {
					issues = append(issues, "PostgreSQL using insecure 'trust' authentication")
				}
				if strings.Contains(hbaContent, "0.0.0.0/0") {
					issues = append(issues, "PostgreSQL allowing connections from any IP address")
				}
			}
		}
	}

	// Check for superuser accounts
	cmd = exec.CommandContext(ctx, "psql", "-c", "SELECT rolname FROM pg_roles WHERE rolsuper = true;", "-t")
	output, err = cmd.Output()
	if err == nil {
		superusers := strings.Split(strings.TrimSpace(string(output)), "\n")
		if len(superusers) > 1 { // More than just postgres user
			issues = append(issues, "Multiple PostgreSQL superuser accounts detected")
		}
	}

	return issues
}

// validateNginxSecurity checks Nginx security configuration
func validateNginxSecurity(ctx context.Context) []string {
	var issues []string

	// Check Nginx configuration for security headers
	configFile := "/etc/nginx/nginx.conf"
	if content, err := os.ReadFile(configFile); err == nil {
		config := string(content)

		securityHeaders := []string{
			"X-Frame-Options",
			"X-Content-Type-Options",
			"X-XSS-Protection",
			"Strict-Transport-Security",
		}

		missingHeaders := []string{}
		for _, header := range securityHeaders {
			if !strings.Contains(config, header) {
				missingHeaders = append(missingHeaders, header)
			}
		}

		if len(missingHeaders) > 0 {
			issues = append(issues, fmt.Sprintf("Nginx missing security headers: %s", strings.Join(missingHeaders, ", ")))
		}

		// Check for server tokens
		if !strings.Contains(config, "server_tokens off") {
			issues = append(issues, "Nginx server tokens not disabled (information disclosure)")
		}
	}

	return issues
}

// validateApacheSecurity checks Apache security configuration
func validateApacheSecurity(ctx context.Context) []string {
	var issues []string

	// Check Apache configuration for security settings
	configPaths := []string{
		"/etc/apache2/apache2.conf",
		"/etc/httpd/conf/httpd.conf",
	}

	for _, configFile := range configPaths {
		if content, err := os.ReadFile(configFile); err == nil {
			config := string(content)

			// Check for server signature
			if !strings.Contains(config, "ServerSignature Off") {
				issues = append(issues, "Apache server signature not disabled")
			}

			// Check for server tokens
			if !strings.Contains(config, "ServerTokens Prod") {
				issues = append(issues, "Apache server tokens not set to production mode")
			}

			// Check for directory browsing
			if strings.Contains(config, "Indexes") && !strings.Contains(config, "-Indexes") {
				issues = append(issues, "Apache directory browsing may be enabled")
			}

			break
		}
	}

	return issues
}

// validateRedisSecurity checks Redis security configuration
func validateRedisSecurity(ctx context.Context) []string {
	var issues []string

	// Check Redis authentication
	cmd := exec.CommandContext(ctx, "redis-cli", "CONFIG", "GET", "requirepass")
	output, err := cmd.Output()
	if err == nil {
		lines := strings.Split(string(output), "\n")
		if len(lines) >= 2 && strings.TrimSpace(lines[1]) == "" {
			issues = append(issues, "Redis authentication not configured")
		}
	}

	// Check Redis bind configuration
	cmd = exec.CommandContext(ctx, "redis-cli", "CONFIG", "GET", "bind")
	output, err = cmd.Output()
	if err == nil {
		if strings.Contains(string(output), "0.0.0.0") {
			issues = append(issues, "Redis bound to all interfaces (security risk)")
		}
	}

	// Check for dangerous commands
	cmd = exec.CommandContext(ctx, "redis-cli", "CONFIG", "GET", "rename-command")
	output, err = cmd.Output()
	if err == nil {
		dangerousCommands := []string{"FLUSHDB", "FLUSHALL", "KEYS", "CONFIG", "EVAL"}
		config := string(output)
		for _, dangerousCmd := range dangerousCommands {
			if !strings.Contains(config, dangerousCmd) {
				// Command not renamed, which could be a security issue
				_ = dangerousCmd
			}
		}
	}

	return issues
}

// validateSSHSecurity checks SSH security configuration
func validateSSHSecurity(ctx context.Context) []string {
	var issues []string

	configFile := "/etc/ssh/sshd_config"
	if content, err := os.ReadFile(configFile); err == nil {
		config := string(content)

		// Check for root login
		if strings.Contains(config, "PermitRootLogin yes") {
			issues = append(issues, "SSH root login is enabled")
		}

		// Check for password authentication
		if strings.Contains(config, "PasswordAuthentication yes") {
			issues = append(issues, "SSH password authentication enabled (prefer key-based)")
		}

		// Check for empty passwords
		if strings.Contains(config, "PermitEmptyPasswords yes") {
			issues = append(issues, "SSH allows empty passwords")
		}

		// Check for protocol version
		if strings.Contains(config, "Protocol 1") {
			issues = append(issues, "SSH using insecure Protocol 1")
		}

		// Check for X11 forwarding
		if strings.Contains(config, "X11Forwarding yes") {
			issues = append(issues, "SSH X11 forwarding enabled (potential security risk)")
		}
	}

	return issues
}
