package commands_test

import (
	"bytes"
	"context"
	"encoding/json"

	"github.com/jameswlane/devex/apps/cli/internal/commands"
	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/mocks"
	"github.com/jameswlane/devex/apps/cli/internal/types"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var _ = Describe("Agent JSON command adapters", func() {
	var settings config.CrossPlatformSettings
	var repo *mocks.MockRepository
	var out bytes.Buffer
	BeforeEach(func() {
		viper.Reset()
		out.Reset()
		repo = mocks.NewMockRepository()
		settings = config.CrossPlatformSettings{HomeDir: GinkgoT().TempDir(), ApplicationOverrides: []types.CrossPlatformApp{{Name: "tool", Linux: types.OSConfig{InstallMethod: "apt", InstallCommand: "tool", UninstallCommand: "tool"}}}}
	})
	AfterEach(func() { viper.Reset() })
	run := func(cmd *cobra.Command, args ...string) error {
		cmd.SetOut(&out)
		cmd.SetErr(GinkgoWriter)
		cmd.SetArgs(args)
		return cmd.Execute()
	}
	It("uses Cobra writers for effective config and real config-show JSON", func() {
		Expect(run(commands.NewConfigCmd(repo, settings), "effective", "--format", "json")).To(Succeed())
		Expect(json.Valid(out.Bytes())).To(BeTrue())
		Expect(out.String()).To(ContainSubstring(`"precedence"`))
		out.Reset()
		Expect(run(commands.NewConfigCmd(repo, settings), "show", "--format", "json")).To(Succeed())
		Expect(json.Valid(out.Bytes())).To(BeTrue())
	})
	It("registers the platform JSON command on the production root", func() {
		root := commands.NewRootCmd("test", repo, settings)
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		root.SetContext(ctx)
		Expect(run(root, "platform", "--format", "json")).To(HaveOccurred())
		Expect(json.Valid(out.Bytes())).To(BeTrue())
		Expect(out.String()).To(ContainSubstring(`"category":"cancelled"`))
	})
	It("emits structured JSON for platform cancellation", func() {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		cmd := commands.NewPlatformCmd(repo, settings)
		cmd.SetContext(ctx)
		Expect(run(cmd, "--format", "json")).To(HaveOccurred())
		Expect(json.Valid(out.Bytes())).To(BeTrue())
		Expect(out.String()).To(ContainSubstring(`"category":"cancelled"`))
	})
	It("emits plans and typed selection errors without installation output", func() {
		viper.Set("dry-run", true)
		Expect(run(commands.NewInstallCmd(repo, settings), "tool", "--format", "json")).To(Succeed())
		Expect(json.Valid(out.Bytes())).To(BeTrue())
		Expect(out.String()).To(ContainSubstring(`"unknown_effects":true`))
		out.Reset()
		Expect(run(commands.NewInstallCmd(repo, settings), "missing", "--format", "json")).To(HaveOccurred())
		Expect(json.Valid(out.Bytes())).To(BeTrue())
		Expect(out.String()).To(ContainSubstring(`"error"`))
	})
	It("keeps empty and unknown discovery/status results parseable", func() {
		Expect(run(commands.NewListCmd(repo, settings), "available", "--search", "missing", "--format", "json")).To(Succeed())
		Expect(out.Bytes()).To(MatchJSON(`[]`))
		out.Reset()
		Expect(run(commands.NewStatusCmd(repo, settings), "--app", "missing", "--format", "json")).To(HaveOccurred())
		Expect(out.Bytes()).To(MatchJSON(`[]`))
		out.Reset()
		Expect(run(commands.NewListCmd(repo, settings), "unknown", "--format", "json")).To(HaveOccurred())
		Expect(json.Valid(out.Bytes())).To(BeTrue())
	})
})
