package sdk

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/user"
	"runtime"
	"strings"
)

// OperationScope declares the authority needed by one operation, not an entire plugin.
type OperationScope string

const (
	ScopeUser   OperationScope = "user"
	ScopeSystem OperationScope = "system"
)

// UserIdentity uses account database identity and the current user's home environment
// only when no elevated or explicit account selection is involved.
type UserIdentity struct {
	Username string `json:"username"`
	UID      string `json:"uid"`
	GID      string `json:"gid,omitempty"`
	HomeDir  string `json:"home_dir"`
}

// OperationPrivilege is additive plugin command metadata and the execution policy.
// User may be omitted to select the original invoking user (including SUDO_USER).
type OperationPrivilege struct {
	Scope OperationScope `json:"scope"`
	User  *UserIdentity  `json:"user,omitempty"`
}

type AuthorizationKind string

const (
	AuthorizationRequired AuthorizationKind = "authorization-required"
	AuthorizationDenied   AuthorizationKind = "authorization-denied"
)

// AuthorizationError distinguishes authorization failures from command failures.
// NativeExitCode is -1 when no platform process ran. Err retains the native error.
type AuthorizationError struct {
	Kind           AuthorizationKind `json:"kind"`
	Reason         string            `json:"reason"`
	NativeExitCode int               `json:"native_exit_code"`
	Err            error             `json:"-"`
}

func (e *AuthorizationError) Error() string { return fmt.Sprintf("%s: %s", e.Kind, e.Reason) }
func (e *AuthorizationError) Unwrap() error { return e.Err }

// NewAuthorizationRequiredError reports that no authorized operation could start.
func NewAuthorizationRequiredError(reason string, err error) *AuthorizationError {
	return &AuthorizationError{Kind: AuthorizationRequired, Reason: reason, NativeExitCode: -1, Err: err}
}

// NewAuthorizationDeniedError retains the platform's status and underlying error.
func NewAuthorizationDeniedError(reason string, nativeExitCode int, err error) *AuthorizationError {
	return &AuthorizationError{Kind: AuthorizationDenied, Reason: reason, NativeExitCode: nativeExitCode, Err: err}
}

func authorizationRequired(reason string, err error) error {
	return NewAuthorizationRequiredError(reason, err)
}

// NativeExitStatus lets native process-handle adapters preserve status without
// manufacturing exec.ExitError values. Their error type implements ExitCode.
type NativeExitStatus interface{ ExitCode() int }

// ExitStatus preserves native command/authorization exit status for platform adapters.
func ExitStatus(err error) int {
	if err == nil {
		return 0
	}
	var auth *AuthorizationError
	if errors.As(err, &auth) && auth.NativeExitCode >= 0 {
		return auth.NativeExitCode
	}
	var exit NativeExitStatus
	if errors.As(err, &exit) {
		return exit.ExitCode()
	}
	return 1
}

// PrivilegeResolver supplies identity facts; injectable functions make policy testable
// without executing privileged commands or changing the test process identity.
type PrivilegeResolver struct {
	Elevated    bool
	CurrentUser func() (*user.User, error)
	LookupUser  func(string) (*user.User, error)
	Getenv      func(string) string
}

// NewPrivilegeResolver can be configured at startup by platform hosts (for example,
// a Windows token-elevation detector). Configure before concurrent operations.
var NewPrivilegeResolver = defaultPrivilegeResolver

// ResolveUser selects identity from the account database, including the account
// behind sudo. Only an implicit, non-elevated current user may override their home.
func (r PrivilegeResolver) ResolveUser(requested *UserIdentity) (*UserIdentity, error) {
	current, err := r.CurrentUser()
	if err != nil {
		return nil, fmt.Errorf("resolve current account: %w", err)
	}
	account := current
	name := ""
	if requested != nil {
		name = requested.Username
	} else if r.Elevated {
		name = r.Getenv("SUDO_USER")
	}
	if name != "" {
		account, err = r.LookupUser(name)
		if err != nil {
			return nil, fmt.Errorf("resolve intended user %q: %w", name, err)
		}
	}
	if requested != nil && (requested.Username == "" || (requested.UID != "" && requested.UID != account.Uid)) {
		return nil, fmt.Errorf("intended user must name a valid account with matching UID")
	}
	if !r.Elevated && account.Uid != current.Uid {
		return nil, authorizationRequired("run as the intended user; changing another user's identity requires an adapter", nil)
	}
	home := account.HomeDir
	if !r.Elevated && requested == nil && r.Getenv != nil {
		key := "HOME"
		if runtime.GOOS == "windows" {
			key = "USERPROFILE"
		}
		if value := r.Getenv(key); value != "" {
			home = value
		}
	}
	return &UserIdentity{Username: account.Username, UID: account.Uid, GID: account.Gid, HomeDir: home}, nil
}

// ExecutionRequest carries argv and explicit per-operation authority.
type ExecutionRequest struct {
	Privilege OperationPrivilege `json:"privilege"`
	Name      string             `json:"name"`
	Args      []string           `json:"args,omitempty"`
	// Unattended disables stdin and all authorization prompts.
	Unattended bool `json:"unattended,omitempty"`
	// Env contains per-operation environment assignments, also preserved across sudo.
	Env []string `json:"env,omitempty"`
}

// ExecutionPlan is also an install preview: planning never prompts or runs commands.
type ExecutionPlan struct {
	ExecutionRequest
	IntendedUser *UserIdentity `json:"intended_user"`
	Elevate      bool          `json:"elevate"`
	SwitchUser   bool          `json:"switch_user"`
}

func (r PrivilegeResolver) Plan(request ExecutionRequest) (ExecutionPlan, error) {
	request.Unattended = request.Unattended || Unattended()
	plan := ExecutionPlan{ExecutionRequest: request}
	if request.Name == "" {
		return plan, fmt.Errorf("operation requires a command")
	}
	for _, entry := range request.Env {
		if !validEnvironmentAssignment(entry) {
			return plan, fmt.Errorf("operation environment requires NAME=value assignments")
		}
	}
	if request.Privilege.Scope != ScopeUser && request.Privilege.Scope != ScopeSystem {
		return plan, fmt.Errorf("operation requires user or system scope")
	}
	identity, err := r.ResolveUser(request.Privilege.User)
	if err != nil {
		return plan, err
	}
	plan.IntendedUser = identity
	if request.Privilege.Scope == ScopeUser {
		if identity.UID == "0" {
			return plan, authorizationRequired("user-scope operations cannot run as root; invoke as a regular user or preserve SUDO_USER", nil)
		}
		plan.SwitchUser = r.Elevated
	} else {
		plan.Elevate = !r.Elevated
	}
	return plan, nil
}

// OperationIO is independent of exec.Cmd so native authorization APIs can run a
// process and return its native status without constructing an exec.Cmd.
type OperationIO struct {
	Stdin  io.Reader
	Stdout io.Writer
	Stderr io.Writer
}

// PrivilegeAdapter is the platform extension point for authorized execution.
type PrivilegeAdapter interface {
	Run(context.Context, ExecutionPlan, OperationIO) error
}

// CommandPrivilegeAdapter optionally supports argv-based CLI executors/mocks.
type CommandPrivilegeAdapter interface {
	PrivilegeAdapter
	Prepare(context.Context, ExecutionPlan) (*exec.Cmd, error)
}

// DefaultPrivilegeAdapter may be replaced at startup, before concurrent use.
var DefaultPrivilegeAdapter PrivilegeAdapter = defaultPrivilegeAdapter()

type unavailablePrivilegeAdapter struct{}

func (unavailablePrivilegeAdapter) Run(ctx context.Context, plan ExecutionPlan, streams OperationIO) error {
	if plan.Elevate || plan.SwitchUser {
		return authorizationRequired("configure a native platform privilege adapter", nil)
	}
	cmd := NewCommandContext(ctx, plan.Name, plan.Args...)
	configureOperationCommand(cmd, plan.ExecutionRequest)
	if plan.Unattended {
		streams.Stdin = nil
	}
	return runPreparedCommand(cmd, streams)
}

func runPreparedCommand(cmd *exec.Cmd, streams OperationIO) error {
	cmd.Stdin, cmd.Stdout, cmd.Stderr = streams.Stdin, streams.Stdout, streams.Stderr
	return cmd.Run()
}

// UnixSudoAdapter uses sudo only at an explicit elevation/identity boundary.
// A noninteractive probe never prevents execution of a permitted target command.
type UnixSudoAdapter struct {
	LookPath  func(string) (string, error)
	Authorize func(context.Context, string) error
}

func (a UnixSudoAdapter) Prepare(ctx context.Context, plan ExecutionPlan) (*exec.Cmd, error) {
	name, args := plan.Name, append([]string(nil), plan.Args...)
	if plan.Elevate || plan.SwitchUser {
		if runtime.GOOS == "windows" {
			return nil, authorizationRequired("install a Windows UAC privilege adapter", nil)
		}
		lookup := a.LookPath
		if lookup == nil {
			lookup = exec.LookPath
		}
		sudo, err := lookup("sudo")
		if err != nil {
			return nil, authorizationRequired("sudo is unavailable; install sudo or configure a platform privilege adapter", err)
		}
		if plan.Elevate && !plan.Unattended {
			authorize := a.Authorize
			if authorize == nil {
				// Failure of `true` may mean a password is needed or that only the exact
				// target has a NOPASSWD rule. Neither proves denial of the target.
				if err := probeSudo(ctx, sudo); err != nil {
					if ctx.Err() != nil {
						return nil, ctx.Err()
					}
					var exit *exec.ExitError
					if !errors.As(err, &exit) {
						return nil, authorizationRequired("unable to start sudo authorization", err)
					}
				}
			} else if err := authorize(ctx, sudo); err != nil {
				if ctx.Err() != nil {
					return nil, ctx.Err()
				}
				var auth *AuthorizationError
				if errors.As(err, &auth) {
					return nil, err
				}
				var exit *exec.ExitError
				if !errors.As(err, &exit) {
					return nil, authorizationRequired("unable to start sudo authorization", err)
				}
				return nil, NewAuthorizationDeniedError("authorization adapter rejected the operation", exit.ExitCode(), err)
			}
		}
		environment := operationEnvironment(plan.ExecutionRequest)
		target := append([]string{name}, args...)
		if plan.Elevate {
			// sudo parses assignments before the target, retaining command-specific
			// sudoers matching instead of requiring permission to execute env as root.
			// Pass only the variables the operation asked for: the elevated target is
			// a native command that never reads DEVEX_UNATTENDED (sudo -n and closed
			// stdin already enforce unattended mode), and every extra assignment
			// needs SETENV under restrictive sudoers rules.
			target = append(append([]string(nil), plan.Env...), target...)
			args = append([]string{"-n", "--"}, target...)
		} else {
			if plan.IntendedUser == nil || plan.IntendedUser.UID == "0" {
				return nil, fmt.Errorf("user-scope plan requires a non-root identity")
			}
			// sudo -H sets HOME; env also makes account environment explicit and portable.
			identityEnvironment := []string{"HOME=" + plan.IntendedUser.HomeDir, "USER=" + plan.IntendedUser.Username, "LOGNAME=" + plan.IntendedUser.Username}
			environment = append(identityEnvironment, environment...)
			target = append(append([]string{"env"}, environment...), target...)
			args = append([]string{"-n", "-H", "-u", plan.IntendedUser.Username, "--"}, target...)
		}
		name = sudo
	}
	cmd := NewCommandContext(ctx, name, args...)
	configureOperationCommand(cmd, plan.ExecutionRequest)
	return cmd, nil
}

func probeSudo(ctx context.Context, sudo string) error {
	return NewCommandContext(ctx, sudo, "-n", "true").Run()
}

func (a UnixSudoAdapter) Run(ctx context.Context, plan ExecutionPlan, streams OperationIO) error {
	cmd, err := a.Prepare(ctx, plan)
	if err != nil {
		return err
	}
	if plan.Unattended {
		streams.Stdin = nil
	}
	if plan.Elevate || plan.SwitchUser {
		return runSudoCommand(ctx, cmd, streams)
	}
	return runPreparedCommand(cmd, streams)
}

// sudo uses the target exit status too; only its own authorization diagnostic
// distinguishes a rejected launch from an ordinary package-manager failure.
func runSudoCommand(ctx context.Context, cmd *exec.Cmd, streams OperationIO) error {
	var diagnostic diagnosticTail
	if streams.Stderr == nil {
		streams.Stderr = io.Discard
	}
	streams.Stderr = io.MultiWriter(streams.Stderr, &diagnostic)
	// Stable sudo diagnostics let us preserve target errors without classifying
	// every nonzero exit status as an authorization denial.
	if cmd.Env == nil {
		cmd.Env = os.Environ()
	}
	cmd.Env = append(cmd.Env, "LC_ALL=C")
	err := runPreparedCommand(cmd, streams)
	if ctx.Err() != nil {
		return ctx.Err()
	}
	if err != nil {
		message := strings.ToLower(diagnostic.String())
		for _, line := range strings.Split(message, "\n") {
			if strings.HasPrefix(line, "sudo:") && (strings.Contains(line, "password") || strings.Contains(line, "not allowed") || strings.Contains(line, "sudoers") || strings.Contains(line, "terminal is required")) {
				return NewAuthorizationDeniedError("sudo authorization failed; provision credentials or non-prompting sudo authorization", ExitStatus(err), err)
			}
		}
	}
	return err
}

// PrepareOperation is the optional argv-based entry point. Native adapters should
// use RunOperation, which does not require exec.Cmd support.
func PrepareOperation(ctx context.Context, request ExecutionRequest) (*exec.Cmd, error) {
	request.Unattended = request.Unattended || unattendedContext(ctx)
	plan, err := NewPrivilegeResolver().Plan(request)
	if err != nil {
		return nil, err
	}
	adapter, ok := DefaultPrivilegeAdapter.(CommandPrivilegeAdapter)
	if !ok {
		return nil, authorizationRequired("selected adapter requires RunOperation instead of prepared argv", nil)
	}
	return adapter.Prepare(ctx, plan)
}

// RunOperation supports Unix commands and native macOS/UAC process handles alike.
func RunOperation(ctx context.Context, request ExecutionRequest, streams OperationIO) error {
	request.Unattended = request.Unattended || unattendedContext(ctx)
	plan, err := NewPrivilegeResolver().Plan(request)
	if err != nil {
		return err
	}
	if plan.Unattended {
		streams.Stdin = nil
	}
	return DefaultPrivilegeAdapter.Run(ctx, plan, streams)
}

func ExecOperationWithContext(ctx context.Context, privilege OperationPrivilege, name string, args ...string) error {
	return RunOperation(ctx, ExecutionRequest{Privilege: privilege, Name: name, Args: args}, OperationIO{Stdin: os.Stdin, Stdout: os.Stdout, Stderr: os.Stderr})
}

// PluginOperationPrivilege returns nil for legacy metadata without an explicit contract.
func PluginOperationPrivilege(info PluginInfo, args []string) *OperationPrivilege {
	if len(args) == 0 {
		return nil
	}
	for _, command := range info.Commands {
		if command.Name == args[0] {
			return command.Privilege
		}
	}
	return nil
}

// Authorization diagnostics cross the executable plugin boundary as a JSON line;
// the process still exits with its native status rather than a reserved auth code.
const authorizationDiagnosticPrefix = "DEVEX_AUTHORIZATION_ERROR="
const manualRemovalDiagnosticPrefix = "DEVEX_MANUAL_REMOVAL_REQUIRED="
const notInstalledDiagnosticPrefix = "DEVEX_NOT_INSTALLED="

func writeExecutionError(w io.Writer, err error) {
	_, _ = fmt.Fprintf(w, "Error: %v\n", err)
	var manual *ManualRemovalRequiredError
	if errors.As(err, &manual) {
		if data, marshalErr := json.Marshal(manual); marshalErr == nil {
			_, _ = fmt.Fprintf(w, "%s%s\n", manualRemovalDiagnosticPrefix, data)
		}
	}
	var auth *AuthorizationError
	if errors.As(err, &auth) {
		if data, marshalErr := json.Marshal(auth); marshalErr == nil {
			_, _ = fmt.Fprintf(w, "%s%s\n", authorizationDiagnosticPrefix, data)
		}
	}
	var missing *NotInstalledError
	if errors.As(err, &missing) {
		if data, marshalErr := json.Marshal(missing); marshalErr == nil {
			_, _ = fmt.Fprintf(w, "%s%s\n", notInstalledDiagnosticPrefix, data)
		}
	}
}

func decodeExecutionError(diagnostic string, native error) error {
	for _, line := range strings.Split(diagnostic, "\n") {
		if strings.HasPrefix(line, notInstalledDiagnosticPrefix) {
			var missing NotInstalledError
			if err := json.Unmarshal([]byte(strings.TrimPrefix(line, notInstalledDiagnosticPrefix)), &missing); err == nil && len(missing.Packages) > 0 {
				missing.Err = native
				return &missing
			}
		}
		if strings.HasPrefix(line, manualRemovalDiagnosticPrefix) {
			var manual ManualRemovalRequiredError
			if err := json.Unmarshal([]byte(strings.TrimPrefix(line, manualRemovalDiagnosticPrefix)), &manual); err == nil && len(manual.Applications) > 0 {
				manual.Err = native
				return &manual
			}
		}
		if !strings.HasPrefix(line, authorizationDiagnosticPrefix) {
			continue
		}
		var auth AuthorizationError
		if err := json.Unmarshal([]byte(strings.TrimPrefix(line, authorizationDiagnosticPrefix)), &auth); err == nil && (auth.Kind == AuthorizationRequired || auth.Kind == AuthorizationDenied) {
			auth.Err = native
			return &auth
		}
	}
	return native
}

// Keep only the tail of plugin stderr; build output must not grow memory unbounded.
type diagnosticTail struct{ data []byte }

func (d *diagnosticTail) Write(p []byte) (int, error) {
	const limit = 64 * 1024
	n := len(p)
	if n >= limit {
		d.data = append(d.data[:0], p[n-limit:]...)
	} else {
		if extra := len(d.data) + n - limit; extra > 0 {
			d.data = d.data[extra:]
		}
		d.data = append(d.data, p...)
	}
	return n, nil
}
func (d *diagnosticTail) String() string { return string(d.data) }
