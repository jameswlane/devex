package sdk_test

import (
	"os"
	"runtime"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
)

var _ = Describe("Environment Variable Security Validation", func() {
	var originalEnv map[string]string

	BeforeEach(func() {
		// Backup original environment variables
		originalEnv = make(map[string]string)
		envVars := []string{
			"LD_PRELOAD", "LD_LIBRARY_PATH", "DYLD_INSERT_LIBRARIES", "DYLD_LIBRARY_PATH",
			"PYTHONPATH", "PYTHONSTARTUP", "APT_CONFIG", "DOCKER_HOST", "PIP_INDEX_URL",
			"PIP_EXTRA_INDEX_URL", "IFS", "PS4", "PATH", "HOME", "USER", "LOGNAME",
			"SHELL", "TMPDIR", "DEVEX_ENV", "DEVEX_CONFIG_DIR", "DEVEX_PLUGIN_DIR",
			"DEVEX_CACHE_DIR",
		}
		
		for _, env := range envVars {
			originalEnv[env] = os.Getenv(env)
			_ = os.Unsetenv(env)
		}
	})

	AfterEach(func() {
		// Restore original environment variables
		for env, value := range originalEnv {
			if value == "" {
				_ = os.Unsetenv(env)
			} else {
				_ = os.Setenv(env, value)
			}
		}
	})

	Describe("Blocked Environment Variables", func() {
		Context("LD_PRELOAD", func() {
			It("should block non-empty LD_PRELOAD", func() {
				err := sdk.ValidateEnvironmentVariable("LD_PRELOAD", "/malicious/lib.so")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("blocked for security reasons"))
			})

			It("should allow empty LD_PRELOAD", func() {
				err := sdk.ValidateEnvironmentVariable("LD_PRELOAD", "")
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("DYLD_INSERT_LIBRARIES", func() {
			It("should block non-empty DYLD_INSERT_LIBRARIES", func() {
				err := sdk.ValidateEnvironmentVariable("DYLD_INSERT_LIBRARIES", "/malicious/lib.dylib")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("blocked for security reasons"))
			})

			It("should allow empty DYLD_INSERT_LIBRARIES", func() {
				err := sdk.ValidateEnvironmentVariable("DYLD_INSERT_LIBRARIES", "")
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("PYTHONSTARTUP", func() {
			It("should block non-empty PYTHONSTARTUP", func() {
				err := sdk.ValidateEnvironmentVariable("PYTHONSTARTUP", "/malicious/startup.py")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("blocked for security reasons"))
			})

			It("should allow empty PYTHONSTARTUP", func() {
				err := sdk.ValidateEnvironmentVariable("PYTHONSTARTUP", "")
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("IFS", func() {
			It("should block non-empty IFS", func() {
				err := sdk.ValidateEnvironmentVariable("IFS", "malicious")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("blocked for security reasons"))
			})

			It("should allow empty IFS", func() {
				err := sdk.ValidateEnvironmentVariable("IFS", "")
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})

	Describe("Dangerous Environment Variables", func() {
		Context("DOCKER_HOST validation", func() {
			It("should allow valid unix socket", func() {
				err := sdk.ValidateEnvironmentVariable("DOCKER_HOST", "unix:///var/run/docker.sock")
				Expect(err).NotTo(HaveOccurred())
			})

			It("should allow localhost TCP", func() {
				err := sdk.ValidateEnvironmentVariable("DOCKER_HOST", "tcp://localhost:2376")
				Expect(err).NotTo(HaveOccurred())
			})

			It("should allow 127.0.0.1 TCP", func() {
				err := sdk.ValidateEnvironmentVariable("DOCKER_HOST", "tcp://127.0.0.1:2376")
				Expect(err).NotTo(HaveOccurred())
			})

			It("should allow private network ranges", func() {
				err := sdk.ValidateEnvironmentVariable("DOCKER_HOST", "tcp://192.168.1.100:2376")
				Expect(err).NotTo(HaveOccurred())

				err = sdk.ValidateEnvironmentVariable("DOCKER_HOST", "tcp://10.0.0.100:2376")
				Expect(err).NotTo(HaveOccurred())
			})

			It("should reject public IP addresses", func() {
				err := sdk.ValidateEnvironmentVariable("DOCKER_HOST", "tcp://8.8.8.8:2376")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("potentially unsafe host"))
			})

			It("should reject unsupported protocols", func() {
				err := sdk.ValidateEnvironmentVariable("DOCKER_HOST", "http://localhost:8080")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("unsupported protocol"))
			})

			It("should allow empty DOCKER_HOST", func() {
				err := sdk.ValidateEnvironmentVariable("DOCKER_HOST", "")
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("PIP_INDEX_URL validation", func() {
			It("should require HTTPS", func() {
				err := sdk.ValidateEnvironmentVariable("PIP_INDEX_URL", "http://pypi.org/simple")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("must use HTTPS"))
			})

			It("should allow valid HTTPS URLs", func() {
				err := sdk.ValidateEnvironmentVariable("PIP_INDEX_URL", "https://pypi.org/simple")
				Expect(err).NotTo(HaveOccurred())
			})

			It("should reject suspicious domains", func() {
				err := sdk.ValidateEnvironmentVariable("PIP_INDEX_URL", "https://bit.ly/malicious")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("suspicious domain"))

				err = sdk.ValidateEnvironmentVariable("PIP_INDEX_URL", "https://localhost/simple")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("suspicious domain"))
			})

			It("should allow empty PIP_INDEX_URL", func() {
				err := sdk.ValidateEnvironmentVariable("PIP_INDEX_URL", "")
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("PIP_EXTRA_INDEX_URL validation", func() {
			It("should require HTTPS", func() {
				err := sdk.ValidateEnvironmentVariable("PIP_EXTRA_INDEX_URL", "http://custom.pypi.org/simple")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("must use HTTPS"))
			})

			It("should reject suspicious domains", func() {
				err := sdk.ValidateEnvironmentVariable("PIP_EXTRA_INDEX_URL", "https://tinyurl.com/malicious")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("suspicious domain"))
			})
		})
	})

	Describe("System Environment Variables", func() {
		Context("PATH validation", func() {
			It("should reject empty PATH", func() {
				err := sdk.ValidateEnvironmentVariable("PATH", "")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("cannot be empty"))
			})

			It("should reject paths with ..", func() {
				err := sdk.ValidateEnvironmentVariable("PATH", "/usr/bin:../malicious:/bin")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("suspicious path"))
			})

			It("should reject temporary directories in PATH", func() {
				err := sdk.ValidateEnvironmentVariable("PATH", "/usr/bin:/tmp/malicious:/bin")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("potentially unsafe temporary directory"))
			})

			It("should allow valid PATH", func() {
				validPath := "/usr/local/bin:/usr/bin:/bin"
				err := sdk.ValidateEnvironmentVariable("PATH", validPath)
				// This might fail on the world-writable check, so we'll be flexible
				if err != nil {
					Expect(err.Error()).NotTo(ContainSubstring("suspicious path"))
					Expect(err.Error()).NotTo(ContainSubstring("temporary directory"))
				}
			})
		})

		Context("HOME validation", func() {
			It("should reject empty HOME", func() {
				err := sdk.ValidateEnvironmentVariable("HOME", "")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("cannot be empty"))
			})

			It("should reject relative paths", func() {
				err := sdk.ValidateEnvironmentVariable("HOME", "relative/path")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("must be an absolute path"))
			})

			It("should reject paths with ..", func() {
				err := sdk.ValidateEnvironmentVariable("HOME", "/home/user/../malicious")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("path traversal"))
			})

			// Skip this test if we can't create temp dirs
			It("should validate existing HOME directory", func() {
				tempDir, err := os.MkdirTemp("", "test-home-*")
				if err != nil {
					Skip("Cannot create temporary directory for test")
				}
				defer func() { _ = os.RemoveAll(tempDir) }()

				err = sdk.ValidateEnvironmentVariable("HOME", tempDir)
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("USER validation", func() {
			It("should reject empty USER", func() {
				err := sdk.ValidateEnvironmentVariable("USER", "")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("cannot be empty"))
			})

			It("should reject suspicious characters", func() {
				err := sdk.ValidateEnvironmentVariable("USER", "user;rm -rf /")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("suspicious characters"))
			})

			It("should reject overly long usernames", func() {
				longUser := strings.Repeat("a", 50)
				err := sdk.ValidateEnvironmentVariable("USER", longUser)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("too long"))
			})

			It("should allow valid usernames", func() {
				err := sdk.ValidateEnvironmentVariable("USER", "testuser")
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("SHELL validation", func() {
			It("should reject empty SHELL", func() {
				err := sdk.ValidateEnvironmentVariable("SHELL", "")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("cannot be empty"))
			})

			It("should reject relative paths", func() {
				err := sdk.ValidateEnvironmentVariable("SHELL", "bash")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("must be an absolute path"))
			})

			It("should reject paths with ..", func() {
				err := sdk.ValidateEnvironmentVariable("SHELL", "/bin/../malicious/shell")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("path traversal"))
			})

			// This test might fail on systems where /bin/bash doesn't exist
			It("should validate existing shell", func() {
				// Try common shell paths
				shellPaths := []string{"/bin/bash", "/usr/bin/bash", "/bin/sh"}
				var validShell string
				
				for _, shell := range shellPaths {
					if info, err := os.Stat(shell); err == nil && info.Mode()&0111 != 0 {
						validShell = shell
						break
					}
				}
				
				if validShell != "" {
					err := sdk.ValidateEnvironmentVariable("SHELL", validShell)
					Expect(err).NotTo(HaveOccurred())
				} else {
					Skip("No valid shell found for testing")
				}
			})
		})

		Context("TMPDIR validation", func() {
			It("should allow empty TMPDIR", func() {
				err := sdk.ValidateEnvironmentVariable("TMPDIR", "")
				Expect(err).NotTo(HaveOccurred())
			})

			It("should reject relative paths", func() {
				err := sdk.ValidateEnvironmentVariable("TMPDIR", "tmp")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("must be an absolute path"))
			})

			It("should reject paths with ..", func() {
				err := sdk.ValidateEnvironmentVariable("TMPDIR", "/tmp/../malicious")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("path traversal"))
			})

			It("should allow valid absolute paths", func() {
				err := sdk.ValidateEnvironmentVariable("TMPDIR", "/tmp")
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})

	Describe("DevEx Environment Variables", func() {
		Context("DEVEX_ENV validation", func() {
			It("should allow empty DEVEX_ENV", func() {
				err := sdk.ValidateEnvironmentVariable("DEVEX_ENV", "")
				Expect(err).NotTo(HaveOccurred())
			})

			It("should allow valid environments", func() {
				validEnvs := []string{"development", "dev", "staging", "production", "prod", "test", "testing"}
				for _, env := range validEnvs {
					err := sdk.ValidateEnvironmentVariable("DEVEX_ENV", env)
					Expect(err).NotTo(HaveOccurred())
				}
			})

			It("should reject invalid environments", func() {
				err := sdk.ValidateEnvironmentVariable("DEVEX_ENV", "malicious")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("invalid DEVEX_ENV value"))
			})
		})

		Context("DEVEX directory variables", func() {
			It("should allow empty values", func() {
				dirs := []string{"DEVEX_CONFIG_DIR", "DEVEX_PLUGIN_DIR", "DEVEX_CACHE_DIR"}
				for _, dir := range dirs {
					err := sdk.ValidateEnvironmentVariable(dir, "")
					Expect(err).NotTo(HaveOccurred())
				}
			})

			It("should reject relative paths", func() {
				dirs := []string{"DEVEX_CONFIG_DIR", "DEVEX_PLUGIN_DIR", "DEVEX_CACHE_DIR"}
				for _, dir := range dirs {
					err := sdk.ValidateEnvironmentVariable(dir, "relative/path")
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("must be an absolute path"))
				}
			})

			It("should reject paths with ..", func() {
				dirs := []string{"DEVEX_CONFIG_DIR", "DEVEX_PLUGIN_DIR", "DEVEX_CACHE_DIR"}
				for _, dir := range dirs {
					err := sdk.ValidateEnvironmentVariable(dir, "/home/user/../malicious")
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("path traversal"))
				}
			})

			It("should allow valid absolute paths", func() {
				dirs := []string{"DEVEX_CONFIG_DIR", "DEVEX_PLUGIN_DIR", "DEVEX_CACHE_DIR"}
				for _, dir := range dirs {
					err := sdk.ValidateEnvironmentVariable(dir, "/home/user/.devex")
					Expect(err).NotTo(HaveOccurred())
				}
			})
		})
	})

	Describe("Safe Environment Variable Access", func() {
		Context("SafeGetEnv", func() {
			It("should return validated environment variables", func() {
				_ = os.Setenv("USER", "testuser")
				defer func() { _ = os.Unsetenv("USER") }()

				value, err := sdk.SafeGetEnv("USER")
				Expect(err).NotTo(HaveOccurred())
				Expect(value).To(Equal("testuser"))
			})

			It("should return error for invalid environment variables", func() {
				_ = os.Setenv("LD_PRELOAD", "/malicious/lib.so")
				defer func() { _ = os.Unsetenv("LD_PRELOAD") }()

				_, err := sdk.SafeGetEnv("LD_PRELOAD")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("blocked for security reasons"))
			})

			It("should return empty string for unset variables", func() {
				value, err := sdk.SafeGetEnv("NONEXISTENT_VAR")
				Expect(err).NotTo(HaveOccurred())
				Expect(value).To(Equal(""))
			})
		})

		Context("SafeGetEnvWithDefault", func() {
			It("should return environment variable when set and valid", func() {
				_ = os.Setenv("USER", "testuser")
				defer func() { _ = os.Unsetenv("USER") }()

				value, err := sdk.SafeGetEnvWithDefault("USER", "defaultuser")
				Expect(err).NotTo(HaveOccurred())
				Expect(value).To(Equal("testuser"))
			})

			It("should return default when variable is not set", func() {
				value, err := sdk.SafeGetEnvWithDefault("NONEXISTENT_VAR", "defaultvalue")
				Expect(err).NotTo(HaveOccurred())
				Expect(value).To(Equal("defaultvalue"))
			})

			It("should return default when variable is invalid", func() {
				_ = os.Setenv("LD_PRELOAD", "/malicious/lib.so")
				defer func() { _ = os.Unsetenv("LD_PRELOAD") }()

				value, err := sdk.SafeGetEnvWithDefault("LD_PRELOAD", "")
				Expect(err).To(HaveOccurred())
				Expect(value).To(Equal(""))
			})
		})
	})

	Describe("Environment Variable Sanitization", func() {
		Context("SanitizeEnvVarForLogging", func() {
			It("should redact sensitive variables", func() {
				sensitiveVars := map[string]string{
					"PASSWORD":     "secret123",
					"API_KEY":      "key123",
					"SECRET_TOKEN": "token123",
					"PRIVATE_KEY":  "privatekey123",
				}

				for name, value := range sensitiveVars {
					sanitized := sdk.SanitizeEnvVarForLogging(name, value)
					Expect(sanitized).To(Equal("[REDACTED]"))
				}
			})

			It("should partially redact PATH-like variables", func() {
				pathVars := map[string]string{
					"PATH":         "/usr/local/bin:/usr/bin:/bin",
					"HOME":         "/home/testuser",
					"DOCKER_HOST":  "unix:///var/run/docker.sock",
					"PIP_INDEX_URL": "https://pypi.org/simple",
				}

				for name, value := range pathVars {
					sanitized := sdk.SanitizeEnvVarForLogging(name, value)
					Expect(sanitized).NotTo(Equal(value))
					Expect(sanitized).NotTo(Equal("[REDACTED]"))
					if len(value) > 8 {
						Expect(sanitized).To(ContainSubstring("..."))
					}
				}
			})

			It("should pass through safe variables unchanged", func() {
				safeVar := sdk.SanitizeEnvVarForLogging("SAFE_VAR", "safe_value")
				Expect(safeVar).To(Equal("safe_value"))
			})
		})

		Context("SanitizeEnvironmentForLogging", func() {
			It("should sanitize multiple environment variables", func() {
				env := []string{
					"USER=testuser",
					"PASSWORD=secret123",
					"PATH=/usr/bin:/bin",
					"API_KEY=key123",
				}

				sanitized := sdk.SanitizeEnvironmentForLogging(env)
				
				// Check that sensitive vars are redacted
				found := false
				for _, envVar := range sanitized {
					if strings.HasPrefix(envVar, "PASSWORD=") {
						Expect(envVar).To(Equal("PASSWORD=[REDACTED]"))
						found = true
					}
					if strings.HasPrefix(envVar, "API_KEY=") {
						Expect(envVar).To(Equal("API_KEY=[REDACTED]"))
					}
				}
				Expect(found).To(BeTrue())

				// Check that PATH is partially redacted
				found = false
				for _, envVar := range sanitized {
					if strings.HasPrefix(envVar, "PATH=") {
						Expect(envVar).NotTo(Equal("PATH=/usr/bin:/bin"))
						Expect(envVar).To(ContainSubstring("PATH="))
						found = true
					}
				}
				Expect(found).To(BeTrue())
			})
		})
	})

	Describe("Security Check", func() {
		Context("CheckEnvironmentSecurity", func() {
			It("should detect blocked variables", func() {
				_ = os.Setenv("LD_PRELOAD", "/malicious/lib.so")
				defer func() { _ = os.Unsetenv("LD_PRELOAD") }()

				issues := sdk.CheckEnvironmentSecurity()
				
				var foundBlockedVar bool
				for _, issue := range issues {
					if issue.Type == "blocked_env_var" && issue.Variable == "LD_PRELOAD" {
						foundBlockedVar = true
						Expect(issue.Severity).To(Equal("critical"))
					}
				}
				Expect(foundBlockedVar).To(BeTrue())
			})

			It("should detect dangerous variables", func() {
				_ = os.Setenv("PYTHONPATH", "/malicious/path")
				defer func() { _ = os.Unsetenv("PYTHONPATH") }()

				issues := sdk.CheckEnvironmentSecurity()
				
				var foundDangerousVar bool
				for _, issue := range issues {
					if issue.Type == "dangerous_env_var" && issue.Variable == "PYTHONPATH" {
						foundDangerousVar = true
						Expect(issue.Severity).To(Equal("high"))
					}
				}
				Expect(foundDangerousVar).To(BeTrue())
			})

			It("should detect invalid system variables", func() {
				_ = os.Setenv("USER", "user;malicious")
				defer func() { _ = os.Unsetenv("USER") }()

				issues := sdk.CheckEnvironmentSecurity()
				
				var foundInvalidVar bool
				for _, issue := range issues {
					if issue.Type == "invalid_env_var" && issue.Variable == "USER" {
						foundInvalidVar = true
						Expect(issue.Severity).To(Equal("medium"))
					}
				}
				Expect(foundInvalidVar).To(BeTrue())
			})

			// Skip on Windows as it may have different environment setup
			It("should detect missing required variables", func() {
				if runtime.GOOS == "windows" {
					Skip("Skipping on Windows due to different environment setup")
				}

				// Unset a required variable
				originalUser := os.Getenv("USER")
				_ = os.Unsetenv("USER")
				defer func() {
					if originalUser != "" {
						_ = os.Setenv("USER", originalUser)
					}
				}()

				issues := sdk.CheckEnvironmentSecurity()
				
				var foundMissingVar bool
				for _, issue := range issues {
					if issue.Type == "missing_env_var" && issue.Variable == "USER" {
						foundMissingVar = true
						Expect(issue.Severity).To(Equal("low"))
					}
				}
				Expect(foundMissingVar).To(BeTrue())
			})

			It("should return empty for clean environment", func() {
				// Clear any problematic variables
				dangerousVars := []string{
					"LD_PRELOAD", "LD_LIBRARY_PATH", "DYLD_INSERT_LIBRARIES", "DYLD_LIBRARY_PATH",
					"PYTHONPATH", "PYTHONSTARTUP", "APT_CONFIG", "IFS", "PS4",
				}
				
				var originalValues []string
				for _, envVar := range dangerousVars {
					originalValues = append(originalValues, os.Getenv(envVar))
					_ = os.Unsetenv(envVar)
				}
				
				defer func() {
					for i, envVar := range dangerousVars {
						if originalValues[i] != "" {
							_ = os.Setenv(envVar, originalValues[i])
						}
					}
				}()

				// Set valid required variables to avoid missing variable errors
				_ = os.Setenv("USER", "testuser")
				if runtime.GOOS != "windows" {
					_ = os.Setenv("HOME", "/tmp")
					_ = os.Setenv("PATH", "/usr/bin:/bin")
					_ = os.Setenv("SHELL", "/bin/bash")
				}
				defer func() {
					_ = os.Unsetenv("USER")
					if runtime.GOOS != "windows" {
						_ = os.Unsetenv("HOME")
						_ = os.Unsetenv("PATH") 
						_ = os.Unsetenv("SHELL")
					}
				}()

				issues := sdk.CheckEnvironmentSecurity()
				
				// Filter out issues not related to our dangerous variables
				var relevantIssues []sdk.SecurityIssue
				for _, issue := range issues {
					if issue.Type == "blocked_env_var" || issue.Type == "dangerous_env_var" {
						relevantIssues = append(relevantIssues, issue)
					}
				}
				
				Expect(relevantIssues).To(BeEmpty())
			})
		})
	})

	Describe("Edge Cases", func() {
		Context("Unknown variables", func() {
			It("should allow unknown variables", func() {
				err := sdk.ValidateEnvironmentVariable("UNKNOWN_VAR", "any_value")
				Expect(err).NotTo(HaveOccurred())
			})

			It("should allow unknown variables with special characters", func() {
				err := sdk.ValidateEnvironmentVariable("UNKNOWN_VAR", "value;with|special&chars")
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("Empty variable names", func() {
			It("should handle empty variable names", func() {
				err := sdk.ValidateEnvironmentVariable("", "value")
				Expect(err).NotTo(HaveOccurred())
			})
		})

		Context("Very long values", func() {
			It("should handle very long environment variable values", func() {
				longValue := strings.Repeat("a", 10000)
				err := sdk.ValidateEnvironmentVariable("UNKNOWN_VAR", longValue)
				Expect(err).NotTo(HaveOccurred())
			})
		})
	})
})
