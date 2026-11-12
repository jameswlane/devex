//go:build unix

package mcpserver

import (
	"errors"
	"fmt"
	"io"
	"os"
	"runtime/debug"
	"time"

	"golang.org/x/sys/unix"
)

// drainTimeout bounds how long Close waits for output still held by child
// processes that outlive the server.
const drainTimeout = 2 * time.Second

// Stdio is the process's original stdin and stdout, reserved for MCP messages.
type Stdio struct {
	// Stdin carries only MCP requests.
	Stdin *os.File
	// Stdout carries only MCP messages.
	Stdout *os.File
	stderr *os.File
	reader *os.File
	done   chan struct{}
}

// ReserveStdout keeps the process's original stdout for MCP messages and
// points file descriptors 1 and 2 at a pipe whose output is redacted line by
// line (see RedactingWriter) and forwarded to the original stderr. Anything
// else that writes to stdout or stderr afterwards, including installers,
// plugins and their child processes, then cannot corrupt the MCP stream or
// leak credentials into the client's stderr log.
//
// It also keeps the original stdin for MCP requests and points descriptor 0
// at the null device, so installers, plugins, hooks, uninstall commands and
// their child processes read end-of-file instead of consuming MCP requests,
// whether they inherit descriptor 0 or are handed os.Stdin. Close restores
// stderr.
func ReserveStdout() (*Stdio, error) {
	savedIn, err := unix.Dup(0)
	if err != nil {
		return nil, fmt.Errorf("reserve stdin for MCP: %w", err)
	}
	unix.CloseOnExec(savedIn)
	stdin := os.NewFile(uintptr(savedIn), "mcp-stdin")
	null, err := os.Open(os.DevNull)
	if err != nil {
		_ = stdin.Close()
		return nil, fmt.Errorf("open %s for child processes: %w", os.DevNull, err)
	}
	err = unix.Dup2(int(null.Fd()), 0)
	_ = null.Close()
	if err != nil {
		_ = stdin.Close()
		return nil, fmt.Errorf("detach stdin from MCP requests: %w", err)
	}
	// Descriptor 0 stays on the null device even if a later step fails; the
	// server then exits without serving.
	savedOut, err := unix.Dup(1)
	if err != nil {
		_ = stdin.Close()
		return nil, fmt.Errorf("reserve stdout for MCP: %w", err)
	}
	unix.CloseOnExec(savedOut)
	savedErr, err := unix.Dup(2)
	if err != nil {
		_ = unix.Close(savedOut)
		_ = stdin.Close()
		return nil, fmt.Errorf("reserve stderr for MCP diagnostics: %w", err)
	}
	unix.CloseOnExec(savedErr)
	stdout := os.NewFile(uintptr(savedOut), "mcp-stdout")
	stderr := os.NewFile(uintptr(savedErr), "mcp-stderr")
	fail := func(err error) (*Stdio, error) {
		_ = unix.Dup2(savedErr, 1)
		_ = unix.Dup2(savedErr, 2)
		_ = stdout.Close()
		_ = stderr.Close()
		_ = stdin.Close()
		return nil, err
	}

	reader, writer, err := os.Pipe()
	if err != nil {
		return fail(fmt.Errorf("create diagnostics pipe: %w", err))
	}
	// Fd switches the write end to blocking mode, which child processes and
	// os.Stdout/os.Stderr expect once it becomes descriptors 1 and 2.
	writeFd := int(writer.Fd())
	if err := unix.Dup2(writeFd, 1); err != nil {
		_ = reader.Close()
		_ = writer.Close()
		return fail(fmt.Errorf("redirect stdout to diagnostics: %w", err))
	}
	if err := unix.Dup2(writeFd, 2); err != nil {
		_ = reader.Close()
		_ = writer.Close()
		return fail(fmt.Errorf("redirect stderr to diagnostics: %w", err))
	}
	_ = writer.Close() // descriptors 1 and 2 keep the pipe open

	// Fatal runtime errors bypass the pipe so crash reports are never lost.
	_ = debug.SetCrashOutput(stderr, debug.CrashOptions{})

	s := &Stdio{Stdin: stdin, Stdout: stdout, stderr: stderr, reader: reader, done: make(chan struct{})}
	go func() {
		defer close(s.done)
		redacted := NewRedactingWriter(stderr)
		_, _ = io.Copy(redacted, reader)
		_ = redacted.Close()
	}()
	return s, nil
}

// Close points descriptors 1 and 2 back at the original stderr, forwards the
// remaining diagnostics and closes the reserved stdin and stdout.
func (s *Stdio) Close() error {
	restoreOut := unix.Dup2(int(s.stderr.Fd()), 1)
	restoreErr := unix.Dup2(int(s.stderr.Fd()), 2)
	select {
	case <-s.done:
	case <-time.After(drainTimeout):
	}
	_ = s.reader.Close()
	_ = debug.SetCrashOutput(nil, debug.CrashOptions{})
	return errors.Join(restoreOut, restoreErr, s.Stdout.Close(), s.Stdin.Close())
}
