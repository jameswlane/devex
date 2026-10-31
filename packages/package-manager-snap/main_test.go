package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestSnapPlugin(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Snap Plugin Suite")
}

var _ = Describe("Snap Plugin", func() {
	var (
		tempDir string
		logFile string
		plugin  *SnapPlugin
	)

	BeforeEach(func() {
		tempDir = GinkgoT().TempDir()
		logFile = filepath.Join(tempDir, "recorded_commands.log")

		// Create mock snap script that records arguments to logFile
		mockSnap := filepath.Join(tempDir, "snap")
		mockScript := `#!/bin/sh
if [ "$MOCK_SNAP_FAIL" = "1" ]; then
	echo "mock native failure" >&2
	exit 42
fi
echo "$@" >> "` + logFile + `"
exit 0
`
		Expect(os.WriteFile(mockSnap, []byte(mockScript), 0755)).To(Succeed())

		// Create mock sudo script that passes arguments through
		mockSudo := filepath.Join(tempDir, "sudo")
		mockSudoScript := `#!/bin/sh
case "$1" in
	--) shift; exec "$@";;
	*) exec "$@";;
esac
`
		Expect(os.WriteFile(mockSudo, []byte(mockSudoScript), 0755)).To(Succeed())

		GinkgoT().Setenv("PATH", tempDir+string(os.PathListSeparator)+os.Getenv("PATH"))
		plugin = NewSnapPlugin()
	})

	Describe("Update Command", func() {
		It("records snap refresh when updating all snaps with no arguments", func() {
			err := plugin.Execute("update", []string{})
			Expect(err).NotTo(HaveOccurred())

			content, err := os.ReadFile(logFile)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(content)).To(Equal("refresh\n"))
		})

		It("records snap refresh with selected snap names", func() {
			err := plugin.Execute("update", []string{"core", "firefox"})
			Expect(err).NotTo(HaveOccurred())

			content, err := os.ReadFile(logFile)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(content)).To(Equal("refresh core firefox\n"))
		})

		It("records snap refresh with options and flags", func() {
			err := plugin.Execute("update", []string{"--channel=candidate", "vlc"})
			Expect(err).NotTo(HaveOccurred())

			content, err := os.ReadFile(logFile)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(content)).To(Equal("refresh --channel=candidate vlc\n"))
		})

		It("preserves native failure exit status when snap refresh fails", func() {
			GinkgoT().Setenv("MOCK_SNAP_FAIL", "1")
			err := plugin.Execute("update", []string{})
			Expect(err).To(HaveOccurred())
			Expect(sdk.ExitStatus(err)).To(Equal(42))
		})

		It("directly calls handleUpdate with context", func() {
			ctx := context.Background()
			err := plugin.handleUpdate(ctx, []string{"notepad-plus-plus"})
			Expect(err).NotTo(HaveOccurred())

			content, err := os.ReadFile(logFile)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(content)).To(Equal("refresh notepad-plus-plus\n"))
		})
	})

	Describe("Plugin Metadata & Commands", func() {
		It("has update command with accurate description and usage", func() {
			info := plugin.Info()
			Expect(info.Name).To(Equal("package-manager-snap"))

			var updateCmd *sdk.PluginCommand
			for _, cmd := range info.Commands {
				if cmd.Name == "update" {
					c := cmd
					updateCmd = &c
					break
				}
			}
			Expect(updateCmd).NotTo(BeNil())
			Expect(updateCmd.Description).To(Equal("Update installed snaps"))
			Expect(updateCmd.Usage).To(ContainSubstring("Refresh"))
		})

		It("returns error for unknown command", func() {
			err := plugin.Execute("invalid-cmd", []string{})
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("unknown command"))
		})
	})
})
