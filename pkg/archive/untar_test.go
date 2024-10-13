package archive

import (
	"archive/tar"
	"compress/gzip"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestDownloadTarGz(t *testing.T) {
	// Create a mock server to serve the tar.gz file
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("This is a test tar.gz file"))
	}))
	defer server.Close()

	// Test DownloadTarGz
	err := DownloadTarGz(server.URL, "/tmp/test.tar.gz")
	if err != nil {
		t.Errorf("Expected no error, but got: %v", err)
	}

	// Check if the file exists
	if _, err := os.Stat("/tmp/test.tar.gz"); os.IsNotExist(err) {
		t.Errorf("Expected file to exist, but it does not")
	}
}

func TestUntar(t *testing.T) {
	// Create a temporary tar.gz file for testing
	tarFilePath := "/tmp/test.tar.gz"
	outFile, err := os.Create(tarFilePath)
	if err != nil {
		t.Fatal(err)
	}

	// Create a gzip writer
	gzipWriter := gzip.NewWriter(outFile)

	// Create a tar writer
	tarWriter := tar.NewWriter(gzipWriter)

	// Write a test file to the tar archive
	err = tarWriter.WriteHeader(&tar.Header{
		Name: "testfile.txt",
		Mode: 0600,
		Size: int64(len("Hello, World!")),
	})
	if err != nil {
		t.Fatal(err)
	}
	_, err = tarWriter.Write([]byte("Hello, World!"))
	if err != nil {
		t.Fatal(err)
	}

	// Close the tar and gzip writers to ensure everything is flushed
	if err := tarWriter.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gzipWriter.Close(); err != nil {
		t.Fatal(err)
	}
	if err := outFile.Close(); err != nil {
		t.Fatal(err)
	}

	// Test Untar
	err = Untar(tarFilePath, "/tmp")
	if err != nil {
		t.Errorf("Expected no error, but got: %v", err)
	}

	// Check if the test file was extracted
	extractedFilePath := filepath.Join("/tmp", "testfile.txt")
	content, err := os.ReadFile(extractedFilePath)
	if err != nil {
		t.Errorf("Expected no error, but got: %v", err)
	}

	expectedContent := "Hello, World!"
	if string(content) != expectedContent {
		t.Errorf("Expected file content to be '%s', but got: %s", expectedContent, string(content))
	}
}
