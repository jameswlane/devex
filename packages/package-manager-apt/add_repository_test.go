package main_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	main "github.com/jameswlane/devex/packages/package-manager-apt"
	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
	"github.com/jameswlane/devex/packages/plugin-sdk/sdktest"
)

var _ = Describe("APT add-repository sudo workflow", func() {
	// simulateInstall intercepts `install -D -m 0644 <src> <dst>` calls
	// before forwarding to sdktest's recorded fake so os.Stat checks succeed while
	// preserving full sudo/install call recording.
	simulateInstall := func() {
		prev := sdk.NewCommandContext
		sdk.NewCommandContext = func(ctx context.Context, name string, args ...string) *exec.Cmd {
			// Match both `install ...` and the sudo-wrapped `sudo -- install ...`
			argv := append([]string{name}, args...)
			for i := 0; i < len(argv); i++ {
				if argv[i] == "install" && i+5 < len(argv) &&
					argv[i+1] == "-D" && argv[i+2] == "-m" && argv[i+3] == "0644" {
					src, dst := argv[i+4], argv[i+5]
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

	It("downloads to temp files and installs key and source as the user for non-APT paths", func() {
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
			Expect(call.ElevationRequested).To(BeFalse())
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
				"--no-list-cleanup",
				"-o", "Dir::Etc::sourcelist=/dev/null",
				"-o", "Dir::Etc::sourceparts=" + filepath.Dir(sourceFile),
			},
			ElevationRequested: true,
		}))
	})

	It("dearmors ASCII-armored keys in-process without gpg", func() {
		recorder := sdktest.NewRecorder(GinkgoT(), "apt", "apt-get", "gpg", "install", "file")
		simulateInstall()

		armoredKey, err := os.ReadFile(filepath.Join("testdata", "microsoft.asc"))
		Expect(err).NotTo(HaveOccurred())
		// testdata/microsoft.gpg is `gpg --dearmor` output for the same key
		wantKeyring, err := os.ReadFile(filepath.Join("testdata", "microsoft.gpg"))
		Expect(err).NotTo(HaveOccurred())
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write(armoredKey)
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

		Expect(recorder.Calls("gpg")).To(BeEmpty())
		Expect(recorder.Calls("file")).To(BeEmpty())
		Expect(os.ReadFile(keyPath)).To(Equal(wantKeyring))

		installCalls := recorder.Calls("install")
		Expect(installCalls).To(HaveLen(2))
		Expect(installCalls[0].Args[4]).To(Equal(keyPath))
		Expect(installCalls[1].Args[4]).To(Equal(sourceFile))
		for _, call := range installCalls {
			_, statErr := os.Stat(call.Args[3])
			Expect(os.IsNotExist(statErr)).To(BeTrue(), "temp file %s should be cleaned up", call.Args[3])
		}
	})

	It("rejects a dearmor download that is not an armored public key", func() {
		recorder := sdktest.NewRecorder(GinkgoT(), "apt", "apt-get", "gpg", "install", "file")
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			_, _ = w.Write([]byte("-----BEGIN PGP PUBLIC KEY BLOCK-----\nabc123\n-----END PGP PUBLIC KEY BLOCK-----\n"))
		}))
		DeferCleanup(server.Close)

		workDir := GinkgoT().TempDir()
		keyPath := filepath.Join(workDir, "bad.gpg")
		err := main.NewAPTPlugin().Execute("add-repository", []string{
			server.URL + "/key.asc",
			keyPath,
			"deb [signed-by=" + keyPath + "] https://example.com/repo stable main",
			filepath.Join(workDir, "bad.list"),
			"true",
		})
		Expect(err).To(MatchError(ContainSubstring("not a valid ASCII-armored GPG public key")))
		Expect(recorder.Calls("install")).To(BeEmpty())
		Expect(keyPath).NotTo(BeAnExistingFile())
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
				"--no-list-cleanup",
				"-o", "Dir::Etc::sourcelist=/dev/null",
				"-o", "Dir::Etc::sourceparts=" + filepath.Dir(sourceFile),
			},
			ElevationRequested: true,
		}))
	})

	Describe("DearmorPublicKey", func() {
		readFixture := func(name string) []byte {
			data, err := os.ReadFile(filepath.Join("testdata", name))
			Expect(err).NotTo(HaveOccurred())
			return data
		}

		It("keeps every armored block, like gpg --dearmor", func() {
			armored := append(readFixture("microsoft.asc"), readFixture("docker.asc")...)
			// testdata/microsoft-docker.gpg is `gpg --dearmor` output for both keys
			Expect(main.DearmorPublicKey(armored)).To(Equal(readFixture("microsoft-docker.gpg")))
		})

		It("rejects an empty armor block", func() {
			_, err := main.DearmorPublicKey([]byte("-----BEGIN PGP PUBLIC KEY BLOCK-----\n\n-----END PGP PUBLIC KEY BLOCK-----\n"))
			Expect(err).To(HaveOccurred())
		})

		It("rejects a block without its END line", func() {
			armored := strings.Replace(string(readFixture("microsoft.asc")), "-----END PGP PUBLIC KEY BLOCK-----", "", 1)
			_, err := main.DearmorPublicKey([]byte(armored))
			Expect(err).To(MatchError(ContainSubstring("missing its END line")))
		})

		It("rejects a truncated or empty block followed by a valid one", func() {
			docker := string(readFixture("docker.asc"))
			truncated := strings.Replace(string(readFixture("microsoft.asc")), "-----END PGP PUBLIC KEY BLOCK-----", "", 1)
			_, err := main.DearmorPublicKey([]byte(truncated + docker))
			Expect(err).To(MatchError(ContainSubstring("missing its END line")))

			empty := "-----BEGIN PGP PUBLIC KEY BLOCK-----\n\n-----END PGP PUBLIC KEY BLOCK-----\n"
			_, err = main.DearmorPublicKey([]byte(empty + docker))
			Expect(err).To(HaveOccurred())
		})

		It("matches armor markers only on complete lines", func() {
			microsoft := string(readFixture("microsoft.asc"))
			docker := string(readFixture("docker.asc"))

			corrupted := strings.Replace(microsoft, "-----END PGP PUBLIC KEY BLOCK-----", "corrupt: -----END PGP PUBLIC KEY BLOCK-----", 1)
			_, err := main.DearmorPublicKey([]byte(corrupted + docker))
			Expect(err).To(MatchError(ContainSubstring("missing its END line")))

			commented := strings.Replace(microsoft, "-----BEGIN PGP PUBLIC KEY BLOCK-----\n", "-----BEGIN PGP PUBLIC KEY BLOCK-----\nComment: Marker is -----BEGIN PGP PUBLIC KEY BLOCK-----\n", 1)
			Expect(main.DearmorPublicKey([]byte(commented + docker))).To(Equal(readFixture("microsoft-docker.gpg")))
		})
	})
})
