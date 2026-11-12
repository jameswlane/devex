// Package devcontainer turns DevEx application selections into a Development
// Container (https://containers.dev) specification: a devcontainer.json and a
// Dockerfile. Language runtimes and tools become official Dev Container
// Features, plain apt packages become one apt-get layer, and everything that
// only makes sense on the host is reported as skipped with the reason.
package devcontainer

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/jameswlane/devex/apps/cli/internal/types"
)

// DefaultImage is the base image when none is chosen.
const DefaultImage = "mcr.microsoft.com/devcontainers/base:ubuntu-24.04"

// DefaultRemoteUser is the non-root user editors connect as.
const DefaultRemoteUser = "vscode"

// features maps catalog application names to official Dev Container Features.
var features = map[string]string{
	"go":         "ghcr.io/devcontainers/features/go:1",
	"node.js":    "ghcr.io/devcontainers/features/node:1",
	"python":     "ghcr.io/devcontainers/features/python:1",
	"rust":       "ghcr.io/devcontainers/features/rust:1",
	"ruby":       "ghcr.io/devcontainers/features/ruby:1",
	"java":       "ghcr.io/devcontainers/features/java:1",
	"github cli": "ghcr.io/devcontainers/features/github-cli:1",
	"docker":     "ghcr.io/devcontainers/features/docker-outside-of-docker:1",
	"git":        "ghcr.io/devcontainers/features/git:1",
}

// desktopApps are catalog applications with a graphical interface, and fonts;
// neither belongs in a development container.
var desktopApps = map[string]bool{
	"ghostty": true, "ulauncher": true, "typora": true, "zen browser": true,
	"proton mail": true, "proton pass": true, "proton vpn": true,
	"dolphin file manager": true, "kde connect": true, "gnome extensions": true, "gnome tweaks": true,
	"cascadia mono": true, "fira code": true, "ia writer mono": true, "meslo lg s": true,
}

// HasFeature reports whether name maps to a Dev Container Feature, so it needs
// no install definition.
func HasFeature(name string) bool {
	_, ok := features[strings.ToLower(strings.TrimSpace(name))]
	return ok
}

// hostOnlyMethods install desktop or host-wide software.
var hostOnlyMethods = map[string]string{
	"flatpak":  "desktop application (flatpak)",
	"snap":     "snap packages need snapd, which containers do not run",
	"appimage": "desktop application (AppImage)",
	"deb":      "downloaded .deb package; add it to the Dockerfile yourself",
}

var (
	aptPackage = regexp.MustCompile(`^[a-z0-9][a-z0-9+.-]*$`)
	userName   = regexp.MustCompile(`^[a-z_][a-z0-9_-]{0,31}$`)
	imageRef   = regexp.MustCompile(`^[a-z0-9]+([._/:@-][A-Za-z0-9_.-]+)*$`)
)

// Options configures generation.
type Options struct {
	Name  string
	Image string
	// RemoteUser is the non-root user editors connect as (default vscode).
	RemoteUser string
}

// Skipped is an application left out of the container, with the reason.
type Skipped struct {
	App    string `json:"app"`
	Reason string `json:"reason"`
}

// Spec is the generated devcontainer.json.
type Spec struct {
	Name     string                    `json:"name"`
	Build    Build                     `json:"build"`
	Features map[string]map[string]any `json:"features,omitempty"`
	// RemoteUser is the non-root user editors connect as; the Dockerfile
	// creates it unless the base image already provides it.
	RemoteUser string `json:"remoteUser"`
}

// Build points at the generated Dockerfile.
type Build struct {
	Dockerfile string `json:"dockerfile"`
	Context    string `json:"context"`
}

// Result is a generated container definition.
type Result struct {
	Spec        Spec      `json:"spec"`
	Dockerfile  string    `json:"dockerfile"`
	AptPackages []string  `json:"apt_packages"`
	Features    []string  `json:"features"`
	Included    []string  `json:"included"`
	Skipped     []Skipped `json:"skipped"`
}

// ValidateImage checks a base image reference.
func ValidateImage(image string) error {
	if !imageRef.MatchString(image) || strings.Contains(image, "..") {
		return fmt.Errorf("invalid image reference %q", image)
	}
	return nil
}

// Generate maps apps into a container definition.
func Generate(apps []types.CrossPlatformApp, options Options) (Result, error) {
	if options.Image == "" {
		options.Image = DefaultImage
	}
	if err := ValidateImage(options.Image); err != nil {
		return Result{}, err
	}
	if options.Name == "" {
		options.Name = "DevEx"
	}
	if options.RemoteUser == "" {
		options.RemoteUser = DefaultRemoteUser
	}
	if !userName.MatchString(options.RemoteUser) || options.RemoteUser == "root" {
		return Result{}, fmt.Errorf("invalid remote user %q: use a non-root Linux user name such as %s", options.RemoteUser, DefaultRemoteUser)
	}
	result := Result{Included: []string{}, Skipped: []Skipped{}, AptPackages: []string{}, Features: []string{}}
	featureSet := map[string]bool{}
	packageSet := map[string]bool{}
	for _, app := range apps {
		key := strings.ToLower(app.Name)
		if feature, ok := features[key]; ok {
			featureSet[feature] = true
			result.Included = append(result.Included, app.Name)
			continue
		}
		linux := app.Linux
		if linux.InstallMethod == "" {
			linux = app.AllPlatforms
		}
		if reason := skipReason(app, linux); reason != "" {
			result.Skipped = append(result.Skipped, Skipped{App: app.Name, Reason: reason})
			continue
		}
		packages := linux.Packages
		if len(packages) == 0 {
			packages = strings.Fields(linux.InstallCommand)
		}
		valid := len(packages) > 0
		for _, name := range packages {
			if !aptPackage.MatchString(name) {
				valid = false
			}
		}
		if !valid {
			result.Skipped = append(result.Skipped, Skipped{App: app.Name, Reason: "its apt install command is not a plain package list"})
			continue
		}
		for _, name := range packages {
			packageSet[name] = true
		}
		result.Included = append(result.Included, app.Name)
	}
	for feature := range featureSet {
		result.Features = append(result.Features, feature)
	}
	for name := range packageSet {
		result.AptPackages = append(result.AptPackages, name)
	}
	sort.Strings(result.Features)
	sort.Strings(result.AptPackages)
	sort.Strings(result.Included)
	sort.Slice(result.Skipped, func(i, j int) bool { return result.Skipped[i].App < result.Skipped[j].App })

	result.Spec = Spec{Name: options.Name, Build: Build{Dockerfile: "Dockerfile", Context: ".."}, RemoteUser: options.RemoteUser}
	if len(result.Features) > 0 {
		result.Spec.Features = map[string]map[string]any{}
		for _, feature := range result.Features {
			result.Spec.Features[feature] = map[string]any{}
		}
	}
	result.Dockerfile = dockerfile(options.Image, result.AptPackages, options.RemoteUser)
	return result, nil
}

func skipReason(app types.CrossPlatformApp, linux types.OSConfig) string {
	if desktopApps[strings.ToLower(app.Name)] {
		return "desktop application"
	}
	for _, environment := range app.DesktopEnvironments {
		if !strings.EqualFold(environment, "all") {
			return "desktop application"
		}
	}
	method := strings.ToLower(linux.InstallMethod)
	if reason, ok := hostOnlyMethods[method]; ok {
		return reason
	}
	switch {
	case method == "":
		return "no Linux install definition"
	case method != "apt":
		return fmt.Sprintf("installed with %s; add it to the Dockerfile or a Feature yourself", method)
	case len(linux.AptSources) > 0:
		return "needs a third-party apt repository; add the repository to the Dockerfile yourself"
	case len(linux.PreInstall) > 0 || len(linux.PostInstall) > 0:
		return "has setup hooks that DevEx runs on the host"
	}
	return ""
}

// dockerfile renders the Dockerfile. It switches to root first, because a base
// image may default to a non-root USER. It never assumes the base image ships
// the remote user, not even an official Dev Container image (javascript-node
// ships node, not vscode): the user is checked at build time and created when
// missing. Base images must be Debian or Ubuntu based.
func dockerfile(image string, packages []string, remoteUser string) string {
	var builder strings.Builder
	builder.WriteString("# Generated by devex devcontainer generate. Edit freely; regenerate with --force.\n")
	fmt.Fprintf(&builder, "FROM %s\n", image)
	builder.WriteString("\nUSER root\n")
	if len(packages) > 0 {
		builder.WriteString("\nRUN apt-get update \\\n")
		builder.WriteString("    && DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends \\\n")
		for _, name := range packages {
			fmt.Fprintf(&builder, "        %s \\\n", name)
		}
		builder.WriteString("    && rm -rf /var/lib/apt/lists/*\n")
	}
	builder.WriteString("\n# The remote user editors connect as; created unless the image has it.\n")
	fmt.Fprintf(&builder, "RUN if ! id -u %[1]s >/dev/null 2>&1; then \\\n", remoteUser)
	fmt.Fprintf(&builder, "        if getent group %[1]s >/dev/null; then useradd --create-home --shell /bin/bash --gid %[1]s %[1]s; \\\n", remoteUser)
	fmt.Fprintf(&builder, "        else useradd --create-home --shell /bin/bash --user-group %[1]s; fi; \\\n", remoteUser)
	builder.WriteString("    fi\n")
	return builder.String()
}

// Marshal renders devcontainer.json.
func (r Result) Marshal() ([]byte, error) {
	data, err := json.MarshalIndent(r.Spec, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

// Validate checks a devcontainer.json produced by Marshal: it must parse, name
// the Dockerfile build, and reference only known Feature identifiers.
func Validate(data []byte) error {
	var spec Spec
	decoder := json.NewDecoder(strings.NewReader(string(data)))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&spec); err != nil {
		return fmt.Errorf("devcontainer.json: %w", err)
	}
	if spec.Name == "" || spec.Build.Dockerfile == "" {
		return fmt.Errorf("devcontainer.json: name and build.dockerfile are required")
	}
	known := map[string]bool{}
	for _, feature := range features {
		known[feature] = true
	}
	for feature := range spec.Features {
		if !known[feature] {
			return fmt.Errorf("devcontainer.json: unknown feature %q", feature)
		}
	}
	return nil
}
