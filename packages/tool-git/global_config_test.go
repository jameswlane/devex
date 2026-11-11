package main_test

import (
	"fmt"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/jameswlane/devex/packages/plugin-sdk/sdktest"
	main "github.com/jameswlane/devex/packages/tool-git"
)

type defaultSetting struct {
	key   string
	value string
}

var recommendedSettings = []defaultSetting{
	{key: "init.defaultBranch", value: "main"},
	{key: "core.editor", value: "vim"},
	{key: "color.ui", value: "auto"},
	{key: "pull.rebase", value: "false"},
	{key: "push.default", value: "simple"},
	{key: "credential.helper", value: "cache --timeout=3600"},
	{key: "merge.conflictstyle", value: "diff3"},
	{key: "diff.colorMoved", value: "default"},
	{key: "fetch.prune", value: "true"},
}

var _ = Describe("Global Git configuration", func() {
	var (
		plugin   *main.GitPlugin
		recorder *sdktest.Recorder
	)

	BeforeEach(func() {
		plugin = main.NewGitPlugin()
		recorder = sdktest.NewRecorder(GinkgoT(), "git")
	})

	identityCalls := func() []sdktest.Call {
		return []sdktest.Call{
			{Args: []string{"config", "--global", "user.name", "N"}},
			{Args: []string{"config", "--global", "user.email", "E"}},
		}
	}

	scriptSettings := func(settings []defaultSetting, existing map[string]string) {
		for _, setting := range settings {
			response := sdktest.Response{ExitCode: 1}
			if value, ok := existing[setting.key]; ok {
				response = sdktest.Response{Stdout: value + "\n"}
			}
			recorder.Script("git", []string{"config", "--global", "--get", setting.key}, response)
		}
	}

	expectedDefaultCalls := func(settings []defaultSetting, existing map[string]string) []sdktest.Call {
		calls := identityCalls()
		for _, setting := range settings {
			calls = append(calls, sdktest.Call{Args: []string{"config", "--global", "--get", setting.key}})
			if _, ok := existing[setting.key]; !ok {
				calls = append(calls, sdktest.Call{Args: []string{"config", "--global", setting.key, setting.value}})
			}
		}
		return calls
	}

	Describe("config", func() {
		It("sets only the requested user identity by default", func() {
			Expect(plugin.Execute("config", []string{"--name", "N", "--email", "E"})).To(Succeed())
			Expect(recorder.Calls("git")).To(Equal(identityCalls()))
		})

		It("applies only missing recommended settings with --defaults", func() {
			existing := map[string]string{"core.editor": "nano"}
			scriptSettings(recommendedSettings, existing)

			output := sdktest.CaptureStdout(GinkgoT(), func() {
				Expect(plugin.Execute("config", []string{"--name", "N", "--email", "E", "--defaults"})).To(Succeed())
			})

			Expect(recorder.Calls("git")).To(ConsistOf(expectedDefaultCalls(recommendedSettings, existing)))
			Expect(output).To(ContainSubstring("Set init.defaultBranch"))
			Expect(output).To(ContainSubstring("Kept core.editor"))
		})
	})

	It("never overwrites a setting Git cannot read as a single unset value", func() {
		scriptSettings(recommendedSettings, nil)
		// Several credential helpers: --get exits 2.
		recorder.Script("git", []string{"config", "--global", "--get", "credential.helper"}, sdktest.Response{ExitCode: 2})

		output := sdktest.CaptureStdout(GinkgoT(), func() {
			Expect(plugin.Execute("config", []string{"--name", "N", "--email", "E", "--defaults"})).To(Succeed())
		})

		for _, call := range recorder.Calls("git") {
			Expect(call.Args).NotTo(Equal([]string{"config", "--global", "credential.helper", "cache --timeout=3600"}))
		}
		Expect(output).To(ContainSubstring("failed to set credential.helper: read credential.helper before setting it"))
	})

	Describe("setup", func() {
		BeforeEach(func() {
			sdktest.ProvideStdin(GinkgoT(), `{"command":"setup","parameters":{"git_full_name":"N","git_email":"E"},"environment":{}}`)
		})

		It("sets only the requested user identity by default", func() {
			Expect(plugin.Execute("setup", nil)).To(Succeed())
			Expect(recorder.Calls("git")).To(Equal(identityCalls()))
		})

		It("applies only missing recommended settings with --defaults", func() {
			existing := map[string]string{"core.editor": "nano"}
			scriptSettings(recommendedSettings, existing)

			output := sdktest.CaptureStdout(GinkgoT(), func() {
				Expect(plugin.Execute("setup", []string{"--defaults"})).To(Succeed())
			})

			Expect(recorder.Calls("git")).To(ConsistOf(expectedDefaultCalls(recommendedSettings, existing)))
			Expect(output).To(ContainSubstring("Set init.defaultBranch"))
			Expect(output).To(ContainSubstring("Kept core.editor"))
		})
	})

	Describe("aliases", func() {
		It("does not install recommended aliases without --defaults", func() {
			output := sdktest.CaptureStdout(GinkgoT(), func() {
				Expect(plugin.Execute("aliases", nil)).To(Succeed())
			})
			Expect(recorder.Calls("git")).To(BeEmpty())
			Expect(output).To(ContainSubstring("aliases --defaults"))
		})

		It("installs only missing recommended aliases with --defaults", func() {
			aliases := plugin.GetGitAliases()
			settings := make([]defaultSetting, 0, len(aliases))
			for alias, command := range aliases {
				settings = append(settings, defaultSetting{key: fmt.Sprintf("alias.%s", alias), value: command})
			}
			existing := map[string]string{"alias.st": "status --short"}
			scriptSettings(settings, existing)

			output := sdktest.CaptureStdout(GinkgoT(), func() {
				Expect(plugin.Execute("aliases", []string{"--defaults"})).To(Succeed())
			})

			expected := expectedDefaultCalls(settings, existing)
			Expect(recorder.Calls("git")).To(ConsistOf(expected[len(identityCalls()):]))
			Expect(output).To(ContainSubstring("Set alias.co"))
			Expect(output).To(ContainSubstring("Kept alias.st"))
		})
	})

	Describe("command metadata", func() {
		It("advertises the defaults opt-in for every recommended-settings command", func() {
			commands := make(map[string]map[string]string)
			for _, command := range plugin.Info().Commands {
				commands[command.Name] = command.Flags
			}
			for _, name := range []string{"config", "setup", "aliases"} {
				Expect(commands[name]).To(HaveKey("defaults"), name)
			}
		})
	})
})
