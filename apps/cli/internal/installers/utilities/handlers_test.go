package utilities_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/jameswlane/devex/apps/cli/internal/installers/utilities"
	"github.com/jameswlane/devex/apps/cli/internal/mocks"
	"github.com/jameswlane/devex/apps/cli/internal/utils"
)

var _ = Describe("Handler Registry", func() {
	Describe("NewHandlerRegistry", func() {
		It("should create registry with default handlers", func() {
			registry := utilities.NewHandlerRegistry()

			// Check that default Docker handlers are registered
			Expect(registry.HasHandler("docker")).To(BeTrue())
			Expect(registry.HasHandler("docker-ce")).To(BeTrue())
			Expect(registry.HasHandler("nginx")).To(BeTrue())
		})
	})

	Describe("custom handlers", func() {
		var registry *utilities.HandlerRegistry
		var executed bool
		var testHandler func() error

		BeforeEach(func() {
			registry = utilities.NewHandlerRegistry()
			executed = false
			testHandler = func() error {
				executed = true
				return nil
			}
		})

		It("should register and execute custom handlers", func() {
			registry.Register("test-package", testHandler)

			Expect(registry.HasHandler("test-package")).To(BeTrue())

			err := registry.Execute("test-package")
			Expect(err).ToNot(HaveOccurred())
			Expect(executed).To(BeTrue())
		})

		It("should return nil for packages without handlers", func() {
			err := registry.Execute("nonexistent-package")
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Describe("ExecutePostInstallHandler", func() {
		var executed bool
		var testHandler func() error

		BeforeEach(func() {
			executed = false
			testHandler = func() error {
				executed = true
				return nil
			}
		})

		It("should execute handler for registered package", func() {
			utilities.RegisterHandler("test-registry-package", testHandler)

			err := utilities.ExecutePostInstallHandler("test-registry-package")
			Expect(err).ToNot(HaveOccurred())
			Expect(executed).To(BeTrue())
		})

		Context("with mocked commands", func() {
			var originalExec utils.Interface

			BeforeEach(func() {
				// Store original CommandExec
				originalExec = utils.CommandExec

				// Create mock executor
				mockExec := mocks.NewMockCommandExecutor()
				utils.CommandExec = mockExec
			})

			AfterEach(func() {
				utils.CommandExec = originalExec
			})

			It("should handle package variations", func() {
				// Test that docker variations work
				Expect(utilities.DefaultRegistry.HasHandler("docker")).To(BeTrue())

				// Test execution with mocked commands
				err := utilities.ExecutePostInstallHandler("docker")
				// Should not fail with mocked commands
				Expect(err).ToNot(HaveOccurred())
			})
		})
	})
})
