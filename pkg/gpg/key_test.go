package gpg

import (
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"testing"

	"github.com/jameswlane/devex/pkg/testutils"
)

func TestDownloadGPGKey(t *testing.T) {
	// Create a mock server to serve the GPG key
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		if _, err := w.Write([]byte("This is a test GPG key")); err != nil {
			t.Errorf("Failed to write response: %v", err)
		}
	}))
	defer server.Close()

	// Test DownloadGPGKey
	err := DownloadGPGKey(server.URL, "/tmp/test.gpg")
	if err != nil {
		t.Errorf("Expected no error, but got: %v", err)
	}

	// Check if the file exists and has the correct content
	content, err := os.ReadFile("/tmp/test.gpg")
	if err != nil {
		t.Errorf("Expected no error, but got: %v", err)
	}

	expectedContent := "This is a test GPG key"
	if string(content) != expectedContent {
		t.Errorf("Expected file content to be '%s', but got: %s", expectedContent, string(content))
	}
}

func TestAddGPGKeyToKeyring(t *testing.T) {
	// Mock the exec.Command to simulate successful key addition
	execCommand = testutils.MockExecCommand
	defer func() { execCommand = exec.Command }() // Reset after test

	// Test AddGPGKeyToKeyring
	err := AddGPGKeyToKeyring("/tmp/test.gpg")
	if err != nil {
		t.Errorf("Expected no error, but got: %v", err)
	}
}
