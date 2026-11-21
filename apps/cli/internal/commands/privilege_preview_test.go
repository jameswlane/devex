package commands

import (
	"bytes"
	"context"
	"os"
	"os/user"
	"path/filepath"
	"runtime"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"

	"github.com/jameswlane/devex/apps/cli/internal/security"
	"github.com/jameswlane/devex/apps/cli/internal/testhelper"
	"github.com/jameswlane/devex/apps/cli/internal/types"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Install privilege previews", func() {
	It("shows installed metadata and original account without executing an install", func() {
		if runtime.GOOS == "windows" || os.Geteuid() == 0 {
			Skip("requires a non-root Unix runner; elevated planning is covered by SDK contracts")
		}
		home := GinkgoT().TempDir()
		GinkgoT().Setenv("HOME", "/root")
		original := sdk.NewPrivilegeResolver
		resolver := original()
		account, err := resolver.CurrentUser()
		Expect(err).NotTo(HaveOccurred())
		copyAccount := *account
		copyAccount.HomeDir = home
		resolver.Elevated = true
		resolver.CurrentUser = func() (*user.User, error) {
			return &user.User{Username: "root", Uid: "0", HomeDir: "/root"}, nil
		}
		resolver.LookupUser = func(string) (*user.User, error) { return &copyAccount, nil }
		resolver.Getenv = func(key string) string {
			if key == "SUDO_USER" {
				return account.Username
			}
			return os.Getenv(key)
		}
		sdk.NewPrivilegeResolver = func() sdk.PrivilegeResolver { return resolver }
		DeferCleanup(func() { sdk.NewPrivilegeResolver = original })
		pluginDir := filepath.Join(home, ".devex", "plugins")
		Expect(os.MkdirAll(pluginDir, 0755)).To(Succeed())
		script := "#!/bin/sh\nif [ \"$1\" = --plugin-info ]; then\n echo '{\"name\":\"package-manager-brew\",\"commands\":[{\"name\":\"install\",\"privilege\":{\"scope\":\"user\"}}]}'\nelse\n exit 42\nfi\n"
		Expect(os.WriteFile(filepath.Join(pluginDir, "devex-plugin-package-manager-brew"), []byte(script), 0755)).To(Succeed())
		var out bytes.Buffer
		apps := []types.CrossPlatformApp{{Name: "example", AllPlatforms: types.OSConfig{InstallMethod: "brew"}}}
		Expect(previewInstallation(context.Background(), apps, nil, &out)).To(Succeed())
		current, err := user.Current()
		Expect(err).NotTo(HaveOccurred())
		Expect(out.String()).To(ContainSubstring("scope: user, intended user: " + current.Username))
		Expect(out.String()).To(ContainSubstring("elevation required: false"))
	})
	It("shows unspecified privilege for legacy or unavailable plugins", func() {
		var out bytes.Buffer
		apps := []types.CrossPlatformApp{{Name: "example", AllPlatforms: types.OSConfig{InstallMethod: "legacy"}}}
		Expect(previewInstallation(context.Background(), apps, nil, &out)).To(Succeed())
		Expect(out.String()).To(ContainSubstring("scope: unspecified"))
	})
	It("previews configuration copies and backups without writes even with unavailable plugin metadata", func(ctx SpecContext) {
		home := GinkgoT().TempDir()
		testhelper.SetProfileEnv(GinkgoT(), home)
		source, destination := filepath.Join(home, "source"), filepath.Join(home, "config")
		Expect(os.WriteFile(source, []byte("new"), 0600)).To(Succeed())
		Expect(os.WriteFile(destination, []byte("original"), 0600)).To(Succeed())
		apps := []types.CrossPlatformApp{{Name: "preview-config", AllPlatforms: types.OSConfig{
			InstallMethod: "unavailable", ConfigFiles: []types.ConfigFile{{Source: "$HOME/source", Destination: "~/config"}},
		}}}
		var out bytes.Buffer
		Expect(previewInstallation(ctx, apps, nil, &out)).To(Succeed())
		Expect(out.String()).To(And(ContainSubstring("Copy "+source), ContainSubstring("Back up "+destination), ContainSubstring(".devex-bak")))
		data, err := os.ReadFile(destination)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(data)).To(Equal("original"))
		entries, err := os.ReadDir(home)
		Expect(err).NotTo(HaveOccurred())
		Expect(entries).To(HaveLen(2))
	})

	It("never runs a plugin the organization policy denies while previewing", func() {
		home := GinkgoT().TempDir()
		testhelper.SetProfileEnv(GinkgoT(), home)
		teamDir := filepath.Join(home, "team")
		GinkgoT().Setenv("DEVEX_TEAM_CONFIG_DIR", teamDir)
		Expect(os.MkdirAll(teamDir, 0o750)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(teamDir, "policy.yaml"), []byte("version: '1'\nplugins:\n  deny: [package-manager-brew]\n"), 0o600)).To(Succeed())
		security.ResetOrgPolicyCache()
		DeferCleanup(security.ResetOrgPolicyCache)
		pluginDir := filepath.Join(home, ".devex", "plugins")
		Expect(os.MkdirAll(pluginDir, 0o755)).To(Succeed())
		marker := filepath.Join(home, "ran")
		script := "#!/bin/sh\ntouch " + marker + "\necho '{\"name\":\"package-manager-brew\"}'\n"
		Expect(os.WriteFile(filepath.Join(pluginDir, "devex-plugin-package-manager-brew"), []byte(script), 0o755)).To(Succeed())

		var out bytes.Buffer
		apps := []types.CrossPlatformApp{{Name: "example", AllPlatforms: types.OSConfig{InstallMethod: "brew"}}}
		Expect(previewInstallation(context.Background(), apps, nil, &out)).To(Succeed())
		Expect(marker).NotTo(BeAnExistingFile())
	})
})
