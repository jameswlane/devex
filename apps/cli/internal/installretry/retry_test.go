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
		err := p.Run(context.Background(), APT, &output, func(w io.Writer) error { attempts++; _, _ = fmt.Fprint(w, transient); return failure })
		Expect(attempts).To(Equal(5))
		Expect(waits).To(Equal([]time.Duration{time.Second, 2 * time.Second, 3 * time.Second, 3 * time.Second}))
		Expect(errors.Is(err, failure)).To(BeTrue())
		Expect(output.String()).To(ContainSubstring("failed after 5 attempts"))
		Expect(strings.Count(output.String(), transient)).To(Equal(5))
	})
	It("stops executing once an attempt succeeds", func() {
		err := p.Run(context.Background(), APT, &output, func(w io.Writer) error {
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
		err := p.Run(context.Background(), APT, &output, func(w io.Writer) error {
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
		err := p.Run(ctx, APT, &output, func(w io.Writer) error { attempts++; _, _ = fmt.Fprint(w, transient); return failure })
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
		Expect(p.Run(ctx, APT, &output, func(io.Writer) error { attempts++; return nil })).To(MatchError(context.Canceled))
		Expect(attempts).To(BeZero())
	})
	It("allows disabling retries with a single attempt", func() {
		p.Attempts = 1
		Expect(p.Run(context.Background(), APT, &output, func(w io.Writer) error { attempts++; _, _ = fmt.Fprint(w, transient); return failure })).To(HaveOccurred())
		Expect(attempts).To(Equal(1))
		Expect(waits).To(BeEmpty())
	})
	DescribeTable("classifies native acquisition failures conservatively", func(diagnostic string, retry bool) {
		Expect(APT.Transient(failure, diagnostic)).To(Equal(retry))
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
	DescribeTable("limits eligible operations", func(plugin string, args []string, eligible bool) {
		_, ok := Eligible(plugin, args)
		Expect(ok).To(Equal(eligible))
	},
		Entry("install", "package-manager-apt", []string{"install", "git", "libc6:amd64"}, true),
		Entry("remove", "package-manager-apt", []string{"remove", "git"}, false),
		Entry("package removal suffix", "package-manager-apt", []string{"install", "git-"}, false),
		Entry("unsafe option", "package-manager-apt", []string{"install", "--reinstall", "git"}, false),
		Entry("script", "package-manager-script", []string{"install", "script.sh"}, false),
		Entry("dnf install", "package-manager-dnf", []string{"install", "git", "NetworkManager", "perl-Net_SSLeay", "vim-2:9.1.0-1.fc44.x86_64"}, true),
		Entry("dnf remove", "package-manager-dnf", []string{"remove", "git"}, false),
		Entry("dnf group", "package-manager-dnf", []string{"install", "@development-tools"}, false),
		Entry("dnf glob", "package-manager-dnf", []string{"install", "python3-*"}, false),
		Entry("dnf local or remote rpm", "package-manager-dnf", []string{"install", "https://example.com/x.rpm"}, false),
		Entry("dnf relative rpm file", "package-manager-dnf", []string{"install", "example.rpm"}, false),
		Entry("dnf option", "package-manager-dnf", []string{"install", "--nogpgcheck", "git"}, false),
		Entry("other platform", "package-manager-pacman", []string{"install", "git"}, false))
	It("labels DNF retry notices", func() {
		err := p.Run(context.Background(), DNF, &output, func(w io.Writer) error {
			attempts++
			_, _ = fmt.Fprint(w, dnf5PackageDownload("Curl error (52): Server returned nothing (no headers, no data)"))
			return errors.New("exit status 1")
		})
		Expect(err).To(MatchError(ContainSubstring("DNF install failed after 5 attempts")))
		Expect(output.String()).To(ContainSubstring("DNF install attempt 1/5 failed"))
		Expect(attempts).To(Equal(5))
	})
	// Diagnostics below were captured from real dnf5 5.4 and dnf4 4.24 runs
	// against a local fault-injecting repository; only URLs are shortened.
	DescribeTable("classifies native DNF failures conservatively", func(diagnostic string, retry bool) {
		Expect(DNF.Transient(errors.New("exit status 1"), diagnostic)).To(Equal(retry))
	}, Entry("dnf5 metadata DNS", dnf5Metadata("Curl error (6): Could not resolve hostname for http://repo/repodata/repomd.xml [Could not resolve host: repo]"), true),
		Entry("dnf5 metadata refused", dnf5Metadata("Curl error (7): Could not connect to server for http://repo/repodata/repomd.xml [Failed to connect to repo port 80 after 0 ms: Could not connect to server]"), true),
		Entry("dnf5 metadata timeout", dnf5Metadata("Curl error (28): Timeout was reached for http://repo/repodata/repomd.xml [Connection timed out after 2000 milliseconds]"), true),
		Entry("dnf5 metadata 503", dnf5Metadata("Status code: 503 for http://repo/repodata/repomd.xml (IP: 127.0.0.1)"), true),
		Entry("dnf5 metadata 404", dnf5Metadata("Status code: 404 for http://repo/repodata/repomd.xml (IP: 127.0.0.1)"), false),
		Entry("dnf5 package 503", dnf5PackageDownload("Status code: 503 for http://repo/p.rpm (IP: 127.0.0.1)"), true),
		Entry("dnf5 package 503 with the plugin SDK exit line", dnf5PackageDownload("Status code: 503 for http://repo/p.rpm (IP: 127.0.0.1)")+"Error: exit status 1\n", true),
		Entry("plugin SDK authorization failure", dnf5PackageDownload("Status code: 503 for http://repo/p.rpm (IP: 127.0.0.1)")+"Error: sudo: a password is required\nDEVEX_AUTHORIZATION_ERROR={}\n", false),
		Entry("dnf5 package 429", dnf5PackageDownload("Status code: 429 for http://repo/p.rpm (IP: 127.0.0.1)"), true),
		Entry("dnf5 package empty reply", dnf5PackageDownload("Curl error (52): Server returned nothing (no headers, no data) for http://repo/p.rpm [Empty reply from server]"), true),
		Entry("dnf5 package 404", dnf5PackageDownload("Status code: 404 for http://repo/p.rpm (IP: 127.0.0.1)"), false),
		Entry("dnf5 package 403", dnf5PackageDownload("Status code: 403 for http://repo/p.rpm (IP: 127.0.0.1)"), false),
		Entry("dnf5 mixed mirrors", dnf5PackageDownload("Status code: 503 for http://a/p.rpm (IP: 127.0.0.1)")+">>> Status code: 404 for http://b/p.rpm (IP: 127.0.0.2)\n", false),
		Entry("dnf5 TLS certificate", dnf5Metadata("Curl error (60): SSL peer certificate or SSH remote key was not OK for https://repo/repodata/repomd.xml"), false),
		Entry("dnf5 missing package", "Updating and loading repositories:\nRepositories loaded.\nFailed to resolve the transaction:\nNo match for argument: nosuchpkg\n", false),
		Entry("dnf5 rpm transaction", "Running transaction\nTransaction failed: Rpm transaction failed.\n", false),
		Entry("dnf5 signature", dnf5PackageDownload("Status code: 503 for http://repo/p.rpm (IP: 127.0.0.1)")+"OpenPGP check for package \"p\" has failed\n", false),
		Entry("cause without download summary", ">>> Curl error (7): Could not connect to server\n", false),
		Entry("dnf4 metadata refused", "Errors during downloading metadata for repository 't':\n  - Curl error (7): Could not connect to server for http://repo/repodata/repomd.xml [Failed to connect to repo port 80 after 0 ms: Could not connect to server]\nError: Failed to download metadata for repo 't': Cannot download repomd.xml: Cannot download repodata/repomd.xml: All mirrors were tried\n", true),
		Entry("dnf4 metadata 503", "Errors during downloading metadata for repository 't':\n  - Status code: 503 for http://repo/repodata/repomd.xml (IP: 127.0.0.1)\nError: Failed to download metadata for repo 't': Cannot download repomd.xml: Cannot download repodata/repomd.xml: All mirrors were tried\n", true),
		Entry("dnf4 metadata 404", "Errors during downloading metadata for repository 't':\n  - Status code: 404 for http://repo/repodata/repomd.xml (IP: 127.0.0.1)\nError: Failed to download metadata for repo 't': Cannot download repomd.xml: Cannot download repodata/repomd.xml: All mirrors were tried\n", false),
		Entry("dnf4 package download without a cause on stderr", "Error: Error downloading packages:\n  p-1.0-1.noarch: Cannot download, all mirrors were already tried without success\n", false),
		Entry("dnf4 missing package", "Error: Unable to find a match: nosuchpkg\n", false),
		Entry("dnf4 other fatal error", "Errors during downloading metadata for repository 't':\n  - Status code: 503 for http://repo/repodata/repomd.xml (IP: 127.0.0.1)\nError: Failed to download metadata for repo 't'\nError: Transaction test error\n", false))
	It("keeps random waits inside the equal-jitter interval", func() {
		for i := 0; i < 100; i++ {
			d := defaultJitter(8 * time.Second)
			Expect(d).To(BeNumerically(">=", 4*time.Second))
			Expect(d).To(BeNumerically("<=", 8*time.Second))
		}
	})
	It("validates bounds before executing any operation", func() {
		p.Attempts = 0
		Expect(p.Run(context.Background(), APT, &output, func(io.Writer) error { attempts++; return nil })).To(HaveOccurred())
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

func dnf5Metadata(cause string) string {
	return "Updating and loading repositories:\n>>> " + cause + "\n>>> " + cause + "\n>>> Usable URL not found\nFailed to download metadata (baseurl: \"http://repo/\") for repository \"t\": Usable URL not found\n"
}

func dnf5PackageDownload(cause string) string {
	return "Updating and loading repositories:\nRepositories loaded.\n>>> " + cause + "\n>>> No more mirrors to try - All mirrors were already tried without success\nFailed to download packages\n Librepo error: Cannot download p.rpm: All mirrors were tried\n"
}
