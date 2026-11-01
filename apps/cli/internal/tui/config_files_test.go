package tui

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/configfiles"
	"github.com/jameswlane/devex/apps/cli/internal/installers"
	"github.com/jameswlane/devex/apps/cli/internal/mocks"
	"github.com/jameswlane/devex/apps/cli/internal/types"
	"github.com/jameswlane/devex/packages/plugin-sdk/sdktest"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type configStageExecutor struct {
	CommandExecutor
	observe func(string)
}

func (e configStageExecutor) ExecuteCommand(ctx context.Context, command string) (*exec.Cmd, error) {
	e.observe(command)
	return e.CommandExecutor.ExecuteCommand(ctx, command)
}

var _ = Describe("Application config_files lifecycle", func() {
	for _, path := range []string{"plain", "TUI"} {
		Context(path, func() {
			var home, source, destination string
			var recorder *sdktest.Recorder
			var app types.CrossPlatformApp
			var hookObserved bool
			BeforeEach(func() {
				recorder = sdktest.NewRecorder(GinkgoT(), "config-test", "config-post-test")
				home = GinkgoT().TempDir()
				hookObserved = false
				GinkgoT().Setenv("HOME", home)
				GinkgoT().Setenv("CONFIG_TEST_HOME", home)
				source = filepath.Join(home, "source")
				destination = filepath.Join(home, ".config", "example", "config")
				Expect(os.WriteFile(source, []byte("configured"), 0600)).To(Succeed())
				app = types.CrossPlatformApp{Name: "config-test", AllPlatforms: types.OSConfig{
					InstallMethod: "generic", InstallCommand: "config-test", ConfigFiles: []types.ConfigFile{{Source: "$CONFIG_TEST_HOME/source", Destination: "~/.config/example/config"}},
				}}
			})
			run := func(ctx SpecContext) {
				if path == "plain" {
					installers.EnableTestMode()
					defer installers.DisableTestMode()
					app.AllPlatforms.InstallMethod = "apt"
					Expect(installers.InstallCrossPlatformApp(ctx, app, config.CrossPlatformSettings{}, mocks.NewMockRepository())).To(Succeed())
				} else {
					installer := NewStreamingInstaller(nil, &MockRepository{}, ctx, config.CrossPlatformSettings{})
					defer installer.cancel()
					installer.performanceAnalyzer = nil
					installer.executor = configStageExecutor{CommandExecutor: installer.executor, observe: func(command string) {
						if command == "config-test" && len(app.AllPlatforms.PostInstall) > 0 {
							Expect(destination).NotTo(BeAnExistingFile(), "config_files run after package installation")
						}
						if command == "config-post-test" {
							data, err := os.ReadFile(destination)
							Expect(err).NotTo(HaveOccurred())
							Expect(string(data)).To(Equal("configured"))
							hookObserved = true
						}
					}}
					Expect(installer.InstallApp(ctx, app, config.CrossPlatformSettings{})).To(Succeed())
					Expect(recorder.Calls("config-test")).To(HaveLen(1))
				}
			}
			It("copies the selected platform files before post-install hooks and records ownership", func(ctx SpecContext) {
				after := filepath.Join(home, "after")
				app.AllPlatforms.PostInstall = []types.InstallCommand{{Command: "config-post-test", Copy: &types.CopyCommand{Source: destination, Destination: after}}}
				run(ctx)
				if path == "plain" {
					data, err := os.ReadFile(after)
					Expect(err).NotTo(HaveOccurred())
					Expect(string(data)).To(Equal("configured"))
				} else {
					Expect(hookObserved).To(BeTrue())
				}
				result, err := configfiles.ReadResult(app.Name)
				Expect(err).NotTo(HaveOccurred())
				Expect(result.Warnings).To(BeEmpty())
				Expect(result.Files).To(HaveLen(1))
				Expect(result.Files[0].Destination).To(Equal(destination))
				Expect(configfiles.Cleanup(app.Name)).To(Succeed())
				Expect(destination).NotTo(BeAnExistingFile())
			})
			It("backs up the conflicting file and preserves the backup during cleanup", func(ctx SpecContext) {
				Expect(os.MkdirAll(filepath.Dir(destination), 0700)).To(Succeed())
				Expect(os.WriteFile(destination, []byte("user original"), 0600)).To(Succeed())
				run(ctx)
				result, err := configfiles.ReadResult(app.Name)
				Expect(err).NotTo(HaveOccurred())
				Expect(result.Files).To(HaveLen(1))
				backup := result.Files[0].Backup
				Expect(backup).To(HaveSuffix(".devex-bak"))
				data, err := os.ReadFile(backup)
				Expect(err).NotTo(HaveOccurred())
				Expect(string(data)).To(Equal("user original"))
				Expect(configfiles.Cleanup(app.Name)).To(Succeed())
				Expect(backup).To(BeAnExistingFile())
			})
			DescribeTable("records a contextual warning and continues other mappings",
				func(ctx SpecContext, failure string) {
					invalid := types.ConfigFile{Source: source, Destination: filepath.Join(home, "invalid")}
					switch failure {
					case "missing":
						invalid.Source = filepath.Join(home, "missing")
					case "traversal":
						invalid.Destination = home + "/nested/../invalid"
					case "outside":
						invalid.Destination = filepath.Join(GinkgoT().TempDir(), "invalid")
					case "symlink":
						outside := GinkgoT().TempDir()
						Expect(os.Symlink(outside, filepath.Join(home, "link"))).To(Succeed())
						invalid.Destination = filepath.Join(home, "link", "invalid")
					case "permission":
						denied := filepath.Join(home, "denied")
						Expect(os.Mkdir(denied, 0500)).To(Succeed())
						DeferCleanup(func() { Expect(os.Chmod(denied, 0700)).To(Succeed()) })
						invalid.Destination = filepath.Join(denied, "invalid")
					}
					app.AllPlatforms.ConfigFiles = append([]types.ConfigFile{invalid}, app.AllPlatforms.ConfigFiles...)
					run(ctx)
					result, err := configfiles.ReadResult(app.Name)
					Expect(err).NotTo(HaveOccurred())
					Expect(result.Warnings).To(HaveLen(1))
					Expect(result.Warnings[0]).To(ContainSubstring(app.Name))
					Expect(result.Warnings[0]).To(ContainSubstring(invalid.Source))
					Expect(result.Warnings[0]).To(ContainSubstring(invalid.Destination))
					Expect(result.Files).To(HaveLen(1))
					Expect(destination).To(BeAnExistingFile())
				},
				Entry("missing source", "missing"),
				Entry("traversal", "traversal"),
				Entry("destination outside HOME", "outside"),
				Entry("symlink escape", "symlink"),
				Entry("permission failure", "permission"),
			)
			It("preserves a user-edited managed file and an untracked declared destination", func(ctx SpecContext) {
				run(ctx)
				Expect(os.WriteFile(destination, []byte("user edits"), 0600)).To(Succeed())
				untracked := filepath.Join(home, "untracked")
				Expect(os.WriteFile(untracked, []byte("user file"), 0600)).To(Succeed())
				Expect(configfiles.Cleanup(app.Name)).To(Succeed())
				Expect(destination).To(BeAnExistingFile())
				Expect(untracked).To(BeAnExistingFile())
			})
		})
	}
})
