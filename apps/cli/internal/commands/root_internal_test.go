package commands

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/log"
	"github.com/jameswlane/devex/apps/cli/internal/mocks"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

var _ = Describe("Bare root command", func() {
	var repo *mocks.MockRepository
	var output bytes.Buffer
	var options rootOptions
	var calls int

	BeforeEach(func() {
		viper.Reset()
		DeferCleanup(viper.Reset)
		for _, key := range []string{"CI", "GITHUB_ACTIONS", "GITLAB_CI", "DEVEX_NONINTERACTIVE", "DEVEX_NON_INTERACTIVE", "TERM", "DEVEX_CONFIG"} {
			GinkgoT().Setenv(key, "")
		}
		GinkgoT().Setenv("HOME", GinkgoT().TempDir())
		repo = mocks.NewMockRepository()
		output.Reset()
		calls = 0
		options = rootOptions{
			runSetup:    func(*cobra.Command, []string) error { calls++; return nil },
			hasTerminal: func(*cobra.Command) bool { return true },
		}
	})

	run := func() error {
		cmd := newRootCmd("test", repo, config.CrossPlatformSettings{}, options)
		cmd.SetOut(&output)
		cmd.SetErr(&output)
		cmd.SetArgs([]string{})
		return cmd.Execute()
	}

	It("invokes setup exactly once with no args, preserves context, and returns without root bootstrap", func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		options.runSetup = func(cmd *cobra.Command, args []string) error {
			calls++
			Expect(cmd.Context()).To(BeIdenticalTo(ctx))
			Expect(args).To(BeEmpty())
			return nil
		}
		cmd := newRootCmd("test", repo, config.CrossPlatformSettings{}, options)
		cmd.SetOut(&output)
		cmd.SetArgs([]string{})
		preRuns := 0
		preRun := cmd.PersistentPreRunE
		cmd.PersistentPreRunE = func(cmd *cobra.Command, args []string) error {
			preRuns++
			return preRun(cmd, args)
		}
		Expect(cmd.ExecuteContext(ctx)).To(Succeed())
		Expect(calls).To(Equal(1))
		Expect(preRuns).To(Equal(1))
		Expect(filepath.Join(os.Getenv("HOME"), ".devex", "plugins")).NotTo(BeADirectory())
	})

	It("propagates setup errors without retrying", func() {
		failure := errors.New("setup failed")
		options.runSetup = func(*cobra.Command, []string) error { calls++; return failure }
		Expect(run()).To(MatchError(failure))
		Expect(calls).To(Equal(1))
	})

	It("prints next steps when applications are already tracked", func() {
		Expect(repo.AddApp("git")).To(Succeed())
		Expect(run()).To(Succeed())
		Expect(calls).To(BeZero())
		Expect(output.String()).To(ContainSubstring("devex status"))
	})

	DescribeTable("prints next steps for non-interactive environments", func(key, value string) {
		GinkgoT().Setenv(key, value)
		Expect(run()).To(Succeed())
		Expect(calls).To(BeZero())
		Expect(output.String()).To(ContainSubstring("devex setup"))
	},
		Entry("CI", "CI", "true"),
		Entry("dumb terminal", "TERM", "dumb"),
		Entry("environment opt-out", "DEVEX_NONINTERACTIVE", "1"),
		Entry("conventional environment opt-out", "DEVEX_NON_INTERACTIVE", "true"),
	)

	It("honors non-interactive configuration", func() {
		viper.SetConfigType("yaml")
		Expect(viper.ReadConfig(strings.NewReader("non-interactive: true\n"))).To(Succeed())
		Expect(run()).To(Succeed())
		Expect(calls).To(BeZero())
	})

	It("does not start setup with redirected streams even when TERM suggests a terminal", func() {
		GinkgoT().Setenv("TERM", "xterm-256color")
		options.hasTerminal = nil // Exercise the real TTY check with pipes/buffers.
		Expect(run()).To(Succeed())
		Expect(calls).To(BeZero())
		Expect(output.String()).To(ContainSubstring("devex --help"))
	})

	It("binds plugin flags to their environment overrides", func() {
		GinkgoT().Setenv("DEVEX_SKIP_PLUGIN_DOWNLOAD", "true")
		GinkgoT().Setenv("DEVEX_OFFLINE", "true")
		cmd := NewRootCmd("test", repo, config.CrossPlatformSettings{})
		Expect(initializeConfig(cmd)).To(Succeed())
		Expect(viper.GetBool("skip-plugin-download")).To(BeTrue())
		Expect(viper.GetBool("offline")).To(BeTrue())
	})
})

var _ = Describe("Runtime logging configuration", func() {
	BeforeEach(func() {
		viper.Reset()
		DeferCleanup(viper.Reset)
		DeferCleanup(log.InitTestLogger)
		GinkgoT().Setenv("HOME", GinkgoT().TempDir())
		for _, key := range []string{"DEVEX_LOG_LEVEL", "DEVEX_VERBOSE", "DEVEX_DEBUG", "DEBUG", "DEVEX_QUIET", "DEVEX_CONFIG"} {
			GinkgoT().Setenv(key, "")
		}
	})
	DescribeTable("filters logs after resolving flags, environment and config", func(args []string, env, configText, expected string) {
		var output bytes.Buffer
		log.InitDefaultLogger(&output)
		if env != "" {
			GinkgoT().Setenv("DEVEX_LOG_LEVEL", env)
		}
		cmd := NewRootCmd("test", nil, config.CrossPlatformSettings{})
		if configText != "" {
			path := filepath.Join(GinkgoT().TempDir(), "config.yaml")
			Expect(os.WriteFile(path, []byte(configText), 0600)).To(Succeed())
			args = append(args, "--config", path)
		}
		Expect(cmd.ParseFlags(args)).To(Succeed())
		Expect(initializeConfig(cmd)).To(Succeed())
		log.Debug("debug-marker")
		log.Info("info-marker")
		log.Warn("warn-marker")
		log.Error("error-marker", errors.New("action needed"))
		for _, level := range []string{"debug", "info", "warn", "error"} {
			if strings.Contains(expected, level) {
				Expect(output.String()).To(ContainSubstring(level + "-marker"))
			} else {
				Expect(output.String()).NotTo(ContainSubstring(level + "-marker"))
			}
		}
	},
		Entry("debug flag", []string{"--log-level", "debug"}, "", "", "debug info warn error"),
		Entry("default info", []string{}, "", "", "info warn error"),
		Entry("warn env overrides config", []string{}, "warn", "log-level: debug", "warn error"),
		Entry("error flag overrides env", []string{"--log-level", "error"}, "debug", "", "error"),
		Entry("config level", []string{}, "", "log-level: warn", "warn error"),
		Entry("verbose shorthand", []string{"--verbose"}, "", "", "debug info warn error"),
		Entry("explicit info beats verbose", []string{"--verbose", "--log-level", "info"}, "", "", "info warn error"),
		Entry("quiet beats debug", []string{"--quiet", "--log-level", "debug", "--verbose"}, "", "", "error"),
		Entry("quiet config", []string{}, "", "quiet: true", "error"),
	)
	DescribeTable("keeps debug environment aliases compatible", func(key string) {
		GinkgoT().Setenv(key, "true")
		var output bytes.Buffer
		log.InitDefaultLogger(&output)
		cmd := NewRootCmd("test", nil, config.CrossPlatformSettings{})
		Expect(initializeConfig(cmd)).To(Succeed())
		log.Debug("legacy-debug-marker")
		Expect(output.String()).To(ContainSubstring("legacy-debug-marker"))
	}, Entry("DEVEX_DEBUG", "DEVEX_DEBUG"), Entry("DEBUG", "DEBUG"))
	It("lets an explicit false quiet flag override the environment", func() {
		GinkgoT().Setenv("DEVEX_QUIET", "true")
		var output bytes.Buffer
		log.InitDefaultLogger(&output)
		cmd := NewRootCmd("test", nil, config.CrossPlatformSettings{})
		Expect(cmd.ParseFlags([]string{"--quiet=false"})).To(Succeed())
		Expect(initializeConfig(cmd)).To(Succeed())
		log.Info("visible-info-marker")
		Expect(output.String()).To(ContainSubstring("visible-info-marker"))
	})

	It("rejects invalid levels even with quiet", func() {
		cmd := NewRootCmd("test", nil, config.CrossPlatformSettings{})
		Expect(cmd.ParseFlags([]string{"--log-level", "trace", "--quiet"})).To(Succeed())
		Expect(initializeConfig(cmd)).To(MatchError(ContainSubstring("invalid log-level")))
	})
})
