package sdk_test

import (
	"bytes"
	"context"
	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"os"
	"path/filepath"
)

var _ = Describe("Legacy directory migration", func() {
	var p sdk.Paths
	var output bytes.Buffer
	var home string
	write := func(path, content string) {
		Expect(os.MkdirAll(filepath.Dir(path), 0700)).To(Succeed())
		Expect(os.WriteFile(path, []byte(content), 0600)).To(Succeed())
	}
	BeforeEach(func() {
		home = GinkgoT().TempDir()
		p = sdk.ResolvePathsFor("linux", home, func(string) string { return "" })
		output.Reset()
	})
	It("moves the legacy layout and reads missing paths before migration", func() {
		old := filepath.Join(home, ".devex")
		entries := map[string]string{"config.yaml": "config", "config/applications.yaml": "apps", "datastore.db": "database", "plugins/plugin": "binary", "backups/backup": "backup", "cache/download": "cache"}
		for name, value := range entries {
			write(filepath.Join(old, name), value)
		}
		write(filepath.Join(p.Data, "logs/log"), "log")
		Expect(p.ReadPath(filepath.Join(p.Config, "config.yaml"))).To(Equal(filepath.Join(old, "config.yaml")))
		Expect(p.Migrate(context.Background(), false, &output)).To(Succeed())
		for path, value := range map[string]string{filepath.Join(p.Config, "config.yaml"): "config", filepath.Join(p.Config, "config/applications.yaml"): "apps", filepath.Join(p.State, "datastore.db"): "database", filepath.Join(p.Plugins(), "plugin"): "binary", filepath.Join(p.Backups(), "backup"): "backup", filepath.Join(p.Logs(), "log"): "log"} {
			Expect(os.ReadFile(path)).To(Equal([]byte(value)))
		}
		Expect(filepath.Join(old, "cache")).NotTo(BeADirectory())
		Expect(filepath.Join(old, "README")).To(BeAnExistingFile())
		Expect(output.String()).To(ContainSubstring("Moved"))
		output.Reset()
		Expect(p.Migrate(context.Background(), false, &output)).To(Succeed())
		Expect(output.String()).NotTo(ContainSubstring("Moved"))
	})
	It("continues partial migration and preserves existing destination conflicts", func() {
		old := filepath.Join(home, ".devex", "config")
		write(filepath.Join(old, "existing.yaml"), "old")
		write(filepath.Join(p.Config, "config", "existing.yaml"), "new")
		write(filepath.Join(old, "remaining.yaml"), "remaining")
		Expect(p.Migrate(context.Background(), false, &output)).To(Succeed())
		Expect(os.ReadFile(filepath.Join(p.Config, "config", "existing.yaml"))).To(Equal([]byte("new")))
		Expect(os.ReadFile(filepath.Join(old, "existing.yaml"))).To(Equal([]byte("old")))
		Expect(os.ReadFile(filepath.Join(p.Config, "config", "remaining.yaml"))).To(Equal([]byte("remaining")))
		Expect(p.Migrate(context.Background(), false, &output)).To(Succeed())
	})
	It("previews without creating destinations or removing cache", func() {
		old := filepath.Join(home, ".devex")
		write(filepath.Join(old, "config.yaml"), "config")
		write(filepath.Join(old, "cache/file"), "cache")
		Expect(p.Migrate(context.Background(), true, &output)).To(Succeed())
		Expect(output.String()).To(ContainSubstring("Would move"))
		Expect(output.String()).To(ContainSubstring("Would discard"))
		Expect(p.Config).NotTo(BeADirectory())
		Expect(filepath.Join(old, "README")).NotTo(BeAnExistingFile())
		Expect(filepath.Join(old, "cache/file")).To(BeAnExistingFile())
	})
	It("refuses symlinked source roots, nested files, and destination ancestors", func() {
		outside := GinkgoT().TempDir()
		write(filepath.Join(outside, "config.yaml"), "outside")
		old := filepath.Join(home, ".devex")
		Expect(os.Symlink(outside, old)).To(Succeed())
		Expect(p.Migrate(context.Background(), false, &output)).To(MatchError(ContainSubstring(sdk.ErrMigrationRefused.Error())))
		Expect(os.Remove(old)).To(Succeed())
		write(filepath.Join(old, "config/real"), "real")
		Expect(os.Symlink(filepath.Join(outside, "config.yaml"), filepath.Join(old, "config/link"))).To(Succeed())
		Expect(p.Migrate(context.Background(), false, &output)).To(MatchError(ContainSubstring(sdk.ErrMigrationRefused.Error())))
		Expect(filepath.Join(old, "config/real")).To(BeAnExistingFile())
		Expect(os.Remove(filepath.Join(old, "config/link"))).To(Succeed())
		Expect(os.MkdirAll(filepath.Dir(p.Config), 0700)).To(Succeed())
		Expect(os.Symlink(outside, p.Config)).To(Succeed())
		Expect(p.Migrate(context.Background(), false, &output)).To(MatchError(ContainSubstring(sdk.ErrMigrationRefused.Error())))
		Expect(os.ReadFile(filepath.Join(outside, "config.yaml"))).To(Equal([]byte("outside")))
	})
	It("honors cancellation and leaves recoverable sources", func() {
		source := filepath.Join(home, ".devex/config.yaml")
		write(source, "config")
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		Expect(p.Migrate(ctx, false, &output)).To(MatchError(context.Canceled))
		Expect(source).To(BeAnExistingFile())
		Expect(p.Migrate(context.Background(), false, &output)).To(Succeed())
	})
})
