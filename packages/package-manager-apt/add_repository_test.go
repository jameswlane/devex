package main_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	main "github.com/jameswlane/devex/packages/package-manager-apt"
	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
	"github.com/jameswlane/devex/packages/plugin-sdk/sdktest"
)

var _ = Describe("APT add-repository sudo workflow", func() {
	// simulateInstall intercepts elevated `install -D -m 0644 <src> <dst>` calls
	// before forwarding to sdktest's recorded fake so os.Stat checks succeed while
	// preserving full sudo/install call recording.
	simulateInstall := func() {
		prev := sdk.NewCommandContext
		sdk.NewCommandContext = func(ctx context.Context, name string, args ...string) *exec.Cmd {
			for i := 0; i < len(args); i++ {
				if args[i] == "install" && i+5 < len(args) &&
					args[i+1] == "-D" && args[i+2] == "-m" && args[i+3] == "0644" {
					src, dst := args[i+4], args[i+5]
					data, err := os.ReadFile(src)
					Expect(err).NotTo(HaveOccurred())
					Expect(os.MkdirAll(filepath.Dir(dst), 0o755)).To(Succeed())
					Expect(os.WriteFile(dst, data, 0o644)).To(Succeed())
					break
				}
			}
			return prev(ctx, name, args...)
		}
		DeferCleanup(func() { sdk.NewCommandContext = prev })
	}

	It("downloads to temp files and installs key, source, and validation via sudo", func() {
		recorder := sdktest.NewRecorder(GinkgoT(), "apt", "apt-get", "gpg", "install", "file")
		simulateInstall()

		keyPayload := []byte("binary-gpg-keyring-bytes")
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write(keyPayload)
		}))
		DeferCleanup(server.Close)

		workDir := GinkgoT().TempDir()
		keyPath := filepath.Join(workDir, "keyrings", "vendor.gpg")
		sourceFile := filepath.Join(workDir, "sources.list.d", "vendor.list")
		sourceLine := "deb [signed-by=" + keyPath + "] https://example.com/repo stable main"

		plugin := main.NewAPTPlugin()
		Expect(plugin.Execute("add-repository", []string{
			server.URL + "/key.gpg",
			keyPath,
			sourceLine,
			sourceFile,
		})).To(Succeed())

		installCalls := recorder.Calls("install")
		Expect(installCalls).To(HaveLen(2))
		for _, call := range installCalls {
			Expect(call.ElevationRequested).To(BeTrue())
			Expect(call.Args).To(HaveLen(5))
			Expect(call.Args[:3]).To(Equal([]string{"-D", "-m", "0644"}))
			// Temp file must have been removed after installation
			_, statErr := os.Stat(call.Args[3])
			Expect(os.IsNotExist(statErr)).To(BeTrue(), "temp file %s should be cleaned up", call.Args[3])
		}
		Expect(installCalls[0].Args[4]).To(Equal(keyPath))
		Expect(installCalls[1].Args[4]).To(Equal(sourceFile))
		Expect(recorder.Calls("gpg")).To(BeEmpty())

		Expect(os.ReadFile(keyPath)).To(Equal(keyPayload))
		Expect(os.ReadFile(sourceFile)).To(Equal([]byte(sourceLine + "\n")))

		Expect(recorder.Calls("apt-get")).To(ContainElement(sdktest.Call{
			Args: []string{
				"update",
				"-o", "Dir::Etc::sourcelist=/dev/null",
				"-o", "Dir::Etc::sourceparts=" + filepath.Dir(sourceFile),
			},
			ElevationRequested: true,
		}))
	})

	It("dearmors ASCII-armored keys in temp files without sudo before installing via sudo", func() {
		recorder := sdktest.NewRecorder(GinkgoT(), "apt", "apt-get", "gpg", "install", "file")
		recorder.Fallback("file", sdktest.Response{Stdout: "GPG key public ring\n"})
		simulateInstall()

		armoredKey := "-----BEGIN PGP PUBLIC KEY BLOCK-----\nabc123\n-----END PGP PUBLIC KEY BLOCK-----\n"
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte(armoredKey))
		}))
		DeferCleanup(server.Close)

		workDir := GinkgoT().TempDir()
		keyPath := filepath.Join(workDir, "keyrings", "vendor-dearmored.gpg")
		sourceFile := filepath.Join(workDir, "sources.list.d", "vendor-dearmored.list")
		sourceLine := "deb [signed-by=" + keyPath + "] https://example.com/repo stable main"

		plugin := main.NewAPTPlugin()
		Expect(plugin.Execute("add-repository", []string{
			server.URL + "/key.asc",
			keyPath,
			sourceLine,
			sourceFile,
			"true",
		})).To(Succeed())

		gpgCalls := recorder.Calls("gpg")
		Expect(gpgCalls).To(HaveLen(1))
		Expect(gpgCalls[0].ElevationRequested).To(BeFalse())
		Expect(gpgCalls[0].Args).To(HaveLen(5))
		Expect(gpgCalls[0].Args[:3]).To(Equal([]string{"--yes", "--dearmor", "-o"}))
		dearmoredTemp := gpgCalls[0].Args[3]
		downloadTemp := gpgCalls[0].Args[4]

		installCalls := recorder.Calls("install")
		Expect(installCalls).To(HaveLen(2))
		Expect(installCalls[0]).To(Equal(sdktest.Call{
			Args:               []string{"-D", "-m", "0644", dearmoredTemp, keyPath},
			ElevationRequested: true,
		}))
		Expect(installCalls[1].ElevationRequested).To(BeTrue())
		Expect(installCalls[1].Args[:3]).To(Equal([]string{"-D", "-m", "0644"}))
		Expect(installCalls[1].Args[4]).To(Equal(sourceFile))

		// Both download and dearmored temp files must be cleaned up
		for _, tmpPath := range []string{downloadTemp, dearmoredTemp, installCalls[1].Args[3]} {
			_, statErr := os.Stat(tmpPath)
			Expect(os.IsNotExist(statErr)).To(BeTrue(), "temp file %s should be cleaned up", tmpPath)
		}
	})

	It("skips reinstalling source file when it already contains the source line", func() {
		recorder := sdktest.NewRecorder(GinkgoT(), "apt", "apt-get", "gpg", "install", "file")
		simulateInstall()

		workDir := GinkgoT().TempDir()
		keyPath := filepath.Join(workDir, "vendor.gpg")
		sourceFile := filepath.Join(workDir, "vendor.list")
		sourceLine := "deb [signed-by=" + keyPath + "] https://example.com/repo stable main"

		Expect(os.WriteFile(keyPath, []byte("existing-key"), 0o644)).To(Succeed())
		Expect(os.WriteFile(sourceFile, []byte(sourceLine+"\n"), 0o644)).To(Succeed())

		plugin := main.NewAPTPlugin()
		Expect(plugin.Execute("add-repository", []string{
			"https://example.com/key.gpg",
			keyPath,
			sourceLine,
			sourceFile,
		})).To(Succeed())

		Expect(recorder.Calls("install")).To(BeEmpty())
		Expect(recorder.Calls("apt-get")).To(ContainElement(sdktest.Call{
			Args: []string{
				"update",
				"-o", "Dir::Etc::sourcelist=/dev/null",
				"-o", "Dir::Etc::sourceparts=" + filepath.Dir(sourceFile),
			},
			ElevationRequested: true,
		}))
	})
})
