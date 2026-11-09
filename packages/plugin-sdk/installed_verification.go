package sdk

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// VerifyInstalledPlugin compares the installed executable against a freshly
// verified registry payload, including archived releases. It executes no plugin.
func (d *Downloader) VerifyInstalledPlugin(ctx context.Context, name string, checksums, signatures bool) error {
	if name == "" || filepath.Base(name) != name || name == "." || name == ".." {
		return fmt.Errorf("invalid plugin name")
	}
	metadata, err := d.fetchPluginDirect(ctx, name)
	if err != nil {
		return err
	}
	binary, ok := metadata.Binaries[runtime.GOOS+"-"+runtime.GOARCH]
	if !ok {
		return fmt.Errorf("plugin platform metadata missing")
	}
	if checksums && binary.Checksum == "" {
		return fmt.Errorf("required plugin checksum missing")
	}
	dir, err := os.MkdirTemp("", "devex-verify-plugin-*")
	if err != nil {
		return err
	}
	defer func() { _ = os.RemoveAll(dir) }()
	verifier := NewSecureDownloader(DownloaderConfig{RegistryURL: d.registryURL, PluginDir: dir, CacheDir: d.cacheDir, VerifyChecksums: checksums, VerifySignatures: signatures, PublicKeyPath: d.publicKeyPath})
	verifier.SetSilent(true)
	if err := verifier.downloadAndVerifyPlugin(ctx, name, metadata.Version, binary); err != nil {
		return err
	}
	base := "devex-plugin-" + name
	if runtime.GOOS == "windows" {
		base += ".exe"
	}
	expected, err := os.Open(filepath.Join(dir, base))
	if err != nil {
		return err
	}
	defer func() { _ = expected.Close() }()
	digest, err := openFileSHA256(expected)
	if err != nil {
		return err
	}
	_, err = VerifyArtifact(ctx, filepath.Join(d.pluginDir, base), ArtifactVerification{SHA256: digest, Policy: "required"})
	return err
}
