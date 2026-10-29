package utils_test

import (
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/jameswlane/devex/apps/cli/internal/utils"
)

var _ = Describe("RequiredDependenciesHint", func() {
	onPath := func(available ...string) func(string) (string, error) {
		return func(name string) (string, error) {
			for _, a := range available {
				if a == name {
					return "/usr/bin/" + name, nil
				}
			}
			return "", errors.New("not found")
		}
	}

	It("uses apt-get when it is available", func() {
		Expect(utils.RequiredDependenciesHintWith("linux", onPath("apt-get", "brew"))).To(Equal(
			"install curl git unzip with your package manager (for example: sudo apt-get install curl git unzip)"))
	})

	It("uses dnf on systems without apt-get", func() {
		Expect(utils.RequiredDependenciesHintWith("linux", onPath("dnf"))).To(ContainSubstring("sudo dnf install curl git unzip"))
	})

	It("names the tools without an example when no known package manager is found", func() {
		Expect(utils.RequiredDependenciesHintWith("linux", onPath())).To(Equal("install curl git unzip with your package manager"))
	})
})

var _ = Describe("OS-specific dependencies", func() {
	It("requires only git on Windows", func() {
		Expect(utils.RequiredDependenciesForOS("windows")).To(Equal([]utils.Dependency{{Name: "git", Command: "git"}}))
	})
	It("requires curl, git and unzip on Linux and macOS", func() {
		for _, goos := range []string{"linux", "darwin"} {
			Expect(utils.RequiredDependenciesForOS(goos)).To(Equal([]utils.Dependency{
				{Name: "curl", Command: "curl"}, {Name: "git", Command: "git"}, {Name: "unzip", Command: "unzip"},
			}))
		}
	})
	DescribeTable("Windows package manager hints", func(binary, command string) {
		lookup := func(name string) (string, error) {
			if name == binary {
				return name, nil
			}
			return "", errors.New("not found")
		}
		Expect(utils.RequiredDependenciesHintWith("windows", lookup)).To(ContainSubstring(command))
	}, Entry("winget", "winget", "winget install Git.Git"),
		Entry("Chocolatey", "choco", "choco install git"), Entry("Scoop", "scoop", "scoop install git"),
		Entry("no manager", "", "winget install Git.Git"))
})
