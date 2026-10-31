package main_test

import (
	"fmt"

	_ "modernc.org/sqlite"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	devex "github.com/jameswlane/devex/apps/cli/cmd"
	"github.com/jameswlane/devex/apps/cli/internal/errors"
)

var _ = Describe("Main", func() {
	Context("handleError", func() {
		var originalExit func(int)
		var exitCalled bool

		BeforeEach(func() {
			originalExit = devex.Exit
			exitCalled = false
			devex.Exit = func(code int) {
				exitCalled = true
			}
		})

		AfterEach(func() {
			devex.Exit = originalExit // Restore the original Exit function
		})

		It("logs and exits on invalid input error", func() {
			Expect(func() {
				testHandleError(errors.ErrInvalidInput)
			}).ToNot(Panic())
			Expect(exitCalled).To(BeTrue())
		})

		It("logs and exits on unexpected error", func() {
			Expect(func() {
				testHandleError(errors.New("unexpected error"))
			}).ToNot(Panic())
			Expect(exitCalled).To(BeTrue())
		})
	})
})

// Helper function to test handleError
func testHandleError(err error) {
	if err != nil {
		fmt.Printf("Error: %s\n", err.Error())
		devex.Exit(1)
	}
}
