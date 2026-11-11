package tui

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/jameswlane/devex/apps/cli/internal/types"
)

// InstalledApps checks native package state before setup confirmation and execution.
// APT uses one query; other supported managers use at most four concurrent probes.
// Unknown managers and failed probes are conservatively left in the install plan.
func InstalledApps(ctx context.Context, apps []types.CrossPlatformApp) map[string]bool {
	result := InstalledAptApps(ctx, apps)
	var mu sync.Mutex
	var wg sync.WaitGroup
	slots := make(chan struct{}, 4)
	for _, app := range apps {
		cfg := app.GetOSConfig()
		if cfg.InstalledBinary != "" {
			if binaryInstalled(cfg) {
				result[app.Name] = true
			}
			continue
		}
		if cfg.InstallMethod != "mise" && cfg.InstallMethod != "flatpak" {
			continue
		}
		request, err := types.ResolvePackageInstall(cfg.InstallMethod, cfg.InstallCommand, cfg.Packages, cfg.Options)
		if err != nil || len(request.Packages) == 0 || len(request.Options) > 0 {
			continue
		}
		select {
		case slots <- struct{}{}:
		case <-ctx.Done():
			wg.Wait()
			return result
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() { <-slots }()
			for _, pkg := range request.Packages {
				args := []string{"info", pkg}
				if cfg.InstallMethod == "mise" {
					args = []string{"where", pkg}
				}
				if exec.CommandContext(ctx, cfg.InstallMethod, args...).Run() != nil {
					return
				} //nolint:gosec // fixed manager names and argv, no shell
			}
			mu.Lock()
			result[app.Name] = true
			mu.Unlock()
		}()
	}
	wg.Wait()
	return result
}

// binaryInstalled reports whether an app's declared executable already exists,
// on PATH or in the directories script installers commonly use but a setup
// shell may not have on PATH yet (~/.local/bin, ~/bin, the app's destination
// and its declared path entries).
func binaryInstalled(cfg types.OSConfig) bool {
	name := cfg.InstalledBinary
	if strings.ContainsAny(name, `/\`) {
		return isExecutable(name)
	}
	if _, err := exec.LookPath(name); err == nil {
		return true
	}
	var dirs []string
	if home, err := os.UserHomeDir(); err == nil {
		dirs = append(dirs, filepath.Join(home, ".local", "bin"), filepath.Join(home, "bin"))
	}
	if cfg.Destination != "" {
		dirs = append(dirs, expandHome(cfg.Destination))
	}
	for _, step := range cfg.PostInstall {
		for _, dir := range step.Path {
			dirs = append(dirs, expandHome(dir))
		}
	}
	for _, dir := range dirs {
		if isExecutable(filepath.Join(dir, name)) {
			return true
		}
	}
	return false
}

func isExecutable(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular() && info.Mode().Perm()&0o111 != 0
}

func expandHome(path string) string {
	if rest, ok := strings.CutPrefix(path, "~/"); ok {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, rest)
		}
	}
	return path
}
