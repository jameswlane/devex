package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/viper"

	"github.com/jameswlane/devex/apps/cli/internal/agentapi"
	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/inventory"
	"github.com/jameswlane/devex/apps/cli/internal/mocks"
	"github.com/jameswlane/devex/apps/cli/internal/platform"
	"github.com/jameswlane/devex/apps/cli/internal/testhelper"
	"github.com/jameswlane/devex/apps/cli/internal/types"
)

type channelTestInstaller struct {
	uninstalled []string
	installed   map[string]bool
}

func (i *channelTestInstaller) Install(string, types.Repository) error { return nil }

func (i *channelTestInstaller) Uninstall(command string, _ types.Repository) error {
	i.uninstalled = append(i.uninstalled, command)
	return nil
}

func (i *channelTestInstaller) IsInstalled(command string) (bool, error) {
	if i.installed != nil {
		return i.installed[command], nil
	}
	return true, nil
}

func channelTestSettings(homeDir string) config.CrossPlatformSettings {
	return config.CrossPlatformSettings{
		HomeDir: homeDir,
		ApplicationOverrides: []types.CrossPlatformApp{
			{
				Name:        "Kimi Code",
				Description: "Moonshot AI coding agent CLI",
				Category:    "AI Coding Agents",
				Default:     false,
				Linux: types.OSConfig{
					InstallMethod:    "curlpipe",
					DownloadURL:      "https://code.kimi.com/kimi-code/install.sh",
					InstallScope:     "user",
					UninstallCommand: "rm -f ~/.kimi-code/bin/kimi",
					OfficialSupport:  true,
					Alternatives: []types.OSConfig{
						{
							InstallMethod:    "mise",
							InstallCommand:   "node@lts npm:@moonshot-ai/kimi-code@latest",
							UninstallCommand: "npm:@moonshot-ai/kimi-code",
							OfficialSupport:  true,
						},
					},
				},
			},
			{
				Name:        "Ghostty",
				Description: "GPU-accelerated terminal",
				Category:    "Terminal Emulators",
				Default:     false,
				Linux: types.OSConfig{
					InstallMethod:    "apt",
					InstallCommand:   "ghostty",
					UninstallCommand: "ghostty",
					OfficialSupport:  true,
					Alternatives: []types.OSConfig{
						{
							InstallMethod:    "snap",
							InstallCommand:   "ghostty --classic",
							UninstallCommand: "ghostty",
							OfficialSupport:  false,
						},
					},
				},
			},
			{
				Name:        "ScriptOnly",
				Description: "Tool whose curlpipe channel cannot uninstall automatically",
				Category:    "Development Tools",
				Default:     false,
				Linux: types.OSConfig{
					InstallMethod:   "curlpipe",
					DownloadURL:     "https://example.invalid/install.sh",
					InstallScope:    "user",
					OfficialSupport: true,
					Alternatives: []types.OSConfig{
						{
							InstallMethod:    "mise",
							InstallCommand:   "scriptonly@latest",
							UninstallCommand: "scriptonly",
							OfficialSupport:  false,
						},
					},
				},
			},
		},
	}
}

type channelInstallPlanEnvelope struct {
	Apps  []agentapi.PlannedApp `json:"apps"`
	Roots []string              `json:"roots"`
	Error *agentapi.Error       `json:"error"`
}

var _ = Describe("Install Channels CLI", func() {
	var (
		tempHome string
		repo     *mocks.MockRepository
		settings config.CrossPlatformSettings
		out      bytes.Buffer
		errOut   bytes.Buffer
	)

	runRoot := func(args ...string) error {
		out.Reset()
		errOut.Reset()
		cmd := NewRootCmd("test", repo, settings)
		cmd.SetOut(&out)
		cmd.SetErr(&errOut)
		cmd.SetArgs(args)
		return cmd.Execute()
	}

	BeforeEach(func() {
		viper.Reset()
		DeferCleanup(viper.Reset)
		DeferCleanup(types.PinCatalogOSForTesting("linux"))
		DeferCleanup(types.PinCatalogPlatformForTesting(platform.DetectionResult{OS: "linux", Distribution: "ubuntu", Version: "26.04"}))

		tempHome = GinkgoT().TempDir()
		testhelper.SetProfileEnv(GinkgoT(), tempHome)
		GinkgoT().Setenv("DEVEX_CHANNEL", "")
		GinkgoT().Setenv("DEVEX_CHANNEL_ORDER", "")

		repo = mocks.NewMockRepository()
		settings = channelTestSettings(tempHome)
		out.Reset()
		errOut.Reset()
	})

	Describe("channel preference resolution through root command", func() {
		It("selects an alternative channel via --channel on install dry-run JSON", func() {
			Expect(runRoot("install", "Kimi Code", "--channel", "mise", "--dry-run", "--format", "json", "--offline")).To(Succeed())

			var env channelInstallPlanEnvelope
			Expect(json.Unmarshal(out.Bytes(), &env)).To(Succeed())
			Expect(env.Error).To(BeNil())
			Expect(env.Apps).To(HaveLen(1))
			Expect(env.Apps[0].Channel).To(Equal("mise"))
			Expect(env.Apps[0].Method).To(Equal("mise"))
			Expect(env.Apps[0].Channels).To(ConsistOf(
				agentapi.PlanChannel{Name: "curlpipe", Official: true, Default: true},
				agentapi.PlanChannel{Name: "mise", Official: true, Default: false},
			))
		})

		It("honors DEVEX_CHANNEL through root without being shadowed by BindPFlags", func() {
			GinkgoT().Setenv("DEVEX_CHANNEL", "mise")

			Expect(runRoot("install", "Kimi Code", "--dry-run", "--format", "json", "--offline")).To(Succeed())

			var env channelInstallPlanEnvelope
			Expect(json.Unmarshal(out.Bytes(), &env)).To(Succeed())
			Expect(env.Error).To(BeNil())
			Expect(env.Apps).To(HaveLen(1))
			Expect(env.Apps[0].Channel).To(Equal("mise"))
			Expect(env.Apps[0].Method).To(Equal("mise"))
		})

		It("reads channels: and channel_order: from config.yaml and obeys flag > env > channels > channel_order", func() {
			cfgDir := filepath.Join(tempHome, ".config", "devex")
			Expect(os.MkdirAll(cfgDir, 0o700)).To(Succeed())
			cfgYAML := "channels:\n  kimi-code: mise\nchannel_order:\n  - snap\n  - apt\n"
			Expect(os.WriteFile(filepath.Join(cfgDir, "config.yaml"), []byte(cfgYAML), 0o600)).To(Succeed())

			// Config applies channels: to Kimi Code and channel_order: to Ghostty.
			Expect(runRoot("install", "Kimi Code", "Ghostty", "--dry-run", "--format", "json", "--offline")).To(Succeed())
			var env channelInstallPlanEnvelope
			Expect(json.Unmarshal(out.Bytes(), &env)).To(Succeed())
			Expect(env.Error).To(BeNil())
			byName := map[string]agentapi.PlannedApp{}
			for _, app := range env.Apps {
				byName[app.Name] = app
			}
			Expect(byName["Kimi Code"].Channel).To(Equal("mise"))
			Expect(byName["Ghostty"].Channel).To(Equal("snap"))

			// --channel official overrides config preferences.
			Expect(runRoot("install", "Ghostty", "--channel", "official", "--dry-run", "--format", "json", "--offline")).To(Succeed())
			env = channelInstallPlanEnvelope{}
			Expect(json.Unmarshal(out.Bytes(), &env)).To(Succeed())
			Expect(env.Error).To(BeNil())
			Expect(env.Apps[0].Channel).To(Equal("apt"))
		})

		It("reports offered channels when --channel is unknown or missing on some selected apps", func() {
			err := runRoot("install", "Kimi Code", "--channel", "brew", "--dry-run", "--offline")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring(`Kimi Code has no channel "brew" on linux`))
			Expect(err.Error()).To(ContainSubstring(`curlpipe (official, default)`))
			Expect(err.Error()).To(ContainSubstring(`mise`))

			// Multi-app request: Kimi Code offers mise, Ghostty does not.
			err = runRoot("install", "Kimi Code", "Ghostty", "--channel", "mise", "--dry-run", "--offline")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring(`Ghostty`))
			Expect(err.Error()).To(ContainSubstring(`channel "mise"`))
		})
	})

	Describe("showing channels in list and status", func() {
		It("includes selected channel and offered channels in devex list available JSON and table output", func() {
			Expect(runRoot("list", "available", "--search", "Ghostty", "--format", "json", "--offline")).To(Succeed())
			var apps []agentapi.AvailableApp
			Expect(json.Unmarshal(out.Bytes(), &apps)).To(Succeed())
			Expect(apps).To(HaveLen(1))
			Expect(apps[0].Channel).To(Equal("apt"))
			Expect(apps[0].Channels).To(HaveLen(2))
			Expect(apps[0].Channels[0].Name).To(Equal("apt"))
			Expect(apps[0].Channels[0].Official).To(BeTrue())
			Expect(apps[0].Channels[1].Name).To(Equal("snap"))
			Expect(apps[0].Channels[1].Official).To(BeFalse())

			Expect(runRoot("list", "available", "--search", "Ghostty", "--offline")).To(Succeed())
			Expect(out.String()).To(ContainSubstring("apt (official)"))
			Expect(out.String()).To(ContainSubstring("snap"))
		})

		It("uses the recorded channel in devex status even when config prefers another channel", func() {
			viper.Set("channel_order", []string{"snap"})

			aptGhostty, err := settings.ApplicationOverrides[1].WithChannel("apt")
			Expect(err).NotTo(HaveOccurred())
			Expect(repo.SaveApp(aptGhostty.ToLegacyAppConfig())).To(Succeed())
			Expect(inventory.Record(context.Background(), repo, aptGhostty, inventory.RecordOptions{
				Query: func(context.Context, string, string) (bool, string, error) {
					return true, "1.0.0", nil
				},
			})).To(Succeed())

			statusResult, err := agentapi.Status(context.Background(), settings, repo, agentapi.StatusRequest{Names: []string{"Ghostty"}}, func(_ context.Context, _ types.AppConfig) (bool, error) {
				return true, nil
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(statusResult).To(HaveLen(1))
			Expect(statusResult[0].Channel).To(Equal("apt"))
			Expect(statusResult[0].InstallMethod).To(Equal("apt"))
		})
	})

	Describe("recorded channel precedence for update, upgrade, and uninstall", func() {
		It("uses the recorded channel (explicit and legacy receipts) for update and upgrade despite config preferences", func() {
			viper.Set("channel_order", []string{"snap"})

			recorder := &maintenanceRecorder{}
			oldExec := maintenanceExecutor
			DeferCleanup(func() { maintenanceExecutor = oldExec })
			maintenanceExecutor = func(context.Context, pluginBootstrapProvider) (packageMaintenanceExecutor, error) {
				return recorder, nil
			}

			// Explicit receipt channel: apt (while config prefers snap).
			aptGhostty, err := settings.ApplicationOverrides[1].WithChannel("apt")
			Expect(err).NotTo(HaveOccurred())
			Expect(inventory.Record(context.Background(), repo, aptGhostty, inventory.RecordOptions{
				Query: func(context.Context, string, string) (bool, string, error) {
					return true, "1.0.0", nil
				},
			})).To(Succeed())

			cmd := newPackageMaintenanceCmdWithRepo("upgrade", repo, settings, nil)
			cmd.SetOut(&out)
			cmd.SetErr(&errOut)
			cmd.SetArgs([]string{"Ghostty"})
			Expect(cmd.Execute()).To(Succeed())
			Expect(recorder.calls).To(Equal([]string{"package-manager-apt upgrade-packages ghostty"}))

			// Legacy receipt with only Manager="apt" and no Channel field still selects apt over snap preference.
			recorder.calls = nil
			Expect(repo.Set(inventory.ReceiptKey("Ghostty"), `{"manager":"apt","packages":["ghostty"]}`)).To(Succeed())
			cmdUpdate := newPackageMaintenanceCmdWithRepo("update", repo, settings, nil)
			cmdUpdate.SetOut(&out)
			cmdUpdate.SetErr(&errOut)
			cmdUpdate.SetArgs([]string{"Ghostty"})
			Expect(cmdUpdate.Execute()).To(Succeed())
			Expect(recorder.calls).To(Equal([]string{"package-manager-apt update"}))
		})

		It("uses the recorded channel for uninstall even when config prefers another channel", func() {
			viper.Set("channel_order", []string{"snap"})

			// Legacy receipt with Manager="apt" (no channel field) and tracked Ghostty.
			aptGhostty, err := settings.ApplicationOverrides[1].WithChannel("apt")
			Expect(err).NotTo(HaveOccurred())
			Expect(repo.SaveApp(aptGhostty.ToLegacyAppConfig())).To(Succeed())
			Expect(repo.Set(inventory.ReceiptKey("Ghostty"), `{"manager":"apt","packages":["ghostty"]}`)).To(Succeed())

			var usedMethods []string
			inst := &channelTestInstaller{}
			DeferCleanup(SetUninstallInstallerForTest(func(_ context.Context, method string) types.BaseInstaller {
				usedMethods = append(usedMethods, method)
				return inst
			}))

			cmd := NewUninstallCmd(repo, settings)
			cmd.SetOut(&out)
			cmd.SetErr(&errOut)
			cmd.SetArgs([]string{"--app", "Ghostty", "--force"})
			Expect(cmd.Execute()).To(Succeed())
			Expect(usedMethods).To(Equal([]string{"apt"}))
			Expect(inst.uninstalled).To(Equal([]string{"ghostty"}))
		})
	})

	Describe("channel switching on install", func() {
		It("describes a channel switch in dry-run without executing or mutating repo", func() {
			aptGhostty, err := settings.ApplicationOverrides[1].WithChannel("apt")
			Expect(err).NotTo(HaveOccurred())
			Expect(repo.SaveApp(aptGhostty.ToLegacyAppConfig())).To(Succeed())
			Expect(inventory.Record(context.Background(), repo, aptGhostty, inventory.RecordOptions{
				Query: func(context.Context, string, string) (bool, string, error) {
					return true, "1.0.0", nil
				},
			})).To(Succeed())

			DeferCleanup(SetUninstallInstallerForTest(func(context.Context, string) types.BaseInstaller {
				Fail("dry-run must never execute an uninstall installer")
				return nil
			}))

			snapGhostty, err := settings.ApplicationOverrides[1].WithChannel("snap")
			Expect(err).NotTo(HaveOccurred())

			Expect(previewInstallationWithRepo(context.Background(), []types.CrossPlatformApp{snapGhostty}, repo, nil, &out)).To(Succeed())
			Expect(out.String()).To(ContainSubstring(`Would switch Ghostty from channel apt to snap`))

			// Repo still has the old apt receipt intact.
			rec, ok := inventory.ReadReceipt(repo, "Ghostty")
			Expect(ok).To(BeTrue())
			Expect(rec.Channel).To(Equal("apt"))
		})

		It("uninstalls the recorded channel first when switching channels", func() {
			aptGhostty, err := settings.ApplicationOverrides[1].WithChannel("apt")
			Expect(err).NotTo(HaveOccurred())
			Expect(repo.SaveApp(aptGhostty.ToLegacyAppConfig())).To(Succeed())
			Expect(inventory.Record(context.Background(), repo, aptGhostty, inventory.RecordOptions{
				Query: func(context.Context, string, string) (bool, string, error) {
					return true, "1.0.0", nil
				},
			})).To(Succeed())

			var usedMethods []string
			inst := &channelTestInstaller{}
			DeferCleanup(SetUninstallInstallerForTest(func(_ context.Context, method string) types.BaseInstaller {
				usedMethods = append(usedMethods, method)
				return inst
			}))

			snapGhostty, err := settings.ApplicationOverrides[1].WithChannel("snap")
			Expect(err).NotTo(HaveOccurred())

			Expect(prepareChannelSwitches(context.Background(), &out, []types.CrossPlatformApp{snapGhostty}, repo)).To(Succeed())
			Expect(out.String()).To(ContainSubstring("Switching Ghostty from channel apt to snap: uninstalling apt channel first"))
			Expect(usedMethods).To(ContainElement("apt"))
			Expect(inst.uninstalled).To(Equal([]string{"ghostty"}))

			// Old tracking records were cleared so the new channel can install cleanly.
			_, err = repo.GetApp("Ghostty")
			Expect(err).To(HaveOccurred())
			_, ok := inventory.ReadReceipt(repo, "Ghostty")
			Expect(ok).To(BeFalse())
		})

		It("fails with devex uninstall guidance when the recorded channel cannot uninstall", func() {
			curlScript, err := settings.ApplicationOverrides[2].WithChannel("curlpipe")
			Expect(err).NotTo(HaveOccurred())
			Expect(repo.SaveApp(curlScript.ToLegacyAppConfig())).To(Succeed())
			Expect(inventory.Record(context.Background(), repo, curlScript, inventory.RecordOptions{
				Query: func(context.Context, string, string) (bool, string, error) {
					return true, "1.0.0", nil
				},
			})).To(Succeed())

			miseScript, err := settings.ApplicationOverrides[2].WithChannel("mise")
			Expect(err).NotTo(HaveOccurred())

			err = prepareChannelSwitches(context.Background(), &out, []types.CrossPlatformApp{miseScript}, repo)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring(`cannot switch ScriptOnly from channel "curlpipe" to "mise"`))
			Expect(err.Error()).To(ContainSubstring(`devex uninstall ScriptOnly`))

			// Repo was not mutated.
			rec, ok := inventory.ReadReceipt(repo, "ScriptOnly")
			Expect(ok).To(BeTrue())
			Expect(rec.Channel).To(Equal("curlpipe"))
		})

		It("keeps the recorded non-default channel on install when no explicit preference is set (S1)", func() {
			snapGhostty, err := settings.ApplicationOverrides[1].WithChannel("snap")
			Expect(err).NotTo(HaveOccurred())
			Expect(repo.AddApp(snapGhostty.Name)).To(Succeed())
			Expect(inventory.Record(context.Background(), repo, snapGhostty, inventory.RecordOptions{
				Query: func(context.Context, string, string) (bool, string, error) {
					return true, "1.0.0", nil
				},
			})).To(Succeed())

			// Text dry-run with no explicit preference must keep snap and not switch to apt.
			Expect(runRoot("install", "Ghostty", "--dry-run", "--offline")).To(Succeed())
			Expect(out.String()).NotTo(ContainSubstring("Would switch"))
			Expect(out.String()).To(ContainSubstring("Would install Ghostty via channel snap"))

			// JSON dry-run with no explicit preference must keep snap as well.
			Expect(runRoot("install", "Ghostty", "--dry-run", "--format", "json", "--offline")).To(Succeed())
			var env channelInstallPlanEnvelope
			Expect(json.Unmarshal(out.Bytes(), &env)).To(Succeed())
			Expect(env.Error).To(BeNil())
			Expect(env.Apps).To(HaveLen(1))
			Expect(env.Apps[0].Channel).To(Equal("snap"))

			// Explicit --channel apt does describe the switch on plain dry-run.
			Expect(runRoot("install", "Ghostty", "--channel", "apt", "--dry-run", "--offline")).To(Succeed())
			Expect(out.String()).To(ContainSubstring("Would switch Ghostty from channel snap to apt"))
		})

		It("refuses uninstall without clearing receipt when recorded channel is no longer in catalog and saved definition does not match (S5)", func() {
			// Save a mismatched apt definition alongside a receipt naming a removed channel.
			aptGhostty, err := settings.ApplicationOverrides[1].WithChannel("apt")
			Expect(err).NotTo(HaveOccurred())
			Expect(repo.AddApp("Ghostty")).To(Succeed())
			defData, err := json.Marshal(aptGhostty)
			Expect(err).NotTo(HaveOccurred())
			Expect(repo.Set("installed-app-definition:ghostty", string(defData))).To(Succeed())
			Expect(repo.Set(inventory.ReceiptKey("Ghostty"), `{"manager":"legacy-mgr","channel":"legacy-ppa","packages":["ghostty"]}`)).To(Succeed())

			inst := &channelTestInstaller{}
			DeferCleanup(SetUninstallInstallerForTest(func(context.Context, string) types.BaseInstaller {
				return inst
			}))

			cmd := NewUninstallCmd(repo, settings)
			cmd.SetOut(&out)
			cmd.SetErr(&errOut)
			cmd.SetArgs([]string{"--app", "Ghostty", "--force"})
			err = cmd.Execute()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("legacy-ppa"))
			Expect(inst.uninstalled).To(BeEmpty())

			// Receipt must still be present.
			rec, ok := inventory.ReadReceipt(repo, "Ghostty")
			Expect(ok).To(BeTrue())
			Expect(rec.Channel).To(Equal("legacy-ppa"))

			// channel_switch must also reject the mismatched saved definition rather than uninstalling apt.
			snapGhostty, err := settings.ApplicationOverrides[1].WithChannel("snap")
			Expect(err).NotTo(HaveOccurred())
			err = prepareChannelSwitches(context.Background(), &out, []types.CrossPlatformApp{snapGhostty}, repo)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("legacy-ppa"))
			Expect(inst.uninstalled).To(BeEmpty())
		})

		It("shows selected channel and offered channels in text dry-run even when not switching (S10)", func() {
			Expect(runRoot("install", "Kimi Code", "--channel", "mise", "--dry-run", "--offline")).To(Succeed())
			Expect(out.String()).To(ContainSubstring("Would install Kimi Code via channel mise (channels: curlpipe (official, default), mise (official))"))

			Expect(runRoot("install", "Kimi Code", "--dry-run", "--offline")).To(Succeed())
			Expect(out.String()).To(ContainSubstring("Would install Kimi Code via channel curlpipe (channels: curlpipe (official, default), mise (official))"))
		})

		It("refuses channel switches on JSON dry-run, --expect-plan, and executeSnapshotObserved (S3)", func() {
			// Prepare a valid plan digest for Ghostty via snap before Ghostty is installed.
			Expect(runRoot("install", "Ghostty", "--channel", "snap", "--dry-run", "--format", "json", "--offline")).To(Succeed())
			var planned struct {
				Digest string          `json:"digest"`
				Error  *agentapi.Error `json:"error"`
			}
			Expect(json.Unmarshal(out.Bytes(), &planned)).To(Succeed())
			Expect(planned.Error).To(BeNil())
			Expect(planned.Digest).NotTo(BeEmpty())

			// Now record Ghostty as installed through apt.
			aptGhostty, err := settings.ApplicationOverrides[1].WithChannel("apt")
			Expect(err).NotTo(HaveOccurred())
			Expect(repo.AddApp(aptGhostty.Name)).To(Succeed())
			Expect(inventory.Record(context.Background(), repo, aptGhostty, inventory.RecordOptions{
				Query: func(context.Context, string, string) (bool, string, error) {
					return true, "1.0.0", nil
				},
			})).To(Succeed())

			expectedMsg := `Ghostty is installed through channel "apt"; run 'devex uninstall Ghostty' first, then install it with --channel snap`

			// JSON dry-run refuses the switch.
			err = runRoot("install", "Ghostty", "--channel", "snap", "--dry-run", "--format", "json", "--offline")
			Expect(err).To(HaveOccurred())
			var switchErrEnv channelInstallPlanEnvelope
			Expect(json.Unmarshal(out.Bytes(), &switchErrEnv)).To(Succeed())
			Expect(switchErrEnv.Error).NotTo(BeNil())
			Expect(switchErrEnv.Error.Message).To(ContainSubstring(expectedMsg))

			// Confirmed plan (--expect-plan) refuses the switch before any removal or install.
			DeferCleanup(SetUninstallInstallerForTest(func(context.Context, string) types.BaseInstaller {
				Fail("confirmed plan must never uninstall an existing channel")
				return nil
			}))
			err = runRoot("install", "Ghostty", "--channel", "snap", "--expect-plan", planned.Digest, "--offline")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring(expectedMsg))

			// Re-check inside executeSnapshotObserved also refuses directly.
			snapGhostty, err := settings.ApplicationOverrides[1].WithChannel("snap")
			Expect(err).NotTo(HaveOccurred())
			err = executeSnapshotObserved(context.Background(), &out, &errOut, agentapi.ExecutionSnapshot{
				Roots:    []types.CrossPlatformApp{snapGhostty},
				Settings: settings,
			}, repo, nil, nil)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring(expectedMsg))

			// Receipt remains intact.
			rec, ok := inventory.ReadReceipt(repo, "Ghostty")
			Expect(ok).To(BeTrue())
			Expect(rec.Channel).To(Equal("apt"))
		})

		It("refuses channel switches on --rollback-on-failure before any removal (S6)", func() {
			aptGhostty, err := settings.ApplicationOverrides[1].WithChannel("apt")
			Expect(err).NotTo(HaveOccurred())
			Expect(repo.AddApp(aptGhostty.Name)).To(Succeed())
			Expect(inventory.Record(context.Background(), repo, aptGhostty, inventory.RecordOptions{
				Query: func(context.Context, string, string) (bool, string, error) {
					return true, "1.0.0", nil
				},
			})).To(Succeed())

			DeferCleanup(SetUninstallInstallerForTest(func(context.Context, string) types.BaseInstaller {
				Fail("--rollback-on-failure must never uninstall an existing channel")
				return nil
			}))

			expectedMsg := `Ghostty is installed through channel "apt"; run 'devex uninstall Ghostty' first, then install it with --channel snap`

			err = runRoot("install", "Ghostty", "--channel", "snap", "--rollback-on-failure", "--offline")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring(expectedMsg))

			// Direct runRollbackBatch call also refuses before any removal or install.
			snapGhostty, err := settings.ApplicationOverrides[1].WithChannel("snap")
			Expect(err).NotTo(HaveOccurred())
			err = runRollbackBatch(context.Background(), &errOut, []types.CrossPlatformApp{snapGhostty}, repo, settings, nil)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring(expectedMsg))

			rec, ok := inventory.ReadReceipt(repo, "Ghostty")
			Expect(ok).To(BeTrue())
			Expect(rec.Channel).To(Equal("apt"))
		})
	})
})
