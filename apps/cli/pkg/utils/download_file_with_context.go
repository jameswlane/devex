package utils

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
)

func DownloadFileWithContext(ctx context.Context, url string, filepath string) error {
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer func(Body io.ReadCloser) {
		err := Body.Close()
		if err != nil {
			fmt.Println("Failed to close response body", err)
		}
	}(resp.Body)

	out, err := os.Create(filepath)
	if err != nil {
		return err
	}
	defer func(out *os.File) {
		err := out.Close()
		if err != nil {
			fmt.Println("Failed to close output file", err)
		}
	}(out)

	_, err = io.Copy(out, resp.Body)
	return err
}
