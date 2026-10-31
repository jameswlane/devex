package commands

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/cobra"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"

	"github.com/jameswlane/devex/apps/cli/internal/types"
)

var _ = Describe("Inline curlpipe removal", func() {
	It("returns the typed manual-removal error with the application name", func() {
		app := types.CrossPlatformApp{Name: "neovim", AllPlatforms: types.OSConfig{
			InstallMethod: "curlpipe", InstallCommand: "https://example.com/install.sh",
		}}
		err := uninstallApplication(&cobra.Command{}, app, nil)
		var manual *sdk.ManualRemovalRequiredError
		Expect(errors.As(err, &manual)).To(BeTrue())
		Expect(manual.Applications).To(Equal([]string{app.Name}))
	})

	It("runs an explicit uninstall command without requiring the curlpipe plugin", func() {
		marker := filepath.Join(GinkgoT().TempDir(), "removed")
		app := types.CrossPlatformApp{Name: "example", AllPlatforms: types.OSConfig{
			InstallMethod: "curlpipe", UninstallCommand: fmt.Sprintf("printf removed > '%s'", marker),
		}}
		cmd := &cobra.Command{}
		cmd.SetContext(context.Background())
		Expect(uninstallApplication(cmd, app, nil)).To(Succeed())
		contents, err := os.ReadFile(marker)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(contents)).To(Equal("removed"))
	})

	It("honors the caller's cancellation before running the uninstall command", func() {
		marker := filepath.Join(GinkgoT().TempDir(), "removed")
		app := types.CrossPlatformApp{Name: "example", AllPlatforms: types.OSConfig{
			InstallMethod: "curlpipe", UninstallCommand: fmt.Sprintf("printf removed > '%s'", marker),
		}}
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		cmd := &cobra.Command{}
		cmd.SetContext(ctx)
		err := uninstallApplication(cmd, app, nil)
		Expect(errors.Is(err, context.Canceled)).To(BeTrue())
		Expect(marker).NotTo(BeAnExistingFile())
	})
})
