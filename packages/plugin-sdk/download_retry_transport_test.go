package sdk

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"syscall"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type retryRoundTripper func(*http.Request) (*http.Response, error)

func (f retryRoundTripper) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

type retryReader func([]byte) (int, error)

func (f retryReader) Read(p []byte) (int, error) { return f(p) }

var _ = Describe("Response body retry transport failures", func() {
	DescribeTable("preserves read errors without declaring the registry unreachable",
		func(cancelRead bool) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			attempts := 0
			readErr := &net.OpError{Op: "read", Net: "tcp", Err: syscall.ECONNRESET}
			client := &http.Client{Transport: retryRoundTripper(func(*http.Request) (*http.Response, error) {
				attempts++
				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(retryReader(func([]byte) (int, error) {
					if cancelRead {
						cancel()
					}
					return 0, readErr
				}))}, nil
			})}
			d := &Downloader{retryBackoff: func(context.Context, int, time.Duration) error { return nil }}
			_, err := d.doWithRetry(ctx, client, http.MethodGet, "http://registry.invalid", func(resp *http.Response) error {
				_, err := io.ReadAll(resp.Body)
				return err
			})
			if cancelRead {
				Expect(errors.Is(err, context.Canceled)).To(BeTrue())
				Expect(attempts).To(Equal(1))
			} else {
				Expect(errors.Is(err, readErr)).To(BeTrue())
				Expect(attempts).To(Equal(downloadMaxAttempts))
			}
			var unavailable *RegistryUnavailableError
			Expect(errors.As(err, &unavailable)).To(BeFalse())
		},
		Entry("retrying connection resets", false),
		Entry("stopping immediately on caller cancellation", true),
	)
	It("cancels the production backoff timer before its delay elapses", func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
		defer cancel()
		Expect(defaultRetryBackoff(ctx, 1, time.Second)).To(MatchError(context.DeadlineExceeded))
	})
})
