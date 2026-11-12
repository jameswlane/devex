// Package execinputs pins the local files a confirmed install reads while it
// runs. A confirmed plan records each source file's content digest and each
// marker file's presence; execution carries that record in its context and
// refuses to use a file that differs from what was confirmed. Without a
// record in the context every check passes, so unconfirmed installs behave
// as before.
package execinputs

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
)

// Inputs are the confirmed local files of an install.
type Inputs struct {
	// Files maps an expanded source path to its content digest
	// ("sha256:<hex>"), or to "missing" when it was not a readable regular file.
	Files map[string]string `json:"files"`
	// Markers maps an expanded marker path to whether it existed as a regular file.
	Markers map[string]bool `json:"markers"`
}

// Missing is the fingerprint of a source that was not a readable regular file.
const Missing = "missing"

type contextKey struct{}

// With returns a context that pins inputs for the operations run with it.
func With(ctx context.Context, inputs Inputs) context.Context {
	return context.WithValue(ctx, contextKey{}, inputs)
}

// Pinned returns the confirmed inputs carried by ctx, if any.
func Pinned(ctx context.Context) (Inputs, bool) {
	return from(ctx)
}

func from(ctx context.Context) (Inputs, bool) {
	inputs, ok := ctx.Value(contextKey{}).(Inputs)
	return inputs, ok
}

// Digest returns the fingerprint of content.
func Digest(data []byte) string {
	sum := sha256.Sum256(data)
	return "sha256:" + hex.EncodeToString(sum[:])
}

// Fingerprint returns the fingerprint of the regular file at path, or Missing.
func Fingerprint(path string) string {
	info, err := os.Stat(path)
	if err != nil || !info.Mode().IsRegular() {
		return Missing
	}
	data, err := os.ReadFile(path) // #nosec G304 -- fingerprinting a configured source path, read-only
	if err != nil {
		return Missing
	}
	return Digest(data)
}

// MarkerPresent reports whether path is an existing regular file.
func MarkerPresent(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.Mode().IsRegular()
}

// ErrChanged reports a local input that differs from the confirmed plan.
var ErrChanged = errors.New("changed after the plan was confirmed")

// VerifyFile checks content read from path against the confirmed inputs.
func VerifyFile(ctx context.Context, path string, data []byte) error {
	inputs, ok := from(ctx)
	if !ok {
		return nil
	}
	confirmed, known := inputs.Files[path]
	if !known {
		return fmt.Errorf("source %s was not part of the confirmed plan: %w", path, ErrChanged)
	}
	if confirmed != Digest(data) {
		return fmt.Errorf("source %s %w; review the plan again", path, ErrChanged)
	}
	return nil
}

// VerifyMarker checks a marker file's presence against the confirmed inputs.
func VerifyMarker(ctx context.Context, path string, present bool) error {
	inputs, ok := from(ctx)
	if !ok {
		return nil
	}
	confirmed, known := inputs.Markers[path]
	if !known || confirmed != present {
		return fmt.Errorf("marker %s %w; review the plan again", path, ErrChanged)
	}
	return nil
}
