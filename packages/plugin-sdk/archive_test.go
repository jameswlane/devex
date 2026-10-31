package sdk_test

import (
	"archive/tar"
	"archive/zip"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"sync/atomic"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
)

const fakeExecutable = "#!/bin/sh\necho plugin\n"

func executableName(plugin string) string {
	name := "devex-plugin-" + plugin
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	return name
}

func tarGzPayload(entries map[string]string) []byte {
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	for name, body := range entries {
		Expect(tw.WriteHeader(&tar.Header{Name: name, Mode: 0o755, Size: int64(len(body)), Typeflag: tar.TypeReg})).To(Succeed())
		_, err := tw.Write([]byte(body))
		Expect(err).ToNot(HaveOccurred())
	}
	Expect(tw.Close()).To(Succeed())
	Expect(gz.Close()).To(Succeed())
	return buf.Bytes()
}

func zipPayload(entries map[string]string) []byte {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for name, body := range entries {
		w, err := zw.Create(name)
		Expect(err).ToNot(HaveOccurred())
		_, err = w.Write([]byte(body))
		Expect(err).ToNot(HaveOccurred())
	}
	Expect(zw.Close()).To(Succeed())
	return buf.Bytes()
}

func sha256Hex(data []byte) string {
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

var _ = Describe("Plugin payload extraction", func() {
	var tempDir string

	BeforeEach(func() {
		var err error
		tempDir, err = os.MkdirTemp("", "plugin-archive-test-*")
		Expect(err).ToNot(HaveOccurred())
	})

	AfterEach(func() {
		_ = os.RemoveAll(tempDir)
	})

	writePayload := func(data []byte) string {
		p := filepath.Join(tempDir, "payload")
		Expect(os.WriteFile(p, data, 0o600)).To(Succeed())
		return p
	}

	It("extracts the plugin executable from a tar.gz archive", func() {
		payload := writePayload(tarGzPayload(map[string]string{
			"README.md":            "docs",
			executableName("demo"): fakeExecutable,
		}))
		dest := filepath.Join(tempDir, "out")

		format, err := sdk.ExtractPluginExecutable(payload, dest, "demo", runtime.GOOS)
		Expect(err).ToNot(HaveOccurred())
		Expect(format).To(Equal(sdk.PayloadFormatTarGz))
		Expect(os.ReadFile(dest)).To(BeEquivalentTo(fakeExecutable))
	})

	It("extracts the plugin executable from a zip archive", func() {
		payload := writePayload(zipPayload(map[string]string{executableName("demo"): fakeExecutable}))
		dest := filepath.Join(tempDir, "out")

		format, err := sdk.ExtractPluginExecutable(payload, dest, "demo", runtime.GOOS)
		Expect(err).ToNot(HaveOccurred())
		Expect(format).To(Equal(sdk.PayloadFormatZip))
		Expect(os.ReadFile(dest)).To(BeEquivalentTo(fakeExecutable))
	})

	It("copies a raw binary payload unchanged", func() {
		payload := writePayload([]byte(fakeExecutable))
		dest := filepath.Join(tempDir, "out")

		format, err := sdk.ExtractPluginExecutable(payload, dest, "demo", runtime.GOOS)
		Expect(err).ToNot(HaveOccurred())
		Expect(format).To(Equal(sdk.PayloadFormatBinary))
		Expect(os.ReadFile(dest)).To(BeEquivalentTo(fakeExecutable))
	})

	It("fails when the archive does not contain the plugin executable", func() {
		payload := writePayload(tarGzPayload(map[string]string{"README.md": "docs"}))

		_, err := sdk.ExtractPluginExecutable(payload, filepath.Join(tempDir, "out"), "demo", runtime.GOOS)
		Expect(err).To(MatchError(sdk.ErrPluginNotInArchive))
	})

	It("never writes outside the destination for traversal entry names", func() {
		payload := writePayload(tarGzPayload(map[string]string{
			"../../" + executableName("demo"): fakeExecutable,
		}))
		dest := filepath.Join(tempDir, "nested", "out")
		Expect(os.MkdirAll(filepath.Dir(dest), 0o750)).To(Succeed())

		_, err := sdk.ExtractPluginExecutable(payload, dest, "demo", runtime.GOOS)
		Expect(err).ToNot(HaveOccurred())
		Expect(os.ReadFile(dest)).To(BeEquivalentTo(fakeExecutable))
		_, statErr := os.Stat(filepath.Join(filepath.Dir(tempDir), executableName("demo")))
		Expect(os.IsNotExist(statErr)).To(BeTrue())
	})
})

var _ = Describe("Downloading plugins from the registry", func() {
	var (
		pluginDir     string
		server        *httptest.Server
		payload       []byte
		checksum      string
		downloadCount atomic.Int32
		downloader    *sdk.Downloader
	)

	platformKey := fmt.Sprintf("%s-%s", runtime.GOOS, runtime.GOARCH)

	BeforeEach(func() {
		var err error
		pluginDir, err = os.MkdirTemp("", "plugin-download-test-*")
		Expect(err).ToNot(HaveOccurred())

		payload = tarGzPayload(map[string]string{
			"README.md":            "docs",
			executableName("demo"): fakeExecutable,
		})
		checksum = sha256Hex(payload)
		downloadCount.Store(0)

		server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.URL.Path {
			case "/api/v1/plugins/demo":
				_ = json.NewEncoder(w).Encode(sdk.PluginMetadata{
					PluginInfo: sdk.PluginInfo{Name: "demo", Version: "1.2.3"},
					Binaries: map[string]sdk.PlatformBinary{
						platformKey: {URL: "https://example.invalid/demo.tar.gz", Checksum: checksum, Size: int64(len(payload))},
					},
				})
			case "/api/v1/plugins/demo/download/" + platformKey:
				downloadCount.Add(1)
				_, _ = w.Write(payload)
			default:
				http.NotFound(w, r)
			}
		}))

		downloader = sdk.NewDownloader(server.URL, pluginDir)
		downloader.SetSilent(true)
	})

	AfterEach(func() {
		server.Close()
		_ = os.RemoveAll(pluginDir)
	})

	installedPath := func() string { return filepath.Join(pluginDir, executableName("demo")) }

	It("installs the extracted executable, not the archive", func() {
		Expect(downloader.DownloadPluginWithContext(context.Background(), "demo")).To(Succeed())

		Expect(os.ReadFile(installedPath())).To(BeEquivalentTo(fakeExecutable))
		format, err := sdk.DetectPayloadFormat(installedPath())
		Expect(err).ToNot(HaveOccurred())
		Expect(format).To(Equal(sdk.PayloadFormatBinary))
	})

	It("leaves no temporary files behind", func() {
		Expect(downloader.DownloadPluginWithContext(context.Background(), "demo")).To(Succeed())

		entries, err := os.ReadDir(pluginDir)
		Expect(err).ToNot(HaveOccurred())
		var names []string
		for _, e := range entries {
			names = append(names, e.Name())
		}
		Expect(names).To(ConsistOf(executableName("demo"), ".devex-plugin-demo.install.json"))
	})

	It("does not download again when the installed plugin matches the registry", func() {
		Expect(downloader.DownloadPluginWithContext(context.Background(), "demo")).To(Succeed())
		Expect(downloader.DownloadPluginWithContext(context.Background(), "demo")).To(Succeed())

		Expect(downloadCount.Load()).To(Equal(int32(1)))
	})

	It("repairs installs where an older SDK saved the archive as the executable", func() {
		Expect(os.WriteFile(installedPath(), payload, 0o755)).To(Succeed()) // #nosec G306 - test fixture

		Expect(downloader.DownloadPluginWithContext(context.Background(), "demo")).To(Succeed())

		Expect(downloadCount.Load()).To(Equal(int32(1)))
		Expect(os.ReadFile(installedPath())).To(BeEquivalentTo(fakeExecutable))
	})

	It("rejects a payload whose checksum does not match and keeps the existing plugin", func() {
		Expect(os.WriteFile(installedPath(), []byte("old"), 0o755)).To(Succeed()) // #nosec G306 - test fixture
		checksum = sha256Hex([]byte("something else"))

		err := downloader.DownloadPluginWithContext(context.Background(), "demo")
		Expect(err).To(MatchError(ContainSubstring("checksum verification failed")))
		Expect(os.ReadFile(installedPath())).To(BeEquivalentTo("old"))
	})
})
