package main_test

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/ProtonMail/go-crypto/openpgp/armor"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	main "github.com/jameswlane/devex/packages/package-manager-mise"
	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
)

var _ = Describe("Verified Mise installer", func() {
	var (
		plugin  *main.MisePlugin
		dir     string
		marker  string
		script  []byte
		signer  *openpgp.Entity
		keyPath string
	)

	sign := func(content []byte) []byte {
		var out bytes.Buffer
		writer, err := openpgp.Sign(&out, signer, nil, nil)
		Expect(err).NotTo(HaveOccurred())
		_, err = writer.Write(content)
		Expect(err).NotTo(HaveOccurred())
		Expect(writer.Close()).To(Succeed())
		return out.Bytes()
	}

	serve := func(body, signature []byte) (string, string) {
		mux := http.NewServeMux()
		mux.HandleFunc("/install.sh", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(body) })
		mux.HandleFunc("/install.sh.sig", func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(signature) })
		server := httptest.NewServer(mux)
		DeferCleanup(server.Close)
		return server.URL + "/install.sh", server.URL + "/install.sh.sig"
	}

	BeforeEach(func() {
		plugin = &main.MisePlugin{PackageManagerPlugin: sdk.NewPackageManagerPlugin(sdk.PluginInfo{Name: "package-manager-mise", Version: "test"}, "mise")}
		plugin.InitLogger(sdk.NewDefaultLogger(true))
		dir = GinkgoT().TempDir()
		marker = filepath.Join(dir, "ran")
		script = []byte(fmt.Sprintf("#!/bin/sh\ntouch %q\n", marker))
		var err error
		signer, err = openpgp.NewEntity("Release", "", "release@example.com", nil)
		Expect(err).NotTo(HaveOccurred())
		keyPath = filepath.Join(dir, "release.asc")
		file, err := os.Create(keyPath)
		Expect(err).NotTo(HaveOccurred())
		armored, err := armor.Encode(file, openpgp.PublicKeyType, nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(signer.Serialize(armored)).To(Succeed())
		Expect(armored.Close()).To(Succeed())
		Expect(file.Close()).To(Succeed())
	})

	verification := func(signatureURL string) sdk.ArtifactVerification {
		return sdk.ArtifactVerification{Policy: "required", SignatureURL: signatureURL, PublicKeyPath: keyPath, Fingerprint: fmt.Sprintf("%X", signer.PrimaryKey.Fingerprint)}
	}

	It("runs the installer only after its signature verifies", func() {
		scriptURL, signatureURL := serve(script, sign(script))
		Expect(main.InstallMiseFrom(plugin, context.Background(), scriptURL, verification(signatureURL))).To(Succeed())
		Expect(marker).To(BeAnExistingFile())
	})

	It("refuses to run a tampered installer", func() {
		tampered := append(append([]byte(nil), script...), []byte("echo injected\n")...)
		scriptURL, signatureURL := serve(tampered, sign(script))
		err := main.InstallMiseFrom(plugin, context.Background(), scriptURL, verification(signatureURL))
		Expect(err).To(MatchError(ContainSubstring("verification failed")))
		Expect(marker).NotTo(BeAnExistingFile())
	})

	It("refuses to run when the signature is unavailable", func() {
		scriptURL, _ := serve(script, nil)
		err := main.InstallMiseFrom(plugin, context.Background(), scriptURL, verification(scriptURL+".missing"))
		Expect(err).To(HaveOccurred())
		Expect(marker).NotTo(BeAnExistingFile())
	})

	It("pins the official installer to mise's release key", func() {
		metadata := sdk.MiseInstallVerification()
		Expect(metadata.Policy).To(Equal("required"))
		Expect(metadata.Fingerprint).To(Equal(sdk.MiseReleaseKeyFingerprint))
	})
})

var _ = Describe("Bash shell integration", func() {
	It("activates mise from ~/.local/bin when that directory is not on PATH", func() {
		home := GinkgoT().TempDir()
		userBin := filepath.Join(home, ".local", "bin")
		Expect(os.MkdirAll(userBin, 0o755)).To(Succeed())
		fakeMise := "#!/bin/sh\necho 'export MISE_ACTIVATED=yes'\n"
		Expect(os.WriteFile(filepath.Join(userBin, "mise"), []byte(fakeMise), 0o755)).To(Succeed())

		plugin := &main.MisePlugin{PackageManagerPlugin: sdk.NewPackageManagerPlugin(sdk.PluginInfo{Name: "package-manager-mise", Version: "test"}, "mise")}
		Expect(main.ConfigureBashIntegration(plugin, home)).To(Succeed())

		cmd := exec.Command("bash", "-c", `. "$HOME/.bashrc"; printf %s "$MISE_ACTIVATED"`)
		cmd.Env = []string{"HOME=" + home, "PATH=/usr/bin:/bin"}
		output, err := cmd.CombinedOutput()
		Expect(err).NotTo(HaveOccurred(), string(output))
		Expect(string(output)).To(Equal("yes"))
	})
})

var _ = Describe("HandleEnsureInstalled shell activation", func() {
	var (
		plugin   *main.MisePlugin
		home     string
		shellDir string
	)

	writeShellStub := func(name string) string {
		path := filepath.Join(shellDir, name)
		Expect(os.WriteFile(path, []byte("#!/bin/sh\nexit 0\n"), 0o755)).To(Succeed())
		return path
	}

	BeforeEach(func() {
		plugin = &main.MisePlugin{PackageManagerPlugin: sdk.NewPackageManagerPlugin(sdk.PluginInfo{Name: "package-manager-mise", Version: "test"}, "mise")}
		plugin.InitLogger(sdk.NewDefaultLogger(true))

		home = GinkgoT().TempDir()
		binDir := GinkgoT().TempDir()
		shellDir = GinkgoT().TempDir()
		Expect(os.WriteFile(filepath.Join(binDir, "mise"), []byte("#!/bin/sh\nexit 0\n"), 0o755)).To(Succeed())

		GinkgoT().Setenv("HOME", home)
		GinkgoT().Setenv("PATH", binDir+string(os.PathListSeparator)+os.Getenv("PATH"))
	})

	It("configures fish shell activation when mise is already installed", func() {
		GinkgoT().Setenv("SHELL", writeShellStub("fish"))

		Expect(plugin.HandleEnsureInstalled(context.Background(), nil)).To(Succeed())

		configPath := filepath.Join(home, ".config", "fish", "config.fish")
		content, err := os.ReadFile(configPath)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(content)).To(ContainSubstring("mise activate fish"))
	})

	It("does not duplicate bash activation when called twice", func() {
		GinkgoT().Setenv("SHELL", writeShellStub("bash"))

		Expect(plugin.HandleEnsureInstalled(context.Background(), nil)).To(Succeed())
		Expect(plugin.HandleEnsureInstalled(context.Background(), nil)).To(Succeed())

		bashrcPath := filepath.Join(home, ".bashrc")
		content, err := os.ReadFile(bashrcPath)
		Expect(err).NotTo(HaveOccurred())
		Expect(strings.Count(string(content), "mise activate bash")).To(Equal(1))
	})

	It("leaves zsh config unchanged when activation is already present", func() {
		GinkgoT().Setenv("SHELL", writeShellStub("zsh"))

		zshrcPath := filepath.Join(home, ".zshrc")
		existing := "# existing config\neval \"$(mise activate zsh)\"\n"
		Expect(os.WriteFile(zshrcPath, []byte(existing), 0o644)).To(Succeed())

		Expect(plugin.HandleEnsureInstalled(context.Background(), nil)).To(Succeed())

		content, err := os.ReadFile(zshrcPath)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(content)).To(Equal(existing))
	})
})

var _ = Describe("Zsh shell integration", func() {
	It("activates mise from ~/.local/bin when that directory is not on PATH", func() {
		zshPath, err := exec.LookPath("zsh")
		if err != nil {
			Skip("zsh not installed")
		}

		home := GinkgoT().TempDir()
		userBin := filepath.Join(home, ".local", "bin")
		Expect(os.MkdirAll(userBin, 0o755)).To(Succeed())
		fakeMise := "#!/bin/sh\necho 'export MISE_ACTIVATED=yes'\n"
		Expect(os.WriteFile(filepath.Join(userBin, "mise"), []byte(fakeMise), 0o755)).To(Succeed())

		plugin := &main.MisePlugin{PackageManagerPlugin: sdk.NewPackageManagerPlugin(sdk.PluginInfo{Name: "package-manager-mise", Version: "test"}, "mise")}
		Expect(main.ConfigureZshIntegration(plugin, home)).To(Succeed())

		cmd := exec.Command(zshPath, "-f", "-c", `. "$HOME/.zshrc"; printf %s "$MISE_ACTIVATED"`)
		// An empty PATH keeps a system-wide mise from shadowing the fixture.
		cmd.Env = []string{"HOME=" + home, "PATH=" + GinkgoT().TempDir()}
		output, err := cmd.CombinedOutput()
		Expect(err).NotTo(HaveOccurred(), string(output))
		Expect(string(output)).To(Equal("yes"))
	})
})

var _ = Describe("Fish shell integration", func() {
	It("activates mise from ~/.local/bin when that directory is not on PATH", func() {
		fishPath, err := exec.LookPath("fish")
		if err != nil {
			Skip("fish not installed")
		}

		home := GinkgoT().TempDir()
		userBin := filepath.Join(home, ".local", "bin")
		Expect(os.MkdirAll(userBin, 0o755)).To(Succeed())
		fakeMise := "#!/bin/sh\necho 'set -gx MISE_ACTIVATED yes'\n"
		Expect(os.WriteFile(filepath.Join(userBin, "mise"), []byte(fakeMise), 0o755)).To(Succeed())

		plugin := &main.MisePlugin{PackageManagerPlugin: sdk.NewPackageManagerPlugin(sdk.PluginInfo{Name: "package-manager-mise", Version: "test"}, "mise")}
		Expect(main.ConfigureFishIntegration(plugin, home)).To(Succeed())

		cmd := exec.Command(fishPath, "--no-config", "-c", `source "$HOME/.config/fish/config.fish"; printf %s "$MISE_ACTIVATED"`)
		// An empty PATH keeps a system-wide mise from shadowing the fixture.
		cmd.Env = []string{"HOME=" + home, "PATH=" + GinkgoT().TempDir()}
		output, err := cmd.CombinedOutput()
		Expect(err).NotTo(HaveOccurred(), string(output))
		Expect(string(output)).To(Equal("yes"))
	})
})
