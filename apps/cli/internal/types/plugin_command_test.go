package types_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/jameswlane/devex/apps/cli/internal/types"
)

var _ = Describe("ParsePluginCommand", func() {
	It("splits the plugin name from its command and arguments", func() {
		plugin, args, err := types.ParsePluginCommand("  package-manager-mise   ensure-installed --flag ")
		Expect(err).NotTo(HaveOccurred())
		Expect(plugin).To(Equal("package-manager-mise"))
		Expect(args).To(Equal([]string{"ensure-installed", "--flag"}))
	})

	DescribeTable("rejects malformed commands",
		func(command, message string) {
			_, _, err := types.ParsePluginCommand(command)
			Expect(err).To(MatchError(ContainSubstring(message)))
		},
		Entry("empty", "", "must name a plugin and a command"),
		Entry("no command", "package-manager-mise", "must name a plugin and a command"),
		Entry("path in the name", "../bin/sh ensure-installed", "invalid plugin name"),
		Entry("upper case name", "Package-Manager-Mise ensure-installed", "invalid plugin name"),
		Entry("flag before the command", "package-manager-mise --describe ensure-installed", "must name a command before any flags"),
	)
})
