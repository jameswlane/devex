package selfupdate

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	devexpaths "github.com/jameswlane/devex/packages/plugin-sdk/paths"
)

// NotificationPolicy contains only cheap inputs; suppression is decided before
// resolving cache paths, starting goroutines, or creating an HTTP client.
type NotificationPolicy struct {
	Version, Format                            string
	TTY, CI, NonInteractive, Disabled, Offline bool
}

func (p NotificationPolicy) Enabled() bool {
	if !p.TTY || p.CI || p.NonInteractive || p.Disabled || p.Offline {
		return false
	}
	if strings.EqualFold(p.Format, "json") || strings.EqualFold(p.Format, "yaml") {
		return false
	}
	v, err := ParseVersion(p.Version)
	return err == nil && v.Prerelease() == ""
}

// CacheDir follows #209's proposed cache layout without migrating other storage.
func CacheDir() (string, error) {
	dirs, err := devexpaths.Resolve()
	return dirs.Cache, err
}

type notificationCache struct {
	CheckedAt time.Time `json:"checked_at"`
	Latest    string    `json:"latest,omitempty"`
}

// CheckNotification runs in the detached helper. Completed failures are cached
// too, so offline/rate-limited invocations do not retry for 24 hours.
func CheckNotification(ctx context.Context, client *Client, current, dir string, now time.Time) string {
	path := filepath.Join(dir, "update-check.json")
	var cached notificationCache
	data, err := os.ReadFile(path)
	if err == nil && json.Unmarshal(data, &cached) == nil && !cached.CheckedAt.After(now) && now.Sub(cached.CheckedAt) < 24*time.Hour {
		return notificationMessage(cached.Latest, current)
	}
	if err := ctx.Err(); err != nil {
		return ""
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return ""
	}
	// Hold the lock throughout networking. Exclusive creation avoids
	// duplicate checks by concurrent invocations; stale locks expire quickly.
	lockPath := path + ".lock"
	if info, err := os.Stat(lockPath); err == nil && now.Sub(info.ModTime()) > time.Minute {
		_ = os.Remove(lockPath)
	}
	lock, err := os.OpenFile(lockPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if err != nil {
		return ""
	}
	_ = lock.Close()
	defer func() { _ = os.Remove(lockPath) }()
	// Another process may have filled the cache before we obtained the lock.
	if data, err := os.ReadFile(path); err == nil && json.Unmarshal(data, &cached) == nil && !cached.CheckedAt.After(now) && now.Sub(cached.CheckedAt) < 24*time.Hour {
		return notificationMessage(cached.Latest, current)
	}
	release, err := client.Find(ctx, "", false)
	// An interrupted lookup has no result to cache and must remain retryable.
	if ctx.Err() != nil {
		return ""
	}
	cached = notificationCache{CheckedAt: now}
	if err == nil {
		cached.Latest = release.Tag
	}
	_ = writeCache(path, cached)
	return notificationMessage(cached.Latest, current)
}

func writeCache(path string, cached notificationCache) bool {
	data, err := json.Marshal(cached)
	if err != nil {
		return false
	}
	temp, err := os.CreateTemp(filepath.Dir(path), ".update-check-*")
	if err != nil {
		return false
	}
	defer func() { _ = os.Remove(temp.Name()) }()
	_, err = temp.Write(data)
	closeErr := temp.Close()
	if err != nil || closeErr != nil {
		return false
	}
	if runtime.GOOS == "windows" {
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			return false
		}
	}
	return os.Rename(temp.Name(), path) == nil
}

func notificationMessage(latest, current string) string {
	if !Newer(latest, current) {
		return ""
	}
	return fmt.Sprintf("DevEx %s is available (you have %s). Run 'devex self-update'.", strings.TrimPrefix(latest, "v"), strings.TrimPrefix(current, "v"))
}

// Notification only reads the cache at command completion. A detached helper
// refreshes it even when the foreground command exits before discovery finishes.
type Notification struct {
	current, dir string
}

func StartNotification(ctx context.Context, current, dir string) *Notification {
	return startNotification(ctx, current, dir, launchNotificationHelper)
}

func startNotification(ctx context.Context, current, dir string, launch func(context.Context, string, string) error) *Notification {
	if ctx.Err() != nil {
		return nil
	}
	if _, fresh := cachedNotification(current, dir, time.Now()); !fresh {
		// Starting the process is local; no network request delays the command.
		// The helper owns its timeout and lock, and never writes to the terminal.
		_ = launch(ctx, current, dir)
	}
	return &Notification{current: current, dir: dir}
}

func (n *Notification) Finish() string {
	if n == nil {
		return ""
	}
	message, _ := cachedNotification(n.current, n.dir, time.Now())
	return message
}

func cachedNotification(current, dir string, now time.Time) (string, bool) {
	var cached notificationCache
	data, err := os.ReadFile(filepath.Join(dir, "update-check.json"))
	if err != nil || json.Unmarshal(data, &cached) != nil || cached.CheckedAt.After(now) || now.Sub(cached.CheckedAt) >= 24*time.Hour {
		return "", false
	}
	return notificationMessage(cached.Latest, current), true
}
