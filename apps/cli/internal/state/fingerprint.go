package state

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	devexpaths "github.com/jameswlane/devex/packages/plugin-sdk/paths"
)

const fingerprintKeyName = "resume.key"

// KeyedFingerprint protects secret-bearing plans against offline guessing from
// a copy of the state database. The installation key is never stored in bbolt.
func KeyedFingerprint(data []byte) (string, error) {
	key, err := fingerprintKey()
	if err != nil {
		return "", err
	}
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write(data)
	return fmt.Sprintf("hmac-sha256:%x", mac.Sum(nil)), nil
}

func fingerprintKey() ([]byte, error) {
	dirs, err := devexpaths.Resolve()
	if err != nil {
		return nil, fmt.Errorf("resolve resume key directory: %w", err)
	}
	path := filepath.Join(dirs.State, fingerprintKeyName)
	if err := os.MkdirAll(dirs.State, 0o700); err != nil {
		return nil, fmt.Errorf("create resume key directory: %w", err)
	}
	if err := os.Chmod(dirs.State, 0o700); err != nil {
		return nil, fmt.Errorf("secure resume key directory: %w", err)
	}
	key, err := readFingerprintKey(path)
	if !errors.Is(err, os.ErrNotExist) {
		return key, err
	}
	// Publish a fully written key with an exclusive hard link. Concurrent first
	// callers either publish their key or read the winner's complete key, never a
	// partially written file. A failed publication leaves no key to fingerprint.
	file, err := os.CreateTemp(dirs.State, ".resume-key-*")
	if err != nil {
		return nil, fmt.Errorf("create resume key: %w", err)
	}
	defer os.Remove(file.Name())
	key = make([]byte, 32)
	if _, err = rand.Read(key); err == nil {
		_, err = file.Write(key)
	}
	if err == nil {
		err = file.Sync()
	}
	err = errors.Join(err, file.Close())
	if err != nil {
		return nil, fmt.Errorf("write resume key: %w", err)
	}
	if err := os.Link(file.Name(), path); err != nil && !errors.Is(err, os.ErrExist) {
		return nil, fmt.Errorf("publish resume key: %w", err)
	}
	return readFingerprintKey(path)
}

func readFingerprintKey(path string) ([]byte, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return nil, fmt.Errorf("inspect resume key: %w", err)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("resume key %s must be a regular file", path)
	}
	if err := os.Chmod(path, 0o600); err != nil {
		return nil, fmt.Errorf("secure resume key: %w", err)
	}
	key, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read resume key: %w", err)
	}
	if len(key) != 32 {
		return nil, fmt.Errorf("resume key %s must contain 32 bytes; restore its backup or remove it to reset (old runs will no longer resume)", path)
	}
	return key, nil
}
