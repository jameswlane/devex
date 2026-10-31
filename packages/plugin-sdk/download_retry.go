package sdk

import (
	"context"
	"errors"
	"fmt"
	"math/rand"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const (
	downloadMaxAttempts = 3
	maxRetryAfter       = time.Second
)

// RetryBackoffFunc waits before the next attempt. Attempt starts at 1, and
// retryAfter is the capped delay requested by the server, if any.
type RetryBackoffFunc func(ctx context.Context, attempt int, retryAfter time.Duration) error

// RegistryUnavailableError reports a DNS or connection failure with recovery guidance.
type RegistryUnavailableError struct {
	Err error
}

func (e *RegistryUnavailableError) Error() string {
	return fmt.Sprintf("plugin registry is unreachable: %v; check your network connection and retry, or use --offline with already-installed plugins", e.Err)
}

// Unwrap returns the underlying network error.
func (e *RegistryUnavailableError) Unwrap() error {
	return e.Err
}

func defaultRetryBackoff(ctx context.Context, attempt int, retryAfter time.Duration) error {
	delay := retryAfter
	if delay <= 0 {
		delay = 100 * time.Millisecond * time.Duration(1<<(attempt-1))
		delay = time.Duration(float64(delay) * (0.75 + rand.Float64()*0.5)) //nolint:gosec // Jitter does not require cryptographic randomness.
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (d *Downloader) doWithRetry(ctx context.Context, client *http.Client, method, requestURL string) (*http.Response, error) {
	var lastNetworkErr error
	for attempt := 1; attempt <= downloadMaxAttempts; attempt++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		req, err := http.NewRequestWithContext(ctx, method, requestURL, nil)
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}
		resp, err := client.Do(req)
		if err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			if !isTransientNetworkError(err) {
				return nil, err
			}
			lastNetworkErr = err
		} else {
			if !isTransientHTTPStatus(resp.StatusCode) {
				return resp, nil
			}
			if attempt == downloadMaxAttempts {
				return resp, nil
			}
			retryAfter := parseRetryAfter(resp.Header.Get("Retry-After"), time.Now())
			_ = resp.Body.Close()
			if err := d.waitForRetry(ctx, attempt, retryAfter); err != nil {
				return nil, err
			}
			continue
		}

		if attempt < downloadMaxAttempts {
			if err := d.waitForRetry(ctx, attempt, 0); err != nil {
				return nil, err
			}
		}
	}
	if isRegistryUnreachable(lastNetworkErr) {
		return nil, &RegistryUnavailableError{Err: lastNetworkErr}
	}
	return nil, lastNetworkErr
}

func (d *Downloader) waitForRetry(ctx context.Context, attempt int, retryAfter time.Duration) error {
	backoff := d.retryBackoff
	if backoff == nil {
		backoff = defaultRetryBackoff
	}
	return backoff(ctx, attempt, retryAfter)
}

func isTransientHTTPStatus(status int) bool {
	return status == http.StatusTooManyRequests || status >= 500 && status <= 599
}

func isTransientNetworkError(err error) bool {
	var networkErr net.Error
	return errors.As(err, &networkErr)
}

func isRegistryUnreachable(err error) bool {
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) {
		return true
	}
	var opErr *net.OpError
	return errors.As(err, &opErr)
}

func parseRetryAfter(value string, now time.Time) time.Duration {
	value = strings.TrimSpace(value)
	if value == "" {
		return 0
	}
	var delay time.Duration
	if seconds, err := strconv.Atoi(value); err == nil {
		if seconds < 0 {
			return 0
		}
		delay = time.Duration(seconds) * time.Second
	} else if retryAt, err := http.ParseTime(value); err == nil {
		delay = retryAt.Sub(now)
		if delay < 0 {
			return 0
		}
	} else {
		return 0
	}
	if delay > maxRetryAfter {
		return maxRetryAfter
	}
	return delay
}
