package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"golang.org/x/crypto/ssh"
)

// SSHOptions describes an explicitly requested SSH operation. Generate creates
// an unencrypted Ed25519 key; callers must preview this before setting Apply.
type SSHOptions struct {
	Mode, Path string
	Apply      bool
}

// SSHRunner allows key generation to be tested without invoking OpenSSH.
type SSHRunner func(context.Context, string, ...string) error

func runSSHKeygen(ctx context.Context, name string, args ...string) error {
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	// Never include subprocess output in errors: it could contain key material.
	if err := exec.CommandContext(ctx, name, args...).Run(); err != nil {
		return fmt.Errorf("ssh-keygen failed; check OpenSSH installation and directory permissions: %w", err)
	}
	return nil
}

// SetupSSH selects an existing key or generates a fresh key without replacing
// user files. Existing files are read only, so no backup is necessary.
func SetupSSH(ctx context.Context, opts SSHOptions, out io.Writer, run SSHRunner) error {
	if runtime.GOOS != "linux" {
		return errors.New("SSH setup currently supports Linux only")
	}
	if opts.Mode != "generate" && opts.Mode != "existing" {
		return errors.New("SSH mode must be generate or existing")
	}
	if !filepath.IsAbs(opts.Path) || strings.ContainsAny(opts.Path, "\r\n\x00") {
		return errors.New("SSH key path must be an absolute path")
	}
	dir := filepath.Dir(opts.Path)
	if err := secureSSHDirectory(dir); err != nil {
		return err
	}
	if opts.Mode == "generate" {
		for _, path := range []string{opts.Path, opts.Path + ".pub"} {
			if _, err := os.Lstat(path); !os.IsNotExist(err) {
				return fmt.Errorf("refusing to overwrite SSH key path %s", path)
			}
		}
		if _, err := fmt.Fprintf(out, "Create unencrypted Ed25519 private key %s (0600) and public key %s.pub (0644); no SSH/Git config changes.\n", opts.Path, opts.Path); err != nil {
			return err
		}
		if !opts.Apply {
			_, err := fmt.Fprintln(out, "Preview only; use --apply to consent to an unencrypted key.")
			return err
		}
		// Isolate ssh-keygen from final destinations; link publishes without clobbering
		// even if another process creates a key after the preview.
		stage, err := os.MkdirTemp(dir, ".devex-ssh-")
		if err != nil {
			return fmt.Errorf("create SSH staging directory: %w", err)
		}
		defer func() { _ = os.RemoveAll(stage) }()
		key := filepath.Join(stage, "key")
		if run == nil {
			run = runSSHKeygen
		}
		if err := run(ctx, "ssh-keygen", "-q", "-t", "ed25519", "-N", "", "-f", key); err != nil {
			return err
		}
		if err := os.Chmod(key, 0600); err != nil {
			return err
		}
		if err := os.Chmod(key+".pub", 0644); err != nil {
			return err
		}
		if _, err := readSSHPublic(key); err != nil {
			return err
		}
		if err := os.Link(key, opts.Path); err != nil {
			return fmt.Errorf("publish SSH private key without overwriting: %w", err)
		}
		if err := os.Link(key+".pub", opts.Path+".pub"); err != nil {
			_ = os.Remove(opts.Path)
			return fmt.Errorf("publish SSH public key without overwriting: %w", err)
		}
	}
	pub, err := readSSHPublic(opts.Path)
	if err != nil {
		return err
	}
	if _, err := fmt.Fprintf(out, "Public key (register with your Git host):\n%s\n", pub); err != nil {
		return err
	}
	_, err = fmt.Fprintf(out, "Load explicitly in your current shell: ssh-add %q\nNo agent was started and no remote host was contacted.\n", opts.Path)
	return err
}

func secureSSHDirectory(dir string) error {
	// Reject symlinks in every ancestor rather than following an unexpected path.
	for path := dir; ; path = filepath.Dir(path) {
		info, err := os.Lstat(path)
		if err != nil {
			return fmt.Errorf("SSH directory must exist (create it with mode 0700): %w", err)
		}
		if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
			return errors.New("SSH directory ancestors must be real directories")
		}
		if path == dir && info.Mode().Perm()&0077 != 0 {
			return errors.New("SSH key directory must have mode 0700; adjust it explicitly before continuing")
		}
		if filepath.Dir(path) == path {
			break
		}
	}
	return nil
}

func readSSHPublic(path string) (string, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return "", fmt.Errorf("inspect SSH private key: %w", err)
	}
	if !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 {
		return "", errors.New("SSH private key must be a regular file with mode 0600 or stricter")
	}
	info, err = os.Lstat(path + ".pub")
	if err != nil {
		return "", fmt.Errorf("SSH public key is required beside the private key: %w", err)
	}
	if !info.Mode().IsRegular() || info.Size() > 16384 {
		return "", errors.New("SSH public key must be a small regular file")
	}
	data, err := os.ReadFile(path + ".pub")
	if err != nil {
		return "", err
	}
	fields := strings.Fields(string(data))
	if len(fields) < 2 || (!strings.HasPrefix(fields[0], "ssh-") && !strings.HasPrefix(fields[0], "ecdsa-sha2-")) || strings.ContainsAny(strings.TrimSpace(string(data)), "\r\n") {
		return "", errors.New("invalid SSH public key; private key material is never displayed")
	}
	if _, _, _, rest, err := ssh.ParseAuthorizedKey(data); err != nil || len(rest) != 0 {
		return "", errors.New("invalid SSH public key encoding")
	}
	// Omit comments: only emit the validated public key fields.
	return fields[0] + " " + fields[1], nil
}

func (p *GitPlugin) HandleSSH(ctx context.Context, args []string) error {
	opts := SSHOptions{}
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--mode", "--path":
			if i+1 >= len(args) {
				return errors.New("SSH --mode and --path require values")
			}
			flag := args[i]
			i++
			if flag == "--mode" {
				opts.Mode = args[i]
			} else {
				opts.Path = args[i]
			}
		case "--apply":
			opts.Apply = true
		default:
			return fmt.Errorf("unknown SSH option: %s", args[i])
		}
	}
	return SetupSSH(ctx, opts, os.Stdout, nil)
}
