package sdk

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// RegistryClient provides secure access to the plugin registry API
type RegistryClient struct {
	baseURL    string
	apiKey     string
	secretKey  string
	client     *http.Client
	userAgent  string
}

// RegistryConfig configures the registry client
type RegistryConfig struct {
	BaseURL   string
	APIKey    string
	SecretKey string
	Timeout   time.Duration
	UserAgent string
}

// NewRegistryClient creates a new authenticated registry client
func NewRegistryClient(config RegistryConfig) *RegistryClient {
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}
	if config.UserAgent == "" {
		config.UserAgent = "devex-cli/1.0"
	}

	return &RegistryClient{
		baseURL:   strings.TrimSuffix(config.BaseURL, "/"),
		apiKey:    config.APIKey,
		secretKey: config.SecretKey,
		userAgent: config.UserAgent,
		client: &http.Client{
			Timeout: config.Timeout,
		},
	}
}

// APIResponse represents a standard API response
type APIResponse struct {
	Success bool            `json:"success"`
	Data    json.RawMessage `json:"data,omitempty"`
	Error   string          `json:"error,omitempty"`
	Code    int             `json:"code,omitempty"`
}

// PluginUploadRequest represents a plugin upload request
type PluginUploadRequest struct {
	Name        string            `json:"name"`
	Version     string            `json:"version"`
	Description string            `json:"description"`
	Author      string            `json:"author,omitempty"`
	Tags        []string          `json:"tags,omitempty"`
	Platforms   map[string]string `json:"platforms"` // platform -> download URL
}

// PluginInfo represents detailed plugin information
type PluginAPIInfo struct {
	PluginMetadata
	Downloads    int64     `json:"downloads"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	Verified     bool      `json:"verified"`
	PublisherID  string    `json:"publisher_id"`
}

// GetRegistry fetches the complete plugin registry with authentication
func (c *RegistryClient) GetRegistry(ctx context.Context) (*PluginRegistry, error) {
	resp, err := c.authenticatedRequest(ctx, "GET", "/v1/registry", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch registry: %w", err)
	}

	var apiResp APIResponse
	if err := json.Unmarshal(resp, &apiResp); err != nil {
		return nil, fmt.Errorf("failed to parse API response: %w", err)
	}

	if !apiResp.Success {
		return nil, fmt.Errorf("API error: %s", apiResp.Error)
	}

	var registry PluginRegistry
	if err := json.Unmarshal(apiResp.Data, &registry); err != nil {
		return nil, fmt.Errorf("failed to parse registry data: %w", err)
	}

	return &registry, nil
}

// GetPlugin fetches detailed information about a specific plugin
func (c *RegistryClient) GetPlugin(ctx context.Context, pluginName string) (*PluginAPIInfo, error) {
	path := fmt.Sprintf("/v1/plugins/%s", url.PathEscape(pluginName))
	resp, err := c.authenticatedRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch plugin info: %w", err)
	}

	var apiResp APIResponse
	if err := json.Unmarshal(resp, &apiResp); err != nil {
		return nil, fmt.Errorf("failed to parse API response: %w", err)
	}

	if !apiResp.Success {
		return nil, fmt.Errorf("API error: %s", apiResp.Error)
	}

	var pluginInfo PluginAPIInfo
	if err := json.Unmarshal(apiResp.Data, &pluginInfo); err != nil {
		return nil, fmt.Errorf("failed to parse plugin data: %w", err)
	}

	return &pluginInfo, nil
}

// SearchPlugins searches for plugins with authentication
func (c *RegistryClient) SearchPlugins(ctx context.Context, query string, tags []string, limit int) ([]PluginAPIInfo, error) {
	params := url.Values{}
	if query != "" {
		params.Set("q", query)
	}
	if len(tags) > 0 {
		params.Set("tags", strings.Join(tags, ","))
	}
	if limit > 0 {
		params.Set("limit", strconv.Itoa(limit))
	}

	path := "/v1/plugins/search"
	if len(params) > 0 {
		path += "?" + params.Encode()
	}

	resp, err := c.authenticatedRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to search plugins: %w", err)
	}

	var apiResp APIResponse
	if err := json.Unmarshal(resp, &apiResp); err != nil {
		return nil, fmt.Errorf("failed to parse API response: %w", err)
	}

	if !apiResp.Success {
		return nil, fmt.Errorf("API error: %s", apiResp.Error)
	}

	var plugins []PluginAPIInfo
	if err := json.Unmarshal(apiResp.Data, &plugins); err != nil {
		return nil, fmt.Errorf("failed to parse search results: %w", err)
	}

	return plugins, nil
}

// UploadPlugin uploads a new plugin or version to the registry
func (c *RegistryClient) UploadPlugin(ctx context.Context, uploadReq PluginUploadRequest) error {
	reqBody, err := json.Marshal(uploadReq)
	if err != nil {
		return fmt.Errorf("failed to marshal upload request: %w", err)
	}

	resp, err := c.authenticatedRequest(ctx, "POST", "/v1/plugins", reqBody)
	if err != nil {
		return fmt.Errorf("failed to upload plugin: %w", err)
	}

	var apiResp APIResponse
	if err := json.Unmarshal(resp, &apiResp); err != nil {
		return fmt.Errorf("failed to parse API response: %w", err)
	}

	if !apiResp.Success {
		return fmt.Errorf("upload failed: %s", apiResp.Error)
	}

	return nil
}

// DeletePlugin removes a plugin from the registry (admin only)
func (c *RegistryClient) DeletePlugin(ctx context.Context, pluginName string) error {
	path := fmt.Sprintf("/v1/plugins/%s", url.PathEscape(pluginName))
	resp, err := c.authenticatedRequest(ctx, "DELETE", path, nil)
	if err != nil {
		return fmt.Errorf("failed to delete plugin: %w", err)
	}

	var apiResp APIResponse
	if err := json.Unmarshal(resp, &apiResp); err != nil {
		return fmt.Errorf("failed to parse API response: %w", err)
	}

	if !apiResp.Success {
		return fmt.Errorf("delete failed: %s", apiResp.Error)
	}

	return nil
}

// GetStats returns registry statistics
func (c *RegistryClient) GetStats(ctx context.Context) (map[string]interface{}, error) {
	resp, err := c.authenticatedRequest(ctx, "GET", "/v1/stats", nil)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch stats: %w", err)
	}

	var apiResp APIResponse
	if err := json.Unmarshal(resp, &apiResp); err != nil {
		return nil, fmt.Errorf("failed to parse API response: %w", err)
	}

	if !apiResp.Success {
		return nil, fmt.Errorf("API error: %s", apiResp.Error)
	}

	var stats map[string]interface{}
	if err := json.Unmarshal(apiResp.Data, &stats); err != nil {
		return nil, fmt.Errorf("failed to parse stats data: %w", err)
	}

	return stats, nil
}

// authenticatedRequest makes an authenticated request to the registry API
func (c *RegistryClient) authenticatedRequest(ctx context.Context, method, path string, body []byte) ([]byte, error) {
	fullURL := c.baseURL + path
	
	var bodyReader io.Reader
	if body != nil {
		bodyReader = bytes.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, fullURL, bodyReader)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Add headers
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	// Add authentication headers if credentials are provided
	if c.apiKey != "" && c.secretKey != "" {
		c.addAuthHeaders(req, body)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(respBody))
	}

	return respBody, nil
}

// addAuthHeaders adds HMAC-based authentication headers
func (c *RegistryClient) addAuthHeaders(req *http.Request, body []byte) {
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)
	
	// Create signature payload: METHOD + PATH + TIMESTAMP + BODY
	payload := req.Method + req.URL.Path
	if req.URL.RawQuery != "" {
		payload += "?" + req.URL.RawQuery
	}
	payload += timestamp
	if body != nil {
		payload += string(body)
	}

	// Create HMAC signature
	mac := hmac.New(sha256.New, []byte(c.secretKey))
	mac.Write([]byte(payload))
	signature := hex.EncodeToString(mac.Sum(nil))

	// Add authentication headers
	req.Header.Set("X-API-Key", c.apiKey)
	req.Header.Set("X-Timestamp", timestamp)
	req.Header.Set("X-Signature", signature)
}

// ValidateSignature validates an HMAC signature (for server-side use)
func ValidateSignature(apiKey, secretKey, method, path, query, timestamp, bodyStr, receivedSignature string) bool {
	// Recreate the payload
	payload := method + path
	if query != "" {
		payload += "?" + query
	}
	payload += timestamp + bodyStr

	// Create expected signature
	mac := hmac.New(sha256.New, []byte(secretKey))
	mac.Write([]byte(payload))
	expectedSignature := hex.EncodeToString(mac.Sum(nil))

	// Compare signatures using constant-time comparison
	return hmac.Equal([]byte(receivedSignature), []byte(expectedSignature))
}

// RateLimitInfo represents rate limit information from API responses
type RateLimitInfo struct {
	Limit     int   `json:"limit"`
	Remaining int   `json:"remaining"`
	ResetAt   int64 `json:"reset_at"`
}

// IsRateLimited checks if we're currently rate limited
func (r *RateLimitInfo) IsRateLimited() bool {
	return r.Remaining <= 0 && time.Now().Unix() < r.ResetAt
}

// TimeUntilReset returns the duration until rate limit resets
func (r *RateLimitInfo) TimeUntilReset() time.Duration {
	resetTime := time.Unix(r.ResetAt, 0)
	return time.Until(resetTime)
}

// Enhanced Downloader with secure registry client
type SecureDownloader struct {
	*Downloader
	registryClient *RegistryClient
}

// NewSecureDownloader creates a downloader with authenticated registry access
func NewSecureDownloaderWithAuth(config DownloaderConfig, registryConfig RegistryConfig) *SecureDownloader {
	downloader := NewSecureDownloader(config)
	registryClient := NewRegistryClient(registryConfig)
	
	return &SecureDownloader{
		Downloader:     downloader,
		registryClient: registryClient,
	}
}

// GetAvailablePlugins fetches plugins using authenticated API
func (sd *SecureDownloader) GetAvailablePlugins() (map[string]PluginMetadata, error) {
	ctx := context.Background()
	registry, err := sd.registryClient.GetRegistry(ctx)
	if err != nil {
		// Fallback to unauthenticated method
		return sd.Downloader.GetAvailablePlugins()
	}
	return registry.Plugins, nil
}

// SearchPlugins searches using authenticated API with enhanced features
func (sd *SecureDownloader) SearchPlugins(query string) (map[string]PluginMetadata, error) {
	ctx := context.Background()
	plugins, err := sd.registryClient.SearchPlugins(ctx, query, nil, 100)
	if err != nil {
		// Fallback to unauthenticated method
		return sd.Downloader.SearchPlugins(query)
	}

	// Convert to PluginMetadata map
	results := make(map[string]PluginMetadata)
	for _, plugin := range plugins {
		results[plugin.Name] = plugin.PluginMetadata
	}
	return results, nil
}

// GetPluginDetails returns detailed plugin information from authenticated API
func (sd *SecureDownloader) GetPluginDetails(pluginName string) (*PluginAPIInfo, error) {
	ctx := context.Background()
	return sd.registryClient.GetPlugin(ctx, pluginName)
}
