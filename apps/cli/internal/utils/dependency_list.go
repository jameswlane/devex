package utils

import (
	"fmt"
	"os/exec"
	"runtime"
	"strings"
)

var RequiredDependencies = requiredDependencies(runtime.GOOS)

func requiredDependencies(goos string) []Dependency {
	if goos == "windows" {
		return []Dependency{{Name: "git", Command: "git"}}
	}
	return []Dependency{
		{Name: "curl", Command: "curl"},
		{Name: "git", Command: "git"},
		{Name: "unzip", Command: "unzip"},
	}
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
	return requiredDependenciesHint(runtime.GOOS, exec.LookPath)
}

func requiredDependenciesHint(goos string, lookPath func(string) (string, error)) string {
	names := make([]string, 0, len(requiredDependencies(goos)))
	for _, dep := range requiredDependencies(goos) {
		names = append(names, dep.Name)
	}
	list := strings.Join(names, " ")
	commands := installCommands
	if goos == "windows" {
		commands = []struct{ binary, install string }{
			{"winget", "winget install Git.Git"},
			{"choco", "choco install git"},
			{"scoop", "scoop install git"},
		}
		for _, pm := range commands {
			if _, err := lookPath(pm.binary); err == nil {
				return fmt.Sprintf("install git with your package manager (for example: %s)", pm.install)
			}
		}
		return "install git with winget, Chocolatey, or Scoop (for example: winget install Git.Git)"
	}
	for _, pm := range commands {
		if _, err := lookPath(pm.binary); err == nil {
			return fmt.Sprintf("install %s with your package manager (for example: %s %s)", list, pm.install, list)
		}
	}
	return fmt.Sprintf("install %s with your package manager", list)
}
