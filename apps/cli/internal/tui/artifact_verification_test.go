package tui

import (
	"context"
	"crypto/sha256"
	"fmt"
	"net/http"
	"net/http/httptest"
	"time"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Streaming download integrity", func() {
	It("rejects a tampered script before execution", func() {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = fmt.Fprint(w, "#!/bin/bash\nexit 91\n") }))
		defer server.Close()
		installer := &StreamingInstaller{ctx: context.Background(), config: InstallerConfig{HTTPTimeout: time.Second, MaxScriptSize: 1024}}
		err := installer.downloadValidateExecute(context.Background(), "tool", server.URL, sdk.ArtifactVerification{Policy: "required", SHA256: fmt.Sprintf("%x", sha256.Sum256([]byte("expected")))})
		Expect(err).To(MatchError(ContainSubstring("checksum mismatch")))
	})
	It("rejects missing required metadata before execution", func() {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = fmt.Fprint(w, "#!/bin/bash\nexit 91\n") }))
		defer server.Close()
		installer := &StreamingInstaller{ctx: context.Background(), config: InstallerConfig{HTTPTimeout: time.Second, MaxScriptSize: 1024}}
		Expect(installer.downloadValidateExecute(context.Background(), "tool", server.URL, sdk.ArtifactVerification{Policy: "required"})).To(MatchError(ContainSubstring("required verification metadata is missing")))
	})
})
