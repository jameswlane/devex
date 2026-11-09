package installretry

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/viper"
)

func TestRetry(t *testing.T) { RegisterFailHandler(Fail); RunSpecs(t, "Install Retry Suite") }

var _ = Describe("Bounded install retries", func() {
	var p Policy
	var waits []time.Duration
	var output bytes.Buffer
	var attempts int
	failure := errors.New("exit status 100")
	transient := "E: Failed to fetch https://repo/p.deb 503 Service Unavailable\n"
	BeforeEach(func() {
		waits = nil
		attempts = 0
		output.Reset()
		p = Policy{Attempts: 5, InitialDelay: time.Second, MaxDelay: 3 * time.Second,
			Sleep:  func(_ context.Context, d time.Duration) error { waits = append(waits, d); return nil },
			Jitter: func(d time.Duration) time.Duration { return d }}
	})
	It("caps exponential delays, bounds executions, forwards stderr and preserves final cause", func() {
		err := p.Run(context.Background(), &output, func(w io.Writer) error { attempts++; _, _ = fmt.Fprint(w, transient); return failure })
		Expect(attempts).To(Equal(5))
		Expect(waits).To(Equal([]time.Duration{time.Second, 2 * time.Second, 3 * time.Second, 3 * time.Second}))
		Expect(errors.Is(err, failure)).To(BeTrue())
		Expect(output.String()).To(ContainSubstring("failed after 5 attempts"))
		Expect(strings.Count(output.String(), transient)).To(Equal(5))
	})
	It("stops executing once an attempt succeeds", func() {
		err := p.Run(context.Background(), &output, func(w io.Writer) error {
			attempts++
			if attempts == 1 {
				_, _ = fmt.Fprint(w, transient)
				return failure
			}
			return nil
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(attempts).To(Equal(2))
	})
	It("does not retry a permanent failure or use stale diagnostics from an earlier attempt", func() {
		err := p.Run(context.Background(), &output, func(w io.Writer) error {
			attempts++
			if attempts == 1 {
				_, _ = fmt.Fprint(w, transient)
			}
			return failure
		})
		Expect(err).To(BeIdenticalTo(failure))
		Expect(attempts).To(Equal(2))
		Expect(waits).To(HaveLen(1))
	})
	It("cancels during injected sleep before another execution", func() {
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		p.Sleep = func(ctx context.Context, _ time.Duration) error { cancel(); return ctx.Err() }
		err := p.Run(ctx, &output, func(w io.Writer) error { attempts++; _, _ = fmt.Fprint(w, transient); return failure })
		Expect(err).To(MatchError(context.Canceled))
		Expect(attempts).To(Equal(1))
	})
	It("cancels the real timer immediately", func() {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		Expect(sleep(ctx, time.Hour)).To(MatchError(context.Canceled))
	})
	It("does not invoke an operation with an already canceled context", func() {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		Expect(p.Run(ctx, &output, func(io.Writer) error { attempts++; return nil })).To(MatchError(context.Canceled))
		Expect(attempts).To(BeZero())
	})
	It("allows disabling retries with a single attempt", func() {
		p.Attempts = 1
		Expect(p.Run(context.Background(), &output, func(w io.Writer) error { attempts++; _, _ = fmt.Fprint(w, transient); return failure })).To(HaveOccurred())
		Expect(attempts).To(Equal(1))
		Expect(waits).To(BeEmpty())
	})
	DescribeTable("classifies native acquisition failures conservatively", func(diagnostic string, retry bool) {
		Expect(Transient(failure, diagnostic)).To(Equal(retry))
	}, Entry("DNS", "E: Failed to fetch https://repo Temporary failure resolving 'repo'", true),
		Entry("timeout", "E: Failed to fetch https://repo Connection timed out", true),
		Entry("mixed permanent and transient archives", "E: Failed to fetch https://repo/a.deb 503 Service Unavailable\nE: Failed to fetch https://repo/b.deb 404 Not Found", false),
		Entry("unrelated temporary warning", "W: Connection timed out\nE: Failed to fetch https://repo/b.deb 404 Not Found", false),
		Entry("permanent 404", "E: Failed to fetch https://repo 404 Not Found", false),
		Entry("missing package", "Unable to locate package git", false),
		Entry("stale update warning", "W: Failed to fetch https://repo 503 Service Unavailable\nE: Package git is not available", false),
		Entry("unexpected fatal error", "E: Failed to fetch https://repo 503 Service Unavailable\nE: Invalid operation", false),
		Entry("configuration", "invalid config: timeout", false),
		Entry("dpkg script failure", "E: Failed to fetch https://repo 503 Service Unavailable\ndpkg: post-installation script failed", false),
		Entry("authorization", "E: Failed to fetch https://repo 503 Service Unavailable\npermission denied", false),
		Entry("signature", "E: Failed to fetch https://repo 503 Service Unavailable\nrepository is not signed", false))
	DescribeTable("limits eligible operations", func(plugin string, args []string, eligible bool) { Expect(Eligible(plugin, args)).To(Equal(eligible)) },
		Entry("install", "package-manager-apt", []string{"install", "git", "libc6:amd64"}, true),
		Entry("remove", "package-manager-apt", []string{"remove", "git"}, false),
		Entry("package removal suffix", "package-manager-apt", []string{"install", "git-"}, false),
		Entry("unsafe option", "package-manager-apt", []string{"install", "--reinstall", "git"}, false),
		Entry("script", "package-manager-script", []string{"install", "script.sh"}, false),
		Entry("other platform", "package-manager-dnf", []string{"install", "git"}, false))
	It("keeps random waits inside the equal-jitter interval", func() {
		for i := 0; i < 100; i++ {
			d := defaultJitter(8 * time.Second)
			Expect(d).To(BeNumerically(">=", 4*time.Second))
			Expect(d).To(BeNumerically("<=", 8*time.Second))
		}
	})
	It("validates bounds before executing any operation", func() {
		p.Attempts = 0
		Expect(p.Run(context.Background(), &output, func(io.Writer) error { attempts++; return nil })).To(HaveOccurred())
		Expect(attempts).To(BeZero())
	})

	It("bounds retained diagnostics", func() {
		tail := &diagnosticTail{}
		_, err := tail.Write([]byte(strings.Repeat("x", 100000)))
		Expect(err).NotTo(HaveOccurred())
		_, err = tail.Write([]byte("last"))
		Expect(err).NotTo(HaveOccurred())
		Expect(tail.data).To(HaveLen(32768))
		Expect(string(tail.data)).To(HaveSuffix("last"))
	})
	It("reads environment and config with validation before execution", func() {
		viper.Reset()
		DeferCleanup(viper.Reset)
		viper.SetEnvPrefix("DEVEX")
		viper.SetEnvKeyReplacer(strings.NewReplacer("-", "_"))
		viper.AutomaticEnv()
		GinkgoT().Setenv("DEVEX_INSTALL_RETRY_ATTEMPTS", "2")
		configured, err := Configured()
		Expect(err).NotTo(HaveOccurred())
		Expect(configured.Attempts).To(Equal(2))
		viper.Set("install-retry-attempts", 6)
		_, err = Configured()
		Expect(err).To(HaveOccurred())
		viper.Set("install-retry-attempts", 1)
		viper.Set("install-retry-delay", "2m")
		_, err = Configured()
		Expect(err).To(HaveOccurred())
	})
})
