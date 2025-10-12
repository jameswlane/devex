package commands_test

import (
	"context"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/jameswlane/devex/apps/cli/internal/commands"
	"github.com/jameswlane/devex/apps/cli/internal/installers/utilities"
)

var _ = Describe("Validation Functions", func() {
	Describe("ValidateDockerConfig", func() {
		It("accepts valid Docker configurations", func() {
			err := commands.ValidateDockerConfig("postgres16", "postgres:16", "5432:5432", "POSTGRES_HOST_AUTH_METHOD=trust")
			Expect(err).ToNot(HaveOccurred())
		})

		It("rejects invalid container names", func() {
			err := commands.ValidateDockerConfig("../malicious", "postgres:16", "5432:5432", "")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("invalid container name"))
		})

		It("rejects invalid Docker images", func() {
			err := commands.ValidateDockerConfig("postgres16", "../../malicious", "5432:5432", "")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("invalid Docker image"))
		})

		It("rejects invalid port mappings", func() {
			err := commands.ValidateDockerConfig("postgres16", "postgres:16", "malicious", "")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("invalid port mapping"))
		})

		It("rejects invalid environment variables", func() {
			err := commands.ValidateDockerConfig("postgres16", "postgres:16", "5432:5432", "malicious=../../bad")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("invalid environment variable"))
		})

		It("accepts empty environment variables", func() {
			err := commands.ValidateDockerConfig("postgres16", "postgres:16", "5432:5432", "")
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Describe("ValidatePath", func() {
		var tempDir string

		BeforeEach(func() {
			var err error
			tempDir, err = os.MkdirTemp("", "validation-test-*")
			Expect(err).ToNot(HaveOccurred())
		})

		AfterEach(func() {
			if tempDir != "" {
				os.RemoveAll(tempDir)
			}
		})

		It("accepts paths within the base directory", func() {
			validPath := filepath.Join(tempDir, "subdir", "file.txt")
			err := utilities.ValidatePath(validPath, tempDir)
			Expect(err).ToNot(HaveOccurred())
		})

		It("rejects directory traversal attempts", func() {
			maliciousPath := filepath.Join(tempDir, "..", "..", "etc", "passwd")
			err := utilities.ValidatePath(maliciousPath, tempDir)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("path traversal detected"))
		})

		It("rejects paths with .. components", func() {
			maliciousPath := tempDir + "/../../../etc/passwd"
			err := utilities.ValidatePath(maliciousPath, tempDir)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("path traversal detected"))
		})

		It("handles relative paths correctly", func() {
			err := utilities.ValidatePath("subdir/file.txt", tempDir)
			Expect(err).ToNot(HaveOccurred())
		})

		It("rejects relative paths with traversal", func() {
			err := utilities.ValidatePath("../../../etc/passwd", tempDir)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("path traversal detected"))
		})
	})

	Describe("ValidateShellCommand", func() {
		It("accepts valid shell paths and usernames", func() {
			err := commands.ValidateShellCommand("/bin/zsh", "testuser")
			Expect(err).ToNot(HaveOccurred())
		})

		It("accepts various valid shell paths", func() {
			validShells := []string{
				"/bin/bash",
				"/usr/bin/zsh",
				"/usr/local/bin/fish",
				"/opt/homebrew/bin/zsh",
			}
			for _, shell := range validShells {
				err := commands.ValidateShellCommand(shell, "testuser")
				Expect(err).ToNot(HaveOccurred())
			}
		})

		It("accepts various valid usernames", func() {
			validUsers := []string{
				"testuser",
				"user123",
				"user.name",
				"user-name",
				"user_name",
			}
			for _, user := range validUsers {
				err := commands.ValidateShellCommand("/bin/bash", user)
				Expect(err).ToNot(HaveOccurred())
			}
		})

		It("rejects invalid shell paths", func() {
			invalidShells := []string{
				"../../../bin/sh",
				"/bin/sh; rm -rf /",
				"/bin/sh && malicious",
				"relative/path",
				"",
			}
			for _, shell := range invalidShells {
				err := commands.ValidateShellCommand(shell, "testuser")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("invalid shell path"))
			}
		})

		It("rejects invalid usernames", func() {
			invalidUsers := []string{
				"user; rm -rf /",
				"user && malicious",
				"user|pipe",
				"user$(injection)",
				"",
			}
			for _, user := range invalidUsers {
				err := commands.ValidateShellCommand("/bin/bash", user)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("invalid username"))
			}
		})
	})

	Describe("ExecuteSecureShellChange", func() {
		It("validates input before execution", func() {
			ctx := context.Background()

			// Test with invalid shell path
			err := commands.ExecuteSecureShellChange(ctx, "invalid_shell", "testuser")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("invalid shell path"))

			// Test with invalid username
			err = commands.ExecuteSecureShellChange(ctx, "/bin/bash", "invalid; user")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("invalid username"))
		})

		It("constructs secure chsh command", func() {
			ctx := context.Background()

			// This will fail because we're not actually running as root, but we can verify
			// that the validation passes and the command structure is correct
			err := commands.ExecuteSecureShellChange(ctx, "/bin/bash", "testuser")

			// The validation should pass, but the actual chsh command will fail
			// which is expected in a test environment
			Expect(err).To(HaveOccurred())
			// Should not be a validation error, but a command execution error
			Expect(err.Error()).ToNot(ContainSubstring("invalid"))
		})
	})
})
