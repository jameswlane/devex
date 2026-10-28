package utils

import (
	"fmt"
	"os/exec"
	"strings"
)

var RequiredDependencies = []Dependency{
	{Name: "curl", Command: "curl"},
	{Name: "git", Command: "git"},
	{Name: "unzip", Command: "unzip"},
}

// installCommands maps a package manager binary to its install command, in lookup order.
var installCommands = []struct{ binary, install string }{
	{"apt-get", "sudo apt-get install"},
	{"dnf", "sudo dnf install"},
	{"pacman", "sudo pacman -S"},
	{"zypper", "sudo zypper install"},
	{"apk", "sudo apk add"},
	{"brew", "brew install"},
}

// RequiredDependenciesHint tells the user how to install the required dependencies,
// naming the first package manager found on PATH.
func RequiredDependenciesHint() string {
	return requiredDependenciesHint(exec.LookPath)
}

func requiredDependenciesHint(lookPath func(string) (string, error)) string {
	names := make([]string, 0, len(RequiredDependencies))
	for _, dep := range RequiredDependencies {
		names = append(names, dep.Name)
	}
	list := strings.Join(names, " ")
	for _, pm := range installCommands {
		if _, err := lookPath(pm.binary); err == nil {
			return fmt.Sprintf("install %s with your package manager (for example: %s %s)", list, pm.install, list)
		}
	}
	return fmt.Sprintf("install %s with your package manager", list)
}
