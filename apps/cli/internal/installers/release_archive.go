package installers

import (
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"

	"github.com/jameswlane/devex/apps/cli/internal/log"
	"github.com/jameswlane/devex/apps/cli/internal/types"
	"github.com/jameswlane/devex/apps/cli/internal/utils"
)

// releaseArchiveSuffixes are the tarball formats a curlpipe download_url may
// name instead of an install script.
var releaseArchiveSuffixes = []string{".tar.gz", ".tgz", ".tar.xz", ".txz", ".tar.bz2", ".tbz2", ".tar"}

// IsReleaseArchive reports whether a curlpipe download_url names a release
// tarball rather than an install script.
func IsReleaseArchive(downloadURL string) bool {
	parsed, err := url.Parse(downloadURL)
	if err != nil {
		return false
	}
	name := strings.ToLower(path.Base(parsed.Path))
	for _, suffix := range releaseArchiveSuffixes {
		if strings.HasSuffix(name, suffix) {
			return true
		}
	}
	return false
}

// Seams for tests.
var (
	runArchiveCommand = func(ctx context.Context, name string, args ...string) (string, error) {
		output, err := exec.CommandContext(ctx, name, args...).CombinedOutput()
		return string(output), err
	}
	runSystemCommand = func(ctx context.Context, name string, args ...string) (string, error) {
		return utils.ExecOperationWithContext(ctx, sdk.OperationPrivilege{Scope: sdk.ScopeSystem}, name, args...)
	}
)

// InstallReleaseArchive downloads and verifies a release tarball, then
// installs it into the app's destination. When the archive holds
// installed_binary at its top level, only that executable is installed into
// destination (for example /usr/local/bin). Otherwise the whole archive is
// extracted into destination (for example /opt), keeping its runtime files
// next to the binary.
func InstallReleaseArchive(ctx context.Context, app types.AppConfig) error {
	if app.InstallDir == "" || !filepath.IsAbs(app.InstallDir) {
		return fmt.Errorf("release archive for %s needs an absolute destination, got %q", app.Name, app.InstallDir)
	}
	if strings.ContainsAny(app.InstalledBinary, `/\`) || app.InstalledBinary == ".." {
		return fmt.Errorf("installed_binary for %s must be a file name, got %q", app.Name, app.InstalledBinary)
	}
	parsed, err := url.Parse(app.DownloadURL)
	if err != nil || parsed.Scheme != "https" {
		return fmt.Errorf("release archive URL for %s must use https: %q", app.Name, app.DownloadURL)
	}

	workDir, err := os.MkdirTemp("", "devex-archive-*")
	if err != nil {
		return fmt.Errorf("failed to create archive work directory: %w", err)
	}
	defer func() { _ = os.RemoveAll(workDir) }()
	archive := filepath.Join(workDir, path.Base(parsed.Path))

	verification, cleanup, err := PrepareArtifact(ctx, app.DownloadURL, app.Verification)
	if err != nil {
		return err
	}
	defer cleanup()
	if verification.CachedPath != "" {
		// PrepareArtifact made a private copy for this install; use it in place.
		archive = verification.CachedPath
	} else if output, err := runArchiveCommand(ctx, "curl", "-fsSL", "-o", archive, "--", app.DownloadURL); err != nil {
		return fmt.Errorf("failed to download %s: %w: %s", app.DownloadURL, err, strings.TrimSpace(output))
	}
	verified, err := sdk.VerifyArtifact(ctx, archive, verification)
	if err != nil {
		return fmt.Errorf("download integrity verification failed: %w", err)
	}
	log.Info("Download verification", "checksum", verified.ChecksumVerified, "signature", verified.SignatureVerified)

	listing, err := runArchiveCommand(ctx, "tar", "-tf", archive)
	if err != nil {
		return fmt.Errorf("failed to read archive %s: %w: %s", path.Base(parsed.Path), err, strings.TrimSpace(listing))
	}
	binaryEntry := ""
	for _, entry := range strings.Split(strings.TrimSpace(listing), "\n") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		clean := path.Clean(entry)
		if path.IsAbs(entry) || clean == ".." || strings.HasPrefix(clean, "../") {
			return fmt.Errorf("archive %s contains an unsafe path %q", path.Base(parsed.Path), entry)
		}
		if app.InstalledBinary != "" && clean == app.InstalledBinary {
			binaryEntry = entry
		}
	}

	if binaryEntry != "" {
		extractDir := filepath.Join(workDir, "extract")
		if err := os.Mkdir(extractDir, 0o700); err != nil {
			return err
		}
		if output, err := runArchiveCommand(ctx, "tar", "-xf", archive, "-C", extractDir, "--", binaryEntry); err != nil {
			return fmt.Errorf("failed to extract %s: %w: %s", app.InstalledBinary, err, strings.TrimSpace(output))
		}
		binary := filepath.Join(extractDir, app.InstalledBinary)
		if info, err := os.Lstat(binary); err != nil || !info.Mode().IsRegular() {
			return fmt.Errorf("archive entry %s is not a regular file", app.InstalledBinary)
		}
		log.Info("Installing release binary", "binary", app.InstalledBinary, "destination", app.InstallDir)
		if output, err := runSystemCommand(ctx, "install", "-D", "-m", "0755", binary, filepath.Join(app.InstallDir, app.InstalledBinary)); err != nil {
			return fmt.Errorf("failed to install %s into %s: %w: %s", app.InstalledBinary, app.InstallDir, err, strings.TrimSpace(output))
		}
		return nil
	}

	log.Info("Extracting release archive", "archive", path.Base(parsed.Path), "destination", app.InstallDir)
	if output, err := runSystemCommand(ctx, "mkdir", "-p", app.InstallDir); err != nil {
		return fmt.Errorf("failed to create %s: %w: %s", app.InstallDir, err, strings.TrimSpace(output))
	}
	if output, err := runSystemCommand(ctx, "tar", "--no-same-owner", "-xf", archive, "-C", app.InstallDir); err != nil {
		return fmt.Errorf("failed to extract %s into %s: %w: %s", path.Base(parsed.Path), app.InstallDir, err, strings.TrimSpace(output))
	}
	return nil
}
