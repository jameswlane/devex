package selfupdate

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

const maxBinarySize = 256 << 20

type Updater struct {
	Client     *Client
	Executable func() (string, error)
	OS, Arch   string
	// VerifySignature is nil when cosign is unavailable. A verification failure
	// always aborts before any installed binary is changed.
	VerifySignature func(context.Context, []byte, []byte, string) error
	Managed         func(context.Context, string) bool
}

func NewUpdater(client *Client) *Updater {
	u := &Updater{Client: client, Executable: os.Executable, OS: runtime.GOOS, Arch: runtime.GOARCH, Managed: packageManaged}
	if cosign, err := exec.LookPath("cosign"); err == nil {
		u.VerifySignature = func(ctx context.Context, checksums, bundle []byte, dir string) error {
			checksumsPath := filepath.Join(dir, "devex_checksums.txt")
			bundlePath := checksumsPath + ".sigstore.json"
			if err := os.WriteFile(checksumsPath, checksums, 0600); err != nil {
				return err
			}
			if err := os.WriteFile(bundlePath, bundle, 0600); err != nil {
				return err
			}
			cmd := exec.CommandContext(ctx, cosign, "verify-blob", "--bundle="+bundlePath,
				`--certificate-identity-regexp=^https://github\.com/jameswlane/devex/\.github/workflows/`,
				"--certificate-oidc-issuer=https://token.actions.githubusercontent.com", checksumsPath)
			if output, err := cmd.CombinedOutput(); err != nil {
				return fmt.Errorf("cosign verification failed: %w: %s", err, strings.TrimSpace(string(output)))
			}
			return nil
		}
	}
	return u
}

func packageManaged(ctx context.Context, path string) bool {
	normalized := filepath.ToSlash(path)
	for _, marker := range []string{"/Cellar/", "/Homebrew/", "/nix/store/", "/scoop/", "/chocolatey/", "/usr/bin/", "/bin/"} {
		if (marker != "/bin/" && strings.Contains(normalized, marker)) || (marker == "/bin/" && filepath.Dir(path) == "/bin") {
			return true
		}
	}
	for _, query := range [][]string{{"dpkg-query", "-S", path}, {"rpm", "-qf", path}, {"pacman", "-Qo", path}, {"apk", "info", "--who-owns", path}} {
		tool, err := exec.LookPath(query[0])
		if err == nil && exec.CommandContext(ctx, tool, query[1:]...).Run() == nil {
			return true
		}
	}
	return false
}

func (u *Updater) target(ctx context.Context) (string, error) {
	path, err := u.Executable()
	if err != nil {
		return "", fmt.Errorf("locate DevEx executable: %w", err)
	}
	path, err = filepath.EvalSymlinks(path)
	if err != nil {
		return "", fmt.Errorf("resolve DevEx executable: %w", err)
	}
	if u.Managed != nil && u.Managed(ctx, path) {
		return "", fmt.Errorf("DevEx at %s is managed by a package manager; update it using that package manager", path)
	}
	dir := filepath.Dir(path)
	info, err := os.Stat(dir)
	if err != nil {
		return "", fmt.Errorf("inspect install directory: %w", err)
	}
	if info.Mode().Perm()&0222 == 0 {
		return "", fmt.Errorf("install directory %s is not writable; reinstall DevEx in a user-owned directory", dir)
	}
	probe, err := os.CreateTemp(dir, ".devex-write-test-*")
	if err != nil {
		return "", fmt.Errorf("install directory %s is not writable; reinstall DevEx in a user-owned directory: %w", dir, err)
	}
	_ = probe.Close()
	_ = os.Remove(probe.Name())
	return path, nil
}

func (u *Updater) Install(ctx context.Context, release Release) error {
	path, err := u.target(ctx)
	if err != nil {
		return err
	}
	name := fmt.Sprintf("devex_%s_%s_%s.tar.gz", strings.TrimPrefix(release.Tag, "v"), u.OS, u.Arch)
	archive, err := u.Client.asset(ctx, release, name, maxBinarySize)
	if err != nil {
		return err
	}
	checksums, err := u.Client.asset(ctx, release, "devex_checksums.txt", 8<<20)
	if err != nil {
		return err
	}
	dir, err := os.MkdirTemp(filepath.Dir(path), ".devex-update-*")
	if err != nil {
		return fmt.Errorf("create update workspace: %w", err)
	}
	defer func() { _ = os.RemoveAll(dir) }()
	if u.VerifySignature != nil {
		bundle, err := u.Client.asset(ctx, release, "devex_checksums.txt.sigstore.json", 8<<20)
		if err != nil {
			return err
		}
		if err := u.VerifySignature(ctx, checksums, bundle, dir); err != nil {
			return fmt.Errorf("verify release signature (installation unchanged): %w", err)
		}
	}
	sum := sha256.Sum256(archive)
	expected := ""
	for _, line := range strings.Split(string(checksums), "\n") {
		fields := strings.Fields(line)
		if len(fields) == 2 && strings.TrimPrefix(fields[1], "*") == name {
			expected = fields[0]
			break
		}
	}
	if expected == "" || !strings.EqualFold(expected, hex.EncodeToString(sum[:])) {
		return fmt.Errorf("checksum mismatch for %s; installation unchanged, retry or report this release", name)
	}
	binary, err := extractBinary(archive, u.OS)
	if err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return u.replace(path, binary)
}

func extractBinary(archive []byte, goos string) ([]byte, error) {
	gz, err := gzip.NewReader(bytes.NewReader(archive))
	if err != nil {
		return nil, fmt.Errorf("open release archive: %w", err)
	}
	defer func() { _ = gz.Close() }()
	reader := tar.NewReader(gz)
	name := "devex"
	if goos == "windows" {
		name += ".exe"
	}
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("read release archive: %w", err)
		}
		// Read only the binary; never extract archive paths to the filesystem.
		if header.Name != name && header.Name != "bin/"+name {
			continue
		}
		if header.Typeflag != tar.TypeReg || header.Size <= 0 || header.Size > maxBinarySize {
			return nil, fmt.Errorf("release contains an invalid DevEx binary")
		}
		data, err := io.ReadAll(io.LimitReader(reader, maxBinarySize+1))
		if err != nil {
			return nil, fmt.Errorf("extract DevEx binary: %w", err)
		}
		return data, nil
	}
	return nil, fmt.Errorf("release archive is missing %s; reinstall from a valid release", name)
}

func stage(path string, data []byte) (string, error) {
	file, err := os.CreateTemp(filepath.Dir(path), ".devex-binary-*")
	if err != nil {
		return "", err
	}
	name := file.Name()
	defer func() { _ = file.Close() }()
	if _, err = file.Write(data); err == nil {
		err = file.Chmod(0755)
	}
	if err == nil {
		err = file.Sync()
	}
	if closeErr := file.Close(); err == nil {
		err = closeErr
	}
	if err != nil {
		_ = os.Remove(name)
		return "", err
	}
	return name, nil
}

func (u *Updater) replace(path string, binary []byte) error {
	next, err := stage(path, binary)
	if err != nil {
		return fmt.Errorf("stage DevEx binary: %w", err)
	}
	defer func() { _ = os.Remove(next) }() // #nosec G703 -- stage returns only an os.CreateTemp path beside the resolved executable.
	previous := filepath.Join(filepath.Dir(path), "devex.previous")
	if u.OS == "windows" {
		if err := os.Remove(previous); err != nil && !os.IsNotExist(err) {
			return fmt.Errorf("remove old rollback binary: %w", err)
		}
		if err := os.Rename(path, previous); err != nil {
			return fmt.Errorf("preserve running executable: %w", err)
		}
	} else {
		old, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read previous DevEx binary: %w", err)
		}
		backup, err := stage(path, old)
		if err != nil {
			return fmt.Errorf("stage rollback binary: %w", err)
		}
		defer func() { _ = os.Remove(backup) }()            // #nosec G703 -- stage returns only an os.CreateTemp path beside the resolved executable.
		if err := os.Rename(backup, previous); err != nil { // #nosec G703 -- backup is created by os.CreateTemp; previous is a fixed sibling name.
			return fmt.Errorf("preserve rollback binary: %w", err)
		}
	}
	if err := os.Rename(next, path); err != nil { // #nosec G703 -- next is created by os.CreateTemp and path is the resolved executable, never an archive path.
		if u.OS == "windows" {
			if restoreErr := os.Rename(previous, path); restoreErr != nil {
				return fmt.Errorf("install failed (%w), restore %s manually: %w", err, previous, restoreErr)
			}
		}
		return fmt.Errorf("replace DevEx binary; previous version preserved at %s: %w", previous, err)
	}
	return nil
}

func (u *Updater) Rollback(ctx context.Context) error {
	path, err := u.target(ctx)
	if err != nil {
		return err
	}
	binary, err := os.ReadFile(filepath.Join(filepath.Dir(path), "devex.previous"))
	if err != nil {
		return fmt.Errorf("read devex.previous; rollback requires a successful earlier self-update: %w", err)
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	return u.replace(path, binary)
}
