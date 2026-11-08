// Package catalog reads the registry application API without coupling it to CLI commands.
package catalog

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/jameswlane/devex/apps/cli/internal/types"
)

// APIError distinguishes HTTP, transport, and invalid-response failures.
type APIError struct {
	Kind   string
	Status int
	Err    error
}

func (e *APIError) Error() string {
	return fmt.Sprintf("catalog %s (HTTP %d): %v", e.Kind, e.Status, e.Err)
}
func (e *APIError) Unwrap() error { return e.Err }

type Alternative struct {
	Method  string `json:"method"`
	Command string `json:"command"`
}
type Platform struct {
	InstallMethod   string        `json:"installMethod"`
	InstallCommand  string        `json:"installCommand"`
	OfficialSupport bool          `json:"officialSupport"`
	Alternatives    []Alternative `json:"alternatives"`
}
type Application struct {
	Name                string               `json:"name"`
	Description         string               `json:"description"`
	Category            string               `json:"category"`
	Default             bool                 `json:"default"`
	DesktopEnvironments []string             `json:"desktopEnvironments"`
	Platforms           map[string]*Platform `json:"platforms"`
}

// Map is the single conversion boundary for both list and detail responses.
func Map(a Application) (types.CrossPlatformApp, error) {
	if strings.TrimSpace(a.Name) == "" || a.Platforms == nil {
		return types.CrossPlatformApp{}, &APIError{Kind: "schema", Err: fmt.Errorf("missing name or platforms")}
	}
	convert := func(p *Platform) types.OSConfig {
		if p == nil {
			return types.OSConfig{}
		}
		c := types.OSConfig{InstallMethod: p.InstallMethod, InstallCommand: p.InstallCommand, OfficialSupport: p.OfficialSupport}
		for _, alt := range p.Alternatives {
			c.Alternatives = append(c.Alternatives, types.OSConfig{InstallMethod: alt.Method, InstallCommand: alt.Command})
		}
		return c
	}
	return types.CrossPlatformApp{Name: a.Name, Description: a.Description, Category: a.Category, Default: a.Default, DesktopEnvironments: a.DesktopEnvironments, Linux: convert(a.Platforms["linux"]), MacOS: convert(a.Platforms["macos"]), Windows: convert(a.Platforms["windows"])}, nil
}

type Filters struct{ Search, Category, Platform string }
type Client struct {
	URL  string
	HTTP *http.Client
}

func NewClient(baseURL string) *Client {
	return &Client{URL: strings.TrimRight(baseURL, "/"), HTTP: &http.Client{Timeout: 5 * time.Second}}
}
func (c *Client) request(ctx context.Context, path string, query url.Values, etag string, target any) (string, bool, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.URL+"/api/v1/applications"+path+"?"+query.Encode(), nil)
	if err != nil {
		return "", false, &APIError{Kind: "request", Err: err}
	}
	if etag != "" {
		req.Header.Set("If-None-Match", etag)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return "", false, &APIError{Kind: "transport", Err: err}
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode == http.StatusNotModified {
		return resp.Header.Get("ETag"), true, nil
	}
	if resp.StatusCode != http.StatusOK {
		return "", false, &APIError{Kind: "http", Status: resp.StatusCode, Err: fmt.Errorf("registry returned %s", resp.Status)}
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, 16<<20)).Decode(target); err != nil {
		return "", false, &APIError{Kind: "schema", Err: err}
	}
	return resp.Header.Get("ETag"), false, nil
}

// List traverses numbered pages; ETags apply only to a complete single-page result.
// Multi-page snapshots are refreshed in full to avoid treating a first-page ETag
// as validation of later pages.
func (c *Client) List(ctx context.Context, filters Filters, etag string) ([]types.CrossPlatformApp, string, bool, error) {
	ctx, cancel := context.WithTimeout(ctx, 15*time.Second)
	defer cancel()
	var apps []types.CrossPlatformApp
	var firstETag string
	for page := 1; page <= 1000; page++ {
		query := url.Values{"page": {strconv.Itoa(page)}, "limit": {"100"}}
		for key, value := range map[string]string{"search": filters.Search, "category": filters.Category, "platform": filters.Platform} {
			if value != "" {
				query.Set(key, value)
			}
		}
		var response struct {
			Items      *[]Application `json:"items"`
			Pagination *struct {
				Page    int  `json:"page"`
				HasNext bool `json:"hasNext"`
			} `json:"pagination"`
		}
		tag, unchanged, err := c.request(ctx, "", query, etag, &response)
		if err != nil {
			return nil, "", false, err
		}
		if unchanged {
			return nil, tag, true, nil
		}
		if response.Items == nil || response.Pagination == nil || response.Pagination.Page != page {
			return nil, "", false, &APIError{Kind: "schema", Err: fmt.Errorf("missing items or invalid pagination")}
		}
		for _, a := range *response.Items {
			app, err := Map(a)
			if err != nil {
				return nil, "", false, err
			}
			apps = append(apps, app)
		}
		if page == 1 {
			firstETag = tag
		}
		if !response.Pagination.HasNext {
			return apps, firstETag, false, nil
		}
		if len(*response.Items) == 0 {
			break
		}
		firstETag = ""
		etag = ""
	}
	return nil, "", false, &APIError{Kind: "schema", Err: fmt.Errorf("pagination limit exceeded or empty continuation page")}
}

func (c *Client) Get(ctx context.Context, name string) (types.CrossPlatformApp, error) {
	var a Application
	_, unchanged, err := c.request(ctx, "/"+url.PathEscape(name), nil, "", &a)
	if err != nil {
		return types.CrossPlatformApp{}, err
	}
	if unchanged {
		return types.CrossPlatformApp{}, &APIError{Kind: "schema", Err: fmt.Errorf("unexpected 304 for detail")}
	}
	return Map(a)
}
