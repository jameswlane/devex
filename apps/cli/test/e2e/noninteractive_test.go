//go:build e2e

package e2e_test

import (
	"os"
	"path/filepath"
	"regexp"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// interactiveEnvironment clears every signal setup.IsInteractiveMode treats as
// non-interactive, so only the config file can select the plain install path.
var interactiveEnvironment = map[string]string{
	"DEVEX_NONINTERACTIVE": "",
	"CI":                   "",
	"GITHUB_ACTIONS":       "",
	"GITLAB_CI":            "",
	"TERM":                 "xterm-256color",
}

var _ = Describe("Non-interactive install configuration", func() {
	It("rejects an unknown application before installing anything", func() {
		home := sandbox.isolatedHome()
		fixture := fixturePath("unknown-app.yaml")
		// The fixture's string entries are all names no settings file defines.
		unknown, _ := fixtureApplications(fixture)
		Expect(unknown).NotTo(BeEmpty(), "unknown-app.yaml must name its unknown apps as strings")
		output, err := sandbox.run(home, sandbox.binary, time.Minute, nil, "install", "--non-interactive", "--config", fixture)
		Expect(err).To(HaveOccurred(), "an unknown application must fail the install:\n%s", output)
		for _, name := range unknown {
			Expect(output).To(ContainSubstring(name), "the error must name the unknown application")
		}
		Expect(output).NotTo(MatchRegexp(`(?m)^installing `), "nothing may be installed when the config is invalid")
		Expect(output).NotTo(MatchRegexp(`(?m)^installed \d+ of \d+ applications$`), "validation must fail before the install loop")
	})

	It("takes the plain install path from non-interactive: true in the config alone", func() {
		home := sandbox.isolatedHome()
		fixture := fixturePath("config-noninteractive.yaml")
		byName, inline := fixtureApplications(fixture)
		overrides := map[string]string{
			// An unreachable registry makes every plugin install fail, so the
			// spec proves which path ran without changing the system.
			"DEVEX_PLUGIN_REGISTRY_URL": "http://127.0.0.1:1",
		}
		for key, value := range interactiveEnvironment {
			overrides[key] = value
		}
		// run leaves stdin closed and attaches no TTY; a TUI would fail or hang here.
		output, err := sandbox.run(home, sandbox.binary, 2*time.Minute, overrides, "install", "--config", fixture)
		Expect(err).To(HaveOccurred(), "installs cannot succeed without the registry:\n%s", output)
		Expect(output).NotTo(ContainSubstring("\x1b[?1049h"), "the install must not open the Bubble Tea screen")
		for _, name := range append(byName, inline...) {
			quoted := regexp.QuoteMeta(name)
			Expect(output).To(MatchRegexp(`(?m)^installing %s\.\.\.$`, quoted), "plain output must announce %s", name)
			Expect(output).To(MatchRegexp(`(?m)^failed %s: `, quoted), "plain output must report the %s failure", name)
		}
		Expect(output).To(MatchRegexp(`(?m)^installed 0 of %d applications$`, len(byName)+len(inline)), "plain output must end with the summary")
	})
})

var _ = Describe("Non-interactive uninstall", func() {
	It("returns a nonzero exit code for an unknown application", func() {
		home := sandbox.isolatedHome()
		output, err := sandbox.run(home, sandbox.binary, time.Minute, nil, "uninstall", "--app", "not-a-devex-application", "--force")
		Expect(err).To(HaveOccurred(), "an unknown application must fail uninstall:\n%s", output)
		Expect(output).To(ContainSubstring("not-a-devex-application"))
		Expect(output).NotTo(ContainSubstring("panic"))
	})
})

var _ = Describe("Bare devex", func() {
	It("exits with helpful next steps and no plugin initialization when stdin is closed", func() {
		home := sandbox.isolatedHome()
		// Clear CI and opt-out variables: stdin is /dev/null and TERM alone must not start setup.
		output, err := sandbox.run(home, sandbox.binary, 5*time.Second, interactiveEnvironment)
		Expect(err).NotTo(HaveOccurred(), "bare devex should print next steps:\n%s", output)
		Expect(output).To(ContainSubstring("devex setup"))
		Expect(output).To(ContainSubstring("devex --help"))
		Expect(output).NotTo(ContainSubstring("\x1b[?1049h"))
		_, err = os.Stat(sandbox.paths(home, nil).Plugins())
		Expect(os.IsNotExist(err)).To(BeTrue(), "bare devex must not bootstrap plugins")
		logs, err := filepath.Glob(filepath.Join(sandbox.paths(home, nil).Logs(), "*.log"))
		Expect(err).NotTo(HaveOccurred())
		for _, path := range logs {
			contents, err := os.ReadFile(path)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(contents)).NotTo(ContainSubstring("Platform detection cached"))
			Expect(string(contents)).NotTo(ContainSubstring("Downloading required plugins"))
		}
	})
})

var _ = Describe("Non-interactive setup Git configuration", func() {
	It("sets a missing identity and preserves it on subsequent setup", func() {
		home := sandbox.isolatedHome()
		config := filepath.Join(sandbox.paths(home, nil).Config, "config.yaml")
		Expect(os.MkdirAll(filepath.Dir(config), 0750)).To(Succeed())
		Expect(os.WriteFile(config, []byte("applications: []\n"), 0600)).To(Succeed())
		overrides := map[string]string{
			"GIT_CONFIG_GLOBAL":   filepath.Join(home, ".gitconfig"),
			"GIT_CONFIG_NOSYSTEM": "1",
		}
		setup := func(name, email string) {
			output, err := sandbox.run(home, sandbox.binary, 3*time.Minute, overrides,
				"setup", "--non-interactive", "--shell", "bash",
				"--git-name", name, "--git-email", email)
			Expect(err).NotTo(HaveOccurred(), "setup must succeed using released plugins:\n%s", output)
		}
		identity := func(key, expected string) {
			output, err := sandbox.run(home, "git", time.Minute, overrides, "config", "--global", "--get", key)
			Expect(err).NotTo(HaveOccurred(), "%s", output)
			Expect(output).To(Equal(expected + "\n"))
		}
		setup("Setup User", "setup@example.com")
		identity("user.name", "Setup User")
		identity("user.email", "setup@example.com")
		setup("Replacement User", "replacement@example.com")
		identity("user.name", "Setup User")
		identity("user.email", "setup@example.com")
	})
})
