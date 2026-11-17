//go:build !windows

package tui

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/ProtonMail/go-crypto/openpgp/armor"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/jameswlane/devex/packages/plugin-sdk/sdktest"
)

func testAptKey(name string) *openpgp.Entity {
	entity, err := openpgp.NewEntity(name, "", name+"@example.test", nil)
	Expect(err).NotTo(HaveOccurred())
	return entity
}

func fingerprintOf(entity *openpgp.Entity) string {
	return fmt.Sprintf("%X", entity.PrimaryKey.Fingerprint)
}

func armoredKeyring(entities ...*openpgp.Entity) []byte {
	var buf bytes.Buffer
	w, err := armor.Encode(&buf, openpgp.PublicKeyType, nil)
	Expect(err).NotTo(HaveOccurred())
	for _, entity := range entities {
		Expect(entity.Serialize(w)).To(Succeed())
	}
	Expect(w.Close()).To(Succeed())
	return buf.Bytes()
}

var _ = Describe("Pinned APT keys", func() {
	var (
		pinned, other *openpgp.Entity
		recorder      *sdktest.Recorder
		installer     *StreamingInstaller
		served        []byte
		keyURL        string
	)

	BeforeEach(func() {
		pinned, other = testAptKey("pinned"), testAptKey("other")
		recorder = sdktest.NewRecorder(GinkgoT(), "mkdir", "install")
		installer = &StreamingInstaller{
			ctx:      context.Background(),
			config:   DefaultInstallerConfig(),
			executor: NewDefaultCommandExecutor(),
		}
		server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write(served)
		}))
		DeferCleanup(server.Close)
		previous := aptKeyHTTPClient
		aptKeyHTTPClient = server.Client()
		DeferCleanup(func() { aptKeyHTTPClient = previous })
		DeferCleanup(func() { served = nil })
		served = armoredKeyring(other, pinned)
		keyURL = server.URL + "/key.gpg"
	})

	It("installs only the pinned key when the download matches", func() {
		keyName := filepath.Join(GinkgoT().TempDir(), "pinned.gpg")
		Expect(installer.addModernGPGKey(context.Background(), keyURL, keyName, fingerprintOf(pinned), true)).To(Succeed())
		calls := recorder.Calls("install")
		Expect(calls).To(HaveLen(1))
		Expect(calls[0].ElevationRequested).To(BeTrue())
		Expect(calls[0].Args).To(HaveLen(4))
		Expect(calls[0].Args[:2]).To(Equal([]string{"-m", "0644"}))
		Expect(calls[0].Args[3]).To(Equal(keyName))
	})

	It("fails the install and writes nothing when the fingerprint does not match", func() {
		served = armoredKeyring(other)
		keyName := filepath.Join(GinkgoT().TempDir(), "pinned.gpg")
		err := installer.addModernGPGKey(context.Background(), keyURL, keyName, fingerprintOf(pinned), true)
		Expect(err).To(MatchError(ContainSubstring("fingerprint mismatch")))
		Expect(err.Error()).To(ContainSubstring(fingerprintOf(pinned)))
		Expect(err.Error()).To(ContainSubstring(fingerprintOf(other)))
		Expect(recorder.Calls("install")).To(BeEmpty())
	})

	It("keeps only the pinned key from a multi-key download", func() {
		key, err := pinnedAptKey(armoredKeyring(other, pinned), fingerprintOf(pinned))
		Expect(err).NotTo(HaveOccurred())
		entities, err := openpgp.ReadKeyRing(bytes.NewReader(key))
		Expect(err).NotTo(HaveOccurred())
		Expect(entities).To(HaveLen(1))
		Expect(fingerprintOf(entities[0])).To(Equal(fingerprintOf(pinned)))
	})

	It("accepts binary keyrings and spaced lowercase pins", func() {
		var binary bytes.Buffer
		Expect(pinned.Serialize(&binary)).To(Succeed())
		fp := fingerprintOf(pinned)
		spaced := "0x" + strings.ToLower(fp[:20]+" "+fp[20:])
		_, err := pinnedAptKey(binary.Bytes(), spaced)
		Expect(err).NotTo(HaveOccurred())
	})

	It("rejects downloads that are not keys", func() {
		_, err := pinnedAptKey([]byte("<html>not a key</html>"), fingerprintOf(pinned))
		Expect(err).To(MatchError(ContainSubstring("cannot parse")))
	})
})
