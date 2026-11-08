package main

import (
	"context"
	"fmt"
	"net/url"
	"path/filepath"
	"regexp"
	"strings"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
)

var (
	repositoryID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.:-]*$`)
	coprProject  = regexp.MustCompile(`^@?[A-Za-z0-9][A-Za-z0-9_-]*/[A-Za-z0-9][A-Za-z0-9_.-]*$`)
	dnfVersion   = regexp.MustCompile(`(?m)^(?:dnf5 version |dnf version )?([45])\.[0-9]+`)
)

func validHTTPS(value string) bool {
	u, err := url.Parse(value)
	return err == nil && u.Scheme == "https" && u.Hostname() != "" &&
		u.User == nil && u.Fragment == "" && !strings.ContainsAny(value, "\r\n\t ")
}

func (p *DnfPlugin) handleRepository(ctx context.Context, command string, args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("%s requires exactly one argument", command)
	}
	value := args[0]
	switch command {
	case "enable-copr":
		if !coprProject.MatchString(value) {
			return fmt.Errorf("invalid COPR project %q: expected owner/project", value)
		}
		return sdk.ExecCommandWithContext(ctx, true, "dnf", "copr", "enable", "-y", value)
	case "import-key":
		if !validHTTPS(value) && (!filepath.IsAbs(value) || filepath.Clean(value) != value || strings.ContainsAny(value, "\r\n\x00")) {
			return fmt.Errorf("invalid key source: expected HTTPS URL or absolute file path")
		}
		return sdk.ExecCommandWithContext(ctx, true, "rpm", "--import", value)
	case "add-repository":
		u, _ := url.Parse(value)
		if !validHTTPS(value) || !strings.HasSuffix(u.Path, ".repo") {
			return fmt.Errorf("invalid repository source: expected HTTPS .repo URL")
		}
	case "enable-repository":
		if !repositoryID.MatchString(value) {
			return fmt.Errorf("invalid repository ID %q", value)
		}
	}
	// Probe the actual dnf executable, which may be a DNF5 symlink. Do not
	// retry mutations with another syntax after failure.
	output, err := sdk.ExecCommandOutputWithContext(ctx, "dnf", "--version")
	if err != nil {
		return fmt.Errorf("detect DNF version: %w", err)
	}
	match := dnfVersion.FindStringSubmatch(strings.TrimSpace(output))
	if match == nil {
		return fmt.Errorf("unsupported DNF version output %q", strings.TrimSpace(output))
	}
	var native []string
	if command == "add-repository" {
		if match[1] == "5" {
			native = []string{"config-manager", "addrepo", "--from-repofile=" + value}
		} else {
			native = []string{"config-manager", "--add-repo", value}
		}
	} else if match[1] == "5" {
		native = []string{"config-manager", "setopt", value + ".enabled=1"}
	} else {
		native = []string{"config-manager", "--set-enabled", value}
	}
	return sdk.ExecCommandWithContext(ctx, true, "dnf", native...)
}
