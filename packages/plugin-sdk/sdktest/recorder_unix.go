//go:build !windows

package sdktest

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
)

func (r *Recorder) fakePath(command string) string {
	return filepath.Join(r.binDir, command)
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
