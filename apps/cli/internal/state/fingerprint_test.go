package state

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/jameswlane/devex/apps/cli/internal/types"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Installation fingerprint key", func() {
	var dir string
	BeforeEach(func() { dir = filepath.Join(GinkgoT().TempDir(), "state"); GinkgoT().Setenv("DEVEX_STATE_DIR", dir) })
	It("persists a private 32-byte key outside the database and produces stable keyed fingerprints", func() {
		data := []byte(`{"pin":"1234"}`)
		first, err := KeyedFingerprint(data)
		Expect(err).NotTo(HaveOccurred())
		second, err := KeyedFingerprint(data)
		Expect(err).NotTo(HaveOccurred())
		Expect(second).To(Equal(first))
		Expect(first).To(HavePrefix("hmac-sha256:"))
		Expect(first).NotTo(Equal(fmt.Sprintf("hmac-sha256:%x", sha256.Sum256(data))))
		keyPath := filepath.Join(dir, fingerprintKeyName)
		key, err := os.ReadFile(keyPath)
		Expect(err).NotTo(HaveOccurred())
		Expect(key).To(HaveLen(32))
		info, err := os.Stat(keyPath)
		Expect(err).NotTo(HaveOccurred())
		Expect(info.Mode().Perm()).To(Equal(os.FileMode(0o600)))
		info, err = os.Stat(dir)
		Expect(err).NotTo(HaveOccurred())
		Expect(info.Mode().Perm()).To(Equal(os.FileMode(0o700)))
	})
	It("invalidates existing fingerprints after a key reset", func() {
		first, err := KeyedFingerprint([]byte("same plan"))
		Expect(err).NotTo(HaveOccurred())
		Expect(os.Remove(filepath.Join(dir, fingerprintKeyName))).To(Succeed())
		second, err := KeyedFingerprint([]byte("same plan"))
		Expect(err).NotTo(HaveOccurred())
		Expect(second).NotTo(Equal(first))
	})
	It("shares the winning key across concurrent first callers", func() {
		var wg sync.WaitGroup
		results := make([]string, 8)
		errs := make([]error, 8)
		for i := range results {
			wg.Add(1)
			go func() { defer wg.Done(); results[i], errs[i] = KeyedFingerprint([]byte("plan")) }()
		}
		wg.Wait()
		for i := range results {
			Expect(errs[i]).NotTo(HaveOccurred())
			Expect(results[i]).To(Equal(results[0]))
		}
	})
	It("rejects malformed key files rather than using weak keys", func() {
		Expect(os.MkdirAll(dir, 0o700)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(dir, fingerprintKeyName), []byte("short"), 0o600)).To(Succeed())
		_, err := KeyedFingerprint([]byte("plan"))
		Expect(err).To(MatchError(ContainSubstring("must contain 32 bytes")))
	})
	It("authenticates application settings including secret values", func() {
		apps := []types.CrossPlatformApp{{Name: "app"}}
		first, err := ApplicationFingerprint(apps, map[string]string{"password": "1234"})
		Expect(err).NotTo(HaveOccurred())
		same, err := ApplicationFingerprint(apps, map[string]string{"password": "1234"})
		Expect(err).NotTo(HaveOccurred())
		Expect(same).To(Equal(first))
		changed, err := ApplicationFingerprint(apps, map[string]string{"password": "1235"})
		Expect(err).NotTo(HaveOccurred())
		Expect(changed).NotTo(Equal(first))
	})
})
