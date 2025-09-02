package status

import (
	"context"
	"os/exec"
	"strings"

	"github.com/jameswlane/devex/apps/cli/internal/types"
)

// RunHealthCheck performs application-specific health checks
func RunHealthCheck(ctx context.Context, app *types.AppConfig) string {
	switch strings.ToLower(app.Name) {
	case "docker":
		return checkDockerHealth(ctx)
	case "git":
		return checkGitHealth(ctx)
	case "node", "nodejs":
		return checkNodeHealth(ctx)
	case "mysql":
		return checkMySQLHealth(ctx)
	case "postgresql", "postgres":
		return checkPostgreSQLHealth(ctx)
	case "redis":
		return checkRedisHealth(ctx)
	case "nginx":
		return checkNginxHealth(ctx)
	case "apache2", "httpd":
		return checkApacheHealth(ctx)
	case "jenkins":
		return checkJenkinsHealth(ctx)
	case "elasticsearch":
		return checkElasticsearchHealth(ctx)
	case "python", "python3":
		return checkPythonHealth(ctx)
	case "go":
		return checkGoHealth(ctx)
	case "rust":
		return checkRustHealth(ctx)
	case "java":
		return checkJavaHealth(ctx)
	default:
		return "healthy"
	}
}

func checkDockerHealth(ctx context.Context) string {
	// Check if Docker daemon is accessible
	cmd := exec.CommandContext(ctx, "docker", "info")
	if err := cmd.Run(); err != nil {
		return "Docker daemon not accessible"
	}
	return "healthy"
}

func checkGitHealth(ctx context.Context) string {
	// Check if Git is properly configured
	cmd := exec.CommandContext(ctx, "git", "config", "--global", "--get", "user.name")
	if err := cmd.Run(); err != nil {
		return "Git config not found"
	}
	return "healthy"
}

func checkNodeHealth(ctx context.Context) string {
	// Check Node.js health via npm doctor (if available)
	cmd := exec.CommandContext(ctx, "npm", "doctor")
	if err := cmd.Run(); err != nil {
		return "Node.js environment issues detected"
	}
	return "healthy"
}

func checkMySQLHealth(ctx context.Context) string {
	// Check MySQL connection
	cmd := exec.CommandContext(ctx, "mysqladmin", "ping", "-s")
	if err := cmd.Run(); err != nil {
		return "MySQL server not accessible"
	}
	return "healthy"
}

func checkPostgreSQLHealth(ctx context.Context) string {
	// Check PostgreSQL connection
	cmd := exec.CommandContext(ctx, "pg_isready")
	if err := cmd.Run(); err != nil {
		return "PostgreSQL server not ready"
	}
	return "healthy"
}

func checkRedisHealth(ctx context.Context) string {
	// Check Redis connection
	cmd := exec.CommandContext(ctx, "redis-cli", "ping")
	output, err := cmd.Output()
	if err != nil || !strings.Contains(string(output), "PONG") {
		return "Redis server not responding"
	}
	return "healthy"
}

func checkNginxHealth(ctx context.Context) string {
	// Check Nginx configuration
	cmd := exec.CommandContext(ctx, "nginx", "-t")
	if err := cmd.Run(); err != nil {
		return "Nginx configuration test failed"
	}
	return "healthy"
}

func checkApacheHealth(ctx context.Context) string {
	// Check Apache configuration
	cmd := exec.CommandContext(ctx, "apache2ctl", "configtest")
	if err := cmd.Run(); err != nil {
		// Try httpd command as fallback
		cmd = exec.CommandContext(ctx, "httpd", "-t")
		if err := cmd.Run(); err != nil {
			return "Apache configuration test failed"
		}
	}
	return "healthy"
}

func checkJenkinsHealth(ctx context.Context) string {
	// Check Jenkins via HTTP if running
	cmd := exec.CommandContext(ctx, "curl", "-s", "-f", "http://localhost:8080/login")
	if err := cmd.Run(); err != nil {
		return "Jenkins web interface not accessible"
	}
	return "healthy"
}

func checkElasticsearchHealth(ctx context.Context) string {
	// Check Elasticsearch cluster health
	cmd := exec.CommandContext(ctx, "curl", "-s", "http://localhost:9200/_cluster/health")
	output, err := cmd.Output()
	if err != nil {
		return "Elasticsearch cluster not accessible"
	}

	if strings.Contains(string(output), `"status":"red"`) {
		return "Elasticsearch cluster status is red"
	}
	return "healthy"
}

func checkPythonHealth(ctx context.Context) string {
	// Check Python installation and pip
	cmd := exec.CommandContext(ctx, "python3", "-c", "import pip; print('OK')")
	if err := cmd.Run(); err != nil {
		return "Python pip module not available"
	}
	return "healthy"
}

func checkGoHealth(ctx context.Context) string {
	// Check Go environment
	cmd := exec.CommandContext(ctx, "go", "env", "GOPATH")
	if err := cmd.Run(); err != nil {
		return "Go environment not properly configured"
	}
	return "healthy"
}

func checkRustHealth(ctx context.Context) string {
	// Check Rust toolchain
	cmd := exec.CommandContext(ctx, "rustc", "--version")
	if err := cmd.Run(); err != nil {
		return "Rust compiler not available"
	}
	return "healthy"
}

func checkJavaHealth(ctx context.Context) string {
	// Check Java runtime
	cmd := exec.CommandContext(ctx, "java", "-version")
	if err := cmd.Run(); err != nil {
		return "Java runtime not available"
	}
	return "healthy"
}
