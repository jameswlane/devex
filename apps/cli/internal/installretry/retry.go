// Package installretry retries only transient APT package acquisition failures.
package installretry

import (
	"context"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"regexp"
	"strings"
	"time"

	"github.com/spf13/viper"
)

// Policy bounds both the number of executions and the delay between them.
// Sleep and Jitter are injectable so tests never need wall-clock delays.
type Policy struct {
	Attempts     int
	InitialDelay time.Duration
	MaxDelay     time.Duration
	Sleep        func(context.Context, time.Duration) error
	Jitter       func(time.Duration) time.Duration
}

func Configured() (Policy, error) {
	p := Policy{Attempts: 3, InitialDelay: time.Second, MaxDelay: 8 * time.Second}
	if viper.IsSet("install-retry-attempts") {
		p.Attempts = viper.GetInt("install-retry-attempts")
	}
	if viper.IsSet("install-retry-delay") {
		p.InitialDelay = viper.GetDuration("install-retry-delay")
	}
	if viper.IsSet("install-retry-max-delay") {
		p.MaxDelay = viper.GetDuration("install-retry-max-delay")
	}
	return p, p.validate()
}

func (p Policy) validate() error {
	if p.Attempts < 1 || p.Attempts > 5 {
		return fmt.Errorf("install-retry-attempts must be between 1 and 5")
	}
	if p.InitialDelay <= 0 || p.MaxDelay < p.InitialDelay || p.MaxDelay > time.Minute {
		return fmt.Errorf("install retry delays must satisfy 0 < delay <= max-delay <= 1m")
	}
	return nil
}

// Eligible deliberately excludes arbitrary scripts, updates, removal, and other
// managers until their native failure and idempotency contracts are verified.
func Eligible(plugin string, args []string) bool {
	if plugin != "package-manager-apt" || len(args) < 2 || args[0] != "install" {
		return false
	}
	for _, arg := range args[1:] {
		if !packageIdentifier.MatchString(arg) || strings.HasSuffix(arg, "-") {
			return false
		}
	}
	return true
}

var packageIdentifier = regexp.MustCompile(`^[a-z0-9][a-z0-9+.-]*(?::[a-z0-9-]+)?(?:=[A-Za-z0-9.+:~_-]+)?$`)

// Transient requires a native acquisition diagnostic. Exit status alone is not
// enough: APT uses status 100 for both permanent and transient failures.
func Transient(err error, diagnostic string) bool {
	if err == nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	s := strings.ToLower(diagnostic)
	for _, permanent := range []string{"permission denied", "password is required", "authentication", "authorization", "not signed", "no_pubkey", "unable to locate package", "has no installation candidate", "unmet dependencies", "dpkg:", "sub-process /usr/bin/dpkg", "post-installation script", "hash sum mismatch"} {
		if strings.Contains(s, permanent) {
			return false
		}
	}
	// An update warning earlier in the plugin must not make a later,
	// unrelated install failure retryable. Require APT's fatal fetch diagnostic,
	// and reject any other native fatal error except the acquisition summary.
	acquisition := false
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "e:") {
			continue
		}
		if strings.HasPrefix(line, "e: failed to fetch ") {
			// Every failed archive must be transient; one temporary error
			// must not mask a permanent 404 in the same transaction.
			if !transientAcquisition(line) {
				return false
			}
			acquisition = true
			continue
		}
		if !strings.HasPrefix(line, "e: unable to fetch some archives") {
			return false
		}
	}
	return acquisition
}

func transientAcquisition(s string) bool {
	for _, transient := range []string{"temporary failure resolving", "connection timed out", "connection failed", "connection reset", "could not connect", " 429 ", " 502 ", " 503 ", " 504 "} {
		if strings.Contains(s, transient) {
			return true
		}
	}
	return false
}

func defaultJitter(d time.Duration) time.Duration {
	// #nosec G404 -- Retry scheduling needs variation, not cryptographic randomness.
	return d/2 + time.Duration(rand.Int64N(int64(d-d/2)+1))
}

func sleep(ctx context.Context, delay time.Duration) error {
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

// Run forwards native stderr immediately while retaining only a bounded tail for
// classification. Only the package operation is repeated, never its lifecycle.
func (p Policy) Run(ctx context.Context, diagnostic io.Writer, run func(io.Writer) error) error {
	if err := p.validate(); err != nil {
		return err
	}
	if diagnostic == nil {
		diagnostic = io.Discard
	}
	wait := p.Sleep
	if wait == nil {
		wait = sleep
	}
	jitter := p.Jitter
	if jitter == nil {
		jitter = defaultJitter
	}
	delay := p.InitialDelay
	for attempt := 1; attempt <= p.Attempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		tail := &diagnosticTail{}
		err := run(io.MultiWriter(diagnostic, tail))
		if ctx.Err() != nil {
			return ctx.Err()
		}
		if err == nil {
			return nil
		}
		if !Transient(err, string(tail.data)) {
			return err
		}
		if attempt == p.Attempts {
			fmt.Fprintf(diagnostic, "APT install failed after %d attempts: %v\n", attempt, err)
			return fmt.Errorf("APT install failed after %d attempts: %w", attempt, err)
		}
		pause := jitter(delay)
		// Even an injected jitter function cannot break the delay bound.
		if pause < 0 {
			pause = 0
		}
		if pause > delay {
			pause = delay
		}
		fmt.Fprintf(diagnostic, "APT install attempt %d/%d failed; retrying in %s: %v\n", attempt, p.Attempts, pause, err)
		if err := wait(ctx, pause); err != nil {
			return err
		}
		if delay > p.MaxDelay/2 {
			delay = p.MaxDelay
		} else {
			delay *= 2
		}
	}
	return nil
}

type diagnosticTail struct{ data []byte }

func (t *diagnosticTail) Write(data []byte) (int, error) {
	const limit = 32 * 1024
	n := len(data)
	if n >= limit {
		t.data = append(t.data[:0], data[n-limit:]...)
	} else {
		if len(t.data)+n > limit {
			t.data = t.data[len(t.data)+n-limit:]
		}
		t.data = append(t.data, data...)
	}
	return n, nil
}
