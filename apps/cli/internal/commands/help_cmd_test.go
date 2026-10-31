package commands_test

import (
	"bytes"
	"os"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/jameswlane/devex/apps/cli/internal/commands"
	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/mocks"
)

var _ = Describe("Help Command", func() {
	var rootCmd *cobra.Command
	var repo *mocks.MockRepository
	var settings config.CrossPlatformSettings
	var out bytes.Buffer

	setEnv := func(key, value string) {
		old, present := os.LookupEnv(key)
		Expect(os.Setenv(key, value)).To(Succeed())
		DeferCleanup(func() {
			if present {
				_ = os.Setenv(key, old)
			} else {
				_ = os.Unsetenv(key)
			}
		})
	}

	run := func(args ...string) error {
		rootCmd.SetOut(&out)
		rootCmd.SetErr(&out)
		rootCmd.SetArgs(args)
		return rootCmd.Execute()
	}

	BeforeEach(func() {
		viper.Reset()
		viper.SetConfigType("yaml")
		DeferCleanup(viper.Reset)

		tempHome := GinkgoT().TempDir()
		setEnv("HOME", tempHome)

		for _, key := range []string{"CI", "GITHUB_ACTIONS", "GITLAB_CI", "DEVEX_NONINTERACTIVE", "DEVEX_NON_INTERACTIVE", "TERM"} {
			setEnv(key, "")
		}

		repo = mocks.NewMockRepository()
		settings = config.CrossPlatformSettings{}
		out.Reset()

		rootCmd = commands.NewRootCmd("test", repo, settings)
	})

	It("lists help command only once in root help", func() {
		Expect(run("--help")).To(Succeed())
		output := out.String()

		lines := strings.Split(output, "\n")
		helpCount := 0
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "help ") || strings.HasPrefix(trimmed, "help\t") || trimmed == "help" {
				helpCount++
			}
		}

		Expect(helpCount).To(Equal(1))
		Expect(output).To(ContainSubstring("Show contextual help with interactive viewer"))
		Expect(output).NotTo(ContainSubstring("Help about any command"))
	})

	It("prints the plain command list when run with no TTY", func() {
		// When running in tests, stdin/stdout is not a TTY or CI is handled
		Expect(run("help")).To(Succeed())
		output := out.String()

		Expect(output).To(ContainSubstring("Available Commands:"))
		Expect(output).To(ContainSubstring("install"))
		Expect(output).To(ContainSubstring("setup"))
		Expect(output).To(ContainSubstring("Show contextual help with interactive viewer"))
	})

	It("prints the plain command list when --non-interactive flag is passed", func() {
		Expect(run("help", "--non-interactive")).To(Succeed())
		output := out.String()

		Expect(output).To(ContainSubstring("Available Commands:"))
		Expect(output).To(ContainSubstring("install"))
	})

	It("shows command help directly for 'help install' without topic search fallback", func() {
		Expect(run("help", "install")).To(Succeed())
		output := out.String()

		Expect(output).To(ContainSubstring("devex install [apps...]"))
		Expect(output).NotTo(ContainSubstring("Topic 'install' not found"))
		Expect(output).NotTo(ContainSubstring("Searching for related topics"))
	})

	It("shows subcommand help directly for 'help template info'", func() {
		Expect(run("help", "template", "info")).To(Succeed())
		output := out.String()

		Expect(output).To(ContainSubstring("devex template info <template-name>"))
		Expect(output).NotTo(ContainSubstring("Topic 'template info' not found"))
	})

	It("falls back to topic search when no command matches", func() {
		Expect(run("help", "overview")).To(Succeed())
		output := out.String()

		Expect(output).To(ContainSubstring("DevEx Overview"))
	})

	It("handles non-existent topics gracefully with search suggestions", func() {
		Expect(run("help", "nonexistenttopic")).To(Succeed())
		output := out.String()

		Expect(output).To(ContainSubstring("Topic 'nonexistenttopic' not found. Searching for related topics..."))
		Expect(output).To(ContainSubstring("No help topics found for 'nonexistenttopic'"))
	})

	It("supports --list flag to display all available topics", func() {
		Expect(run("help", "--list")).To(Succeed())
		output := out.String()

		Expect(output).To(ContainSubstring("Getting Started:"))
		Expect(output).To(ContainSubstring("overview"))
	})
})
