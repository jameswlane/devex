package tui

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type artifactTransport func(*http.Request) (*http.Response, error)

func (f artifactTransport) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

var _ = Describe("Streaming download integrity", func() {
	BeforeEach(func() {
		GinkgoT().Setenv("HOME", GinkgoT().TempDir())
		previous := http.DefaultTransport
		http.DefaultTransport = artifactTransport(func(req *http.Request) (*http.Response, error) {
			return &http.Response{StatusCode: http.StatusOK, Header: make(http.Header), Body: io.NopCloser(strings.NewReader("#!/bin/bash\nexit 91\n")), Request: req}, nil
		})
		DeferCleanup(func() { http.DefaultTransport = previous })
	})

	It("rejects a tampered script before execution", func() {
		installer := &StreamingInstaller{ctx: context.Background(), config: InstallerConfig{HTTPTimeout: time.Second, MaxScriptSize: 1024}}
		err := installer.downloadValidateExecute(context.Background(), "tool", "https://example.test/script", sdk.ArtifactVerification{Policy: "required", SHA256: fmt.Sprintf("%x", sha256.Sum256([]byte("expected")))})
		Expect(err).To(MatchError(ContainSubstring("checksum mismatch")))
	})
	It("retains the script size limit on checksum-pinned downloads", func() {
		installer := &StreamingInstaller{ctx: context.Background(), config: InstallerConfig{HTTPTimeout: time.Second, MaxScriptSize: 1}}
		digest := fmt.Sprintf("%x", sha256.Sum256([]byte("#!/bin/bash\nexit 91\n")))
		Expect(installer.downloadValidateExecute(context.Background(), "tool", "https://example.test/script", sdk.ArtifactVerification{SHA256: digest})).To(MatchError(ContainSubstring("maximum allowed size")))
	})

	It("rejects missing required metadata before execution", func() {
		installer := &StreamingInstaller{ctx: context.Background(), config: InstallerConfig{HTTPTimeout: time.Second, MaxScriptSize: 1024}}
		Expect(installer.downloadValidateExecute(context.Background(), "tool", "https://example.test/script", sdk.ArtifactVerification{Policy: "required"})).To(MatchError(ContainSubstring("required verification metadata is missing")))
	})
})
