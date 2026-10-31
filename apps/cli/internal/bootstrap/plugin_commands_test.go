package bootstrap_test

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/jameswlane/devex/apps/cli/internal/commands"
	"github.com/jameswlane/devex/apps/cli/internal/config"
	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/viper"
)

var _ = Describe("Registered plugin commands", func() {
	var home, dir string
	var registry *httptest.Server
	var payload []byte
	var checksum string
	var requests []string
	var stdout, stderr bytes.Buffer
	var execute func(context.Context, ...string) error

	script := func(name, version string) []byte {
		info, err := json.Marshal(sdk.PluginInfo{Name: name, Version: version, Description: "recorder", Commands: []sdk.PluginCommand{{Name: "record"}, {Name: "fail"}, {Name: "wait"}}})
		Expect(err).NotTo(HaveOccurred())
		return []byte("#!/bin/sh\nif [ \"$1\" = --plugin-info ]; then\n printf '%s\\n' '" + string(info) + "'\n exit 0\nfi\ncase \"$1\" in\n fail) echo failure >&2; exit 23;;\n wait) exec sleep 30;;\nesac\nprintf '%s\\n' \"$@\"\ncat\necho diagnostic >&2\n")
	}
	install := func(name, version string) string {
		path := filepath.Join(dir, "devex-plugin-"+name)
		Expect(os.WriteFile(path, script(name, version), 0o755)).To(Succeed())
		return path
	}

	BeforeEach(func() {
		if runtime.GOOS == "windows" {
			Skip("shell recorder requires a POSIX host")
		}
		home = GinkgoT().TempDir()
		GinkgoT().Setenv("HOME", home)
		// Keep the fake home authoritative even when the suite is launched under sudo.
		original := sdk.NewPrivilegeResolver
		resolver := original()
		resolver.Elevated = false
		sdk.NewPrivilegeResolver = func() sdk.PrivilegeResolver { return resolver }
		DeferCleanup(func() { sdk.NewPrivilegeResolver = original })
		viper.Reset()
		DeferCleanup(viper.Reset)
		dir = filepath.Join(home, ".devex", "plugins")
		Expect(os.MkdirAll(dir, 0o750)).To(Succeed())
		payload = script("recorder", "2.0.0")
		checksum = fmt.Sprintf("%x", sha256.Sum256(payload))
		requests = nil
		registry = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			requests = append(requests, r.URL.Path)
			metadata := sdk.PluginMetadata{PluginInfo: sdk.PluginInfo{Name: "recorder", Version: "2.0.0", Description: "recorder plugin"}, Binaries: map[string]sdk.PlatformBinary{runtime.GOOS + "-" + runtime.GOARCH: {URL: registry.URL + "/binary", Checksum: checksum, Size: int64(len(payload))}}}
			switch r.URL.Path {
			case "/api/v1/registry":
				_ = json.NewEncoder(w).Encode(sdk.PluginRegistry{Plugins: map[string]sdk.PluginMetadata{"recorder": metadata}})
			case "/api/v1/plugins/recorder":
				_ = json.NewEncoder(w).Encode(metadata)
			case "/binary", "/api/v1/plugins/recorder/download/" + runtime.GOOS + "-" + runtime.GOARCH:
				_, _ = w.Write(payload)
			default:
				http.NotFound(w, r)
			}
		}))
		DeferCleanup(registry.Close)
		GinkgoT().Setenv("DEVEX_PLUGIN_REGISTRY_URL", registry.URL)
		stdout.Reset()
		stderr.Reset()
		execute = func(ctx context.Context, args ...string) error {
			stdout.Reset()
			stderr.Reset()
			root := commands.NewRootCmd("test", nil, config.CrossPlatformSettings{})
			root.SetIn(strings.NewReader("input\n"))
			root.SetOut(&stdout)
			root.SetErr(&stderr)
			root.SetArgs(append([]string{"plugin"}, args...))
			return root.ExecuteContext(ctx)
		}
	})

	It("registers useful help without contacting the registry", func() {
		Expect(execute(context.Background(), "--help")).To(Succeed())
		Expect(stdout.String()).To(ContainSubstring("run"))
		Expect(stdout.String()).To(ContainSubstring("update"))
		Expect(requests).To(BeEmpty())
	})
	It("lists installed versions, paths, latest versions, and update status as JSON and a table", func() {
		path := install("recorder", "1.0.0")
		Expect(execute(context.Background(), "list", "--format", "json")).To(Succeed())
		var rows []map[string]any
		Expect(json.Unmarshal(stdout.Bytes(), &rows)).To(Succeed())
		Expect(rows).To(HaveLen(1))
		Expect(rows[0]).To(HaveKeyWithValue("version", "1.0.0"))
		Expect(rows[0]).To(HaveKeyWithValue("latest_version", "2.0.0"))
		Expect(rows[0]).To(HaveKeyWithValue("path", path))
		Expect(rows[0]).To(HaveKeyWithValue("update_available", true))
		Expect(execute(context.Background(), "list")).To(Succeed())
		Expect(stdout.String()).To(ContainSubstring("update available"))
	})
	It("handles an absent plugin directory and invalid list format", func() {
		Expect(os.Remove(dir)).To(Succeed())
		Expect(execute(context.Background(), "list", "--format", "json")).To(Succeed())
		Expect(strings.TrimSpace(stdout.String())).To(Equal("[]"))
		Expect(execute(context.Background(), "list", "--format", "yaml")).To(MatchError(ContainSubstring("use table or json")))
	})
	It("shows local metadata and forwards arguments, stdin, stdout, and stderr without registry access", func() {
		install("recorder", "1.0.0")
		Expect(execute(context.Background(), "info", "recorder")).To(Succeed())
		Expect(stdout.String()).To(ContainSubstring("Version: 1.0.0"))
		Expect(execute(context.Background(), "run", "recorder", "record", "--literal", "two words", "--", "-x")).To(Succeed())
		Expect(stdout.String()).To(Equal("record\n--literal\ntwo words\n--\n-x\ninput\n"))
		Expect(stderr.String()).To(ContainSubstring("diagnostic"))
		Expect(requests).To(BeEmpty())
	})
	It("preserves child exit status and cancellation", func() {
		install("recorder", "1.0.0")
		err := execute(context.Background(), "run", "recorder", "fail")
		var child *exec.ExitError
		Expect(errors.As(err, &child)).To(BeTrue())
		Expect(child.ExitCode()).To(Equal(23))
		ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
		defer cancel()
		Expect(execute(ctx, "run", "recorder", "wait")).To(MatchError(context.DeadlineExceeded))
	})
	It("rejects missing plugins, unknown commands, traversal, and incomplete arguments", func() {
		install("recorder", "1.0.0")
		Expect(execute(context.Background(), "run", "absent", "record")).To(MatchError(ContainSubstring("devex plugin install absent")))
		Expect(execute(context.Background(), "run", "recorder", "unknown")).To(MatchError(ContainSubstring("unknown command")))
		Expect(execute(context.Background(), "install", "../bad")).To(HaveOccurred())
		Expect(execute(context.Background(), "run", "recorder")).To(HaveOccurred())
		Expect(requests).To(BeEmpty())
	})
	It("searches and installs checksum-verified plugins through the registry", func() {
		Expect(execute(context.Background(), "search", "recorder")).To(Succeed())
		Expect(stdout.String()).To(ContainSubstring("recorder 2.0.0"))
		Expect(execute(context.Background(), "install", "recorder")).To(Succeed())
		Expect(filepath.Join(dir, "devex-plugin-recorder")).To(BeAnExistingFile())
		Expect(filepath.Join(dir, ".devex-plugin-recorder.install.json")).To(BeAnExistingFile())
	})
	It("updates named plugins and --all and removes binaries with their install records", func() {
		install("recorder", "1.0.0")
		Expect(execute(context.Background(), "update", "recorder")).To(Succeed())
		Expect(execute(context.Background(), "info", "recorder")).To(Succeed())
		Expect(stdout.String()).To(ContainSubstring("Version: 2.0.0"))
		Expect(execute(context.Background(), "update", "--all")).To(Succeed())
		Expect(execute(context.Background(), "remove", "recorder")).To(Succeed())
		Expect(stdout.String()).To(ContainSubstring("Commands that will stop working"))
		Expect(filepath.Join(dir, "devex-plugin-recorder")).NotTo(BeAnExistingFile())
		Expect(filepath.Join(dir, ".devex-plugin-recorder.install.json")).NotTo(BeAnExistingFile())
	})
	It("retains the previous binary and record when an update fails checksum verification", func() {
		path := install("recorder", "1.0.0")
		record := filepath.Join(dir, ".devex-plugin-recorder.install.json")
		Expect(os.WriteFile(record, []byte("old record"), 0o600)).To(Succeed())
		checksum = strings.Repeat("0", 64)
		Expect(execute(context.Background(), "update", "--all")).To(MatchError(ContainSubstring("checksum verification failed")))
		data, err := os.ReadFile(path)
		Expect(err).NotTo(HaveOccurred())
		Expect(data).To(Equal(script("recorder", "1.0.0")))
		data, err = os.ReadFile(record)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(data)).To(Equal("old record"))
	})
	It("keeps the previous executable when a checksum-valid download has invalid metadata", func() {
		path := install("recorder", "1.0.0")
		payload = []byte("#!/bin/sh\necho invalid-json\n")
		checksum = fmt.Sprintf("%x", sha256.Sum256(payload))
		Expect(execute(context.Background(), "update", "recorder")).To(MatchError(ContainSubstring("invalid metadata")))
		data, err := os.ReadFile(path)
		Expect(err).NotTo(HaveOccurred())
		Expect(data).To(Equal(script("recorder", "1.0.0")))
	})

	It("protects critical plugins unless forced and identifies affected commands", func() {
		path := install("package-manager-apt", "1.0.0")
		Expect(execute(context.Background(), "remove", "package-manager-apt")).To(MatchError(ContainSubstring("without --force")))
		Expect(path).To(BeAnExistingFile())
		Expect(execute(context.Background(), "remove", "package-manager-apt", "--force")).To(Succeed())
		Expect(stdout.String()).To(ContainSubstring("devex install"))
		Expect(path).NotTo(BeAnExistingFile())
	})
	It("continues update --all after a failure and reports it", func() {
		install("a-missing", "1.0.0")
		install("recorder", "1.0.0")
		Expect(execute(context.Background(), "update", "--all")).To(MatchError(ContainSubstring("a-missing")))
		Expect(stdout.String()).To(ContainSubstring("Updated recorder"))
	})
	It("supports offline local listing and rejects network operations", func() {
		install("recorder", "1.0.0")
		Expect(execute(context.Background(), "list", "--offline", "--format", "json")).To(Succeed())
		Expect(stdout.String()).To(ContainSubstring(`"latest_version":"unknown"`))
		Expect(execute(context.Background(), "search", "--offline")).To(MatchError(ContainSubstring("disabled by --offline")))
		Expect(requests).To(BeEmpty())
	})
	It("lists installed plugins when the registry is unavailable", func() {
		install("recorder", "1.0.0")
		registry.Close()
		Expect(execute(context.Background(), "list", "--format", "json")).To(Succeed())
		Expect(stdout.String()).To(ContainSubstring(`"latest_version":"unknown"`))
		Expect(stderr.String()).To(ContainSubstring("latest registry versions unavailable"))
	})

	It("cancels a registry request with the command context", func() {
		slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() }))
		DeferCleanup(slow.Close)
		GinkgoT().Setenv("DEVEX_PLUGIN_REGISTRY_URL", slow.URL)
		ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
		defer cancel()
		Expect(execute(ctx, "search")).To(MatchError(ContainSubstring("context deadline exceeded")))
	})

	It("validates all update targets and the --all selection before downloading", func() {
		install("recorder", "1.0.0")
		Expect(execute(context.Background(), "update")).To(HaveOccurred())
		Expect(execute(context.Background(), "update", "recorder", "--all")).To(HaveOccurred())
		Expect(execute(context.Background(), "update", "recorder", "missing")).To(HaveOccurred())
		Expect(requests).To(BeEmpty())
	})
})
