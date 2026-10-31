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
	if dir := os.Getenv("DEVEX_CACHE_DIR"); dir != "" {
		return dir, nil
	}
	if os.Getenv("LOCALAPPDATA") != "" && os.PathSeparator == '\\' {
		return filepath.Join(os.Getenv("LOCALAPPDATA"), "DevEx", "cache"), nil
	}
	if dir := os.Getenv("XDG_CACHE_HOME"); filepath.IsAbs(dir) {
		return filepath.Join(dir, "devex"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".cache", "devex"), nil
}

type notificationCache struct {
	CheckedAt time.Time `json:"checked_at"`
	Latest    string    `json:"latest,omitempty"`
}

// CheckNotification runs in the caller's background goroutine. Failed attempts
// are cached too, so offline/rate-limited invocations do not retry for 24 hours.
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
	// Reserve this day's attempt before networking. Exclusive creation avoids
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
	cached = notificationCache{CheckedAt: now}
	if !writeCache(path, cached) {
		return ""
	}
	release, err := client.Find(ctx, "", false)
	if err != nil {
		return ""
	}
	cached.Latest = release.Tag
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

// Notification never waits for the network at command completion. Very short
// commands may exit before discovery completes; subsequent runs use the cache.
type Notification struct {
	result <-chan string
	cancel context.CancelFunc
}

func StartNotification(ctx context.Context, check func(context.Context) string) *Notification {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	result := make(chan string, 1)
	go func() { result <- check(ctx) }()
	return &Notification{result: result, cancel: cancel}
}

func (n *Notification) Finish() string {
	if n == nil {
		return ""
	}
	defer n.cancel()
	select {
	case message := <-n.result:
		return message
	default:
		return ""
	}
}
