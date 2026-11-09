package setup

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/jameswlane/devex/apps/cli/internal/bootstrap"
	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
	"github.com/jameswlane/devex/packages/plugin-sdk/sdktest"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Installed plugin validation integrity", func() {
	DescribeTable("reports verified and bypassed checks accurately", func(tampered, bypass, wantValid, wantVerified bool) {
		sdktest.NewRecorder(GinkgoT())
		payload := []byte("#!/bin/sh\nexit 0\n")
		var server *httptest.Server
		server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if strings.Contains(r.URL.Path, "/download/") {
				_, _ = w.Write(payload)
				return
			}
			_ = json.NewEncoder(w).Encode(sdk.PluginMetadata{PluginInfo: sdk.PluginInfo{Name: "demo", Version: "1.0.0"}, Binaries: map[string]sdk.PlatformBinary{runtime.GOOS + "-" + runtime.GOARCH: {URL: server.URL, Checksum: fmt.Sprintf("%x", sha256.Sum256(payload)), Size: int64(len(payload))}}})
		}))
		defer server.Close()
		GinkgoT().Setenv("DEVEX_PLUGIN_REGISTRY_URL", server.URL)
		boot, err := bootstrap.NewPluginBootstrap(true)
		Expect(err).NotTo(HaveOccurred())
		dir := boot.GetManager().GetPluginDir()
		Expect(os.MkdirAll(dir, 0700)).To(Succeed())
		installed := payload
		if tampered {
			installed = []byte("tampered")
		}
		Expect(os.WriteFile(filepath.Join(dir, "devex-plugin-demo"), installed, 0700)).To(Succeed())
		validator := NewPluginValidator(boot, PluginValidatorConfig{VerifyChecksums: true, AllowInsecurePlugins: bypass, VerificationTimeout: time.Second * 5})
		result := validator.validateSinglePlugin(context.Background(), "demo", map[string]bool{"demo": true})
		Expect(result.IsValid).To(Equal(wantValid))
		Expect(result.ChecksumValid).To(Equal(wantVerified))
		Expect(result.SignatureValid).To(BeFalse())
	},
		Entry("valid installed binary", false, false, true, true),
		Entry("tampered installed binary", true, false, false, false),
		Entry("explicit insecure bypass", true, true, true, false),
	)
})
