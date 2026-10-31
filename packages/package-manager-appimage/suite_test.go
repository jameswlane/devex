package main

import (
	"fmt"
	"os"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestAppimagePlugin(t *testing.T) {
	// is-installed exits the process by design, so dispatch it in a child test binary.
	if name := os.Getenv("DEVEX_APPIMAGE_STATUS_HELPER"); name != "" {
		if err := NewAppimagePlugin().Execute("is-installed", []string{name}); err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
		os.Exit(99) // A returned handler must fail the exit-code contract.
	}
	RegisterFailHandler(Fail)
	RunSpecs(t, "AppImage Plugin Suite")
}
