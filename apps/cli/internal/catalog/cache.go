package catalog

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/jameswlane/devex/apps/cli/internal/types"
)

type Snapshot struct {
	URL     string
	ETag    string
	Checked time.Time
	Apps    []types.CrossPlatformApp
}
type Result struct {
	Apps        []types.CrossPlatformApp
	Source, URL string
	Age         time.Duration
	Warning     error
}
type Store struct {
	Client  *Client
	Dir     string
	TTL     time.Duration
	Offline bool
}

// Load prefers a fresh cache, revalidates stale entries, and preserves stale data
// on registry failure. Cache writes are atomic and never prevent using live data.
func (s Store) Load(ctx context.Context, bundled []types.CrossPlatformApp) Result {
	path := filepath.Join(s.Dir, fmt.Sprintf("applications-%x.json", sha256.Sum256([]byte(s.Client.URL))))
	var cached Snapshot
	data, err := os.ReadFile(path)
	valid := err == nil && json.Unmarshal(data, &cached) == nil && cached.URL == s.Client.URL && !cached.Checked.IsZero() && cached.Apps != nil
	age := time.Since(cached.Checked)
	if valid && age >= 0 && age < s.TTL {
		return Result{Apps: cached.Apps, Source: "cache", URL: s.Client.URL, Age: age}
	}
	var fetchErr error
	if s.Offline {
		fetchErr = fmt.Errorf("offline mode enabled")
	}
	if !s.Offline {
		tag := ""
		if valid {
			tag = cached.ETag
		}
		apps, etag, unchanged, err := s.Client.List(ctx, Filters{}, tag)
		fetchErr = err
		if err == nil && unchanged && !valid {
			fetchErr = fmt.Errorf("registry returned 304 without a cached catalog")
		}
		if err == nil && (!unchanged || valid) {
			if unchanged {
				apps = cached.Apps
				etag = cached.ETag
			}
			snapshot := Snapshot{URL: s.Client.URL, ETag: etag, Checked: time.Now(), Apps: apps}
			if apps == nil {
				snapshot.Apps = []types.CrossPlatformApp{}
			}
			writeErr := writeSnapshot(path, snapshot)
			source := "registry"
			if unchanged {
				source = "cache (revalidated)"
			}
			return Result{Apps: apps, Source: source, URL: s.Client.URL, Warning: writeErr}
		}
	}
	if valid {
		return Result{Apps: cached.Apps, Source: "cache (stale)", URL: s.Client.URL, Age: age, Warning: fetchErr}
	}
	return Result{Apps: bundled, Source: "bundled", URL: s.Client.URL, Warning: fetchErr}
}

func writeSnapshot(path string, snapshot Snapshot) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	data, err := json.Marshal(snapshot)
	if err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".applications-*")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(file.Name()) }()
	if _, err = file.Write(data); err != nil {
		_ = file.Close()
		return err
	}
	if err = file.Close(); err != nil {
		return err
	}
	return os.Rename(file.Name(), path)
}
