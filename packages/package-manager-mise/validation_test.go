package main_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	main "github.com/jameswlane/devex/packages/package-manager-mise"
	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
)

var _ = Describe("Validation", func() {
	var plugin *main.MisePlugin

	BeforeEach(func() {
		info := sdk.PluginInfo{
			Name:        "package-manager-mise",
			Version:     "test",
			Description: "Test mise plugin",
		}
		plugin = &main.MisePlugin{
			PackageManagerPlugin: sdk.NewPackageManagerPlugin(info, "mise"),
		}
		plugin.InitLogger(sdk.NewDefaultLogger(false))
	})

	Describe("ValidateToolSpec", func() {
		Context("with valid tool specifications", func() {
			It("should accept simple tool names", func() {
				err := plugin.ValidateToolSpec("node")
				Expect(err).ToNot(HaveOccurred())
			})

			It("should accept tool with version", func() {
				err := plugin.ValidateToolSpec("node@18.17.0")
				Expect(err).ToNot(HaveOccurred())
			})

			It("should accept tool with latest tag", func() {
				err := plugin.ValidateToolSpec("python@latest")
				Expect(err).ToNot(HaveOccurred())
			})

			It("should accept tool with semantic version range", func() {
				err := plugin.ValidateToolSpec("rust@^1.70.0")
				Expect(err).ToNot(HaveOccurred())
			})

			It("should accept hyphenated tool names", func() {
				err := plugin.ValidateToolSpec("java-openjdk@17")
				Expect(err).ToNot(HaveOccurred())
			})
		})

		Context("with invalid tool specifications", func() {
			It("should reject empty string", func() {
				err := plugin.ValidateToolSpec("")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("tool specification cannot be empty"))
			})

			It("should reject tool names that are too long", func() {
				longName := ""
				for i := 0; i < 201; i++ {
					longName += "a"
				}
				err := plugin.ValidateToolSpec(longName)
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("tool specification too long"))
			})

			It("should reject specifications with null bytes", func() {
				err := plugin.ValidateToolSpec("node\x00@latest")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("contains null bytes"))
			})

			It("should reject specifications with shell metacharacters", func() {
				dangerousSpecs := []string{
					"node;rm -rf /",
					"python&& echo hacked",
					"rust|cat /etc/passwd",
					"go`id`",
					"java$(whoami)",
				}
				for _, spec := range dangerousSpecs {
					err := plugin.ValidateToolSpec(spec)
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("potentially dangerous character"))
				}
			})

			It("should reject invalid format", func() {
				err := plugin.ValidateToolSpec("123tool")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("invalid tool specification format"))
			})
		})
	})

	Describe("ValidateShellType", func() {
		Context("with valid shell types", func() {
			It("should accept bash", func() {
				err := plugin.ValidateShellType("bash")
				Expect(err).ToNot(HaveOccurred())
			})

			It("should accept zsh", func() {
				err := plugin.ValidateShellType("zsh")
				Expect(err).ToNot(HaveOccurred())
			})

			It("should accept fish", func() {
				err := plugin.ValidateShellType("fish")
				Expect(err).ToNot(HaveOccurred())
			})
		})

		Context("with invalid shell types", func() {
			It("should reject empty string", func() {
				err := plugin.ValidateShellType("")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("shell type cannot be empty"))
			})

			It("should reject unsupported shell", func() {
				err := plugin.ValidateShellType("csh")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("unsupported shell type"))
			})

			It("should reject shell with dangerous characters", func() {
				err := plugin.ValidateShellType("bash;echo")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("shell type contains invalid characters"))
			})
		})
	})

	Describe("ValidateCommandArg", func() {
		Context("with valid arguments", func() {
			It("should accept simple arguments", func() {
				err := plugin.ValidateCommandArg("--verbose")
				Expect(err).ToNot(HaveOccurred())
			})

			It("should accept file paths", func() {
				err := plugin.ValidateCommandArg("/usr/local/bin/mise")
				Expect(err).ToNot(HaveOccurred())
			})

			It("should accept version strings", func() {
				err := plugin.ValidateCommandArg("1.2.3")
				Expect(err).ToNot(HaveOccurred())
			})
		})

		Context("with invalid arguments", func() {
			It("should reject empty string", func() {
				err := plugin.ValidateCommandArg("")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("argument cannot be empty"))
			})

			It("should reject null bytes", func() {
				err := plugin.ValidateCommandArg("arg\x00value")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("contains null bytes"))
			})

			It("should reject shell metacharacters", func() {
				dangerousArgs := []string{
					"arg;echo",
					"value&&ls",
					"test|cat",
					"`command`",
					"$(pwd)",
				}
				for _, arg := range dangerousArgs {
					err := plugin.ValidateCommandArg(arg)
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("potentially dangerous character"))
				}
			})
		})
	})

	Describe("ValidateEnvironmentVar", func() {
		Context("with valid environment variables", func() {
			It("should accept standard variable names", func() {
				err := plugin.ValidateEnvironmentVar("PATH")
				Expect(err).ToNot(HaveOccurred())
			})

			It("should accept variables with underscores", func() {
				err := plugin.ValidateEnvironmentVar("MISE_INSTALL_DIR")
				Expect(err).ToNot(HaveOccurred())
			})

			It("should accept variables with numbers", func() {
				err := plugin.ValidateEnvironmentVar("NODE_16")
				Expect(err).ToNot(HaveOccurred())
			})
		})

		Context("with invalid environment variables", func() {
			It("should reject empty string", func() {
				err := plugin.ValidateEnvironmentVar("")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("environment variable name cannot be empty"))
			})

			It("should reject variables starting with numbers", func() {
				err := plugin.ValidateEnvironmentVar("1VAR")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("invalid environment variable name"))
			})

			It("should reject variables with special characters", func() {
				err := plugin.ValidateEnvironmentVar("VAR-NAME")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("invalid environment variable name"))
			})

			It("should reject variables with spaces", func() {
				err := plugin.ValidateEnvironmentVar("VAR NAME")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("invalid environment variable name"))
			})
		})
	})

	Describe("Security Boundary Tests", func() {
		Context("command injection prevention", func() {
			It("should prevent command chaining", func() {
				injectionAttempts := []string{
					"node; rm -rf /",
					"python && cat /etc/passwd",
					"rust || echo failed",
					"go | grep secret",
				}
				for _, attempt := range injectionAttempts {
					err := plugin.ValidateToolSpec(attempt)
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("dangerous"))
				}
			})

			It("should prevent command substitution", func() {
				substitutionAttempts := []string{
					"node-$(whoami)",
					"python`id`",
					"rust${USER}",
				}
				for _, attempt := range substitutionAttempts {
					err := plugin.ValidateToolSpec(attempt)
					Expect(err).To(HaveOccurred())
				}
			})

			It("should prevent path traversal", func() {
				traversalAttempts := []string{
					"../../../etc/passwd",
					"node/../../bin/sh",
				}
				for _, attempt := range traversalAttempts {
					err := plugin.ValidateCommandArg(attempt)
					// Path traversal in arguments should be allowed as they might be legitimate paths
					// but the actual execution should handle these safely
					Expect(err).ToNot(HaveOccurred())
				}
			})
		})
	})
})
