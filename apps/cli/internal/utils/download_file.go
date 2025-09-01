package utils

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"github.com/spf13/afero"

	"github.com/jameswlane/devex/apps/cli/internal/fs"
)

func DownloadFile(url, destination string) error {
	outFile, err := fs.AppFs.Create(destination)
	if err != nil {
		return fmt.Errorf("failed to create destination file: %w", err)
	}
	defer func(outFile afero.File) {
		err := outFile.Close()
		if err != nil {
			fmt.Println("Failed to close destination file", err)
		}
	}(outFile)

	ctx := context.Background()
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to download file: %w", err)
	}
	defer resp.Body.Close()

	if _, err := io.Copy(outFile, resp.Body); err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}
