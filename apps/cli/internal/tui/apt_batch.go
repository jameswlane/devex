package tui

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"sync"

	"github.com/jameswlane/devex/apps/cli/internal/state"
	"github.com/jameswlane/devex/apps/cli/internal/types"
)

// aptPackageName matches Debian package names (policy 5.6.1) plus an optional
// ":arch" qualifier. Anything else stays on the per-app install path.
var aptPackageName = regexp.MustCompile(`^[a-z0-9][a-z0-9+.-]+(:[a-z0-9-]+)?$`)

// queryInstalledDebPackages reports which of the given packages dpkg considers
// installed. dpkg-query exits non-zero when some names are unknown but still
// prints the known ones, so the output is parsed regardless of the exit code.
// Tests replace it.
var queryInstalledDebPackages = func(ctx context.Context, packages []string) map[string]bool {
	installed := make(map[string]bool, len(packages))
	if len(packages) == 0 {
		return installed
	}
	args := append([]string{"-W", "-f=${Package}\t${Status}\n"}, packages...)
	output, _ := exec.CommandContext(ctx, "dpkg-query", args...).Output() //nolint:gosec // package names are validated by aptPackageName
	for _, line := range strings.Split(string(output), "\n") {
		name, status, found := strings.Cut(line, "\t")
		if found && strings.HasSuffix(strings.TrimSpace(status), "install ok installed") {
			installed[strings.TrimSpace(name)] = true
		}
	}
	return installed
}

// aptRunState is shared by all apps in one installation run so APT work is not
// repeated per app.
type aptRunState struct {
	mu sync.Mutex
	// listsFresh is true after a successful `apt-get update` with no APT
	// sources added since.
	listsFresh bool
	// sourcesAdded holds the source files written during this run.
	sourcesAdded map[string]bool
	// preinstalled holds apps whose packages were all installed before the run.
	preinstalled map[string]bool
	// batchInstalled holds apps whose packages the batch install already installed.
	batchInstalled map[string]bool
}

func (s *aptRunState) mark(set *map[string]bool, key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if *set == nil {
		*set = make(map[string]bool)
	}
	(*set)[key] = true
}

func (s *aptRunState) has(set map[string]bool, key string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return set[key]
}

// aptPackages returns the package names of an apt app, or nil when any name is
// not a plain Debian package name.
func aptPackages(osConfig types.OSConfig) []string {
	packages := strings.Fields(osConfig.InstallCommand)
	for _, pkg := range packages {
		if !aptPackageName.MatchString(pkg) {
			return nil
		}
	}
	return packages
}

// isAptApp reports whether the app installs through APT on this platform.
func isAptApp(app types.CrossPlatformApp) bool {
	return app.GetOSConfig().InstallMethod == "apt"
}

// addAptSources writes the app's APT sources and keys once per run. Writing a
// source makes the package lists stale, so the next ensureAptLists updates them.
func (si *StreamingInstaller) addAptSources(ctx context.Context, osConfig types.OSConfig) error {
	for _, source := range osConfig.AptSources {
		if si.apt.has(si.apt.sourcesAdded, source.SourceName) {
			continue
		}
		si.sendLog("INFO", fmt.Sprintf("Adding APT source: %s", source.SourceName))

		// SECURITY: Validate GPG key with fingerprint if provided
		if source.KeySource != "" {
			si.sendLog("INFO", fmt.Sprintf("Adding GPG key from: %s", source.KeySource))

			// Use modern GPG keyring approach (no apt-key)
			si.sendLog("INFO", "Using modern GPG keyring approach")
			if err := si.addModernGPGKey(ctx, source.KeySource, source.KeyName, source.RequireDearmor); err != nil {
				return fmt.Errorf("failed to add GPG key for %s: %w", source.SourceName, err)
			}
		}

		if source.SourceRepo != "" {
			addSourceCmd := fmt.Sprintf("echo '%s' | sudo -n tee %s > /dev/null",
				source.SourceRepo, source.SourceName)
			if err := si.executeCommandStream(ctx, addSourceCmd); err != nil {
				return fmt.Errorf("failed to add APT source %s: %w", source.SourceName, err)
			}
		}

		si.apt.mark(&si.apt.sourcesAdded, source.SourceName)
		si.apt.mu.Lock()
		si.apt.listsFresh = false
		si.apt.mu.Unlock()
	}
	return nil
}

// ensureAptLists runs `apt-get update` unless it already ran in this
// installation and no APT source was added since.
func (si *StreamingInstaller) ensureAptLists(ctx context.Context) error {
	si.apt.mu.Lock()
	fresh := si.apt.listsFresh
	si.apt.mu.Unlock()
	if fresh {
		si.sendLog("INFO", "Package lists already updated in this run")
		return nil
	}

	si.sendLog("INFO", "Updating package lists...")
	if err := si.executeCommandStream(ctx, "sudo -n apt-get update"); err != nil {
		return err
	}
	si.apt.mu.Lock()
	si.apt.listsFresh = true
	si.apt.mu.Unlock()
	return nil
}

// prepareAptRun does the APT work for the whole run up front:
//
//  1. finds apt apps whose packages are already installed (they are skipped);
//  2. adds every remaining app's APT sources, then updates the package lists once;
//  3. installs every remaining package in a single `apt-get install`.
//
// If the batch install fails, nothing is marked as installed and each app falls
// back to its own install, so one bad package does not block the others.
// Apps with pre-install steps are left out of the batch because those steps
// must run before their packages are installed.
func (si *StreamingInstaller) prepareAptRun(ctx context.Context, apps []types.CrossPlatformApp) error {
	var candidates []types.CrossPlatformApp
	var allPackages []string
	for _, app := range apps {
		if !isAptApp(app) {
			continue
		}
		packages := aptPackages(app.GetOSConfig())
		if len(packages) == 0 {
			continue
		}
		candidates = append(candidates, app)
		allPackages = append(allPackages, packages...)
	}
	if len(candidates) == 0 {
		return nil
	}

	installed := queryInstalledDebPackages(ctx, allPackages)
	var toBatch []types.CrossPlatformApp
	for _, app := range candidates {
		allInstalled := true
		for _, pkg := range aptPackages(app.GetOSConfig()) {
			if !installed[pkg] {
				allInstalled = false
				break
			}
		}
		switch {
		case allInstalled:
			si.apt.mark(&si.apt.preinstalled, app.Name)
		case len(app.GetOSConfig().PreInstall) == 0:
			toBatch = append(toBatch, app)
		}
	}
	if len(toBatch) == 0 {
		return nil
	}

	if err := AuthorizeInstallation(ctx, toBatch, false); err != nil {
		return err
	}

	var batchApps []types.CrossPlatformApp
	var batchPackages []string
	for _, app := range toBatch {
		if err := state.AuditTargetsStep(ctx, []string{app.Name}, "apt-sources", func() error { return si.addAptSources(ctx, app.GetOSConfig()) }); err != nil {
			// The app's own install reports this error.
			si.sendLog("WARN", fmt.Sprintf("Leaving %s out of the batch install: %v", app.Name, err))
			continue
		}
		batchApps = append(batchApps, app)
		for _, pkg := range aptPackages(app.GetOSConfig()) {
			if !installed[pkg] {
				batchPackages = append(batchPackages, pkg)
			}
		}
	}
	if len(batchApps) == 0 {
		return nil
	}

	targets := make([]string, 0, len(batchApps))
	for _, app := range batchApps {
		targets = append(targets, app.Name)
	}
	if err := state.AuditTargetsStep(ctx, targets, "apt-update", func() error { return si.ensureAptLists(ctx) }); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		si.sendLog("WARN", fmt.Sprintf("Package list update failed; installing apps one at a time: %v", err))
		return nil
	}

	si.sendLog("INFO", fmt.Sprintf("Installing %d APT packages for %d apps in one transaction...", len(batchPackages), len(batchApps)))
	command := "sudo -n apt-get install -y " + strings.Join(batchPackages, " ")
	if err := state.AuditTargetsStep(ctx, targets, "install-batch", func() error { return si.executeCommandStream(ctx, command) }); err != nil {
		if ctx.Err() != nil {
			return ctx.Err()
		}
		si.sendLog("WARN", fmt.Sprintf("Batch APT install failed; installing apps one at a time: %v", err))
		return nil
	}
	for _, app := range batchApps {
		si.apt.mark(&si.apt.batchInstalled, app.Name)
	}
	return nil
}

// InstalledAptApps reports which apt apps already have all their packages
// installed, using one dpkg-query call. The installer skips these apps, so
// setup uses the same check to show them as skipped before installing.
func InstalledAptApps(ctx context.Context, apps []types.CrossPlatformApp) map[string]bool {
	var packages []string
	for _, app := range apps {
		if isAptApp(app) {
			packages = append(packages, aptPackages(app.GetOSConfig())...)
		}
	}
	installed := queryInstalledDebPackages(ctx, packages)
	result := make(map[string]bool)
	for _, app := range apps {
		if !isAptApp(app) {
			continue
		}
		pkgs := aptPackages(app.GetOSConfig())
		if len(pkgs) == 0 {
			continue
		}
		all := true
		for _, pkg := range pkgs {
			if !installed[pkg] {
				all = false
				break
			}
		}
		if all {
			result[app.Name] = true
		}
	}
	return result
}
