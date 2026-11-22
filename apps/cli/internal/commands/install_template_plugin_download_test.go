package commands

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
	"sync/atomic"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/viper"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"

	"github.com/jameswlane/devex/apps/cli/internal/bootstrap"
	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/state"
	"github.com/jameswlane/devex/apps/cli/internal/testhelper"
)

var _ = Describe("Template install with a plugin the bootstrap didn't download", func() {
	const pluginName = "package-manager-docker"
	var (
		requests     atomic.Int32
		marker       string
		templatePath string
		provider     pluginBootstrapProvider
	)

	BeforeEach(func() {
		if runtime.GOOS == "windows" {
			Skip("fixture requires POSIX shell")
		}
		viper.Reset()
		DeferCleanup(viper.Reset)
		home := GinkgoT().TempDir()
		testhelper.SetProfileEnv(GinkgoT(), home)
		for _, kind := range []string{"CONFIG", "DATA", "CACHE", "STATE"} {
			GinkgoT().Setenv("DEVEX_"+kind+"_DIR", filepath.Join(home, strings.ToLower(kind)))
		}
		GinkgoT().Setenv(state.DBPathEnvVar, filepath.Join(home, "state.db"))
		Expect(viper.BindEnv(state.DBPathConfigKey, state.DBPathEnvVar)).To(Succeed())

		marker = filepath.Join(home, "executions")
		info, err := json.Marshal(sdk.PluginInfo{Name: pluginName, Version: "1.0.0"})
		Expect(err).NotTo(HaveOccurred())
		payload := []byte("#!/bin/sh\nif [ \"$1\" = --plugin-info ]; then\n echo '" + string(info) + "'\n exit 0\nfi\necho \"$@\" >> '" + marker + "'\n")
		checksum := fmt.Sprintf("%x", sha256.Sum256(payload))
		requests.Store(0)
		var server *httptest.Server
		server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requests.Add(1)
			platformKey := runtime.GOOS + "-" + runtime.GOARCH
			switch r.URL.Path {
			case "/api/v1/plugins/" + pluginName:
				_ = json.NewEncoder(w).Encode(sdk.PluginMetadata{
					PluginInfo: sdk.PluginInfo{Name: pluginName, Version: "1.0.0"},
					Binaries:   map[string]sdk.PlatformBinary{platformKey: {URL: server.URL + "/binary", Checksum: checksum, Size: int64(len(payload))}},
				})
			case "/binary", "/api/v1/plugins/" + pluginName + "/download/" + platformKey:
				_, _ = w.Write(payload)
			default:
				http.NotFound(w, r)
			}
		}))
		DeferCleanup(server.Close)
		GinkgoT().Setenv("DEVEX_PLUGIN_REGISTRY_URL", server.URL)

		b, err := bootstrap.NewPluginBootstrap(false)
		Expect(err).NotTo(HaveOccurred())
		Expect(b.GetManager().ListPlugins()).NotTo(HaveKey(pluginName))
		provider = func() *bootstrap.PluginBootstrap { return b }

		templatePath = filepath.Join(home, "template.yaml")
		Expect(os.WriteFile(templatePath, []byte(`
schema_version: devex.template/v1
id: missing-plugin
name: Missing Plugin
version: 1.0.0
lifecycle:
  install:
    - id: redis
      plugin:
        name: package-manager-docker
        command: install
        args:
          - redis:7
`), 0o600)).To(Succeed())
	})

	run := func() error {
		return executeTemplateInstall(context.Background(), templatePath, templateInstallOptions{NonInteractive: true}, config.CrossPlatformSettings{}, provider)
	}

	It("downloads and verifies the plugin, then runs the step", func() {
		Expect(run()).To(Succeed())
		Expect(requests.Load()).To(BeNumerically(">", 0))
		executions, err := os.ReadFile(marker)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(executions)).To(ContainSubstring("install redis:7"))
	})

	DescribeTable("does not download when downloads are disabled", func(setting string) {
		viper.Set(setting, true)
		Expect(run()).To(MatchError(And(ContainSubstring(setting), ContainSubstring("devex plugin install "+pluginName))))
		Expect(requests.Load()).To(BeZero())
		Expect(marker).NotTo(BeAnExistingFile())
	}, Entry("offline", "offline"), Entry("skip download", "skip-plugin-download"))
})
