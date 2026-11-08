package main

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"

	sdktest "github.com/jameswlane/devex/packages/plugin-sdk/sdktest"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Declared extensions", func() {
	var recorder *sdktest.Recorder
	var plugin *GNOMEPlugin
	var declaration, archive string
	var bundle []byte
	makeZip := func(metadata string, extra string) []byte {
		var buffer bytes.Buffer
		writer := zip.NewWriter(&buffer)
		file, err := writer.Create("metadata.json")
		Expect(err).NotTo(HaveOccurred())
		_, err = file.Write([]byte(metadata))
		Expect(err).NotTo(HaveOccurred())
		if extra != "" {
			_, err = writer.Create(extra)
			Expect(err).NotTo(HaveOccurred())
		}
		Expect(writer.Close()).To(Succeed())
		return buffer.Bytes()
	}
	write := func(local bool) {
		spec := extensionSpec{UUID: "test@example.org", Settings: []extensionSetting{{Schema: "org.example.test", Key: "enabled", Value: "true"}}}
		if local {
			spec.Archive = "extension.zip"
		}
		data, err := json.Marshal(extensionDeclaration{ShellVersion: "48.2", Extensions: []extensionSpec{spec}})
		Expect(err).NotTo(HaveOccurred())
		Expect(os.WriteFile(declaration, data, 0o600)).To(Succeed())
		Expect(os.WriteFile(archive, bundle, 0o600)).To(Succeed())
	}
	preview := func() string {
		output := sdktest.CaptureStdout(GinkgoT(), func() {
			Expect(plugin.Execute("describe", []string{"install-extensions", "--extensions", declaration})).To(Succeed())
		})
		return fmt.Sprintf("%x", sha256.Sum256([]byte(strings.TrimSpace(output))))
	}
	run := func(hash string) error {
		return plugin.Execute("install-extensions", []string{"--extensions", declaration, "--expected-plan", hash})
	}
	BeforeEach(func() {
		recorder = sdktest.NewRecorder(GinkgoT(), "gdbus", "gnome-extensions", "gsettings")
		GinkgoT().Setenv("XDG_CURRENT_DESKTOP", "GNOME")
		GinkgoT().Setenv("XDG_CACHE_HOME", GinkgoT().TempDir())
		declaration = filepath.Join(GinkgoT().TempDir(), "extensions.json")
		archive = filepath.Join(filepath.Dir(declaration), "extension.zip")
		bundle = makeZip(`{"uuid":"test@example.org","shell-version":["48"]}`, "")
		plugin = NewGNOMEPlugin()
		plugin.extensions.Fetch = func(context.Context, string) ([]byte, error) { Fail("unexpected network access"); return nil, nil }
		recorder.Script("gdbus", []string{"call", "--session", "--dest", "org.gnome.Shell", "--object-path", "/org/gnome/Shell", "--method", "org.freedesktop.DBus.Properties.Get", "org.gnome.Shell", "ShellVersion"}, sdktest.Response{Stdout: "(<'48.2'>,)\n"})
		write(true)
	})
	It("previews without tools, session, cache writes, or network and executes exactly the native argv", func() {
		GinkgoT().Setenv("XDG_CURRENT_DESKTOP", "")
		output := sdktest.CaptureStdout(GinkgoT(), func() {
			Expect(plugin.Execute("describe", []string{"install-extensions", "--extensions", declaration})).To(Succeed())
		})
		Expect(recorder.Calls("gdbus")).To(BeEmpty())
		Expect(recorder.Calls("gnome-extensions")).To(BeEmpty())
		Expect(recorder.Calls("gsettings")).To(BeEmpty())
		Expect(os.ReadDir(os.Getenv("XDG_CACHE_HOME"))).To(BeEmpty())
		var commands [][]string
		Expect(json.Unmarshal([]byte(output), &commands)).To(Succeed())
		GinkgoT().Setenv("XDG_CURRENT_DESKTOP", "GNOME")
		Expect(run(preview())).To(Succeed())
		Expect(recorder.Calls("gnome-extensions")).To(Equal([]sdktest.Call{{Args: commands[1][1:]}, {Args: commands[2][1:]}}))
		Expect(recorder.Calls("gsettings")).To(Equal([]sdktest.Call{{Args: commands[3][1:]}}))
		installed, err := os.ReadFile(commands[1][3])
		Expect(err).NotTo(HaveOccurred())
		Expect(installed).To(Equal(bundle))
	})
	It("requires consent and rejects stale settings, archives, and plans before native calls", func() {
		Expect(plugin.Execute("install-extensions", []string{"--extensions", declaration})).To(MatchError(ContainSubstring("requires accepted")))
		Expect(run("stale")).To(MatchError(ContainSubstring("plan changed")))
		hash := preview()
		bundle = makeZip(`{"uuid":"test@example.org","shell-version":["48"],"version":2}`, "")
		write(true)
		Expect(run(hash)).To(MatchError(ContainSubstring("plan changed")))
		Expect(recorder.Calls("gdbus")).To(BeEmpty())
		Expect(recorder.Calls("gnome-extensions")).To(BeEmpty())
	})
	It("resolves a UUID by Shell version through injected network and validates downloaded metadata", func() {
		write(false)
		var addresses []string
		plugin.extensions.Fetch = func(_ context.Context, address string) ([]byte, error) {
			addresses = append(addresses, address)
			if strings.Contains(address, "extension-info") {
				return []byte(`{"uuid":"test@example.org","download_url":"/download-extension/test.zip?version_tag=42"}`), nil
			}
			return bundle, nil
		}
		Expect(run(preview())).To(Succeed())
		Expect(addresses).To(HaveLen(4))
		Expect(addresses[0]).To(ContainSubstring("shell_version=48.2"))
		Expect(addresses[0]).To(ContainSubstring("uuid=test%40example.org"))
		Expect(addresses[1]).To(Equal("https://extensions.gnome.org/download-extension/test.zip?version_tag=42"))
	})
	It("reports unsupported releases, untrusted sources, and network errors without installing", func() {
		write(false)
		for _, response := range []string{`{"uuid":"test@example.org"}`, `{"uuid":"other","download_url":"/x"}`, `{"uuid":"test@example.org","download_url":"https://evil.example/x"}`, `invalid`} {
			plugin.extensions.Fetch = func(context.Context, string) ([]byte, error) { return []byte(response), nil }
			Expect(run("any")).NotTo(Succeed())
		}
		plugin.extensions.Fetch = func(context.Context, string) ([]byte, error) { return nil, fmt.Errorf("offline") }
		Expect(run("any")).To(MatchError(ContainSubstring("offline")))
		Expect(recorder.Calls("gnome-extensions")).To(BeEmpty())
	})
	DescribeTable("rejects invalid bundles", func(metadata, extra, message string) {
		bundle = makeZip(metadata, extra)
		write(true)
		Expect(run("any")).To(MatchError(ContainSubstring(message)))
		Expect(recorder.Calls("gnome-extensions")).To(BeEmpty())
	}, Entry("UUID mismatch", `{"uuid":"wrong","shell-version":["48"]}`, "", "UUID mismatch"), Entry("incompatible", `{"uuid":"test@example.org","shell-version":["47"]}`, "", "incompatible"), Entry("traversal", `{"uuid":"test@example.org","shell-version":["48"]}`, "../bad", "unsafe"), Entry("bad metadata", `no`, "", "invalid character"))
	It("rejects a different running Shell version before installation", func() {
		hash := preview()
		recorder.Script("gdbus", []string{"call", "--session", "--dest", "org.gnome.Shell", "--object-path", "/org/gnome/Shell", "--method", "org.freedesktop.DBus.Properties.Get", "org.gnome.Shell", "ShellVersion"}, sdktest.Response{Stdout: "(<'49.1'>,)\n"})
		Expect(run(hash)).To(MatchError(ContainSubstring("differs from accepted")))
		Expect(recorder.Calls("gnome-extensions")).To(BeEmpty())
	})
	DescribeTable("propagates failures and stops following mutations", func(tool string, args []string, writes int) {
		hash := preview()
		if tool == "install" {
			tool = "gnome-extensions"
			args = []string{"install", "--force", filepath.Join(os.Getenv("XDG_CACHE_HOME"), "devex", "gnome-extensions", fmt.Sprintf("%x.zip", sha256.Sum256(bundle)))}
		}
		recorder.Script(tool, args, sdktest.Response{ExitCode: 42, Stderr: "native failure"})
		err := run(hash)
		Expect(err).To(MatchError(ContainSubstring("exit status 42")))
		Expect(err).To(MatchError(ContainSubstring("earlier changes remain")))
		Expect(recorder.Calls("gnome-extensions")).To(HaveLen(writes))
	}, Entry("install", "install", nil, 1), Entry("enable", "gnome-extensions", []string{"enable", "test@example.org"}, 2), Entry("settings", "gsettings", []string{"set", "org.example.test", "enabled", "true"}, 2))
	It("binds settings into consent and supports bundled compiled schemas", func() {
		hash := preview()
		data, err := os.ReadFile(declaration)
		Expect(err).NotTo(HaveOccurred())
		Expect(os.WriteFile(declaration, bytes.ReplaceAll(data, []byte(`"value":"true"`), []byte(`"value":"false"`)), 0o600)).To(Succeed())
		Expect(run(hash)).To(MatchError(ContainSubstring("plan changed")))
		Expect(recorder.Calls("gdbus")).To(BeEmpty())
		bundle = makeZip(`{"uuid":"test@example.org","shell-version":["48"]}`, "schemas/gschemas.compiled")
		write(true)
		GinkgoT().Setenv("XDG_DATA_HOME", GinkgoT().TempDir())
		Expect(run(preview())).To(Succeed())
		Expect(recorder.Calls("gsettings")).To(Equal([]sdktest.Call{{Args: []string{"--schemadir", filepath.Join(os.Getenv("XDG_DATA_HOME"), "gnome-shell", "extensions", "test@example.org", "schemas"), "set", "org.example.test", "enabled", "true"}}}))
	})
	DescribeTable("uses GNOME version compatibility rules", func(version, expected string) {
		actual, err := shellCompatibility(version)
		Expect(err).NotTo(HaveOccurred())
		Expect(actual).To(Equal(expected))
	}, Entry("modern patch", "48.2.1", "48"), Entry("future major", "100.1", "100"), Entry("legacy", "3.38.4", "3.38"))
	DescribeTable("rejects unknown or malformed declarations before native commands", func(content string) {
		Expect(os.WriteFile(declaration, []byte(content), 0o600)).To(Succeed())
		Expect(run("any")).NotTo(Succeed())
		Expect(recorder.Calls("gdbus")).To(BeEmpty())
		Expect(recorder.Calls("gnome-extensions")).To(BeEmpty())
	}, Entry("unknown field", `{"shell_version":"48","extensions":[],"surprise":true}`), Entry("empty", `{"shell_version":"48","extensions":[]}`), Entry("development version", `{"shell_version":"49.beta","extensions":[]}`), Entry("trailing content", `{} {}`), Entry("invalid UUID", `{"shell_version":"48","extensions":[{"uuid":"../evil"}]}`), Entry("duplicate UUID", `{"shell_version":"48","extensions":[{"uuid":"test@example.org","archive":"extension.zip"},{"uuid":"test@example.org","archive":"extension.zip"}]}`))
	It("reports missing session D-Bus without writing cache or installing", func() {
		hash := preview()
		recorder.Fallback("gdbus", sdktest.Response{ExitCode: 42})
		recorder.Script("gdbus", []string{"call", "--session", "--dest", "org.gnome.Shell", "--object-path", "/org/gnome/Shell", "--method", "org.freedesktop.DBus.Properties.Get", "org.gnome.Shell", "ShellVersion"}, sdktest.Response{ExitCode: 42})
		Expect(run(hash)).To(MatchError(ContainSubstring("session D-Bus")))
		Expect(recorder.Calls("gnome-extensions")).To(BeEmpty())
		Expect(os.ReadDir(os.Getenv("XDG_CACHE_HOME"))).To(BeEmpty())
	})

	It("bounds HTTP downloads, propagates statuses, and honors cancellation using only a local server", func() {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/ok":
				_, _ = w.Write([]byte("archive"))
			case "/large":
				_, _ = w.Write(bytes.Repeat([]byte("x"), extensionLimit+1))
			default:
				w.WriteHeader(http.StatusNotFound)
			}
		}))
		DeferCleanup(server.Close)
		data, err := fetchExtension(context.Background(), server.URL+"/ok")
		Expect(err).NotTo(HaveOccurred())
		Expect(string(data)).To(Equal("archive"))
		_, err = fetchExtension(context.Background(), server.URL+"/missing")
		Expect(err).To(MatchError(ContainSubstring("HTTP 404")))
		_, err = fetchExtension(context.Background(), server.URL+"/large")
		Expect(err).To(MatchError(ContainSubstring("exceeds 32 MiB")))
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err = fetchExtension(ctx, server.URL+"/ok")
		Expect(err).To(HaveOccurred())
	})

})
