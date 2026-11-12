// Package remoterun runs a validated DevEx install plan on another host over
// SSH, using the system OpenSSH client so the user's SSH configuration, agent
// and keys apply.
//
// Host keys are verified strictly: an unknown host is refused unless the
// caller explicitly accepts new keys. SSH runs in batch mode, so it never
// prompts for a password. The remote plan carries a digest of everything it
// would execute; the install sends the confirmed digest back and the remote
// DevEx re-plans and verifies it in the same process that installs, refusing
// when anything changed. The install runs unattended and its output is
// streamed back.
package remoterun

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/jameswlane/devex/apps/cli/internal/agentapi"
)

// Target is an SSH destination and its authentication inputs.
type Target struct {
	// Host is [user@]host. It must not start with '-'.
	Host string
	Port int
	// Identity is a private key file; empty uses the agent and ssh config.
	Identity string
	// KnownHosts replaces the user's known_hosts file when set.
	KnownHosts string
	// AcceptNewHostKey trusts and records an unknown host key on first use.
	// Changed keys are always refused.
	AcceptNewHostKey bool
	// RemoteDevex is the devex command on the remote host.
	RemoteDevex string
	// SSH is the local ssh executable.
	SSH string
}

var (
	hostPattern = regexp.MustCompile(`^([A-Za-z0-9._-]+@)?[A-Za-z0-9.:_-]+$`)
	appPattern  = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9 ._+-]*$`)
	devexPath   = regexp.MustCompile(`^[A-Za-z0-9._/~-]+$`)
	digestValue = regexp.MustCompile(`^sha256:[0-9a-f]{64}$`)
)

// ErrPlanChanged reports that the remote plan no longer matches the confirmed one.
var ErrPlanChanged = errors.New("the plan changed after it was confirmed; nothing was installed; review it again")

// Validate checks the target.
func (t Target) Validate() error {
	switch {
	case t.Host == "" || strings.HasPrefix(t.Host, "-") || !hostPattern.MatchString(t.Host):
		return fmt.Errorf("invalid SSH target %q: use [user@]host", t.Host)
	case t.Port < 0 || t.Port > 65535:
		return fmt.Errorf("invalid SSH port %d", t.Port)
	case t.RemoteDevex != "" && !devexPath.MatchString(t.RemoteDevex):
		return fmt.Errorf("invalid remote devex command %q", t.RemoteDevex)
	case strings.HasPrefix(t.Identity, "-") || strings.HasPrefix(t.KnownHosts, "-"):
		return errors.New("identity and known-hosts paths must not start with '-'")
	}
	return nil
}

// ValidateApps checks application names before they are sent to a shell.
func ValidateApps(apps []string) error {
	if len(apps) == 0 {
		return errors.New("name at least one application; a remote install never uses the remote host's defaults implicitly")
	}
	for _, app := range apps {
		if !appPattern.MatchString(app) {
			return fmt.Errorf("invalid application name %q", app)
		}
	}
	return nil
}

func (t Target) sshArgs() []string {
	checking := "yes"
	if t.AcceptNewHostKey {
		checking = "accept-new"
	}
	args := []string{"-T", "-o", "BatchMode=yes", "-o", "StrictHostKeyChecking=" + checking, "-o", "ConnectTimeout=15", "-o", "ServerAliveInterval=15", "-o", "ServerAliveCountMax=4"}
	if t.KnownHosts != "" {
		args = append(args, "-o", "UserKnownHostsFile="+t.KnownHosts)
	}
	if t.Identity != "" {
		args = append(args, "-o", "IdentitiesOnly=yes", "-i", t.Identity)
	}
	if t.Port > 0 {
		args = append(args, "-p", strconv.Itoa(t.Port))
	}
	return append(args, "--", t.Host)
}

// quote single-quotes a word for the remote POSIX shell.
func quote(word string) string {
	return "'" + strings.ReplaceAll(word, "'", `'\''`) + "'"
}

func (t Target) remoteCommand(args ...string) string {
	devex := t.RemoteDevex
	if devex == "" {
		devex = "devex"
	}
	words := make([]string, 0, len(args)+2)
	// CI=true selects DevEx's unattended mode: no prompts, sudo -n.
	words = append(words, "CI=true", devex)
	for _, arg := range args {
		words = append(words, quote(arg))
	}
	return strings.Join(words, " ")
}

func (t Target) command(ctx context.Context, args ...string) *exec.Cmd {
	ssh := t.SSH
	if ssh == "" {
		ssh = "ssh"
	}
	command := exec.CommandContext(ctx, ssh, append(t.sshArgs(), t.remoteCommand(args...))...)
	// After cancellation, stop waiting for output pipes that a lingering
	// child process holds open.
	command.WaitDelay = 3 * time.Second
	return command
}

// Explain turns an ssh failure into guidance.
func Explain(err error, stderr string) error {
	var exit *exec.ExitError
	switch {
	case strings.Contains(stderr, "Host key verification failed"), strings.Contains(stderr, "REMOTE HOST IDENTIFICATION HAS CHANGED"):
		return fmt.Errorf("host key verification failed; check the host's key fingerprint and add it to known_hosts (ssh-keyscan) or pass --accept-new-host-key for a first connection: %s", lastLine(stderr))
	case strings.Contains(stderr, "Permission denied"):
		return fmt.Errorf("SSH authentication failed; load a key into your agent or pass --identity: %s", lastLine(stderr))
	case errors.As(err, &exit) && exit.ExitCode() == 255:
		return fmt.Errorf("SSH connection failed: %s", lastLine(stderr))
	case errors.As(err, &exit) && exit.ExitCode() == 127:
		return errors.New("devex is not installed on the remote host or not on its PATH; install it there or pass --remote-devex")
	}
	return err
}

func lastLine(text string) string {
	lines := strings.Split(strings.TrimSpace(text), "\n")
	return strings.TrimSpace(lines[len(lines)-1])
}

// Version returns the remote devex version line.
func (t Target) Version(ctx context.Context) (string, error) {
	var stdout, stderr bytes.Buffer
	command := t.command(ctx, "--version")
	command.Stdout, command.Stderr = &stdout, &stderr
	if err := command.Run(); err != nil {
		return "", Explain(err, stderr.String())
	}
	return strings.TrimSpace(stdout.String()), nil
}

// PlannedApp is a remote plan entry, as shown for confirmation.
type PlannedApp struct {
	Name         string   `json:"name"`
	Method       string   `json:"method"`
	Dependencies []string `json:"dependencies"`
	Effects      []string `json:"effects"`
	Privileges   string   `json:"privileges"`
}

// Plan is the remote install plan. Digest binds it to the remote definitions
// it would execute; Install sends it back to be verified.
type Plan struct {
	Apps            []PlannedApp `json:"apps"`
	DependencyOrder []string     `json:"dependency_order"`
	Warnings        []string     `json:"warnings"`
	// Roots are the applications whose install lifecycles run; other planned
	// dependencies are left to their package manager.
	Roots  []string `json:"roots"`
	Policy struct {
		RollbackOnFailure bool `json:"rollback_on_failure"`
	} `json:"policy"`
	Digest string `json:"digest"`
}

// RemotePlan asks the remote DevEx to plan apps without executing anything.
func (t Target) RemotePlan(ctx context.Context, apps []string) (Plan, error) {
	var stdout, stderr bytes.Buffer
	command := t.command(ctx, append([]string{"install", "--dry-run", "--format", "json"}, apps...)...)
	command.Stdout, command.Stderr = &stdout, &stderr
	err := command.Run()
	var plan Plan
	if decodeErr := json.Unmarshal(stdout.Bytes(), &plan); decodeErr != nil || err != nil {
		if err == nil {
			err = decodeErr
		}
		var apiError struct {
			Error *struct{ Message string } `json:"error"`
		}
		if json.Unmarshal(stdout.Bytes(), &apiError) == nil && apiError.Error != nil {
			return Plan{}, fmt.Errorf("the remote host could not plan the install: %s", apiError.Error.Message)
		}
		return Plan{}, Explain(err, stderr.String())
	}
	if !digestValue.MatchString(plan.Digest) {
		return Plan{}, errors.New("the remote DevEx did not return a plan digest, so a confirmed plan cannot be verified there; upgrade DevEx on the remote host")
	}
	return plan, nil
}

// Install runs the install remotely, streaming its output. The remote DevEx
// installs only if its plan still has the confirmed digest; otherwise
// ErrPlanChanged is returned. Another nonzero remote exit is returned as an
// error naming the exit status.
func (t Target) Install(ctx context.Context, apps []string, digest string, stdout, stderr io.Writer) error {
	if !digestValue.MatchString(digest) {
		return fmt.Errorf("invalid plan digest %q", digest)
	}
	var captured bytes.Buffer
	command := t.command(ctx, append([]string{"install", "--non-interactive", "--expect-plan", digest}, apps...)...)
	command.Stdout = stdout
	command.Stderr = io.MultiWriter(stderr, &captured)
	err := command.Run()
	if ctx.Err() != nil {
		return fmt.Errorf("cancelled; the SSH session was closed and the remote install stops at its next output, but a package operation already running may finish: %w", ctx.Err())
	}
	if err == nil {
		return nil
	}
	if strings.Contains(captured.String(), agentapi.PlanChangedMessage) {
		return ErrPlanChanged
	}
	if strings.Contains(captured.String(), "unknown flag: --expect-plan") {
		return errors.New("the remote DevEx cannot verify a confirmed plan (no --expect-plan); upgrade DevEx on the remote host")
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) && exit.ExitCode() != 255 && exit.ExitCode() != 127 {
		return fmt.Errorf("the remote install failed with exit status %d", exit.ExitCode())
	}
	return Explain(err, captured.String())
}
