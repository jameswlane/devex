// Package installretry retries only transient APT and DNF package acquisition failures.
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

// Manager names a package manager whose native transient failure contract has
// been verified, and classifies its diagnostics.
type Manager struct {
	Name      string
	transient func(string) bool
}

var (
	// APT retries Debian/Ubuntu package acquisition failures.
	APT = Manager{Name: "APT", transient: transientAPT}
	// DNF retries Fedora/RHEL repository metadata and package download failures.
	DNF = Manager{Name: "DNF", transient: transientDNF}
)

var managers = map[string]struct {
	manager    Manager
	identifier *regexp.Regexp
}{
	"package-manager-apt": {APT, regexp.MustCompile(`^[a-z0-9][a-z0-9+.-]*(?::[a-z0-9-]+)?(?:=[A-Za-z0-9.+:~_-]+)?$`)},
	// DNF specs may carry version, epoch and architecture (name-1:2.0-1.fc44.x86_64)
	// but never globs, groups (@), paths, URLs or options.
	"package-manager-dnf": {DNF, regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_+.:~-]*$`)},
}

// Eligible deliberately excludes arbitrary scripts, updates, removal, and other
// managers until their native failure and idempotency contracts are verified.
func Eligible(plugin string, args []string) (Manager, bool) {
	entry, ok := managers[plugin]
	if !ok || len(args) < 2 || args[0] != "install" {
		return Manager{}, false
	}
	for _, arg := range args[1:] {
		if !entry.identifier.MatchString(arg) || strings.HasSuffix(arg, "-") || (entry.manager.Name == DNF.Name && strings.HasSuffix(arg, ".rpm")) {
			return Manager{}, false
		}
	}
	return entry.manager, true
}

// Transient reports whether a failed attempt's native diagnostic is retryable.
func (m Manager) Transient(err error, diagnostic string) bool {
	if err == nil || m.transient == nil || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return false
	}
	return m.transient(strings.ToLower(diagnostic))
}

// transientAPT requires a native acquisition diagnostic. Exit status alone is
// not enough: APT uses status 100 for both permanent and transient failures.
func transientAPT(s string) bool {
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

// transientDNF requires a native download failure whose every cause is a
// transient transport error. DNF exits 1 for every failure, so the status is
// never enough. DNF downloads metadata and all packages before the RPM
// transaction starts, so a download failure leaves the system unchanged.
//
// dnf5 reports each cause on stderr (">>> Curl error (7): ..."); dnf4 does so
// for metadata ("  - Status code: 503 for ...") but prints package download
// causes on stdout only, so a dnf4 package download failure is not retried.
func transientDNF(s string) bool {
	for _, permanent := range []string{"permission denied", "password is required", "superuser", "authentication", "authorization",
		"no match for argument", "unable to find a match", "failed to resolve the transaction", "problem:", "nothing provides",
		"conflicting requests", "transaction failed", "transaction check", "transaction test", "scriptlet",
		"gpg check failed", "openpgp check", "signature", "public key", "checksum", "curl error (60)", "devex_authorization_error"} {
		if strings.Contains(s, permanent) {
			return false
		}
	}
	summary, cause := false, false
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		switch {
		case strings.Contains(line, "failed to download metadata"), strings.Contains(line, "failed to download packages"),
			strings.Contains(line, "error downloading packages"):
			summary = true
		case pluginExitError.MatchString(line):
			// The plugin SDK reports the native exit status after DNF's output.
		case strings.HasPrefix(line, "error:"):
			// Any other fatal DNF error is not a download failure.
			return false
		}
		if code, ok := dnfCause(line, "curl error ("); ok {
			if !transientCurl[code] {
				return false
			}
			cause = true
		} else if code, ok := dnfCause(line, "status code: "); ok {
			// One permanent 404 must not be masked by a 503 from another mirror.
			if !transientStatus[code] {
				return false
			}
			cause = true
		}
	}
	return summary && cause
}

var (
	dnfCode         = regexp.MustCompile(`^\d+`)
	pluginExitError = regexp.MustCompile(`^error: exit status \d+$`)
)

func dnfCause(line, marker string) (string, bool) {
	i := strings.Index(line, marker)
	if i < 0 {
		return "", false
	}
	code := dnfCode.FindString(line[i+len(marker):])
	return code, code != ""
}

// Curl: 5/6 resolve proxy/host, 7 connect, 28 timeout, 35 TLS handshake,
// 52 empty reply, 55/56 send/receive failure.
var transientCurl = map[string]bool{"5": true, "6": true, "7": true, "28": true, "35": true, "52": true, "55": true, "56": true}

var transientStatus = map[string]bool{"429": true, "502": true, "503": true, "504": true}

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
func (p Policy) Run(ctx context.Context, manager Manager, diagnostic io.Writer, run func(io.Writer) error) error {
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
		if !manager.Transient(err, string(tail.data)) {
			return err
		}
		if attempt == p.Attempts {
			fmt.Fprintf(diagnostic, "%s install failed after %d attempts: %v\n", manager.Name, attempt, err)
			return fmt.Errorf("%s install failed after %d attempts: %w", manager.Name, attempt, err)
		}
		pause := jitter(delay)
		// Even an injected jitter function cannot break the delay bound.
		if pause < 0 {
			pause = 0
		}
		if pause > delay {
			pause = delay
		}
		fmt.Fprintf(diagnostic, "%s install attempt %d/%d failed; retrying in %s: %v\n", manager.Name, attempt, p.Attempts, pause, err)
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
