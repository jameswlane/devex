package skillpacks

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

const gitTimeout = 60 * time.Second

// Fetcher resolves Git refs to commit SHAs and materializes a pinned SHA into a
// temporary directory.
type Fetcher interface {
	ResolveRef(ctx context.Context, source, ref string) (string, error)
	Fetch(ctx context.Context, source, sha string) (dir string, cleanup func(), err error)
}

// GitFetcher implements Fetcher using the host git CLI without executing
// repository hooks or prompting on a terminal.
type GitFetcher struct {
	LookPath      func(file string) (string, error)
	RewriteSource func(source string) string
}

// DefaultFetcher returns a GitFetcher wired to the host git binary.
func DefaultFetcher() *GitFetcher {
	return &GitFetcher{
		LookPath: exec.LookPath,
	}
}

func (f *GitFetcher) gitBinary() (string, error) {
	lookPath := f.LookPath
	if lookPath == nil {
		lookPath = exec.LookPath
	}
	bin, err := lookPath("git")
	if err != nil {
		return "", fmt.Errorf("git is required to fetch agent skill packs; install git and ensure it is on PATH: %w", err)
	}
	return bin, nil
}

func (f *GitFetcher) effectiveSource(source string) string {
	if f != nil && f.RewriteSource != nil {
		return f.RewriteSource(source)
	}
	return source
}

// ResolveRef resolves ref on source to a full 40-character lowercase SHA using
// read-only `git ls-remote`. If ref is already a full SHA, it is returned
// normalized without a ref lookup (Fetch verifies the commit exists).
func (f *GitFetcher) ResolveRef(ctx context.Context, source, ref string) (string, error) {
	bin, err := f.gitBinary()
	if err != nil {
		return "", err
	}
	ref = strings.TrimSpace(ref)
	if IsFullSHA(ref) {
		return strings.ToLower(ref), nil
	}
	src := f.effectiveSource(source)
	args := []string{"-c", "core.hooksPath=" + os.DevNull, "ls-remote", src}
	if ref == "" || ref == "HEAD" {
		args = append(args, "HEAD")
	} else {
		args = append(args, ref, "refs/heads/"+ref, "refs/tags/"+ref, "refs/tags/"+ref+"^{}")
	}
	out, err := runGit(ctx, bin, "", args...)
	if err != nil {
		return "", fmt.Errorf("resolve ref %q from %s: %w", ref, source, err)
	}
	refs := map[string]string{}
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		if IsFullSHA(fields[0]) {
			refs[fields[1]] = strings.ToLower(fields[0])
		}
	}
	if ref == "" || ref == "HEAD" {
		if sha, ok := refs["HEAD"]; ok {
			return sha, nil
		}
		return "", fmt.Errorf("could not resolve HEAD from %s", source)
	}
	candidates := []string{
		"refs/tags/" + ref + "^{}",
		"refs/tags/" + ref,
		"refs/heads/" + ref,
		ref,
	}
	for _, candidate := range candidates {
		if sha, ok := refs[candidate]; ok {
			return sha, nil
		}
	}
	return "", fmt.Errorf("ref %q not found in %s", ref, source)
}

// Fetch materializes commit sha from source into a newly created temporary
// directory using a shallow, hook-disabled, detached fetch and verifies HEAD
// matches sha exactly.
func (f *GitFetcher) Fetch(ctx context.Context, source, sha string) (string, func(), error) {
	bin, err := f.gitBinary()
	if err != nil {
		return "", nil, err
	}
	sha = strings.ToLower(strings.TrimSpace(sha))
	if !IsFullSHA(sha) {
		return "", nil, fmt.Errorf("commit SHA %q must be 40 hexadecimal characters", sha)
	}
	tmpDir, err := os.MkdirTemp("", "devex-skillpack-*")
	if err != nil {
		return "", nil, fmt.Errorf("create temporary checkout directory: %w", err)
	}
	cleanup := func() {
		_ = os.RemoveAll(tmpDir)
	}
	src := f.effectiveSource(source)
	hooksArg := "core.hooksPath=" + os.DevNull

	steps := [][]string{
		{"-c", hooksArg, "init", "--quiet"},
		{"-c", hooksArg, "remote", "add", "origin", src},
		{"-c", hooksArg, "fetch", "--depth", "1", "--no-tags", "origin", sha},
		{"-c", hooksArg, "checkout", "--quiet", "--detach", "FETCH_HEAD"},
	}
	for _, step := range steps {
		if _, err := runGit(ctx, bin, tmpDir, step...); err != nil {
			cleanup()
			return "", nil, fmt.Errorf("fetch %s@%s: %w", source, ShortSHA(sha), err)
		}
	}
	headOut, err := runGit(ctx, bin, tmpDir, "-c", hooksArg, "rev-parse", "HEAD")
	if err != nil {
		cleanup()
		return "", nil, fmt.Errorf("verify checked-out HEAD for %s@%s: %w", source, ShortSHA(sha), err)
	}
	actual := strings.ToLower(strings.TrimSpace(headOut))
	if actual != sha {
		cleanup()
		return "", nil, fmt.Errorf("checked-out HEAD %s does not match requested SHA %s", actual, sha)
	}
	return tmpDir, cleanup, nil
}

func runGit(ctx context.Context, bin, dir string, args ...string) (string, error) {
	cmdCtx, cancel := context.WithTimeout(ctx, gitTimeout)
	defer cancel()
	cmd := exec.CommandContext(cmdCtx, bin, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	cmd.WaitDelay = time.Second
	cmd.Env = sanitizedGitEnv(os.Environ())
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		errText := strings.TrimSpace(stderr.String())
		if errText != "" {
			return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, errText)
		}
		return "", fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return stdout.String(), nil
}

func sanitizedGitEnv(base []string) []string {
	env := make([]string, 0, len(base)+3)
	for _, entry := range base {
		if strings.HasPrefix(entry, "GIT_TERMINAL_PROMPT=") ||
			strings.HasPrefix(entry, "GIT_CONFIG_NOSYSTEM=") ||
			strings.HasPrefix(entry, "GIT_CONFIG_GLOBAL=") {
			continue
		}
		env = append(env, entry)
	}
	env = append(env,
		"GIT_TERMINAL_PROMPT=0",
		"GIT_CONFIG_NOSYSTEM=1",
		"GIT_CONFIG_GLOBAL="+os.DevNull,
	)
	return env
}
