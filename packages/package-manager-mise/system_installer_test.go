package main_test

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"

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
