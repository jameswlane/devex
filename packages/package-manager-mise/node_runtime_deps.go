package main

import (
	"context"
	"fmt"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
)

// targetOS is the platform the plugin installs for; tests override it.
var targetOS = runtime.GOOS

// libatomicPatterns are the glob patterns searched for libatomic.so.1. They
// cover the merged-/usr and multiarch layouts of Debian, Ubuntu, Fedora,
// openSUSE and Arch. /sbin/ldconfig is not on a Debian user's PATH, so the
// plugin looks for the file instead of asking the linker cache.
var libatomicPatterns = []string{
	"/lib/*-linux-gnu/libatomic.so.1",
	"/usr/lib/*-linux-gnu/libatomic.so.1",
	"/lib64/libatomic.so.1",
	"/usr/lib64/libatomic.so.1",
	"/lib/libatomic.so.1",
	"/usr/lib/libatomic.so.1",
}

// libatomicPackages maps each system package manager to the package that ships
// libatomic.so.1. Arch's gcc-libs, which every Arch install has, already ships it.
var libatomicPackages = []struct {
	manager string
	args    []string
}{
	{"apt-get", []string{"install", "-y", "libatomic1"}},
	{"dnf", []string{"install", "-y", "libatomic"}},
	{"zypper", []string{"--non-interactive", "install", "libatomic1"}},
}

func libatomicInstalled() bool {
	for _, pattern := range libatomicPatterns {
		if matches, _ := filepath.Glob(pattern); len(matches) > 0 {
			return true
		}
	}
	return false
}

// needsNode reports whether any spec installs a mise node runtime that can be
// Node 25 or later. Explicit pins below 25 (node@24) do not link libatomic.
func needsNode(specs []string) bool {
	for _, spec := range specs {
		if toolName(spec) != "node" {
			continue
		}
		version := strings.TrimPrefix(spec, "node")
		version = strings.TrimPrefix(strings.TrimPrefix(version, "@"), "v")
		major, _, _ := strings.Cut(version, ".")
		if n, err := strconv.Atoi(major); err == nil && n < 25 {
			continue
		}
		return true
	}
	return false
}

// ensureNodeRuntimeDeps installs libatomic.so.1 before mise installs Node on
// Linux. Node 26 (mise's node@lts since 2026) links against it, and fresh
// Debian 12 and Ubuntu 24.04 cloud images do not ship it, so node fails to
// start and every npm: tool installed with it fails too (#703).
func (m *MisePlugin) ensureNodeRuntimeDeps(ctx context.Context, specs []string) error {
	if targetOS != "linux" || !needsNode(specs) || libatomicInstalled() {
		return nil
	}
	for _, pkg := range libatomicPackages {
		if !sdk.CommandExists(pkg.manager) {
			continue
		}
		m.logger.Printf("Installing libatomic, which Node.js needs on Linux, with %s\n", pkg.manager)
		err := sdk.ExecCommandWithContext(ctx, true, pkg.manager, pkg.args...)
		if err != nil && pkg.manager == "apt-get" {
			// A fresh image can have empty package lists; refresh and retry once.
			if updateErr := sdk.ExecCommandWithContext(ctx, true, "apt-get", "update"); updateErr == nil {
				err = sdk.ExecCommandWithContext(ctx, true, pkg.manager, pkg.args...)
			}
		}
		if err != nil {
			return fmt.Errorf("install libatomic for Node.js: %w; run `sudo %s %s` and retry", err, pkg.manager, strings.Join(pkg.args, " "))
		}
		return nil
	}
	m.logger.Warning("libatomic.so.1 was not found and no supported package manager is available; Node.js may fail to start until libatomic is installed")
	return nil
}
