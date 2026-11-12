// Package remote copies local configuration backups to remote object storage
// and back.
//
// A backup directory is packed into one tar archive, encrypted with
// AES-256-GCM under a key derived from a passphrase (scrypt), and uploaded as
// <prefix><id>.<sha256>.devexbak with a <prefix><id>.json manifest holding its
// SHA-256, size and whether it is encrypted. Payload keys are content
// addressed, so a push never replaces the payload an existing manifest names.
// Uploads are read back and verified; pulls verify the checksum and size
// before decrypting, and GCM authenticates the content. A pulled backup
// becomes a new local backup, registered in the local backup index, that the
// existing restore command applies.
package remote

import (
	"archive/tar"
	"bytes"
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"golang.org/x/crypto/scrypt"

	"github.com/jameswlane/devex/apps/cli/internal/backup"
)

const (
	// MaxArchiveBytes bounds a packed backup.
	MaxArchiveBytes = 256 << 20
	archiveSuffix   = ".devexbak"
	manifestSuffix  = ".json"
	magic           = "DEVEXBK1"
	saltSize        = 16
	nonceSize       = 12
	tagSize         = 16
	// encryptionOverhead is what Encrypt adds: magic, salt, nonce and GCM tag.
	encryptionOverhead = len(magic) + saltSize + nonceSize + tagSize
	// MaxObjectBytes bounds a stored payload: a full archive plus encryption.
	MaxObjectBytes = MaxArchiveBytes + encryptionOverhead
)

// maxEntryBytes is the largest file a remote backup holds. Pack and unpack
// share it, so every backup that can be pushed can also be pulled. It is the
// local extraction limit; tests lower it.
var maxEntryBytes int64 = backup.MaxFileSize

// maxEntries is the most archive entries (files and directories) a remote
// backup holds; pack and unpack share it for the same reason. Tests lower it.
var maxEntries = backup.MaxFiles

// ErrNotFound is returned by a Store for a missing object.
var ErrNotFound = errors.New("object not found")

// Object describes a stored object.
type Object struct {
	Key  string
	Size int64
}

// Store is remote object storage.
type Store interface {
	Put(ctx context.Context, key string, data []byte) error
	Get(ctx context.Context, key string) ([]byte, error)
	List(ctx context.Context, prefix string) ([]Object, error)
}

// Policy is the protection policy for pushed backups.
type Policy struct {
	// Passphrase encrypts pushed backups and decrypts pulled ones.
	Passphrase string
	// AllowUnencrypted permits pushing without a passphrase. It must be set
	// explicitly; without it a push with no passphrase fails.
	AllowUnencrypted bool
}

// Manifest describes a remote backup.
type Manifest struct {
	ID        string                 `json:"id"`
	Encrypted bool                   `json:"encrypted"`
	SHA256    string                 `json:"sha256"`
	Size      int64                  `json:"size"`
	PushedAt  time.Time              `json:"pushed_at"`
	Metadata  *backup.BackupMetadata `json:"metadata,omitempty"`
}

// Remote is a remote backup location.
type Remote struct {
	Store  Store
	Prefix string
}

func (r Remote) key(id, suffix string) string { return r.Prefix + id + suffix }

// payloadKey is content addressed: different payloads never share a key, so
// publishing one cannot replace the payload another manifest names.
func (r Remote) payloadKey(manifest Manifest) string {
	return r.Prefix + manifest.ID + "." + manifest.SHA256 + archiveSuffix
}

// Push packs, protects, uploads and verifies the local backup id.
func (r Remote) Push(ctx context.Context, manager *backup.BackupManager, id string, policy Policy) (Manifest, error) {
	dir, err := manager.BackupDir(id)
	if err != nil {
		return Manifest{}, err
	}
	if policy.Passphrase == "" && !policy.AllowUnencrypted {
		return Manifest{}, errors.New("set DEVEX_BACKUP_PASSPHRASE to encrypt the backup, or set backup.remote.encrypt to false to upload it unencrypted")
	}
	// Never replace a published backup: it may be the only usable copy, or
	// another machine's backup with the same timestamp-based id.
	if _, err := r.Store.Get(ctx, r.key(id, manifestSuffix)); err == nil {
		return Manifest{}, fmt.Errorf("remote storage already has backup %s under %q and it was left unchanged; if another machine pushed it, give each machine its own backup.remote.prefix", id, r.Prefix)
	} else if !errors.Is(err, ErrNotFound) {
		return Manifest{}, fmt.Errorf("check remote storage for backup %s: %w", id, err)
	}
	archive, err := pack(dir)
	if err != nil {
		return Manifest{}, err
	}
	payload := archive
	encrypted := policy.Passphrase != ""
	if encrypted {
		if payload, err = Encrypt(archive, policy.Passphrase); err != nil {
			return Manifest{}, err
		}
	}
	if len(payload) > MaxObjectBytes {
		return Manifest{}, fmt.Errorf("backup %s is %d bytes once protected, over the %d-byte remote limit; exclude large files from the backup", id, len(payload), MaxObjectBytes)
	}
	sum := sha256.Sum256(payload)
	manifest := Manifest{ID: id, Encrypted: encrypted, SHA256: hex.EncodeToString(sum[:]), Size: int64(len(payload)), PushedAt: time.Now().UTC()}
	if metadata, err := manager.GetBackup(id); err == nil {
		manifest.Metadata = metadata
	}
	if err := r.Store.Put(ctx, r.payloadKey(manifest), payload); err != nil {
		return Manifest{}, fmt.Errorf("upload backup %s: %w", id, err)
	}
	// Read the upload back so an interrupted or altered transfer is caught now.
	stored, err := r.Store.Get(ctx, r.payloadKey(manifest))
	if err != nil {
		return Manifest{}, fmt.Errorf("verify uploaded backup %s: %w", id, err)
	}
	if err := verify(stored, manifest); err != nil {
		return Manifest{}, fmt.Errorf("verify uploaded backup %s: %w", id, err)
	}
	data, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return Manifest{}, err
	}
	// The manifest is written last and names a payload no other push can
	// overwrite, so a listed backup is always complete.
	if err := r.Store.Put(ctx, r.key(id, manifestSuffix), data); err != nil {
		return Manifest{}, fmt.Errorf("upload manifest for %s: %w", id, err)
	}
	return manifest, nil
}

// Pull downloads, verifies and unpacks remote backup id as a new local backup.
func (r Remote) Pull(ctx context.Context, manager *backup.BackupManager, id string, policy Policy) (Manifest, error) {
	target, err := manager.NewBackupDir(id)
	if err != nil {
		return Manifest{}, err
	}
	data, err := r.Store.Get(ctx, r.key(id, manifestSuffix))
	if err != nil {
		return Manifest{}, fmt.Errorf("download manifest for %s: %w", id, err)
	}
	var manifest Manifest
	if err := json.Unmarshal(data, &manifest); err != nil {
		return Manifest{}, fmt.Errorf("read manifest for %s: %w", id, err)
	}
	if manifest.ID != id {
		return Manifest{}, fmt.Errorf("manifest for %s names backup %q", id, manifest.ID)
	}
	if !validSHA256(manifest.SHA256) {
		return Manifest{}, fmt.Errorf("manifest for %s has no valid SHA-256", id)
	}
	payload, err := r.Store.Get(ctx, r.payloadKey(manifest))
	if err != nil {
		return Manifest{}, fmt.Errorf("download backup %s: %w", id, err)
	}
	if err := verify(payload, manifest); err != nil {
		return Manifest{}, fmt.Errorf("downloaded backup %s: %w", id, err)
	}
	archive := payload
	if manifest.Encrypted {
		if policy.Passphrase == "" {
			return Manifest{}, errors.New("backup is encrypted; set DEVEX_BACKUP_PASSPHRASE")
		}
		if archive, err = Decrypt(payload, policy.Passphrase); err != nil {
			return Manifest{}, err
		}
	}
	staging, err := unpack(archive, filepath.Dir(target))
	if err != nil {
		return Manifest{}, err
	}
	defer func() { _ = os.RemoveAll(staging) }()
	// Publish and index the backup so list, restore and retention see it.
	if _, err := manager.ImportBackup(id, staging); err != nil {
		return Manifest{}, err
	}
	return manifest, nil
}

// List returns the complete remote backups, newest first.
func (r Remote) List(ctx context.Context) ([]Manifest, error) {
	objects, err := r.Store.List(ctx, r.Prefix)
	if err != nil {
		return nil, err
	}
	manifests := []Manifest{}
	for _, object := range objects {
		if !strings.HasSuffix(object.Key, manifestSuffix) {
			continue
		}
		data, err := r.Store.Get(ctx, object.Key)
		if err != nil {
			return nil, err
		}
		var manifest Manifest
		if err := json.Unmarshal(data, &manifest); err != nil || backup.ValidateID(manifest.ID) != nil {
			continue
		}
		manifests = append(manifests, manifest)
	}
	sort.Slice(manifests, func(i, j int) bool { return manifests[i].PushedAt.After(manifests[j].PushedAt) })
	return manifests, nil
}

func validSHA256(sum string) bool {
	decoded, err := hex.DecodeString(sum)
	return err == nil && len(decoded) == sha256.Size && hex.EncodeToString(decoded) == sum
}

func verify(payload []byte, manifest Manifest) error {
	if int64(len(payload)) != manifest.Size {
		return fmt.Errorf("size %d does not match the recorded %d (interrupted or altered transfer)", len(payload), manifest.Size)
	}
	sum := sha256.Sum256(payload)
	if hex.EncodeToString(sum[:]) != manifest.SHA256 {
		return errors.New("checksum does not match the recorded SHA-256 (corrupted or altered transfer)")
	}
	return nil
}

func deriveKey(passphrase string, salt []byte) ([]byte, error) {
	return scrypt.Key([]byte(passphrase), salt, 1<<15, 8, 1, 32)
}

// Encrypt seals plaintext with AES-256-GCM under a key derived from passphrase.
func Encrypt(plaintext []byte, passphrase string) ([]byte, error) {
	salt := make([]byte, saltSize)
	if _, err := rand.Read(salt); err != nil {
		return nil, err
	}
	key, err := deriveKey(passphrase, salt)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	header := append(append([]byte(magic), salt...), nonce...)
	return gcm.Seal(header, nonce, plaintext, []byte(magic)), nil
}

// Decrypt opens data sealed by Encrypt.
func Decrypt(data []byte, passphrase string) ([]byte, error) {
	if len(data) < len(magic)+saltSize || string(data[:len(magic)]) != magic {
		return nil, errors.New("not an encrypted DevEx backup")
	}
	salt := data[len(magic) : len(magic)+saltSize]
	key, err := deriveKey(passphrase, salt)
	if err != nil {
		return nil, err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	rest := data[len(magic)+saltSize:]
	if len(rest) < gcm.NonceSize() {
		return nil, errors.New("encrypted backup is truncated")
	}
	plaintext, err := gcm.Open(nil, rest[:gcm.NonceSize()], rest[gcm.NonceSize():], []byte(magic))
	if err != nil {
		return nil, errors.New("cannot decrypt the backup: wrong passphrase or altered content")
	}
	return plaintext, nil
}

// pack archives the regular files of dir; symlinks are refused.
func pack(dir string) ([]byte, error) {
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, fmt.Errorf("pack backup: %w", err)
	}
	defer func() { _ = root.Close() }()
	var buffer bytes.Buffer
	writer := tar.NewWriter(&buffer)
	entries := 0
	err = fs.WalkDir(root.FS(), ".", func(current string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if current == "." {
			return nil
		}
		// Every header counts, directories included, exactly as unpack counts.
		if entries++; entries > maxEntries {
			return fmt.Errorf("backup has more than %d files and directories, the most a remote backup can hold; push a compressed backup instead (devex config backup create), which stores the configuration as one file", maxEntries)
		}
		rel := current
		// Lstat through the root: a symlink is reported as one, never followed.
		info, err := root.Lstat(rel)
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() && !info.IsDir() {
			return fmt.Errorf("backup contains %s, which is not a regular file or directory", rel)
		}
		// Refuse before uploading anything a pull would reject.
		if info.Mode().IsRegular() && info.Size() > maxEntryBytes {
			return fmt.Errorf("%s is %d bytes, over the %d-byte limit for a file in a remote backup; push an uncompressed backup instead (devex config backup create --compress=false)", rel, info.Size(), maxEntryBytes)
		}
		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		header.Name = filepath.ToSlash(rel)
		if info.IsDir() {
			header.Name += "/"
		}
		header.Uid, header.Gid, header.Uname, header.Gname = 0, 0, "", ""
		if err := writer.WriteHeader(header); err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		file, err := root.Open(rel)
		if err != nil {
			return err
		}
		defer func() { _ = file.Close() }()
		if _, err := io.Copy(writer, file); err != nil {
			return err
		}
		if buffer.Len() > MaxArchiveBytes {
			return fmt.Errorf("backup exceeds the %d-byte remote limit; exclude large files from the backup", MaxArchiveBytes)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("pack backup: %w", err)
	}
	if err := writer.Close(); err != nil {
		return nil, err
	}
	// Close pads and terminates the archive, which can cross the limit.
	if buffer.Len() > MaxArchiveBytes {
		return nil, fmt.Errorf("pack backup: backup exceeds the %d-byte remote limit; exclude large files from the backup", MaxArchiveBytes)
	}
	return buffer.Bytes(), nil
}

// unpack extracts archive into a new staging directory under parent and
// returns it; the caller publishes or removes it. Only regular files and
// directories with safe relative names are accepted.
func unpack(archive []byte, parent string) (staging string, err error) {
	if err := os.MkdirAll(parent, 0o750); err != nil {
		return "", err
	}
	if staging, err = os.MkdirTemp(parent, ".devex-pull-*"); err != nil {
		return "", err
	}
	defer func() {
		if err != nil {
			_ = os.RemoveAll(staging)
			staging = ""
		}
	}()
	return staging, extract(archive, staging)
}

func extract(archive []byte, staging string) error {
	root, err := os.OpenRoot(staging)
	if err != nil {
		return err
	}
	defer func() { _ = root.Close() }()
	reader := tar.NewReader(bytes.NewReader(archive))
	var total int64
	for files := 0; ; files++ {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fmt.Errorf("read backup archive: %w", err)
		}
		if files >= maxEntries {
			return errors.New("backup archive has too many files")
		}
		name := strings.TrimSuffix(header.Name, "/")
		if !fs.ValidPath(name) || path.Clean(name) != name {
			return fmt.Errorf("backup archive entry %q is not a safe relative path", header.Name)
		}
		switch header.Typeflag {
		case tar.TypeDir:
			if err := root.MkdirAll(name, 0o750); err != nil {
				return err
			}
		case tar.TypeReg:
			if header.Size > maxEntryBytes {
				return fmt.Errorf("backup archive entry %q is %d bytes, over the %d-byte limit", name, header.Size, maxEntryBytes)
			}
			total += header.Size
			if total > backup.MaxTotalSize {
				return errors.New("backup archive is too large")
			}
			if dir := path.Dir(name); dir != "." {
				if err := root.MkdirAll(dir, 0o750); err != nil {
					return err
				}
			}
			file, err := root.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
			if err != nil {
				return err
			}
			_, copyErr := io.CopyN(file, reader, header.Size)
			if err := errors.Join(copyErr, file.Close()); err != nil {
				return err
			}
		default:
			return fmt.Errorf("backup archive entry %q is not a regular file or directory", header.Name)
		}
	}
	return root.Close()
}
