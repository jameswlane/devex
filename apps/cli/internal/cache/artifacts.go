package cache

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
)

// InstallCacheOptions controls installation cache access.
type InstallCacheOptions struct {
	Bypass  bool
	Offline bool
	Hit     func(string)
}

// ArtifactKey identifies immutable bytes independently of the application name.
func ArtifactKey(url, digest string) string {
	sum := sha256.Sum256([]byte(url + "\x00" + digest))
	return "artifact-" + hex.EncodeToString(sum[:])
}

// FetchArtifact copies verified bytes into an install-owned file. Cache eviction
// after this call cannot remove the file being installed.
func (cm *CacheManager) FetchArtifact(ctx context.Context, url, destination string, verification sdk.ArtifactVerification, options InstallCacheOptions, download func(context.Context, string, string) error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if options.Bypass && options.Offline {
		return fmt.Errorf("--no-cache and --offline cannot be combined for artifact installation")
	}
	// Signature checks may require network access; offline must never fetch them.
	if options.Offline && verification.SignatureURL != "" {
		return fmt.Errorf("offline signature verification is unavailable; retry online")
	}
	key := ArtifactKey(url, verification.SHA256)
	if !options.Bypass && verification.SHA256 != "" {
		entry, err := cm.GetCacheEntry(key)
		if err != nil {
			return err
		}
		if entry != nil {
			// Copy first, then verify the exact private copy the installer consumes.
			if err := cm.copyToCache(entry.Path, destination); err == nil {
				if _, err := sdk.VerifyArtifact(ctx, destination, verification); err == nil {
					if options.Hit != nil {
						options.Hit(key)
					}
					return nil
				}
			}
			if err := cm.RemoveCacheEntry(key); err != nil {
				return err
			}
		}
	}
	if options.Offline {
		return fmt.Errorf("offline artifact cache miss for %s: install once online with a pinned SHA256", url)
	}
	if err := download(ctx, url, destination); err != nil {
		return err
	}
	result, err := sdk.VerifyArtifact(ctx, destination, verification)
	if err != nil {
		return fmt.Errorf("artifact integrity verification failed: %w", err)
	}
	if !options.Bypass && result.ChecksumVerified {
		if _, err := cm.SetCacheEntry(key, CacheTypeDownload, destination, map[string]string{"url": url, "sha256": verification.SHA256}, 30*24*time.Hour); err != nil {
			return err
		}
	}
	return nil
}

// DownloadArtifact downloads with cancellation and a bounded request duration.
func DownloadArtifact(ctx context.Context, url, destination string) error {
	if err := validateArtifactURL(url); err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	response, err := (&http.Client{Timeout: 5 * time.Minute, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) >= 10 {
			return fmt.Errorf("stopped after 10 redirects")
		}
		return validateArtifactURL(req.URL.String())
	}}).Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed: HTTP %d", response.StatusCode)
	}
	file, err := os.OpenFile(destination, os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	_, copyErr := io.Copy(file, response.Body)
	closeErr := file.Close()
	if copyErr != nil {
		return copyErr
	}
	return closeErr
}

// Validate caches successful immutable-content validation only. The version must
// include the artifact digest and the validator revision, not installed state.
func (cm *CacheManager) Validate(ctx context.Context, app, version, validator string, options InstallCacheOptions, validate func() error) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	key := "validation-" + ArtifactKey(app, version+"\x00"+validator)
	if !options.Bypass {
		entry, err := cm.GetCacheEntry(key)
		if err != nil {
			return err
		}
		if entry != nil {
			data, err := os.ReadFile(entry.Path)
			if err == nil && string(data) == "valid" {
				if options.Hit != nil {
					options.Hit(key)
				}
				return nil
			}
			if err := cm.RemoveCacheEntry(key); err != nil {
				return err
			}
		}
	}
	// This validation is local and can safely run on an offline cache miss.
	if err := validate(); err != nil {
		return err
	}
	if options.Bypass {
		return nil
	}
	file, err := os.CreateTemp("", "devex-validation-*")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(file.Name()) }()
	_, err = file.WriteString("valid")
	closeErr := file.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	_, err = cm.SetCacheEntry(key, CacheTypeMetadata, file.Name(), map[string]string{"app": app, "version": version, "validator": validator}, time.Hour)
	return err
}

func validateArtifactURL(source string) error {
	parsed, err := url.Parse(source)
	if err != nil {
		return fmt.Errorf("invalid artifact URL: %w", err)
	}
	if parsed.Scheme != "https" || parsed.Hostname() == "" || len(source) > 2048 {
		return fmt.Errorf("artifact downloads require an HTTPS URL with a valid hostname (max 2048 characters)")
	}
	hostname := strings.ToLower(parsed.Hostname())
	ip := net.ParseIP(hostname)
	if hostname == "localhost" || strings.HasSuffix(hostname, ".localhost") || (ip != nil && (ip.IsPrivate() || ip.IsLoopback() || ip.IsUnspecified() || ip.IsLinkLocalUnicast())) {
		return fmt.Errorf("artifact download URL cannot target a local or private address")
	}
	return nil
}
