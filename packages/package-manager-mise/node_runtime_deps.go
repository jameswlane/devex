package main

import (
	"bufio"
	"bytes"
	"context"
	"debug/elf"
	"fmt"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
)

// targetOS is the platform the plugin installs for; tests override it.
var targetOS = runtime.GOOS

// libatomicPatterns are the glob patterns searched for libatomic.so.1. They
// cover the merged-/usr and multiarch layouts of Debian, Ubuntu, Fedora,
// openSUSE and Arch. They are checked first; the loader cache is the fallback
// for libraries installed elsewhere.
var libatomicPatterns = []string{
	"/lib/*-linux-gnu/libatomic.so.1",
	"/usr/lib/*-linux-gnu/libatomic.so.1",
	"/lib64/libatomic.so.1",
	"/usr/lib64/libatomic.so.1",
	"/lib/libatomic.so.1",
	"/usr/lib/libatomic.so.1",
}

// ldconfigPaths are where the loader-cache tool lives. It is called by absolute
// path because /sbin is not on a Debian user's PATH.
var ldconfigPaths = []string{"/sbin/ldconfig", "/usr/sbin/ldconfig"}

// nativeELFMachine is the ELF machine Node runs as on this host. A library
// built for another architecture (libatomic1:i386 on amd64, the 32-bit copy
// in Fedora's /usr/lib) does not satisfy Node. Tests override it.
var nativeELFMachine = map[string]elf.Machine{
	"amd64":   elf.EM_X86_64,
	"386":     elf.EM_386,
	"arm64":   elf.EM_AARCH64,
	"arm":     elf.EM_ARM,
	"ppc64le": elf.EM_PPC64,
	"ppc64":   elf.EM_PPC64,
	"s390x":   elf.EM_S390,
	"riscv64": elf.EM_RISCV,
	"loong64": elf.EM_LOONGARCH,
}[runtime.GOARCH]

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

func libatomicInstalled(ctx context.Context) bool {
	for _, pattern := range libatomicPatterns {
		matches, _ := filepath.Glob(pattern)
		for _, path := range matches {
			if nativeLibrary(path) {
				return true
			}
		}
	}
	for _, path := range loaderCacheLibatomic(ctx) {
		if nativeLibrary(path) {
			return true
		}
	}
	return false
}

// nativeLibrary reports whether path resolves to an ELF file for this host's
// architecture. A dangling symlink or a foreign-architecture copy does not count.
func nativeLibrary(path string) bool {
	f, err := elf.Open(path)
	if err != nil {
		return false
	}
	defer func() { _ = f.Close() }()
	if strconv.IntSize == 64 && f.Class != elf.ELFCLASS64 {
		return false // for example Ubuntu's x32 libx32atomic1, which is EM_X86_64
	}
	return nativeELFMachine == elf.EM_NONE || f.Machine == nativeELFMachine
}

// loaderCacheLibatomic lists the libatomic.so.1 paths in the dynamic loader
// cache, which also covers directories outside libatomicPatterns (such as
// /opt prefixes registered in /etc/ld.so.conf.d).
func loaderCacheLibatomic(ctx context.Context) []string {
	for _, ldconfig := range ldconfigPaths {
		cctx, cancel := context.WithTimeout(ctx, 10*time.Second)
		out, err := exec.CommandContext(cctx, ldconfig, "-p").Output()
		cancel()
		if err != nil {
			continue
		}
		var paths []string
		scanner := bufio.NewScanner(bytes.NewReader(out))
		for scanner.Scan() {
			line := strings.TrimSpace(scanner.Text())
			if !strings.HasPrefix(line, "libatomic.so.1 ") {
				continue
			}
			if _, path, ok := strings.Cut(line, "=> "); ok {
				paths = append(paths, strings.TrimSpace(path))
			}
		}
		return paths
	}
	return nil
}

// needsNode reports whether any spec installs a mise node runtime that can be
// Node 25 or later. Explicit pins below 25 (node@24) do not link libatomic, and
// node@system uses the Node already on the machine. mise accepts nodejs as an
// alias for node.
func needsNode(specs []string) bool {
	for _, spec := range specs {
		name := toolName(spec)
		if name != "node" && name != "nodejs" {
			continue
		}
		version := strings.TrimPrefix(spec[len(name):], "@")
		if version == "system" {
			continue
		}
		version = strings.TrimPrefix(version, "v")
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
	if targetOS != "linux" || !needsNode(specs) || libatomicInstalled(ctx) {
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
		if !libatomicInstalled(ctx) {
			// The package can already be installed with its library broken, such as a dangling symlink.
			m.logger.Warning("libatomic.so.1 is still missing after installing it with %s; Node.js may fail to start until the package is reinstalled", pkg.manager)
		}
		return nil
	}
	m.logger.Warning("libatomic.so.1 was not found and no supported package manager is available; Node.js may fail to start until libatomic is installed")
	return nil
}
