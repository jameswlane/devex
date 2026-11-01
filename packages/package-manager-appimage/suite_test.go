package main

import (
	"fmt"
	"os"
	"testing"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestAppimagePlugin(t *testing.T) {
	// Check is-installed's exit status as a real plugin process, in a child test binary.
	if name := os.Getenv("DEVEX_APPIMAGE_STATUS_HELPER"); name != "" {
		// Exit the way sdk.HandleArgs does for a real plugin process.
		err := NewAppimagePlugin().Execute("is-installed", []string{name})
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
		os.Exit(sdk.ExitStatus(err))
	}
	RegisterFailHandler(Fail)
	RunSpecs(t, "AppImage Plugin Suite")
}
