package commands

import (
	"bytes"
	"context"
	"os"
	"os/user"
	"path/filepath"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"

	"github.com/jameswlane/devex/apps/cli/internal/types"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Install privilege previews", func() {
	It("shows installed metadata and original account without executing an install", func() {
		if os.Geteuid() == 0 {
			Skip("requires a non-root runner; elevated planning is covered by SDK contracts")
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
})
