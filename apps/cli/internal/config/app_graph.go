package config

import (
	"fmt"
	"sort"
	"strings"

	"github.com/jameswlane/devex/apps/cli/internal/types"
)

// pluginInstallMethods have a package-manager plugin today. Whether that plugin
// is downloaded doesn't matter: offline configurations stay valid.
var pluginInstallMethods = map[string]bool{
	"apk": true, "appimage": true, "apt": true, "brew": true, "curlpipe": true,
	"deb": true, "dnf": true, "docker": true, "emerge": true, "eopkg": true,
	"flatpak": true, "mise": true, "nixflake": true, "nixpkgs": true, "pacman": true,
	"pip": true, "rpm": true, "snap": true, "xbps": true, "yay": true, "zypper": true,
}

// plannedInstallMethods are recognised but have no plugin yet. Entries using
// them are well-formed; they just can't be installed until a plugin exists.
var plannedInstallMethods = map[string]bool{
	"asdf": true, "cargo": true, "chocolatey": true, "dmg": true, "exe": true,
	"go": true, "macports": true, "makepkg": true, "mas": true, "msi": true,
	"npm": true, "pkg": true, "pkg_add": true, "ports": true, "scoop": true,
	"script": true, "winget": true, "yum": true,
}

// InstallMethodSupport classifies an install method.
type InstallMethodSupport int

const (
	// MethodUnknown is a typo or an unsupported method: the definition is malformed.
	MethodUnknown InstallMethodSupport = iota
	// MethodPlanned is recognised but has no plugin yet.
	MethodPlanned
	// MethodSupported has a package-manager plugin.
	MethodSupported
)

// ClassifyInstallMethod reports whether DevEx can install with a method.
func ClassifyInstallMethod(method string) InstallMethodSupport {
	switch {
	case pluginInstallMethods[method]:
		return MethodSupported
	case plannedInstallMethods[method]:
		return MethodPlanned
	default:
		return MethodUnknown
	}
}

// platformConfigs returns an app's non-empty platform entries by key.
func platformConfigs(app types.CrossPlatformApp) map[string]types.OSConfig {
	configs := map[string]types.OSConfig{}
	for key, osConfig := range map[string]types.OSConfig{
		"linux": app.Linux, "macos": app.MacOS, "windows": app.Windows, "all_platforms": app.AllPlatforms,
	} {
		if osConfig.InstallMethod != "" {
			configs[key] = osConfig
		}
	}
	return configs
}

// ValidateApplicationGraph checks the effective application set as a whole:
// install methods (including alternatives) must be known, and per-platform
// dependencies that name DevEx applications must not form cycles. Dependencies
// that match no application are native packages and are not reported, and
// platform-requirement dependencies are always native packages.
//
// File in each error is the application name.
func ValidateApplicationGraph(apps []types.CrossPlatformApp) (errs []ValidationError) {
	index := map[string]string{} // lower-case name or install command -> app name
	for _, app := range apps {
		index[strings.ToLower(app.Name)] = app.Name
	}
	// A single-word install command is a package name, so a dependency on that
	// package refers to the app. Multi-word commands are scripts or manager
	// invocations and say nothing about the app's identity.
	for _, app := range apps {
		for _, osConfig := range platformConfigs(app) {
			fields := strings.Fields(osConfig.InstallCommand)
			if len(fields) != 1 {
				continue
			}
			if _, taken := index[strings.ToLower(fields[0])]; !taken {
				index[strings.ToLower(fields[0])] = app.Name
			}
		}
	}

	graphs := map[string]map[string][]string{} // platform -> app -> dependency apps
	for _, app := range apps {
		for platform, osConfig := range platformConfigs(app) {
			field := platform
			checkMethod := func(method, where string) {
				if ClassifyInstallMethod(method) == MethodUnknown {
					errs = append(errs, ValidationError{
						File: app.Name, Field: where,
						Message: fmt.Sprintf("unknown install method %q; supported methods: %s", method, strings.Join(sortedKeys(pluginInstallMethods), ", ")),
					})
				}
			}
			checkMethod(osConfig.InstallMethod, field+".install_method")
			for i, alternative := range osConfig.Alternatives {
				checkMethod(alternative.InstallMethod, fmt.Sprintf("%s.alternatives[%d].install_method", field, i))
			}

			for _, dependency := range osConfig.Dependencies {
				target, ok := index[strings.ToLower(dependency)]
				if ok && target == app.Name {
					// e.g. a flatpak app depending on the flatpak package manager.
					ok = false
				}
				if !ok {
					// Not a DevEx application: a native package (curl, apt, flatpak, …).
					continue
				}
				if graphs[platform] == nil {
					graphs[platform] = map[string][]string{}
				}
				graphs[platform][app.Name] = append(graphs[platform][app.Name], target)
			}
		}
	}

	for _, platform := range sortedKeys(graphs) {
		for _, cycle := range findCycles(graphs[platform]) {
			errs = append(errs, ValidationError{
				File: cycle[0], Field: platform + ".dependencies",
				Message: "dependency cycle: " + strings.Join(cycle, " -> "),
			})
		}
	}
	return errs
}

// findCycles returns each distinct dependency cycle once, as a path that
// starts and ends at the same application.
func findCycles(graph map[string][]string) [][]string {
	const (
		unvisited = iota
		active
		done
	)
	state := map[string]int{}
	var stack []string
	var cycles [][]string
	seen := map[string]bool{}

	var visit func(string)
	visit = func(node string) {
		state[node] = active
		stack = append(stack, node)
		for _, next := range graph[node] {
			switch state[next] {
			case unvisited:
				visit(next)
			case active:
				start := 0
				for i, name := range stack {
					if name == next {
						start = i
						break
					}
				}
				cycle := append(append([]string{}, stack[start:]...), next)
				key := canonicalCycle(cycle)
				if !seen[key] {
					seen[key] = true
					cycles = append(cycles, cycle)
				}
			}
		}
		stack = stack[:len(stack)-1]
		state[node] = done
	}
	for _, node := range sortedKeys(graph) {
		if state[node] == unvisited {
			visit(node)
		}
	}
	return cycles
}

// canonicalCycle identifies a cycle independent of where it was entered.
func canonicalCycle(cycle []string) string {
	nodes := append([]string{}, cycle[:len(cycle)-1]...)
	sort.Strings(nodes)
	return strings.Join(nodes, "\x00")
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
