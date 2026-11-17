package tui

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/ProtonMail/go-crypto/openpgp"
)

// maxAptKeySize bounds a downloaded APT key; real keyrings are a few KiB.
const maxAptKeySize = 1 << 20

// aptKeyHTTPClient downloads pinned APT keys. Tests replace it.
var aptKeyHTTPClient = &http.Client{Timeout: 60 * time.Second}

// normalizeFingerprint drops spaces and "0x" so pins compare in one form.
func normalizeFingerprint(fingerprint string) string {
	fp := strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(fingerprint), " ", ""))
	return strings.TrimPrefix(fp, "0X")
}

// pinnedAptKey parses an armored or binary keyring and returns only the key
// whose primary fingerprint matches the pin, in binary form, so the keyring
// that APT trusts contains nothing else from the download.
func pinnedAptKey(data []byte, fingerprint string) ([]byte, error) {
	want := normalizeFingerprint(fingerprint)
	entities, err := openpgp.ReadArmoredKeyRing(bytes.NewReader(data))
	if err != nil {
		entities, err = openpgp.ReadKeyRing(bytes.NewReader(data))
	}
	if err != nil {
		return nil, fmt.Errorf("cannot parse the downloaded GPG key: %w", err)
	}
	found := make([]string, 0, len(entities))
	for _, entity := range entities {
		got := fmt.Sprintf("%X", entity.PrimaryKey.Fingerprint)
		if got == want {
			var out bytes.Buffer
			if err := entity.Serialize(&out); err != nil {
				return nil, fmt.Errorf("cannot write the pinned GPG key %s: %w", want, err)
			}
			return out.Bytes(), nil
		}
		found = append(found, got)
	}
	return nil, fmt.Errorf("GPG key fingerprint mismatch: expected %s, the download has %s; the catalog's key_fingerprint is out of date or the key was tampered with",
		want, strings.Join(found, ", "))
}

// downloadAptKey fetches an APT key over HTTPS with a size limit.
func downloadAptKey(ctx context.Context, keyURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, keyURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := aptKeyHTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download GPG key %s: %w", keyURL, err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download GPG key %s: HTTP %d", keyURL, resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxAptKeySize+1))
	if err != nil {
		return nil, fmt.Errorf("download GPG key %s: %w", keyURL, err)
	}
	if len(data) > maxAptKeySize {
		return nil, fmt.Errorf("download GPG key %s: larger than %d bytes", keyURL, maxAptKeySize)
	}
	return data, nil
}

// addPinnedGPGKey downloads the key, checks it against the pinned fingerprint
// and installs only the matching key as a binary keyring at keyName.
func (si *StreamingInstaller) addPinnedGPGKey(ctx context.Context, keyURL, keyName, fingerprint string) error {
	data, err := downloadAptKey(ctx, keyURL)
	if err != nil {
		return err
	}
	key, err := pinnedAptKey(data, fingerprint)
	if err != nil {
		return fmt.Errorf("%s: %w", keyURL, err)
	}
	si.sendLog("INFO", fmt.Sprintf("GPG key fingerprint verified: %s", normalizeFingerprint(fingerprint)))

	tmp, err := os.CreateTemp("", "devex-apt-key-*.gpg")
	if err != nil {
		return fmt.Errorf("create temporary GPG key file: %w", err)
	}
	defer func() { _ = os.Remove(tmp.Name()) }()
	if _, err := tmp.Write(key); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write temporary GPG key file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("write temporary GPG key file: %w", err)
	}
	return si.executeCommandStream(ctx, fmt.Sprintf("sudo -n install -m 0644 %s %s", tmp.Name(), keyName))
}
