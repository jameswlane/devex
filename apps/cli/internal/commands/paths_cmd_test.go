package commands_test

import (
	"bytes"
	"path/filepath"

	"github.com/jameswlane/devex/apps/cli/internal/commands"
	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/mocks"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/viper"
)

var _ = Describe("Resolved paths command", func() {
	It("prints all resolved directories without plugin initialization", func() {
		home := GinkgoT().TempDir()
		GinkgoT().Setenv("HOME", home)
		for _, key := range []string{"CONFIG", "DATA", "STATE", "CACHE"} {
			GinkgoT().Setenv("DEVEX_"+key+"_DIR", filepath.Join(home, key))
		}
		viper.Reset()
		DeferCleanup(viper.Reset)
		cmd := commands.NewRootCmd("test", mocks.NewMockRepository(), config.CrossPlatformSettings{HomeDir: home})
		var output bytes.Buffer
		cmd.SetOut(&output)
		cmd.SetArgs([]string{"paths"})
		Expect(cmd.Execute()).To(Succeed())
		for _, key := range []string{"CONFIG", "DATA", "STATE", "CACHE"} {
			Expect(output.String()).To(ContainSubstring(filepath.Join(home, key)))
		}
		Expect(output.String()).To(ContainSubstring("Plugins:"))
		Expect(output.String()).To(ContainSubstring("Binary directory:"))
	})
})
