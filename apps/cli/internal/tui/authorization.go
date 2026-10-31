package tui

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/jameswlane/devex/apps/cli/internal/types"
	"golang.org/x/term"
)

// ErrSudoAuthorization identifies failures that must never trigger an install fallback.
var ErrSudoAuthorization = errors.New("sudo authorization expired; run `sudo -v` and retry")

// NeedsElevation reports whether selected installation work requires system access.
func NeedsElevation(apps []types.CrossPlatformApp) bool {
	for _, app := range apps {
		cfg := app.GetOSConfig()
		switch cfg.InstallMethod {
		case "apt", "dnf", "yum", "pacman", "zypper", "apk", "emerge", "eopkg", "snap", "xbps", "deb", "rpm":
			return true
		}
		if strings.Contains(cfg.InstallCommand, "sudo ") {
			return true
		}
		for _, hook := range append(append([]types.InstallCommand{}, cfg.PreInstall...), cfg.PostInstall...) {
			if strings.Contains(hook.Command+hook.Shell, "sudo ") || hook.Scope == "system" {
				return true
			}
		}
	}
	return false
}

// AuthorizeInstallation must run with the foreground terminal, before starting a TUI.
// Automation never prompts and requires credentials to have been established already.
func AuthorizeInstallation(ctx context.Context, apps []types.CrossPlatformApp, interactive bool) error {
	if os.Geteuid() == 0 || !NeedsElevation(apps) {
		return nil
	}
	return authorizeSudo(ctx, interactive)
}

func authorizeSudo(ctx context.Context, interactive bool) error {
	args := []string{"-n", "-v"}
	if interactive && term.IsTerminal(int(os.Stdin.Fd())) {
		args = []string{"-v"}
	}
	cmd := exec.CommandContext(ctx, "sudo", args...)
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("sudo authorization failed or cancelled: %w: %w", ErrSudoAuthorization, err)
	}
	return nil
}
