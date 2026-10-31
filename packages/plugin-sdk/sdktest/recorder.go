// Package sdktest provides isolated fakes for testing plugin command contracts.
package sdktest

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"sort"
	"strings"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
)

// TestingT is the test environment used by Recorder.
type TestingT interface {
	Helper()
	Cleanup(func())
	TempDir() string
	Setenv(key, value string)
	Fatalf(format string, args ...any)
}

// Call is one invocation of a fake executable.
type Call struct {
	Args               []string
	ElevationRequested bool
}

// Response describes the result of one exact executable and argv match.
type Response struct {
	ExitCode int
	Stdout   string
	Stderr   string
}

type scriptedResponse struct {
	args     []string
	response Response
}

// Recorder installs fake executables and records their invocations.
type Recorder struct {
	t         TestingT
	binDir    string
	logDir    string
	responses map[string][]scriptedResponse
}

// NewRecorder isolates the process environment and installs fakes for commands.
// A fake sudo executable is always installed to record elevation requests.
func NewRecorder(t TestingT, commands ...string) *Recorder {
	t.Helper()

	root := t.TempDir()
	recorder := &Recorder{
		t:         t,
		binDir:    filepath.Join(root, "bin"),
		logDir:    filepath.Join(root, "logs"),
		responses: make(map[string][]scriptedResponse),
	}

	for _, dir := range []string{recorder.binDir, recorder.logDir, filepath.Join(root, "home")} {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatalf("create recorder directory: %v", err)
		}
	}

	t.Setenv("HOME", filepath.Join(root, "home"))
	t.Setenv("PATH", recorder.binDir)
	t.Setenv("USER", "devex-test-user")
	t.Setenv("LOGNAME", "devex-test-user")
	t.Setenv("DEVEX_SDKTEST_LOG_DIR", recorder.logDir)
	t.Setenv("DEVEX_SDKTEST_ELEVATED", "0")

	for _, command := range commands {
		recorder.install(command)
	}

	recorder.installSudo()
	recorder.installExecutionBoundary()

	return recorder
}

// Script configures a response for an exact executable and argv match.
// Unmatched invocations succeed without output.
func (r *Recorder) Script(command string, args []string, response Response) {
	r.t.Helper()

	if _, ok := r.responses[command]; !ok {
		r.t.Fatalf("script unknown fake executable %q", command)
	}
	if response.ExitCode < 0 || response.ExitCode > 255 {
		r.t.Fatalf("invalid exit code %d for %q", response.ExitCode, command)
	}

	for i := range r.responses[command] {
		if equalArgs(r.responses[command][i].args, args) {
			r.responses[command][i].response = response
			r.writeCommand(command)
			return
		}
	}

	r.responses[command] = append(r.responses[command], scriptedResponse{
		args:     append([]string(nil), args...),
		response: response,
	})
	r.writeCommand(command)
}

// Calls returns invocations in execution order for this executable only.
func (r *Recorder) Calls(command string) []Call {
	r.t.Helper()

	data, err := os.ReadFile(filepath.Join(r.logDir, command))
	if err != nil {
		r.t.Fatalf("read calls for %q: %v", command, err)
	}
	if len(data) == 0 {
		return nil
	}

	fields := bytes.Split(data, []byte{0})
	if len(fields[len(fields)-1]) != 0 {
		r.t.Fatalf("invalid call log for %q", command)
	}
	fields = fields[:len(fields)-1]
	if len(fields) == 0 {
		r.t.Fatalf("missing elevation marker for %q", command)
	}

	var calls []Call
	for len(fields) > 0 {
		if len(fields) < 2 {
			r.t.Fatalf("incomplete call record for %q", command)
		}
		var argCount int
		if _, scanErr := fmt.Sscanf(string(fields[1]), "%d", &argCount); scanErr != nil || argCount < 0 || len(fields) < argCount+2 {
			r.t.Fatalf("invalid argument count for %q", command)
		}

		call := Call{
			Args:               make([]string, argCount),
			ElevationRequested: string(fields[0]) == "1",
		}
		for i, field := range fields[2 : argCount+2] {
			call.Args[i] = string(field)
		}
		calls = append(calls, call)
		fields = fields[argCount+2:]
	}
	return calls
}

// Reset removes all recorded invocations while retaining scripted responses.
func (r *Recorder) Reset() {
	r.t.Helper()

	for command := range r.responses {
		if err := os.WriteFile(filepath.Join(r.logDir, command), nil, 0o600); err != nil {
			r.t.Fatalf("reset calls for %q: %v", command, err)
		}
	}
}

func (r *Recorder) install(command string) {
	r.t.Helper()

	if command == "" || command == "." || command == ".." || command == "sudo" || filepath.Base(command) != command {
		r.t.Fatalf("invalid fake executable name %q", command)
	}
	if _, exists := r.responses[command]; exists {
		r.t.Fatalf("fake executable %q installed twice", command)
	}

	r.responses[command] = nil
	if err := os.WriteFile(filepath.Join(r.logDir, command), nil, 0o600); err != nil {
		r.t.Fatalf("create call log for %q: %v", command, err)
	}
	r.writeCommand(command)
}

func (r *Recorder) installSudo() {
	r.responses["sudo"] = nil
	if err := os.WriteFile(filepath.Join(r.logDir, "sudo"), nil, 0o600); err != nil {
		r.t.Fatalf("create sudo call log: %v", err)
	}

	r.writeCommand("sudo")
}

// Install process-global hooks alongside the environment; cleanup restores both.
func (r *Recorder) installExecutionBoundary() {
	originalCommand := sdk.NewCommandContext
	originalResolver := sdk.NewPrivilegeResolver
	originalAdapter := sdk.DefaultPrivilegeAdapter
	r.t.Cleanup(func() {
		sdk.NewCommandContext = originalCommand
		sdk.NewPrivilegeResolver = originalResolver
		sdk.DefaultPrivilegeAdapter = originalAdapter
	})
	sdk.NewCommandContext = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		for command := range r.responses {
			fake := filepath.Join(r.binDir, command)
			if name == command || name == fake {
				return exec.CommandContext(ctx, fake, args...)
			}
		}
		// Construct no process for an unregistered name, even if it is absolute.
		return &exec.Cmd{Args: append([]string{name}, args...), Err: fmt.Errorf("sdktest: executable %q is not a registered fake", name)}
	}
	sdk.NewPrivilegeResolver = func() sdk.PrivilegeResolver {
		account := func() (*user.User, error) {
			return &user.User{Username: "devex-test-user", Uid: "1000", Gid: "1000", HomeDir: os.Getenv("HOME")}, nil
		}
		return sdk.PrivilegeResolver{
			Elevated:    false,
			CurrentUser: account,
			LookupUser: func(name string) (*user.User, error) {
				if name != "devex-test-user" {
					return nil, fmt.Errorf("sdktest: account %q is not registered", name)
				}
				return account()
			},
			Getenv: os.Getenv,
		}
	}
	sdk.DefaultPrivilegeAdapter = sdk.UnixSudoAdapter{}
}

// Only authorization probes and explicit registered fake targets are supported.
func (r *Recorder) writeSudoDispatch(script *strings.Builder) {
	script.WriteString(`if [ "$#" -eq 1 ] && [ "$1" = '-v' ]; then exit 0; fi
if [ "$#" -eq 2 ] && [ "$1" = '-n' ] && [ "$2" = 'true' ]; then exit 0; fi
if [ "${1-}" = '-n' ]; then shift; fi
if [ "$#" -eq 1 ] && [ "$1" = '-v' ]; then exit 0; fi
if [ "${1-}" = '--' ]; then shift; fi
case "${1-}" in
`)
	commands := make([]string, 0, len(r.responses))
	for command := range r.responses {
		if command != "sudo" {
			commands = append(commands, command)
		}
	}
	sort.Strings(commands)
	for _, command := range commands {
		fake := filepath.Join(r.binDir, command)
		fmt.Fprintf(script, "  %s|%s) shift; DEVEX_SDKTEST_ELEVATED=1 %s \"$@\"; exit $? ;;\n", shellQuote(command), shellQuote(fake), shellQuote(fake))
	}
	script.WriteString(`  *) printf '%s\n' 'sdktest: sudo target is not a registered fake' >&2; exit 127 ;;
esac
`)
}

func (r *Recorder) writeCommand(command string) {
	r.t.Helper()

	var script strings.Builder
	script.WriteString("#!/bin/sh\n{\n  printf '%s\\0' \"${DEVEX_SDKTEST_ELEVATED:-0}\" \"$#\" \"$@\"\n} >> ")
	script.WriteString(shellQuote(filepath.Join(r.logDir, command)))
	script.WriteString("\n")

	for _, scripted := range r.responses[command] {
		fmt.Fprintf(&script, "if [ \"$#\" -eq %d ]", len(scripted.args))
		for i, arg := range scripted.args {
			fmt.Fprintf(&script, " && [ \"${%d-}\" = %s ]", i+1, shellQuote(arg))
		}
		script.WriteString("; then\n")
		if scripted.response.Stdout != "" {
			fmt.Fprintf(&script, "  printf '%%s' %s\n", shellQuote(scripted.response.Stdout))
		}
		if scripted.response.Stderr != "" {
			fmt.Fprintf(&script, "  printf '%%s' %s >&2\n", shellQuote(scripted.response.Stderr))
		}
		fmt.Fprintf(&script, "  exit %d\nfi\n", scripted.response.ExitCode)
	}
	if command == "sudo" {
		r.writeSudoDispatch(&script)
	} else {
		script.WriteString("exit 0\n")
	}
	r.writeExecutable(command, script.String())
}

func (r *Recorder) writeExecutable(command, script string) {
	r.t.Helper()

	if err := os.WriteFile(filepath.Join(r.binDir, command), []byte(script), 0o700); err != nil {
		r.t.Fatalf("write fake executable %q: %v", command, err)
	}
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'"'"'`) + "'"
}

func equalArgs(left, right []string) bool {
	if len(left) != len(right) {
		return false
	}
	for i := range left {
		if left[i] != right[i] {
			return false
		}
	}
	return true
}
