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
		Expect(utils.RequiredDependenciesHintWith(onPath("apt-get", "brew"))).To(Equal(
			"install curl git unzip with your package manager (for example: sudo apt-get install curl git unzip)"))
	})

	It("uses dnf on systems without apt-get", func() {
		Expect(utils.RequiredDependenciesHintWith(onPath("dnf"))).To(ContainSubstring("sudo dnf install curl git unzip"))
	})

	It("names the tools without an example when no known package manager is found", func() {
		Expect(utils.RequiredDependenciesHintWith(onPath())).To(Equal("install curl git unzip with your package manager"))
	})
})
