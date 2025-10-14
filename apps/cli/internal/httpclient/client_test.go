package httpclient

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	client := New()

	if client == nil {
		t.Fatal("New() returned nil")
	}

	if client.Client == nil {
		t.Error("Client.Client is nil")
	}

	if client.Timeout != DefaultTimeout {
		t.Errorf("Client.Timeout = %v, want %v", client.Timeout, DefaultTimeout)
	}

	if client.userAgent == "" {
		t.Error("Client.userAgent is empty")
	}

	// User agent should contain DevEx
	if !strings.Contains(client.userAgent, "DevEx") {
		t.Errorf("User-Agent doesn't contain 'DevEx': %s", client.userAgent)
	}
}

func TestNewWithTimeout(t *testing.T) {
	customTimeout := 10 * time.Second
	client := NewWithTimeout(customTimeout)

	if client == nil {
		t.Fatal("NewWithTimeout() returned nil")
	}

	if client.Timeout != customTimeout {
		t.Errorf("Client.Timeout = %v, want %v", client.Timeout, customTimeout)
	}
}

func TestClient_Get(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify User-Agent header is set
		userAgent := r.Header.Get("User-Agent")
		if !strings.Contains(userAgent, "DevEx") {
			t.Errorf("User-Agent header doesn't contain 'DevEx': %s", userAgent)
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("test response"))
	}))
	defer server.Close()

	client := New()
	ctx := context.Background()

	resp, err := client.Get(ctx, server.URL)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			t.Errorf("Failed to close response body: %v", err)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Get() status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}

	if string(body) != "test response" {
		t.Errorf("Get() body = %s, want 'test response'", string(body))
	}
}

func TestClient_Get_Context_Canceled(t *testing.T) {
	// Create a test server that sleeps
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := New()
	ctx, cancel := context.WithCancel(context.Background())

	// Cancel immediately
	cancel()

	resp, err := client.Get(ctx, server.URL)
	if resp != nil {
		defer resp.Body.Close()
	}
	if err == nil {
		t.Error("Get() expected error for canceled context, got nil")
	}
}

func TestClient_Download(t *testing.T) {
	testData := "test file content"

	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify User-Agent header is set
		userAgent := r.Header.Get("User-Agent")
		if !strings.Contains(userAgent, "DevEx") {
			t.Errorf("User-Agent header doesn't contain 'DevEx': %s", userAgent)
		}

		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(testData))
	}))
	defer server.Close()

	client := New()
	ctx := context.Background()

	body, err := client.Download(ctx, server.URL)
	if err != nil {
		t.Fatalf("Download() error = %v", err)
	}
	defer func() {
		if err := body.Close(); err != nil {
			t.Errorf("Failed to close body: %v", err)
		}
	}()

	data, err := io.ReadAll(body)
	if err != nil {
		t.Fatalf("ReadAll() error = %v", err)
	}

	if string(data) != testData {
		t.Errorf("Download() body = %s, want %s", string(data), testData)
	}
}

func TestClient_Download_NotFound(t *testing.T) {
	// Create a test server that returns 404
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		_, _ = w.Write([]byte("Not Found"))
	}))
	defer server.Close()

	client := New()
	ctx := context.Background()

	body, err := client.Download(ctx, server.URL)
	if err == nil {
		if body != nil {
			_ = body.Close()
		}
		t.Fatal("Download() expected error for 404 response, got nil")
		return
	}

	if !strings.Contains(err.Error(), "404") {
		t.Errorf("Download() error message should contain '404', got: %v", err)
	}
}

func TestClient_Do(t *testing.T) {
	// Create a test server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify User-Agent header is set
		userAgent := r.Header.Get("User-Agent")
		if !strings.Contains(userAgent, "DevEx") {
			t.Errorf("User-Agent header doesn't contain 'DevEx': %s", userAgent)
		}

		// Verify custom header is preserved
		customHeader := r.Header.Get("X-Custom-Header")
		if customHeader != "test-value" {
			t.Errorf("Custom header not preserved: %s", customHeader)
		}

		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := New()
	req, err := http.NewRequest("GET", server.URL, nil)
	if err != nil {
		t.Fatalf("NewRequest() error = %v", err)
	}

	// Add custom header to verify it's preserved
	req.Header.Set("X-Custom-Header", "test-value")

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("Do() error = %v", err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			t.Errorf("Failed to close response body: %v", err)
		}
	}()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("Do() status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
}

func TestSetVersion(t *testing.T) {
	// Save original version
	originalVersion := Version
	defer func() {
		Version = originalVersion
	}()

	testVersion := "v1.2.3"
	SetVersion(testVersion)

	if Version != testVersion {
		t.Errorf("SetVersion() Version = %s, want %s", Version, testVersion)
	}

	// User agent should reflect new version
	userAgent := buildUserAgent()
	if !strings.Contains(userAgent, testVersion) {
		t.Errorf("buildUserAgent() doesn't contain version %s: %s", testVersion, userAgent)
	}
}

func TestGetUserAgent(t *testing.T) {
	// Save original version
	originalVersion := Version
	defer func() {
		Version = originalVersion
	}()

	testVersion := "v2.0.0"
	SetVersion(testVersion)

	userAgent := GetUserAgent()

	// Check format: DevEx/{version} ({os}; {arch}) Go/{go-version}
	if !strings.Contains(userAgent, "DevEx/") {
		t.Errorf("GetUserAgent() doesn't contain 'DevEx/': %s", userAgent)
	}

	if !strings.Contains(userAgent, testVersion) {
		t.Errorf("GetUserAgent() doesn't contain version %s: %s", testVersion, userAgent)
	}

	if !strings.Contains(userAgent, runtime.GOOS) {
		t.Errorf("GetUserAgent() doesn't contain OS %s: %s", runtime.GOOS, userAgent)
	}

	if !strings.Contains(userAgent, runtime.GOARCH) {
		t.Errorf("GetUserAgent() doesn't contain arch %s: %s", runtime.GOARCH, userAgent)
	}

	if !strings.Contains(userAgent, "Go/") {
		t.Errorf("GetUserAgent() doesn't contain 'Go/': %s", userAgent)
	}
}

func Test_buildUserAgent(t *testing.T) {
	// Save original version
	originalVersion := Version
	defer func() {
		Version = originalVersion
	}()

	tests := []struct {
		name    string
		version string
	}{
		{
			name:    "dev version",
			version: "dev",
		},
		{
			name:    "semantic version",
			version: "v1.0.0",
		},
		{
			name:    "beta version",
			version: "v2.1.0-beta.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SetVersion(tt.version)
			userAgent := buildUserAgent()

			// Expected format: DevEx/{version} ({os}; {arch}) Go/{go-version}
			expectedPrefix := fmt.Sprintf("DevEx/%s", tt.version)
			if !strings.HasPrefix(userAgent, expectedPrefix) {
				t.Errorf("buildUserAgent() = %s, want prefix %s", userAgent, expectedPrefix)
			}

			// Should contain OS and architecture
			expectedPlatform := fmt.Sprintf("(%s; %s)", runtime.GOOS, runtime.GOARCH)
			if !strings.Contains(userAgent, expectedPlatform) {
				t.Errorf("buildUserAgent() = %s, should contain %s", userAgent, expectedPlatform)
			}

			// Should contain Go version
			if !strings.Contains(userAgent, "Go/") {
				t.Errorf("buildUserAgent() = %s, should contain 'Go/'", userAgent)
			}
		})
	}
}

func TestClient_Get_InvalidURL(t *testing.T) {
	client := New()
	ctx := context.Background()

	resp, err := client.Get(ctx, "://invalid-url")
	if resp != nil {
		defer resp.Body.Close()
	}
	if err == nil {
		t.Error("Get() expected error for invalid URL, got nil")
	}
}

func TestDefaultTimeout(t *testing.T) {
	if DefaultTimeout <= 0 {
		t.Errorf("DefaultTimeout should be positive, got %v", DefaultTimeout)
	}

	// Verify it's a reasonable timeout value (between 1s and 5 minutes)
	if DefaultTimeout < time.Second || DefaultTimeout > 5*time.Minute {
		t.Errorf("DefaultTimeout = %v, should be between 1s and 5 minutes", DefaultTimeout)
	}
}
