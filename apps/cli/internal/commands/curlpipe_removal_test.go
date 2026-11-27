package commands

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

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
		if runtime.GOOS == "windows" {
			Skip("curlpipe inline uninstall uses POSIX sh")
		}
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
		if runtime.GOOS == "windows" {
			Skip("curlpipe inline uninstall uses POSIX sh")
		}
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

	It("tells the uninstall shell about --keep-data only when it is set", func() {
		if runtime.GOOS == "windows" {
			Skip("curlpipe inline uninstall uses POSIX sh")
		}
		marker := filepath.Join(GinkgoT().TempDir(), "keep")
		app := types.AppConfig{BaseConfig: types.BaseConfig{Name: "example"}, InstallMethod: "curlpipe",
			UninstallCommand: fmt.Sprintf("printf '%%s' \"${DEVEX_UNINSTALL_KEEP_DATA:-unset}\" > '%s'", marker)}
		for _, tc := range []struct {
			keep      bool
			inherited string
			want      string
		}{{true, "", "1"}, {false, "1", "unset"}} {
			// An inherited value must neither stand in for --keep-data nor leak into a full uninstall.
			GinkgoT().Setenv("DEVEX_UNINSTALL_KEEP_DATA", tc.inherited)
			Expect(uninstallConfiguredApplication(context.Background(), app, nil, nil, tc.keep)).To(Succeed())
			contents, err := os.ReadFile(marker)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(contents)).To(Equal(tc.want), "keepData=%v", tc.keep)
		}
	})
})
