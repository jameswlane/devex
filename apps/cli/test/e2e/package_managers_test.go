//go:build e2e

package e2e_test

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v3"
)

// packageManagerCase is one real install/remove round trip through a
// package-manager plugin. Adding a package manager is one entry here plus its
// fixture in testdata/configs.
type packageManagerCase struct {
	method          string   // install_method; the plugin is package-manager-<method>
	distros         []string // <ID>-<VERSION_ID> from /etc/os-release, e.g. ubuntu-24.04
	fixture         string   // config file under testdata/configs naming only this package
	pkg             string   // application name in the fixture
	verifyInstalled func(home string)
	verifyRemoved   func(home string)
}

var packageManagerCases = []packageManagerCase{
	{
		method:          "apt",
		distros:         []string{"ubuntu-24.04", "debian-12"},
		fixture:         "apt-jq.yaml",
		pkg:             "jq",
		verifyInstalled: dpkgInstalled("jq", "jq", "--version"),
		verifyRemoved:   dpkgRemoved("jq", "jq"),
	},
}

// The install timeout covers apt-get update plus the download on a slow mirror.
const packageInstallTimeout = 10 * time.Minute

var _ = Describe("Package manager install lifecycle", func() {
	for _, entry := range packageManagerCases {
		Describe(entry.method+" installs "+entry.pkg, Ordered, Label("destructive", "pm:"+entry.method), func() {
			var home, fixture string
			var plugins map[string]pluginState

			BeforeAll(func() {
				if os.Getenv("DEVEX_E2E_DESTRUCTIVE") != "1" {
					Skip("requires DEVEX_E2E_DESTRUCTIVE=1 in a disposable sandbox")
				}
				if distro := currentDistro(); !contains(entry.distros, distro) {
					Skip(fmt.Sprintf("%s suite runs on %s, not %s", entry.method, strings.Join(entry.distros, ", "), distro))
				}
				home = sandbox.isolatedHome()
				fixture = fixturePath(entry.fixture)
			})

			It("installs non-interactively from the config file", func() {
				output, err := sandbox.run(home, sandbox.binary, packageInstallTimeout, nil, "install", "--non-interactive", "--config", fixture)
				Expect(err).NotTo(HaveOccurred(), "non-interactive install failed:\n%s", output)
				Expect(output).To(MatchRegexp(`(?m)^installed %s$`, regexp.QuoteMeta(entry.pkg)), "install must report the package")
				Expect(output).To(ContainSubstring("installed 1 of 1 applications"), "install must summarise one installed application")
			})

			It("downloads the package-manager plugin with the registry SHA256", func() {
				name := "package-manager-" + entry.method
				path := pluginPath(home, name)
				info, err := os.Stat(path)
				Expect(err).NotTo(HaveOccurred(), "install must download %s", name)
				Expect(info.Mode().Perm()&0111).NotTo(BeZero(), "%s must be executable", name)
				Expect(checksum(path)).To(Equal(registryChecksum(name)), "%s SHA256 must match the registry for this OS/arch", name)
			})

			It("really installs the package on the system", func() {
				entry.verifyInstalled(home)
			})

			It("records the package so list installed shows it", func() {
				Expect(installedApps(home, fixture)).To(ContainElement(entry.pkg), "list installed must include %s", entry.pkg)
			})

			It("is safe to run again without downloading anything", func() {
				plugins = snapshotPlugins(home)
				// Separate runs by more than coarse filesystem timestamp resolution.
				time.Sleep(1100 * time.Millisecond)
				output, err := sandbox.run(home, sandbox.binary, packageInstallTimeout, nil, "install", "--non-interactive", "--config", fixture)
				Expect(err).NotTo(HaveOccurred(), "repeat install must succeed:\n%s", output)
				Expect(snapshotPlugins(home)).To(Equal(plugins), "repeat install must not download or replace any plugin")
				entry.verifyInstalled(home)
			})

			It("uninstalls the package without prompting and clears tracking", func() {
				// run gives the CLI no stdin, so a prompt reads EOF instead of hanging.
				output, err := sandbox.run(home, sandbox.binary, packageInstallTimeout, nil, "uninstall", "--app", entry.pkg, "--force", "--config", fixture)
				Expect(err).NotTo(HaveOccurred(), "uninstall failed:\n%s", output)
				entry.verifyRemoved(home)
				Expect(installedApps(home, fixture)).NotTo(ContainElement(entry.pkg), "uninstall must clear the tracking record")
			})
		})
	}
})

// dpkgInstalled checks dpkg's status for pkg and runs the installed command.
func dpkgInstalled(pkg, command string, args ...string) func(home string) {
	return func(home string) {
		output, err := sandbox.run(home, "dpkg", 30*time.Second, nil, "-s", pkg)
		Expect(err).NotTo(HaveOccurred(), "dpkg -s %s must succeed:\n%s", pkg, output)
		Expect(output).To(ContainSubstring("Status: install ok installed"), "dpkg must report %s installed", pkg)
		output, err = sandbox.run(home, command, 30*time.Second, nil, args...)
		Expect(err).NotTo(HaveOccurred(), "%s %s must run:\n%s", command, strings.Join(args, " "), output)
	}
}

// dpkgRemoved checks that dpkg no longer reports pkg and command is gone.
func dpkgRemoved(pkg, command string) func(home string) {
	return func(home string) {
		output, err := sandbox.run(home, "dpkg", 30*time.Second, nil, "-s", pkg)
		Expect(err).To(HaveOccurred(), "dpkg -s %s must fail after remove:\n%s", pkg, output)
		_, err = exec.LookPath(command)
		Expect(err).To(HaveOccurred(), "%s must no longer be on PATH", command)
	}
}

// jsonStart finds the encoded app list: an array, or null when nothing is installed.
var jsonStart = regexp.MustCompile(`(?m)^(\[|null$)`)

// installedApps returns the names `devex list installed` reports for config.
func installedApps(home, config string) []string {
	output, err := sandbox.run(home, sandbox.binary, time.Minute, nil, "list", "installed", "--format", "json", "--config", config)
	Expect(err).NotTo(HaveOccurred(), "list installed failed:\n%s", output)
	// Startup diagnostics may precede the JSON document on the combined output.
	location := jsonStart.FindStringIndex(output)
	Expect(location).NotTo(BeNil(), "list installed must print JSON:\n%s", output)
	start := location[0]
	var apps []struct{ Name string }
	Expect(json.Unmarshal([]byte(output[start:]), &apps)).To(Succeed(), "decode list installed JSON:\n%s", output)
	names := make([]string, 0, len(apps))
	for _, app := range apps {
		names = append(names, app.Name)
	}
	return names
}

func currentDistro() string {
	return osRelease("ID") + "-" + osRelease("VERSION_ID")
}

func contains(values []string, value string) bool {
	for _, candidate := range values {
		if candidate == value {
			return true
		}
	}
	return false
}

// fixturePath resolves a config fixture under DEVEX_E2E_TESTDATA.
func fixturePath(name string) string {
	path := filepath.Join(envDefault("DEVEX_E2E_TESTDATA", "/opt/devex-e2e/testdata"), "configs", name)
	_, err := os.Stat(path)
	Expect(err).NotTo(HaveOccurred(), "fixture %s must exist (set DEVEX_E2E_TESTDATA)", path)
	return path
}

// fixtureApplications returns the names a config fixture's applications list
// refers to: string entries as written, inline definitions by their name.
func fixtureApplications(path string) (byName, inline []string) {
	data, err := os.ReadFile(path)
	Expect(err).NotTo(HaveOccurred(), "read fixture %s", path)
	var config struct {
		Applications []yaml.Node `yaml:"applications"`
	}
	Expect(yaml.Unmarshal(data, &config)).To(Succeed(), "parse fixture %s", path)
	for _, node := range config.Applications {
		if node.Kind == yaml.ScalarNode {
			byName = append(byName, node.Value)
			continue
		}
		var app struct {
			Name string `yaml:"name"`
		}
		Expect(node.Decode(&app)).To(Succeed(), "decode inline application in %s", path)
		inline = append(inline, app.Name)
	}
	Expect(len(byName)+len(inline)).To(BeNumerically(">", 0), "fixture %s must list applications", path)
	return byName, inline
}
