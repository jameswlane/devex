package main

import (
	"context"
	"errors"
	"fmt"
	"os/exec"
	"path"
	"regexp"
	"strings"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
)

var rpmPackageName = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9+_.:-]*$`)

// queryFailure wraps err from a native query, adding its captured stderr so
// the diagnostic is not lost.
func queryFailure(context string, err error) error {
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		if stderr := strings.TrimSpace(string(exitErr.Stderr)); stderr != "" {
			return fmt.Errorf("%s: %s: %w", context, stderr, err)
		}
	}
	return fmt.Errorf("%s: %w", context, err)
}

// absent reports whether err is RPM's plain "not installed" result: exit
// status 1 with nothing on stderr.
func absent(err error) bool {
	var exitErr *exec.ExitError
	return errors.As(err, &exitErr) && exitErr.ExitCode() == 1 && len(exitErr.Stderr) == 0
}

func (p *YumPlugin) handleIsInstalled(ctx context.Context, args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("no packages specified")
	}
	for _, pkg := range args {
		if !rpmPackageName.MatchString(pkg) {
			return fmt.Errorf("invalid RPM package name %q", pkg)
		}
	}
	var missing []string
	for _, pkg := range args {
		_, err := sdk.ExecCommandOutputWithContext(ctx, "rpm", "-q", "--", pkg)
		if err == nil {
			continue
		}
		if absent(err) {
			missing = append(missing, pkg)
			continue
		}
		return queryFailure(fmt.Sprintf("RPM query failed for %q", pkg), err)
	}
	if len(missing) > 0 {
		return &sdk.NotInstalledError{Packages: missing}
	}
	return nil
}

// requireInstalledMatch checks the local RPM database for spec the way
// "yum remove" resolves it: a name glob such as "git*" against installed
// packages, otherwise the exact name, then a provided capability.
func requireInstalledMatch(ctx context.Context, spec string) error {
	if strings.HasPrefix(spec, "-") {
		return fmt.Errorf("cannot remove package %q: package specifications must not start with '-'", spec)
	}
	failure := fmt.Sprintf("cannot remove package %q: RPM query failed", spec)
	if strings.ContainsAny(spec, "*?[") {
		// Match globs here: RPM 4.11's "rpm -qa <pattern>" treats "?" as a
		// regular-expression operator, unlike YUM.
		output, err := sdk.ExecCommandOutputWithContext(ctx, "rpm", "-qa", "--queryformat", "%{NAME} %{VERSION} %{RELEASE} %{ARCH}\\n")
		if err != nil {
			return queryFailure(failure, err)
		}
		if _, err := path.Match(spec, ""); err != nil {
			return fmt.Errorf("cannot remove package %q: invalid glob: %w", spec, err)
		}
		for _, line := range strings.Split(output, "\n") {
			fields := strings.Fields(line)
			if len(fields) != 4 {
				continue
			}
			name, evr, arch := fields[0], fields[1]+"-"+fields[2], fields[3]
			for _, candidate := range []string{name, name + "." + arch, name + "-" + evr, name + "-" + evr + "." + arch} {
				if matched, _ := path.Match(spec, candidate); matched {
					return nil
				}
			}
		}
		return fmt.Errorf("cannot remove package %q: no installed package matches", spec)
	}
	for _, selector := range [][]string{{"-q", "--", spec}, {"-q", "--whatprovides", "--", spec}} {
		_, err := sdk.ExecCommandOutputWithContext(ctx, "rpm", selector...)
		if err == nil {
			return nil
		}
		if !absent(err) {
			return queryFailure(failure, err)
		}
	}
	return fmt.Errorf("cannot remove package %q: no installed package matches", spec)
}
