package mcpserver

import (
	"bytes"
	"io"
	"sync"
)

// MaxLogLineLength bounds each diagnostic line a RedactingWriter forwards.
const MaxLogLineLength = 1000

// maxPendingLine bounds the bytes buffered for one unterminated line; the rest
// of an overlong line is dropped.
const maxPendingLine = 64 * 1024

// RedactingWriter forwards diagnostics line by line with credentials, URL
// query strings and secret-looking values removed and each line bounded, so
// installer, plugin and child-process output cannot leak secrets into the MCP
// client's stderr log.
type RedactingWriter struct {
	mu        sync.Mutex
	out       io.Writer
	pending   []byte
	truncated bool
}

// NewRedactingWriter returns a RedactingWriter that writes to out.
func NewRedactingWriter(out io.Writer) *RedactingWriter {
	return &RedactingWriter{out: out}
}

// Write buffers p and forwards every complete line, redacted. It always
// reports len(p) so writers are never told to retry.
func (w *RedactingWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	n := len(p)
	for len(p) > 0 {
		i := bytes.IndexByte(p, '\n')
		if i < 0 {
			w.buffer(p)
			break
		}
		w.buffer(p[:i])
		if err := w.flush(); err != nil {
			return n, err
		}
		p = p[i+1:]
	}
	return n, nil
}

func (w *RedactingWriter) buffer(p []byte) {
	room := maxPendingLine - len(w.pending)
	if len(p) > room {
		p = p[:room]
		w.truncated = true
	}
	w.pending = append(w.pending, p...)
}

func (w *RedactingWriter) flush() error {
	line := bound(redact(string(bytes.TrimRight(w.pending, "\r"))), MaxLogLineLength)
	if w.truncated && len(line) <= MaxLogLineLength {
		line += "…"
	}
	w.pending, w.truncated = w.pending[:0], false
	_, err := io.WriteString(w.out, line+"\n")
	return err
}

// Close forwards a final unterminated line.
func (w *RedactingWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if len(w.pending) == 0 && !w.truncated {
		return nil
	}
	return w.flush()
}
