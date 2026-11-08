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
	"sync/atomic"

	"github.com/jameswlane/devex/apps/cli/internal/bootstrap"
	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/platform"
	"github.com/jameswlane/devex/apps/cli/internal/types"
	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/viper"
)

var _ = Describe("Dynamic action plugin installation", func() {
	var server *httptest.Server
	var payload []byte
	var checksum, pluginDir, marker, inputPath string
	var requests atomic.Int32
	var executor *ActionExecutor
	var action *types.StepAction
	var state *types.SetupState

	BeforeEach(func() {
		if runtime.GOOS == "windows" {
			Skip("fixture requires POSIX shell")
		}
		viper.Reset()
		DeferCleanup(viper.Reset)
		home := GinkgoT().TempDir()
		GinkgoT().Setenv("HOME", home)
		for _, kind := range []string{"CONFIG", "DATA", "CACHE", "STATE"} {
			GinkgoT().Setenv("DEVEX_"+kind+"_DIR", filepath.Join(home, strings.ToLower(kind)))
		}
		original := sdk.NewPrivilegeResolver
		resolver := original()
		resolver.Elevated = false
		sdk.NewPrivilegeResolver = func() sdk.PrivilegeResolver { return resolver }
		DeferCleanup(func() { sdk.NewPrivilegeResolver = original })
		marker = filepath.Join(home, "executions")
		inputPath = filepath.Join(home, "input.json")
		info, err := json.Marshal(sdk.PluginInfo{Name: "action-recorder", Version: "1.0.0", Commands: []sdk.PluginCommand{{Name: "setup"}}})
		Expect(err).NotTo(HaveOccurred())
		payload = []byte("#!/bin/sh\nif [ \"$1\" = --plugin-info ]; then\n echo '" + string(info) + "'\n exit 0\nfi\nprintf '%s\\n' \"$1\" >> '" + marker + "'\ncat > '" + inputPath + "'\necho '{\"status\":\"success\"}'\n")
		checksum = fmt.Sprintf("%x", sha256.Sum256(payload))
		requests.Store(0)
		server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requests.Add(1)
			metadata := sdk.PluginMetadata{PluginInfo: sdk.PluginInfo{Name: "action-recorder", Version: "1.0.0"}, Binaries: map[string]sdk.PlatformBinary{runtime.GOOS + "-" + runtime.GOARCH: {URL: server.URL + "/binary", Checksum: checksum, Size: int64(len(payload))}}}
			switch r.URL.Path {
			case "/api/v1/plugins/action-recorder":
				_ = json.NewEncoder(w).Encode(metadata)
			case "/binary", "/api/v1/plugins/action-recorder/download/" + runtime.GOOS + "-" + runtime.GOARCH:
				_, _ = w.Write(payload)
			default:
				http.NotFound(w, r)
			}
		}))
		DeferCleanup(server.Close)
		GinkgoT().Setenv("DEVEX_PLUGIN_REGISTRY_URL", server.URL)
		b, err := bootstrap.NewPluginBootstrap(false)
		Expect(err).NotTo(HaveOccurred())
		pluginDir = b.GetManager().GetPluginDir()
		executor = NewActionExecutor(b, config.CrossPlatformSettings{}, platform.DetectionResult{OS: runtime.GOOS}, nil)
		action = &types.StepAction{Type: types.ActionTypePlugin, Params: map[string]interface{}{"plugin_name": "action-recorder", "config_mapping": map[string]interface{}{"selected": "{{choice}}"}}}
		state = &types.SetupState{Answers: map[string]interface{}{"choice": "expected"}}
	})

	It("downloads, verifies, rediscovers and executes with the setup input, then reuses the installation offline", func() {
		Expect(executor.Execute(context.Background(), action, state)).To(Succeed())
		Expect(requests.Load()).To(BeNumerically(">", 0))
		Expect(filepath.Join(pluginDir, ".devex-plugin-action-recorder.install.json")).To(BeAnExistingFile())
		data, err := os.ReadFile(inputPath)
		Expect(err).NotTo(HaveOccurred())
		var input sdk.PluginSetupInput
		Expect(json.Unmarshal(data, &input)).To(Succeed())
		Expect(input.Command).To(Equal("setup"))
		Expect(input.Config).To(HaveKeyWithValue("selected", "expected"))
		Expect(input.Parameters).To(HaveKeyWithValue("choice", "expected"))
		count := requests.Load()
		viper.Set("offline", true)
		viper.Set("skip-plugin-download", true)
		Expect(executor.Execute(context.Background(), action, state)).To(Succeed())
		Expect(requests.Load()).To(Equal(count))
		executions, err := os.ReadFile(marker)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(executions)).To(Equal("setup\nsetup\n"))
	})

	DescribeTable("rejects missing plugins without network access when downloads are disabled", func(setting string) {
		viper.Set(setting, true)
		err := executor.Execute(context.Background(), action, state)
		Expect(err).To(MatchError(And(ContainSubstring(setting), ContainSubstring("devex plugin install action-recorder"))))
		Expect(requests.Load()).To(BeZero())
		Expect(marker).NotTo(BeAnExistingFile())
	}, Entry("offline", "offline"), Entry("skip download", "skip-plugin-download"))

	It("honors the bootstrap's skip policy even without a Viper override", func() {
		b, err := bootstrap.NewPluginBootstrap(true)
		Expect(err).NotTo(HaveOccurred())
		executor.pluginBootstrap = b
		Expect(executor.Execute(context.Background(), action, state)).To(MatchError(ContainSubstring("skip-plugin-download")))
		Expect(requests.Load()).To(BeZero())
	})

	It("does not install or execute a binary whose checksum fails verification", func() {
		checksum = strings.Repeat("0", 64)
		Expect(executor.Execute(context.Background(), action, state)).To(MatchError(ContainSubstring("checksum verification failed")))
		Expect(marker).NotTo(BeAnExistingFile())
		Expect(filepath.Join(pluginDir, "devex-plugin-action-recorder")).NotTo(BeAnExistingFile())
		Expect(filepath.Join(pluginDir, ".devex-plugin-action-recorder.install.json")).NotTo(BeAnExistingFile())
	})

	It("does not install or execute a verified payload with invalid executable metadata", func() {
		payload = []byte("#!/bin/sh\necho invalid-metadata\n")
		checksum = fmt.Sprintf("%x", sha256.Sum256(payload))
		Expect(executor.Execute(context.Background(), action, state)).To(MatchError(ContainSubstring("invalid metadata")))
		Expect(filepath.Join(pluginDir, "devex-plugin-action-recorder")).NotTo(BeAnExistingFile())
		Expect(marker).NotTo(BeAnExistingFile())
	})

	It("rejects invalid names before contacting the registry", func() {
		action.Params["plugin_name"] = "../invalid"
		Expect(executor.Execute(context.Background(), action, state)).To(MatchError(ContainSubstring("invalid plugin name")))
		Expect(requests.Load()).To(BeZero())
	})

	It("honors cancellation before downloading or executing", func() {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		Expect(executor.Execute(ctx, action, state)).To(MatchError(context.Canceled))
		Expect(requests.Load()).To(BeZero())
		Expect(marker).NotTo(BeAnExistingFile())
	})

	It("reports an unavailable bootstrap without panicking", func() {
		executor.pluginBootstrap = nil
		Expect(executor.Execute(context.Background(), action, state)).To(MatchError(ContainSubstring("initialize plugins")))
	})
})
