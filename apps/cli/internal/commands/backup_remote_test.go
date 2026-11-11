package commands

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/viper"

	devexpaths "github.com/jameswlane/devex/packages/plugin-sdk/paths"

	"github.com/jameswlane/devex/apps/cli/internal/backup"
	"github.com/jameswlane/devex/apps/cli/internal/backup/remote"
	"github.com/jameswlane/devex/apps/cli/internal/config"
)

type memoryStore struct {
	mu      sync.Mutex
	objects map[string][]byte
}

func (m *memoryStore) Put(_ context.Context, key string, data []byte) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.objects[key] = append([]byte(nil), data...)
	return nil
}

func (m *memoryStore) Get(_ context.Context, key string) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	data, ok := m.objects[key]
	if !ok {
		return nil, remote.ErrNotFound
	}
	return append([]byte(nil), data...), nil
}

func (m *memoryStore) List(_ context.Context, prefix string) ([]remote.Object, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	var objects []remote.Object
	for key := range m.objects {
		if strings.HasPrefix(key, prefix) {
			objects = append(objects, remote.Object{Key: key})
		}
	}
	sort.Slice(objects, func(i, j int) bool { return objects[i].Key < objects[j].Key })
	return objects, nil
}

var _ = Describe("Remote backup commands", func() {
	var (
		settings config.CrossPlatformSettings
		store    *memoryStore
		output   bytes.Buffer
		id       string
	)
	BeforeEach(func() {
		viper.Reset()
		DeferCleanup(viper.Reset)
		home := GinkgoT().TempDir()
		GinkgoT().Setenv("HOME", home)
		for _, key := range []string{"XDG_CONFIG_HOME", "DEVEX_CONFIG_DIR", "DEVEX_BACKUP_PASSPHRASE", "DEVEX_BACKUP_ACCESS_KEY_ID", "DEVEX_BACKUP_SECRET_ACCESS_KEY", "AWS_ACCESS_KEY_ID", "AWS_SECRET_ACCESS_KEY"} {
			GinkgoT().Setenv(key, "")
		}
		settings = config.CrossPlatformSettings{HomeDir: home}
		configDir := devexpaths.ForHome(home).Config
		Expect(os.MkdirAll(filepath.Join(configDir, "config"), 0o750)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(configDir, "config", "config.yaml"), []byte("x: 1\n"), 0o600)).To(Succeed())
		metadata, err := backup.NewBackupManager(configDir).CreateBackup(backup.BackupOptions{Type: "manual", Compress: true})
		Expect(err).NotTo(HaveOccurred())
		id = metadata.ID
		store = &memoryStore{objects: map[string][]byte{}}
		previous := newRemoteBackupStore
		newRemoteBackupStore = func() (remote.Store, error) { return store, nil }
		DeferCleanup(func() { newRemoteBackupStore = previous })
		output.Reset()
	})
	execute := func(args ...string) error {
		cmd := newConfigBackupCmd(settings)
		cmd.SetOut(&output)
		cmd.SetErr(&output)
		cmd.SetArgs(args)
		return cmd.ExecuteContext(context.Background())
	}

	It("requires a passphrase or an explicit unencrypted policy", func() {
		Expect(execute("push", id)).To(MatchError(ContainSubstring("DEVEX_BACKUP_PASSPHRASE")))
		viper.Set("backup.remote.encrypt", false)
		Expect(execute("push", id)).To(Succeed())
		Expect(output.String()).To(ContainSubstring("unencrypted"))
	})

	It("pushes, lists and pulls a backup under the configured prefix", func() {
		GinkgoT().Setenv("DEVEX_BACKUP_PASSPHRASE", "pw")
		viper.Set("backup.remote.prefix", "/team/laptop/")
		Expect(execute("push", id)).To(Succeed())
		Expect(store.objects).To(HaveKey("team/laptop/" + id + ".devexbak"))

		Expect(execute("remote")).To(Succeed())
		Expect(output.String()).To(ContainSubstring(id))

		Expect(execute("pull", id)).To(MatchError(ContainSubstring("already exists locally")))
		dir, err := backup.NewBackupManager(devexpaths.ForHome(settings.HomeDir).Config).BackupDir(id)
		Expect(err).NotTo(HaveOccurred())
		Expect(os.RemoveAll(dir)).To(Succeed())
		Expect(execute("pull", id)).To(Succeed())
		Expect(output.String()).To(ContainSubstring("devex config backup restore " + id))
	})

	It("explains missing storage configuration and credentials", func() {
		newRemoteBackupStore = func() (remote.Store, error) {
			return remote.NewS3(remote.S3Config{Endpoint: viper.GetString("backup.remote.endpoint"), Bucket: viper.GetString("backup.remote.bucket")})
		}
		Expect(execute("remote")).To(MatchError(ContainSubstring("backup.remote.endpoint and backup.remote.bucket")))
		viper.Set("backup.remote.endpoint", "s3.example.com")
		viper.Set("backup.remote.bucket", "b")
		Expect(execute("remote")).To(MatchError(ContainSubstring("DEVEX_BACKUP_ACCESS_KEY_ID")))
		_, err := remote.NewS3(remote.S3Config{Endpoint: "https://s3.example.com", Bucket: "b", AccessKey: "a", SecretKey: "s"})
		Expect(err).To(MatchError(ContainSubstring("not a URL")))
	})
})
