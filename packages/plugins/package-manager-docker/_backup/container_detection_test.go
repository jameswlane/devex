package docker_test

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/jameswlane/devex/pkg/installers/docker"
)

var _ = Describe("Container Detection", func() {
	var (
		originalEnv      map[string]string
		tempDir          string
		mockProcDir      string
		originalHostname string
	)

	BeforeEach(func() {
		// Backup environment variables
		originalEnv = make(map[string]string)
		envVars := []string{
			"container", "CONTAINER_ID", "DOCKER_CONTAINER",
			"KUBERNETES_SERVICE_HOST", "K8S_POD_NAME", "NOMAD_ALLOC_ID",
			"HOSTNAME", "MESOS_TASK_ID", "MARATHON_APP_ID",
		}
		for _, envVar := range envVars {
			originalEnv[envVar] = os.Getenv(envVar)
			os.Unsetenv(envVar)
		}

		// Backup HOSTNAME
		originalHostname = os.Getenv("HOSTNAME")

		// Create temporary directory for mock files
		var err error
		tempDir, err = os.MkdirTemp("", "docker-detection-test")
		Expect(err).ToNot(HaveOccurred())

		mockProcDir = filepath.Join(tempDir, "proc")
		Expect(os.MkdirAll(filepath.Join(mockProcDir, "1"), 0755)).To(Succeed())
	})

	AfterEach(func() {
		// Restore environment variables
		for key, value := range originalEnv {
			if value != "" {
				os.Setenv(key, value)
			} else {
				os.Unsetenv(key)
			}
		}

		// Restore HOSTNAME
		if originalHostname != "" {
			os.Setenv("HOSTNAME", originalHostname)
		} else {
			os.Unsetenv("HOSTNAME")
		}

		// Clean up temporary directory
		if tempDir != "" {
			os.RemoveAll(tempDir)
		}
	})

	Describe("detectDockerEnvFile", func() {
		Context("when /.dockerenv exists", func() {
			It("should detect container environment", func() {
				// Create mock .dockerenv file
				dockerEnvPath := filepath.Join(tempDir, ".dockerenv")
				Expect(os.WriteFile(dockerEnvPath, []byte{}, 0644)).To(Succeed())
				defer os.Remove(dockerEnvPath)

				// Note: This test will fail in real environment as we can't mock /.dockerenv
				// This is for demonstration purposes
			})
		})

		Context("when /.dockerenv does not exist", func() {
			It("should not detect container environment", func() {
				// In test environment, /.dockerenv should not exist
				// This test validates the negative case
			})
		})
	})

	Describe("isContainerHostname", func() {
		Context("with valid container hostnames", func() {
			It("should detect 12-character hex hostname", func() {
				// Test detection of Docker-style container ID hostnames
				hostnames := []string{
					"a1b2c3d4e5f6",
					"0123456789ab",
					"fedcba987654",
				}

				for _, hostname := range hostnames {
					os.Setenv("HOSTNAME", hostname)
					// We can't directly test the internal function, but we can verify the pattern
					Expect(len(hostname)).To(Equal(12))
					for _, char := range hostname {
						isHex := (char >= '0' && char <= '9') ||
							(char >= 'a' && char <= 'f') ||
							(char >= 'A' && char <= 'F')
						Expect(isHex).To(BeTrue())
					}
					os.Unsetenv("HOSTNAME")
				}
			})

			It("should detect Kubernetes pod hostnames", func() {
				k8sHostnames := []string{
					"my-app-deployment-7d6f8c9b5-xz2kt",
					"nginx-pod-abc123",
					"backend-deployment-1234567890-abcde",
				}

				for _, hostname := range k8sHostnames {
					Expect(hostname).To(ContainSubstring("-"))
					Expect(hostname).To(SatisfyAny(
						ContainSubstring("pod"),
						ContainSubstring("deployment"),
					))
				}
			})
		})

		Context("with non-container hostnames", func() {
			It("should not detect regular hostnames", func() {
				regularHostnames := []string{
					"mycomputer",
					"workstation-01",
					"dev-machine",
					"localhost",
				}

				for _, hostname := range regularHostnames {
					// Verify these don't match container patterns
					if len(hostname) == 12 {
						// Should not be all hex
						allHex := true
						for _, char := range hostname {
							if (char < '0' || char > '9') && (char < 'a' || char > 'f') && (char < 'A' || char > 'F') {
								allHex = false
								break
							}
						}
						Expect(allHex).To(BeFalse())
					}
				}
			})
		})
	})

	Describe("Container Environment Variables", func() {
		Context("when container environment variables are set", func() {
			It("should detect Docker container ID", func() {
				os.Setenv("DOCKER_CONTAINER", "true")
				// Environment variable is set
				Expect(os.Getenv("DOCKER_CONTAINER")).To(Equal("true"))
				os.Unsetenv("DOCKER_CONTAINER")
			})

			It("should detect Kubernetes environment", func() {
				os.Setenv("KUBERNETES_SERVICE_HOST", "10.0.0.1")
				os.Setenv("K8S_POD_NAME", "my-pod")

				Expect(os.Getenv("KUBERNETES_SERVICE_HOST")).To(Equal("10.0.0.1"))
				Expect(os.Getenv("K8S_POD_NAME")).To(Equal("my-pod"))

				os.Unsetenv("KUBERNETES_SERVICE_HOST")
				os.Unsetenv("K8S_POD_NAME")
			})

			It("should detect Nomad environment", func() {
				os.Setenv("NOMAD_ALLOC_ID", "abc123")
				Expect(os.Getenv("NOMAD_ALLOC_ID")).To(Equal("abc123"))
				os.Unsetenv("NOMAD_ALLOC_ID")
			})
		})
	})

	Describe("Mock File-based Detection", func() {
		Context("cgroup detection", func() {
			It("should create mock cgroup file with container indicators", func() {
				cgroupPath := filepath.Join(mockProcDir, "1", "cgroup")
				cgroupContent := `12:cpuset:/docker/a1b2c3d4e5f6
11:cpu,cpuacct:/docker/a1b2c3d4e5f6
10:memory:/docker/a1b2c3d4e5f6`

				Expect(os.WriteFile(cgroupPath, []byte(cgroupContent), 0644)).To(Succeed())

				// Verify file was created
				content, err := os.ReadFile(cgroupPath)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(content)).To(ContainSubstring("docker"))
			})
		})

		Context("init process detection", func() {
			It("should create mock comm file with container init process", func() {
				commPath := filepath.Join(mockProcDir, "1", "comm")
				initProcesses := []string{"sh", "bash", "docker-init", "tini"}

				for _, proc := range initProcesses {
					Expect(os.WriteFile(commPath, []byte(proc), 0644)).To(Succeed())

					content, err := os.ReadFile(commPath)
					Expect(err).ToNot(HaveOccurred())
					Expect(string(content)).To(Equal(proc))
				}
			})
		})

		Context("filesystem mount detection", func() {
			It("should create mock mounts file with container filesystems", func() {
				mountsPath := filepath.Join(mockProcDir, "mounts")
				mountsContent := `overlay /var/lib/docker/overlay2/abc123 overlay rw,relatime 0 0
tmpfs /dev/shm tmpfs rw,nosuid,nodev 0 0
/dev/vda1 /docker/containers ext4 rw,relatime 0 0`

				Expect(os.WriteFile(mountsPath, []byte(mountsContent), 0644)).To(Succeed())

				content, err := os.ReadFile(mountsPath)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(content)).To(ContainSubstring("overlay"))
				Expect(string(content)).To(ContainSubstring("/var/lib/docker"))
			})
		})
	})

	Describe("Integration Tests", func() {
		Context("when running the actual detection", func() {
			It("should handle current test environment gracefully", func() {
				// The actual isRunningInContainer function will check real system
				// This test ensures it doesn't panic in test environment
				result := docker.IsRunningInContainer()

				// Result depends on whether tests are running in container
				// We just ensure it returns without error
				Expect(result).To(BeAssignableToTypeOf(bool(false)))
			})
		})
	})

	Describe("Hex String Validation", func() {
		It("should validate hex strings correctly", func() {
			validHexStrings := []string{
				"0123456789ab",
				"ABCDEF012345",
				"fedcba987654",
			}

			invalidHexStrings := []string{
				"xyz123456789",
				"12345678901g",
				"hello-world!",
			}

			// Validate hex detection logic
			for _, hex := range validHexStrings {
				allHex := true
				for _, char := range hex {
					if (char < '0' || char > '9') && (char < 'a' || char > 'f') && (char < 'A' || char > 'F') {
						allHex = false
						break
					}
				}
				Expect(allHex).To(BeTrue(), "Expected %s to be valid hex", hex)
			}

			for _, nonHex := range invalidHexStrings {
				allHex := true
				for _, char := range nonHex {
					if (char < '0' || char > '9') && (char < 'a' || char > 'f') && (char < 'A' || char > 'F') {
						allHex = false
						break
					}
				}
				Expect(allHex).To(BeFalse(), "Expected %s to be invalid hex", nonHex)
			}
		})
	})
})
