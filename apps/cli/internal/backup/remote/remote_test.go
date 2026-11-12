package remote_test

import (
	"archive/tar"
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	devexpaths "github.com/jameswlane/devex/packages/plugin-sdk/paths"

	"github.com/jameswlane/devex/apps/cli/internal/backup"
	"github.com/jameswlane/devex/apps/cli/internal/backup/remote"
)

// fakeStore is in-memory object storage with failure injection.
type fakeStore struct {
	mu         sync.Mutex
	objects    map[string][]byte
	failPut    string
	truncateOn string
	corruptOn  string
	// beforePut runs before each Put, outside the lock, so a spec can
	// interleave a second push at an exact point of the first.
	beforePut func(key string)
}

func newFakeStore() *fakeStore { return &fakeStore{objects: map[string][]byte{}} }

func (f *fakeStore) Put(_ context.Context, key string, data []byte) error {
	if f.beforePut != nil {
		f.beforePut(key)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.failPut != "" && strings.HasSuffix(key, f.failPut) {
		return errors.New("connection reset by peer")
	}
	f.objects[key] = append([]byte(nil), data...)
	return nil
}

func (f *fakeStore) Get(_ context.Context, key string) ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	data, ok := f.objects[key]
	if !ok {
		return nil, remote.ErrNotFound
	}
	data = append([]byte(nil), data...)
	if f.truncateOn != "" && strings.HasSuffix(key, f.truncateOn) {
		data = data[:len(data)/2]
	}
	if f.corruptOn != "" && strings.HasSuffix(key, f.corruptOn) && len(data) > 0 {
		data[len(data)-1] ^= 0xff
	}
	return data, nil
}

func (f *fakeStore) List(_ context.Context, prefix string) ([]remote.Object, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	var objects []remote.Object
	for key, data := range f.objects {
		if strings.HasPrefix(key, prefix) {
			objects = append(objects, remote.Object{Key: key, Size: int64(len(data))})
		}
	}
	sort.Slice(objects, func(i, j int) bool { return objects[i].Key < objects[j].Key })
	return objects, nil
}

var _ = Describe("Remote backups", func() {
	var (
		ctx      context.Context
		store    *fakeStore
		location remote.Remote
		manager  *backup.BackupManager
		home     string
		id       string
		secret   remote.Policy
	)

	BeforeEach(func() {
		ctx = context.Background()
		home = GinkgoT().TempDir()
		GinkgoT().Setenv("HOME", home)
		for _, key := range []string{"XDG_CONFIG_HOME", "XDG_DATA_HOME", "XDG_STATE_HOME", "DEVEX_CONFIG_DIR", "DEVEX_DATA_DIR", "DEVEX_STATE_DIR"} {
			GinkgoT().Setenv(key, "")
		}
		configDir := devexpaths.ForHome(home).Config
		// BackupManager backs up <baseDir>/config, as the backup commands use it.
		Expect(os.MkdirAll(filepath.Join(configDir, "config"), 0o750)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(configDir, "config", "config.yaml"), []byte("git:\n  name: Me\n"), 0o600)).To(Succeed())
		manager = backup.NewBackupManager(configDir)
		metadata, err := manager.CreateBackup(backup.BackupOptions{Description: "test", Type: "manual", Compress: true})
		Expect(err).NotTo(HaveOccurred())
		id = metadata.ID
		store = newFakeStore()
		location = remote.Remote{Store: store, Prefix: "devex-backups/"}
		secret = remote.Policy{Passphrase: "correct horse battery staple"}
	})

	moveAside := func() {
		dir, err := manager.BackupDir(id)
		Expect(err).NotTo(HaveOccurred())
		Expect(os.RemoveAll(dir)).To(Succeed())
	}

	It("pushes an encrypted backup and pulls it back intact", func() {
		original, err := manager.BackupDir(id)
		Expect(err).NotTo(HaveOccurred())
		before, err := os.ReadFile(filepath.Join(original, "config.tar.gz"))
		Expect(err).NotTo(HaveOccurred())

		manifest, err := location.Push(ctx, manager, id, secret)
		Expect(err).NotTo(HaveOccurred())
		Expect(manifest.Encrypted).To(BeTrue())
		payloadKey := "devex-backups/" + id + "." + manifest.SHA256 + ".devexbak"
		Expect(store.objects).To(HaveKey(payloadKey))
		Expect(bytes.Contains(store.objects[payloadKey], before[:16])).To(BeFalse())

		listed, err := location.List(ctx)
		Expect(err).NotTo(HaveOccurred())
		Expect(listed).To(ConsistOf(HaveField("ID", id)))

		moveAside()
		_, err = location.Pull(ctx, manager, id, secret)
		Expect(err).NotTo(HaveOccurred())
		restored, err := manager.BackupDir(id)
		Expect(err).NotTo(HaveOccurred())
		after, err := os.ReadFile(filepath.Join(restored, "config.tar.gz"))
		Expect(err).NotTo(HaveOccurred())
		Expect(after).To(Equal(before))
		Expect(manager.RestoreBackup(id, "")).To(Succeed())
	})

	It("refuses an unencrypted upload unless the policy allows it", func() {
		_, err := location.Push(ctx, manager, id, remote.Policy{})
		Expect(err).To(MatchError(ContainSubstring("DEVEX_BACKUP_PASSPHRASE")))
		Expect(store.objects).To(BeEmpty())

		manifest, err := location.Push(ctx, manager, id, remote.Policy{AllowUnencrypted: true})
		Expect(err).NotTo(HaveOccurred())
		Expect(manifest.Encrypted).To(BeFalse())
	})

	It("does not list a backup whose upload failed", func() {
		store.failPut = ".devexbak"
		_, err := location.Push(ctx, manager, id, secret)
		Expect(err).To(MatchError(ContainSubstring("connection reset")))
		listed, err := location.List(ctx)
		Expect(err).NotTo(HaveOccurred())
		Expect(listed).To(BeEmpty())
	})

	It("catches an interrupted upload when reading it back", func() {
		store.truncateOn = ".devexbak"
		_, err := location.Push(ctx, manager, id, secret)
		Expect(err).To(MatchError(ContainSubstring("interrupted or altered")))
		Expect(store.objects).NotTo(HaveKey("devex-backups/" + id + ".json"))
	})

	DescribeTable("refuses damaged downloads before decrypting",
		func(damage func(), message string) {
			_, err := location.Push(ctx, manager, id, secret)
			Expect(err).NotTo(HaveOccurred())
			moveAside()
			damage()
			_, err = location.Pull(ctx, manager, id, secret)
			Expect(err).To(MatchError(ContainSubstring(message)))
			_, err = manager.BackupDir(id)
			Expect(err).To(HaveOccurred())
		},
		Entry("truncated", func() { store.truncateOn = ".devexbak" }, "interrupted or altered"),
		Entry("corrupted", func() { store.corruptOn = ".devexbak" }, "corrupted or altered"),
	)

	It("refuses a wrong passphrase", func() {
		_, err := location.Push(ctx, manager, id, secret)
		Expect(err).NotTo(HaveOccurred())
		moveAside()
		_, err = location.Pull(ctx, manager, id, remote.Policy{Passphrase: "wrong"})
		Expect(err).To(MatchError(ContainSubstring("wrong passphrase")))
		_, err = location.Pull(ctx, manager, id, remote.Policy{})
		Expect(err).To(MatchError(ContainSubstring("encrypted")))
	})

	It("never overwrites a local backup with the same id", func() {
		_, err := location.Push(ctx, manager, id, secret)
		Expect(err).NotTo(HaveOccurred())
		_, err = location.Pull(ctx, manager, id, secret)
		Expect(err).To(MatchError(ContainSubstring("already exists locally")))
	})

	It("rejects unsafe ids", func() {
		for _, bad := range []string{"../x", "a/b", ".hidden", ""} {
			_, err := location.Pull(ctx, manager, bad, secret)
			Expect(err).To(MatchError(ContainSubstring("invalid backup id")), bad)
		}
	})

	It("refuses archives with unsafe entries", func() {
		var buffer bytes.Buffer
		writer := tar.NewWriter(&buffer)
		Expect(writer.WriteHeader(&tar.Header{Name: "../escape", Mode: 0o600, Size: 1, Typeflag: tar.TypeReg})).To(Succeed())
		_, _ = writer.Write([]byte("x"))
		Expect(writer.Close()).To(Succeed())
		payload, err := remote.Encrypt(buffer.Bytes(), secret.Passphrase)
		Expect(err).NotTo(HaveOccurred())
		publishRaw(store.objects, location.Prefix, "evil", payload)

		_, err = location.Pull(ctx, manager, "evil", secret)
		Expect(err).To(MatchError(ContainSubstring("not a safe relative path")))
		Expect(filepath.Join(filepath.Dir(devexpaths.ForHome(home).Config), "escape")).NotTo(BeAnExistingFile())
	})

	// readAside returns the local backup's archive and removes the backup, as
	// on a machine that has to recover it from remote storage.
	readAside := func() []byte {
		dir, err := manager.BackupDir(id)
		Expect(err).NotTo(HaveOccurred())
		data, err := os.ReadFile(filepath.Join(dir, "config.tar.gz"))
		Expect(err).NotTo(HaveOccurred())
		Expect(manager.DeleteBackup(id)).To(Succeed())
		return data
	}

	expectPullable := func(want []byte) {
		_, err := location.Pull(ctx, manager, id, secret)
		Expect(err).NotTo(HaveOccurred())
		dir, err := manager.BackupDir(id)
		Expect(err).NotTo(HaveOccurred())
		Expect(os.ReadFile(filepath.Join(dir, "config.tar.gz"))).To(Equal(want))
	}

	Context("when the remote id is already in use", func() {
		It("refuses to replace a published backup and leaves it pullable", func() {
			_, err := location.Push(ctx, manager, id, secret)
			Expect(err).NotTo(HaveOccurred())
			published := map[string][]byte{}
			for key, data := range store.objects {
				published[key] = data
			}

			_, err = location.Push(ctx, manager, id, secret)
			Expect(err).To(MatchError(ContainSubstring("already has backup " + id)))
			Expect(err).To(MatchError(ContainSubstring("backup.remote.prefix")))
			Expect(store.objects).To(Equal(published))
			expectPullable(readAside())
		})

		DescribeTable("keeps the published backup pullable when a push of the same id interleaves",
			func(otherManifestFails bool) {
				var otherErr error
				ran := false
				store.beforePut = func(key string) {
					if ran || !strings.HasSuffix(key, ".json") {
						return
					}
					// The first push has uploaded and verified its payload and
					// is about to publish its manifest; a second push of the
					// same id runs to completion (or fails) right now.
					ran = true
					if otherManifestFails {
						store.failPut = ".json"
					}
					_, otherErr = location.Push(ctx, manager, id, secret)
					store.failPut = ""
				}

				_, err := location.Push(ctx, manager, id, secret)
				Expect(err).NotTo(HaveOccurred())
				Expect(ran).To(BeTrue())
				if otherManifestFails {
					Expect(otherErr).To(MatchError(ContainSubstring("upload manifest")))
				} else {
					Expect(otherErr).NotTo(HaveOccurred())
				}
				expectPullable(readAside())
			},
			Entry("and its manifest upload fails", true),
			Entry("and it completes first", false),
		)
	})

	Context("when pulling onto another machine", func() {
		It("registers the pulled backup in a fresh local backup index", func() {
			_, err := location.Push(ctx, manager, id, secret)
			Expect(err).NotTo(HaveOccurred())

			fresh := GinkgoT().TempDir()
			GinkgoT().Setenv("HOME", fresh)
			recovery := backup.NewBackupManager(devexpaths.ForHome(fresh).Config)
			Expect(recovery.ListBackups("", 0)).To(BeEmpty())

			_, err = location.Pull(ctx, recovery, id, secret)
			Expect(err).NotTo(HaveOccurred())
			Expect(recovery.ListBackups("", 0)).To(ConsistOf(HaveField("ID", id)))
			Expect(recovery.RestoreBackup(id, "")).To(Succeed())
			Expect(filepath.Join(devexpaths.ForHome(fresh).Config, "config", "config.yaml")).To(BeARegularFile())
		})

		It("refuses an archive whose metadata names another backup", func() {
			var buffer bytes.Buffer
			writer := tar.NewWriter(&buffer)
			metadata := []byte(`{"id":"other"}`)
			Expect(writer.WriteHeader(&tar.Header{Name: "metadata.json", Mode: 0o600, Size: int64(len(metadata)), Typeflag: tar.TypeReg})).To(Succeed())
			_, _ = writer.Write(metadata)
			Expect(writer.Close()).To(Succeed())
			payload, err := remote.Encrypt(buffer.Bytes(), secret.Passphrase)
			Expect(err).NotTo(HaveOccurred())
			publishRaw(store.objects, location.Prefix, "imported", payload)

			_, err = location.Pull(ctx, manager, "imported", secret)
			Expect(err).To(MatchError(ContainSubstring(`metadata for backup "other"`)))
			_, err = manager.BackupDir("imported")
			Expect(err).To(HaveOccurred())
			Expect(manager.ListBackups("imported", 0)).To(BeEmpty())
		})
	})

	Context("with files near the per-file limit", func() {
		addFile := func(size int64) {
			dir, err := manager.BackupDir(id)
			Expect(err).NotTo(HaveOccurred())
			file, err := os.Create(filepath.Join(dir, "large.bin"))
			Expect(err).NotTo(HaveOccurred())
			Expect(file.Truncate(size)).To(Succeed())
			Expect(file.Close()).To(Succeed())
		}

		It("pulls every backup it pushes, up to the limit", func() {
			DeferCleanup(remote.SetMaxEntryBytes(4096))
			addFile(4096)
			_, err := location.Push(ctx, manager, id, secret)
			Expect(err).NotTo(HaveOccurred())
			expectPullable(readAside())
		})

		It("refuses before uploading a file a pull would reject", func() {
			DeferCleanup(remote.SetMaxEntryBytes(4096))
			addFile(4097)
			_, err := location.Push(ctx, manager, id, secret)
			Expect(err).To(MatchError(ContainSubstring("large.bin is 4097 bytes")))
			Expect(err).To(MatchError(ContainSubstring("--compress=false")))
			Expect(store.objects).To(BeEmpty())
		})

		It("applies the local extraction limit by default", func() {
			addFile(backup.MaxFileSize + 1)
			_, err := location.Push(ctx, manager, id, secret)
			Expect(err).To(MatchError(ContainSubstring("over the 104857600-byte limit")))
			Expect(store.objects).To(BeEmpty())
		})
	})

	Context("with entry counts near the archive limit", func() {
		// The local backup holds config.tar.gz and metadata.json; add a
		// directory with files so the archive has exactly total entries.
		addEntries := func(total int) {
			dir, err := manager.BackupDir(id)
			Expect(err).NotTo(HaveOccurred())
			Expect(os.Mkdir(filepath.Join(dir, "extra"), 0o750)).To(Succeed())
			for i := 0; i < total-3; i++ {
				Expect(os.WriteFile(filepath.Join(dir, "extra", strconv.Itoa(i)), []byte("x"), 0o600)).To(Succeed())
			}
		}

		It("pulls every backup it pushes, up to the limit", func() {
			DeferCleanup(remote.SetMaxEntries(6))
			addEntries(6)
			_, err := location.Push(ctx, manager, id, secret)
			Expect(err).NotTo(HaveOccurred())
			expectPullable(readAside())
		})

		It("refuses before uploading an archive with more entries than a pull accepts", func() {
			DeferCleanup(remote.SetMaxEntries(6))
			addEntries(7)
			_, err := location.Push(ctx, manager, id, secret)
			Expect(err).To(MatchError(ContainSubstring("more than 6 files and directories")))
			Expect(err).To(MatchError(ContainSubstring("compressed backup")))
			Expect(store.objects).To(BeEmpty())
		})

		It("applies the local extraction limit by default", func() {
			addEntries(backup.MaxFiles + 1)
			_, err := location.Push(ctx, manager, id, secret)
			Expect(err).To(MatchError(ContainSubstring("more than 10000 files and directories")))
			Expect(store.objects).To(BeEmpty())
		})
	})

	It("round-trips encryption and detects tampering", func() {
		sealed, err := remote.Encrypt([]byte("hello"), "pw")
		Expect(err).NotTo(HaveOccurred())
		opened, err := remote.Decrypt(sealed, "pw")
		Expect(err).NotTo(HaveOccurred())
		Expect(string(opened)).To(Equal("hello"))
		sealed[len(sealed)-1] ^= 1
		_, err = remote.Decrypt(sealed, "pw")
		Expect(err).To(HaveOccurred())
		_, err = remote.Decrypt([]byte("plain"), "pw")
		Expect(err).To(MatchError(ContainSubstring("not an encrypted")))
	})
})
