package httpclient

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"time"
)

var (
	// Version is the CLI version, set at runtime
	Version = "dev"

	// DefaultTimeout for HTTP requests
	DefaultTimeout = 30 * time.Second
)

// Client wraps http.Client with DevEx User-Agent
type Client struct {
	*http.Client
	userAgent string
}

// New creates a new HTTP client with DevEx User-Agent
func New() *Client {
	return &Client{
		Client: &http.Client{
			Timeout: DefaultTimeout,
		},
		userAgent: buildUserAgent(),
	}
}

// NewWithTimeout creates a new HTTP client with custom timeout
func NewWithTimeout(timeout time.Duration) *Client {
	return &Client{
		Client: &http.Client{
			Timeout: timeout,
		},
		userAgent: buildUserAgent(),
	}
}

// Do executes an HTTP request with DevEx User-Agent
func (c *Client) Do(req *http.Request) (*http.Response, error) {
	req.Header.Set("User-Agent", c.userAgent)
	return c.Client.Do(req)
}

// Get performs a GET request with DevEx User-Agent
func (c *Client) Get(ctx context.Context, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}
	return c.Do(req)
}

// Download downloads a file from URL with DevEx User-Agent
func (c *Client) Download(ctx context.Context, url string) (io.ReadCloser, error) {
	resp, err := c.Get(ctx, url)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		err := resp.Body.Close()
		if err != nil {
			return nil, err
		}
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, resp.Status)
	}

	return resp.Body, nil
}

// SetVersion updates the global version used in User-Agent
func SetVersion(version string) {
	Version = version
}

// buildUserAgent constructs the User-Agent string
// Format: DevEx/v0.0.1 (linux; amd64) Go/1.21.0
func buildUserAgent() string {
	return fmt.Sprintf("DevEx/%s (%s; %s) Go/%s",
		Version,
		runtime.GOOS,
		runtime.GOARCH,
		runtime.Version()[2:], // Remove "go" prefix
	)
}

// GetUserAgent returns the current User-Agent string
func GetUserAgent() string {
	return buildUserAgent()
}
