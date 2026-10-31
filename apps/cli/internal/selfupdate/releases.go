// Package selfupdate discovers and installs signed DevEx CLI releases.
package selfupdate

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/Masterminds/semver/v3"
)

const repository = "/repos/jameswlane/devex"

var cliTag = regexp.MustCompile(`^v[0-9]+\.[0-9]+\.[0-9]+(?:-[0-9A-Za-z.-]+)?$`)

type Asset struct {
	Name string `json:"name"`
	URL  string `json:"browser_download_url"`
}
type Release struct {
	Tag        string  `json:"tag_name"`
	URL        string  `json:"html_url"`
	Prerelease bool    `json:"prerelease"`
	Draft      bool    `json:"draft"`
	Assets     []Asset `json:"assets"`
}

type HTTPClient interface {
	Do(*http.Request) (*http.Response, error)
}

type Client struct {
	HTTP    HTTPClient
	BaseURL string
	Token   string
}

func NewClient() *Client {
	return &Client{HTTP: &http.Client{Timeout: 2 * time.Minute}, BaseURL: "https://api.github.com"}
}

func ParseVersion(version string) (*semver.Version, error) {
	return semver.StrictNewVersion(strings.TrimPrefix(version, "v"))
}

func Newer(latest, current string) bool {
	a, err := ParseVersion(latest)
	if err != nil {
		return false
	}
	b, err := ParseVersion(current)
	return err == nil && a.GreaterThan(b)
}

func (c *Client) get(ctx context.Context, url string, api bool, limit int64) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("create release request: %w", err)
	}
	if api {
		req.Header.Set("Accept", "application/vnd.github+json")
		if c.Token != "" {
			req.Header.Set("Authorization", "Bearer "+c.Token)
		}
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("cannot reach release server; check your connection and retry: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusForbidden || resp.StatusCode == http.StatusTooManyRequests {
		return nil, fmt.Errorf("release server rate limited or denied the request (HTTP %d); set GITHUB_TOKEN or retry later", resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("release download returned HTTP %d; check the version and retry", resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, fmt.Errorf("read release response: %w", err)
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("release response exceeds %d bytes", limit)
	}
	return data, nil
}

func eligible(r Release, pre bool) bool {
	if r.Draft || !cliTag.MatchString(r.Tag) {
		return false
	}
	v, err := ParseVersion(r.Tag)
	return err == nil && (pre || (!r.Prerelease && v.Prerelease() == ""))
}

func (c *Client) Find(ctx context.Context, version string, pre bool) (Release, error) {
	base := strings.TrimRight(c.BaseURL, "/") + repository
	if version != "" {
		if !cliTag.MatchString(version) {
			return Release{}, fmt.Errorf("invalid version %q; use vX.Y.Z (or a prerelease with --pre)", version)
		}
		data, err := c.get(ctx, base+"/releases/tags/"+version, true, 8<<20)
		if err != nil {
			return Release{}, err
		}
		var release Release
		if err := json.Unmarshal(data, &release); err != nil {
			return Release{}, fmt.Errorf("decode release: %w", err)
		}
		if release.Tag != version || !eligible(release, pre) {
			return Release{}, fmt.Errorf("release %s is unavailable or a prerelease; prereleases require --pre", version)
		}
		return release, nil
	}
	var latest Release
	for page := 1; ; page++ {
		data, err := c.get(ctx, fmt.Sprintf("%s/releases?per_page=100&page=%d", base, page), true, 8<<20)
		if err != nil {
			return Release{}, err
		}
		var releases []Release
		if err := json.Unmarshal(data, &releases); err != nil {
			return Release{}, fmt.Errorf("decode releases: %w", err)
		}
		for _, release := range releases {
			if eligible(release, pre) && (latest.Tag == "" || Newer(release.Tag, latest.Tag)) {
				latest = release
			}
		}
		if len(releases) < 100 {
			break
		}
	}
	if latest.Tag == "" {
		return Release{}, fmt.Errorf("no compatible DevEx CLI releases found; retry later or specify --version vX.Y.Z")
	}
	return latest, nil
}

func (c *Client) asset(ctx context.Context, release Release, name string, limit int64) ([]byte, error) {
	for _, asset := range release.Assets {
		if asset.Name == name {
			return c.get(ctx, asset.URL, false, limit)
		}
	}
	return nil, fmt.Errorf("release %s is missing %s; choose another release or reinstall DevEx", release.Tag, name)
}
