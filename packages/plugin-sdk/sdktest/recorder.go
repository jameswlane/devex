// Package sdktest provides isolated fakes for testing plugin command contracts.
package sdktest

import (
	"bytes"
	"context"
	"fmt"
	"io"
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
	fallbacks map[string]Response
	stdin     map[string][][]string
}

// desktopSessionEnv lists variables that identify or connect to a live desktop
// session. Recorder clears them so specs opt in to a fixture session explicitly.
var desktopSessionEnv = []string{
	"XDG_CURRENT_DESKTOP", "XDG_SESSION_DESKTOP", "XDG_SESSION_TYPE", "DESKTOP_SESSION",
	"DBUS_SESSION_BUS_ADDRESS", "DISPLAY", "WAYLAND_DISPLAY",
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
		fallbacks: make(map[string]Response),
		stdin:     make(map[string][][]string),
	}

	home := filepath.Join(root, "home")
	xdg := map[string]string{
		"XDG_CONFIG_HOME": filepath.Join(home, ".config"),
		"XDG_DATA_HOME":   filepath.Join(home, ".local", "share"),
		"XDG_STATE_HOME":  filepath.Join(home, ".local", "state"),
		"XDG_CACHE_HOME":  filepath.Join(home, ".cache"),
		"XDG_RUNTIME_DIR": filepath.Join(root, "run"),
		"TMPDIR":          filepath.Join(root, "tmp"),
	}
	dirs := []string{recorder.binDir, recorder.logDir, home}
	for _, dir := range xdg {
		dirs = append(dirs, dir)
	}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0o700); err != nil {
			t.Fatalf("create recorder directory: %v", err)
		}
	}

	t.Setenv("HOME", home)
	for key, dir := range xdg {
		t.Setenv(key, dir)
	}
	// No live desktop or session bus: desktop plugins see only what a spec sets.
	for _, key := range desktopSessionEnv {
		t.Setenv(key, "")
	}
	t.Setenv("PATH", recorder.binDir)
	t.Setenv(sdk.UnattendedEnv, "")
	t.Setenv("CI", "")
	// Recorded execution variables start empty so a spec sees only what plugin code sets.
	t.Setenv("NONINTERACTIVE", "")
	t.Setenv("DEBIAN_FRONTEND", "")
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

// Fallback configures the response for invocations of command that match no
// Script entry, for argv that embeds nondeterministic paths. Without a fallback,
// unmatched invocations succeed without output. The fake sudo has no fallback.
func (r *Recorder) Fallback(command string, response Response) {
	r.t.Helper()

	if _, ok := r.responses[command]; !ok || command == "sudo" {
		r.t.Fatalf("fallback for unknown fake executable %q", command)
	}
	if response.ExitCode < 0 || response.ExitCode > 255 {
		r.t.Fatalf("invalid exit code %d for %q", response.ExitCode, command)
	}
	r.fallbacks[command] = response
	r.writeCommand(command)
}

// CaptureStdin opts one exact executable and argv into recording its standard
// input. Only opted-in invocations read stdin, and they read it to end of file,
// so supply finite input (such as a strings.Reader or a fixture file); input
// from a pipe that is never closed blocks the fake. Other invocations never
// read stdin. Payloads are text: a NUL byte ends the recorded payload early.
func (r *Recorder) CaptureStdin(command string, args []string) {
	r.t.Helper()

	if _, ok := r.responses[command]; !ok || command == "sudo" {
		r.t.Fatalf("capture stdin for unknown fake executable %q", command)
	}
	for _, captured := range r.stdin[command] {
		if equalArgs(captured, args) {
			r.t.Fatalf("stdin for %q %q captured twice", command, args)
		}
	}
	r.stdin[command] = append(r.stdin[command], append([]string(nil), args...))
	r.writeCommand(command)
}

// Stdin returns, in invocation order, the standard input of each invocation of
// command with exactly args. The argv must have been opted in with CaptureStdin.
func (r *Recorder) Stdin(command string, args []string) []string {
	r.t.Helper()

	for i, captured := range r.stdin[command] {
		if !equalArgs(captured, args) {
			continue
		}
		data, err := os.ReadFile(r.stdinLog(command, i))
		if os.IsNotExist(err) {
			return nil
		}
		if err != nil {
			r.t.Fatalf("read stdin for %q: %v", command, err)
		}
		payloads := strings.Split(string(data), "\x00")
		return payloads[:len(payloads)-1]
	}
	r.t.Fatalf("stdin for %q %q was not captured", command, args)
	return nil
}

// stdinLog is the file holding NUL-terminated stdin payloads for the index-th
// CaptureStdin registration of command.
func (r *Recorder) stdinLog(command string, index int) string {
	return filepath.Join(r.logDir, fmt.Sprintf("%s.stdin.%d", command, index))
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
		for _, key := range []string{sdk.UnattendedEnv, "NONINTERACTIVE", "DEBIAN_FRONTEND"} {
			if err := os.Remove(filepath.Join(r.logDir, command+".env."+key)); err != nil && !os.IsNotExist(err) {
				r.t.Fatalf("reset environment for %q: %v", command, err)
			}
		}
		if err := os.WriteFile(filepath.Join(r.logDir, command), nil, 0o600); err != nil {
			r.t.Fatalf("reset calls for %q: %v", command, err)
		}
		for i := range r.stdin[command] {
			if err := os.Remove(r.stdinLog(command, i)); err != nil && !os.IsNotExist(err) {
				r.t.Fatalf("reset stdin for %q: %v", command, err)
			}
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
if [ "${1-}" = '-H' ]; then
 shift
 if [ "${1-}" = '-u' ]; then shift; shift; fi
fi
# Like sudo, VAR=value assignments count only before "--".
while [ "$#" -gt 0 ]; do
 case "$1" in *=*) export "$1"; shift ;; *) break ;; esac
done
if [ "${1-}" = '--' ]; then shift; fi
if [ "${1-}" = 'env' ]; then
 shift
 while [ "$#" -gt 0 ]; do
  case "$1" in *=*) export "$1"; shift ;; *) break ;; esac
 done
fi
case "${1-}" in *=*) printf 'sudo: %s: command not found\n' "$1" >&2; exit 1 ;; esac
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

// writeCommand regenerates the fake executable for command. The script logs each
// invocation (elevation marker, argc, NUL-separated argv) and the recorded
// environment variables, copies stdin for opted-in argv, then answers with the
// first exact Script match, the fallback, or success; sudo instead dispatches to
// registered fakes.
func (r *Recorder) writeCommand(command string) {
	r.t.Helper()

	var script strings.Builder
	script.WriteString("#!/bin/sh\n{\n  printf '%s\\0' \"${DEVEX_SDKTEST_ELEVATED:-0}\" \"$#\" \"$@\"\n} >> ")
	script.WriteString(shellQuote(filepath.Join(r.logDir, command)))
	script.WriteString("\n")
	for _, key := range []string{sdk.UnattendedEnv, "NONINTERACTIVE", "DEBIAN_FRONTEND"} {
		fmt.Fprintf(&script, "printf '%%s\\0' \"${%s-}\" >> %s\n", key, shellQuote(filepath.Join(r.logDir, command+".env."+key)))
	}

	// Opted-in argv only: copy stdin line by line with shell builtins, keeping a
	// final line that lacks a newline, and end each payload with NUL.
	for i, args := range r.stdin[command] {
		writeArgvMatch(&script, args)
		fmt.Fprintf(&script, "  { while IFS= read -r line; do printf '%%s\\n' \"$line\"; done; printf '%%s\\0' \"$line\"; } >> %s\nfi\n", shellQuote(r.stdinLog(command, i)))
	}
	for _, scripted := range r.responses[command] {
		writeArgvMatch(&script, scripted.args)
		writeResponse(&script, "  ", scripted.response)
		script.WriteString("fi\n")
	}
	if command == "sudo" {
		r.writeSudoDispatch(&script)
	} else {
		writeResponse(&script, "", r.fallbacks[command])
	}
	r.writeExecutable(command, script.String())
}

// writeArgvMatch opens a shell if-block that matches exactly args (count and each
// value, shell-quoted); the caller writes the body and the closing fi.
func writeArgvMatch(script *strings.Builder, args []string) {
	fmt.Fprintf(script, "if [ \"$#\" -eq %d ]", len(args))
	for i, arg := range args {
		fmt.Fprintf(script, " && [ \"${%d-}\" = %s ]", i+1, shellQuote(arg))
	}
	script.WriteString("; then\n")
}

// writeResponse emits shell that prints the response's stdout and stderr verbatim
// and exits with its status, each line prefixed by indent.
func writeResponse(script *strings.Builder, indent string, response Response) {
	if response.Stdout != "" {
		fmt.Fprintf(script, "%sprintf '%%s' %s\n", indent, shellQuote(response.Stdout))
	}
	if response.Stderr != "" {
		fmt.Fprintf(script, "%sprintf '%%s' %s >&2\n", indent, shellQuote(response.Stderr))
	}
	fmt.Fprintf(script, "%sexit %d\n", indent, response.ExitCode)
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

// equalArgs reports whether two argv slices have the same elements in order.
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

// ProvideStdin replaces the test process's stdin with input until cleanup, for
// operations that read an interactive confirmation.
func ProvideStdin(t TestingT, input string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "stdin")
	if err := os.WriteFile(path, []byte(input), 0o600); err != nil {
		t.Fatalf("write stdin fixture: %v", err)
	}
	file, err := os.Open(path)
	if err != nil {
		t.Fatalf("open stdin fixture: %v", err)
	}
	original := os.Stdin
	os.Stdin = file
	t.Cleanup(func() { os.Stdin = original; _ = file.Close() })
}

// CaptureStdout runs fn with the process's stdout redirected and returns what it
// wrote. Like the other helpers it changes process-global state; do not use it
// concurrently.
func CaptureStdout(t TestingT, fn func()) string {
	t.Helper()
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("create stdout pipe: %v", err)
	}
	output := make(chan []byte, 1)
	go func() {
		data, _ := io.ReadAll(reader)
		_ = reader.Close()
		output <- data
	}()
	original := os.Stdout
	os.Stdout = writer
	func() {
		defer func() { os.Stdout = original; _ = writer.Close() }()
		fn()
	}()
	return string(<-output)
}

// CloseStdin replaces the test process's stdin with the null device until cleanup.
func CloseStdin(t TestingT) {
	t.Helper()
	input, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatalf("open null stdin: %v", err)
	}
	original := os.Stdin
	os.Stdin = input
	t.Cleanup(func() { os.Stdin = original; _ = input.Close() })
}

// Environment returns each invocation's value of a recorded execution variable.
// Recorder captures DEVEX_UNATTENDED, NONINTERACTIVE, and DEBIAN_FRONTEND.
func (r *Recorder) Environment(command, key string) []string {
	r.t.Helper()
	if key != sdk.UnattendedEnv && key != "NONINTERACTIVE" && key != "DEBIAN_FRONTEND" {
		r.t.Fatalf("unrecorded environment key %q", key)
	}
	data, err := os.ReadFile(filepath.Join(r.logDir, command+".env."+key))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		r.t.Fatalf("read environment: %v", err)
	}
	values := strings.Split(string(data), "\x00")
	return values[:len(values)-1]
}

// CommandNames returns the advertised command names in declaration order, for
// comparing a plugin's advertised operations with its covered contracts.
func CommandNames(info sdk.PluginInfo) []string {
	names := make([]string, 0, len(info.Commands))
	for _, command := range info.Commands {
		names = append(names, command.Name)
	}
	return names
}
