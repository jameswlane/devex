package bootstrap

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/jameswlane/devex/apps/cli/internal/security"
	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/cobra"
)

var _ = Describe("Organization policy plugin enforcement", func() {
	var home, policyPath string

	BeforeEach(func() {
		home = GinkgoT().TempDir()
		GinkgoT().Setenv("HOME", home)
		teamDir := filepath.Join(home, "team")
		GinkgoT().Setenv("DEVEX_TEAM_CONFIG_DIR", teamDir)
		policyPath = filepath.Join(teamDir, "policy.yaml")
		Expect(os.MkdirAll(teamDir, 0o750)).To(Succeed())
	})

	writePolicy := func(contents string) {
		Expect(os.WriteFile(policyPath, []byte(contents), 0o600)).To(Succeed())
		security.ResetOrgPolicyCache()
	}

	It("refuses a denied plugin before execution and runs an allowed plugin", func() {
		pluginDir := GinkgoT().TempDir()
		marker := filepath.Join(pluginDir, "executed")
		script := fmt.Sprintf(`#!/bin/sh
if [ "${1-}" = --plugin-info ]; then
 echo '{"name":"tool-example","version":"1.0.0"}'
 exit 0
fi
touch %q
`, marker)
		Expect(os.WriteFile(filepath.Join(pluginDir, "devex-plugin-tool-example"), []byte(script), 0o700)).To(Succeed())
		bootstrap := &PluginBootstrap{homeDir: home, manager: sdk.NewExecutableManager(pluginDir)}

		writePolicy("version: '1'\nplugins:\n  deny: [tool-example]\n")
		Expect(bootstrap.ExecutePluginWithIO(context.Background(), "tool-example", nil, sdk.OperationIO{})).To(MatchError(ContainSubstring("plugins.deny")))
		Expect(marker).NotTo(BeAnExistingFile())

		writePolicy("version: '1'\nplugins:\n  allow: [tool-example]\n")
		Expect(bootstrap.ExecutePluginWithIO(context.Background(), "tool-example", nil, sdk.OperationIO{})).To(Succeed())
		Expect(marker).To(BeAnExistingFile())
	})

	It("refuses a denied explicit plugin installation before registry access", func() {
		writePolicy("version: '1'\nplugins:\n  deny: [tool-example]\n")
		bootstrap := &PluginBootstrap{homeDir: home}
		Expect(bootstrap.handleInstallPlugin(&cobra.Command{}, []string{"tool-example"})).To(MatchError(ContainSubstring("plugins.deny")))
	})
})
