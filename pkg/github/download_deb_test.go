package github

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func TestGetLatestDebURL(t *testing.T) {
	t.Parallel() // Add this line to run the test in parallel

	// Create a mock server for GitHub API
	mockResponse := `{
        "tag_name": "v1.0.0",
        "assets": [
            {"name": "example.deb", "browser_download_url": "https://example.com/example.deb"}
        ]
    }`

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte(mockResponse)); err != nil {
			t.Errorf("Failed to write mock response: %v", err)
		}
	}))
	defer server.Close()

	// Pass the mock server URL as the base URL
	client := server.Client()
	url, err := GetLatestDebURL("owner", "repo", server.URL, client)
	if err != nil {
		t.Errorf("Expected no error, but got: %v", err)
	}

	expectedURL := "https://example.com/example.deb"
	if url != expectedURL {
		t.Errorf("Expected URL to be %s, but got: %s", expectedURL, url)
	}
}

func TestDownloadDeb(t *testing.T) {
	t.Parallel() // Add this line to run the test in parallel

	// Create a mock server to serve the .deb file
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("This is a test .deb file")); err != nil {
			t.Errorf("Failed to write mock .deb file: %v", err)
		}
	}))
	defer server.Close()

	// Test DownloadDeb
	err := DownloadDeb(server.URL, "/tmp/test.deb")
	if err != nil {
		t.Errorf("Expected no error, but got: %v", err)
	}

	// Check if the file exists and has the correct content
	content, err := os.ReadFile("/tmp/test.deb")
	if err != nil {
		t.Errorf("Expected no error, but got: %v", err)
	}

	expectedContent := "This is a test .deb file"
	if string(content) != expectedContent {
		t.Errorf("Expected file content to be '%s', but got: %s", expectedContent, string(content))
	}
}
