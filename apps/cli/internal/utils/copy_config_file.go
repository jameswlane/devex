package utils

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/jameswlane/devex/apps/cli/internal/fs"
	"github.com/jameswlane/devex/apps/cli/internal/log"
)

func CopyConfigFiles(srcDir, dstDir string, maxWorkers int) error {
	var wg sync.WaitGroup
	var firstErr error
	var mu sync.Mutex
	sem := make(chan struct{}, maxWorkers)

	if exists, err := fs.DirExists(srcDir); err != nil || !exists {
		return fmt.Errorf("source directory does not exist: %w", err)
	}
	if err := fs.EnsureDir(dstDir, 0o755); err != nil {
		return fmt.Errorf("failed to ensure destination directory: %w", err)
	}

	err := fs.Walk(srcDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return fmt.Errorf("error accessing path %s: %w", path, err)
		}
		if !info.IsDir() {
			wg.Add(1)
			go func(srcPath string) {
				defer wg.Done()
				sem <- struct{}{}
				defer func() { <-sem }()

				dstFile := filepath.Join(dstDir, info.Name())
				if exists, _ := fs.FileExistsAndIsFile(dstFile); exists {
					log.Warn("File already exists, skipping copy", "destination", dstFile)
					return
				}

				if copyErr := CopyFile(srcPath, dstFile); copyErr != nil {
					mu.Lock()
					if firstErr == nil {
						firstErr = fmt.Errorf("failed to copy file: %w", copyErr)
					}
					mu.Unlock()
				}
			}(path)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("error walking source directory: %w", err)
	}

	wg.Wait()
	return firstErr
}
