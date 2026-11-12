// Package catalog reads the registry application API without coupling it to CLI commands.
package catalog

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"

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

// Application is one registry catalog entry. Platforms hold the same fields
// as the YAML catalog's per-platform definitions.
type Application struct {
	Name                string                     `json:"name"`
	Description         string                     `json:"description"`
	Category            string                     `json:"category"`
	Default             bool                       `json:"default"`
	DesktopEnvironments []string                   `json:"desktopEnvironments"`
	Platforms           map[string]json.RawMessage `json:"platforms"`
}

// legacyKeys are the method/command keys older registry seeds wrote for an
// install definition (a platform or one of its alternatives), mapped to the
// YAML catalog keys. They apply only to OSConfig objects: a pre/post-install
// hook's "command" is its own field.
var legacyKeys = map[string]string{"method": "install_method", "command": "install_command"}

var osConfigType = reflect.TypeOf(types.OSConfig{})

// normalizeKeys rewrites the camelCase object keys older registry seeds wrote
// to the YAML catalog's snake_case keys, following the shape of target. Only
// struct field names are rewritten: map keys are data (shell_init names
// shells such as PowerShell) and are kept as served.
func normalizeKeys(value any, target reflect.Type) any {
	for target.Kind() == reflect.Pointer {
		target = target.Elem()
	}
	switch target.Kind() {
	case reflect.Struct:
		object, ok := value.(map[string]any)
		if !ok {
			return value
		}
		fields := yamlFields(target)
		normalized := make(map[string]any, len(object))
		for key, item := range object {
			if _, known := fields[key]; !known {
				if mapped, ok := legacyKeys[key]; ok && target == osConfigType {
					key = mapped
				} else {
					key = snakeCase(key)
				}
			}
			if field, known := fields[key]; known {
				item = normalizeKeys(item, field)
			}
			normalized[key] = item
		}
		return normalized
	case reflect.Map:
		object, ok := value.(map[string]any)
		if !ok {
			return value
		}
		for key, item := range object {
			object[key] = normalizeKeys(item, target.Elem())
		}
		return object
	case reflect.Slice, reflect.Array:
		list, ok := value.([]any)
		if !ok {
			return value
		}
		for i, item := range list {
			list[i] = normalizeKeys(item, target.Elem())
		}
		return list
	default:
		return value
	}
}

// yamlFields maps a struct's YAML keys to their field types.
func yamlFields(target reflect.Type) map[string]reflect.Type {
	fields := make(map[string]reflect.Type, target.NumField())
	for i := 0; i < target.NumField(); i++ {
		field := target.Field(i)
		if !field.IsExported() {
			continue
		}
		name, _, _ := strings.Cut(field.Tag.Get("yaml"), ",")
		if name == "-" {
			continue
		}
		if name == "" {
			name = strings.ToLower(field.Name)
		}
		fields[name] = field.Type
	}
	return fields
}

func snakeCase(key string) string {
	var builder strings.Builder
	for i, r := range key {
		if r >= 'A' && r <= 'Z' {
			if i > 0 {
				builder.WriteByte('_')
			}
			builder.WriteRune(r + ('a' - 'A'))
			continue
		}
		builder.WriteRune(r)
	}
	return builder.String()
}

// decodePlatform decodes one platform definition into the CLI type through its
// YAML field names, so every field the registry serves is kept.
func decodePlatform(raw json.RawMessage) (types.OSConfig, error) {
	var config types.OSConfig
	if len(raw) == 0 || string(raw) == "null" {
		return config, nil
	}
	var generic any
	if err := json.Unmarshal(raw, &generic); err != nil {
		return config, err
	}
	if generic == nil {
		return config, nil
	}
	if _, ok := generic.(map[string]any); !ok {
		return config, fmt.Errorf("platform must be an object")
	}
	data, err := yaml.Marshal(normalizeKeys(generic, osConfigType))
	if err != nil {
		return config, err
	}
	if err := yaml.Unmarshal(data, &config); err != nil {
		return config, err
	}
	return config, nil
}

// Map is the single conversion boundary for both list and detail responses.
func Map(a Application) (types.CrossPlatformApp, error) {
	if strings.TrimSpace(a.Name) == "" || a.Platforms == nil {
		return types.CrossPlatformApp{}, &APIError{Kind: "schema", Err: fmt.Errorf("missing name or platforms")}
	}
	app := types.CrossPlatformApp{Name: a.Name, Description: a.Description, Category: a.Category, Default: a.Default, DesktopEnvironments: a.DesktopEnvironments}
	// The registry serves universal definitions (all_platforms in the YAML
	// catalog) under "all".
	for name, target := range map[string]*types.OSConfig{"all": &app.AllPlatforms, "linux": &app.Linux, "macos": &app.MacOS, "windows": &app.Windows} {
		config, err := decodePlatform(a.Platforms[name])
		if err != nil {
			return types.CrossPlatformApp{}, &APIError{Kind: "schema", Err: fmt.Errorf("application %s platform %s: %w", a.Name, name, err)}
		}
		*target = config
	}
	return app, nil
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
