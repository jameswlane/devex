package config

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

// BenchmarkParallelLoading tests parallel vs sequential loading performance
func BenchmarkParallelLoading(b *testing.B) {
	// Create temporary directory structure
	tempDir, err := os.MkdirTemp("", "devex-benchmark")
	if err != nil {
		b.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	// Test with different file counts
	fileCounts := []int{5, 10, 20, 50, 100}

	for _, count := range fileCounts {
		b.Run(fmt.Sprintf("Files_%d", count), func(b *testing.B) {
			setupBenchmarkFiles(b, tempDir, count)

			b.Run("Sequential", func(b *testing.B) {
				for i := 0; i < b.N; i++ {
					files, err := getDirectoryFiles(tempDir)
					if err != nil {
						b.Fatal(err)
					}
					err = loadFilesSequentially(files, tempDir, "benchmark")
					if err != nil {
						b.Fatal(err)
					}
				}
			})

			b.Run("Parallel", func(b *testing.B) {
				for i := 0; i < b.N; i++ {
					files, err := getDirectoryFiles(tempDir)
					if err != nil {
						b.Fatal(err)
					}
					err = loadFilesInParallel(files, tempDir, "benchmark")
					if err != nil {
						b.Fatal(err)
					}
				}
			})
		})
	}
}

// BenchmarkCachePerformance tests cache hit vs miss performance
func BenchmarkCachePerformance(b *testing.B) {
	tempDir, err := os.MkdirTemp("", "devex-cache-benchmark")
	if err != nil {
		b.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	// Create test files
	testFile := filepath.Join(tempDir, "test.yaml")
	content := []byte("test: value\nmore: data\nconfig: settings")
	os.WriteFile(testFile, content, 0644)

	// Clear cache before benchmarks
	globalConfigCache.clearCache()

	b.Run("Cache_Miss", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			// Clear cache to force miss
			globalConfigCache.clearCache()
			shouldReload, err := globalConfigCache.shouldReloadFile(testFile)
			if err != nil {
				b.Fatal(err)
			}
			if !shouldReload {
				b.Fatal("Expected cache miss")
			}
		}
	})

	b.Run("Cache_Hit", func(b *testing.B) {
		// Prime the cache
		globalConfigCache.shouldReloadFile(testFile)

		for i := 0; i < b.N; i++ {
			shouldReload, err := globalConfigCache.shouldReloadFile(testFile)
			if err != nil {
				b.Fatal(err)
			}
			if shouldReload {
				b.Fatal("Expected cache hit")
			}
		}
	})
}

// BenchmarkRecursiveTraversal tests recursive vs flat directory performance
func BenchmarkRecursiveTraversal(b *testing.B) {
	tempDir, err := os.MkdirTemp("", "devex-recursive-benchmark")
	if err != nil {
		b.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	// Create nested directory structure
	setupNestedBenchmarkFiles(b, tempDir, 5, 3) // 5 files per level, 3 levels deep

	b.Run("Flat_Directory", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, err := getDirectoryFiles(tempDir)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("Recursive_Depth_1", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, err := getDirectoryFilesRecursive(tempDir, 1)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("Recursive_Depth_3", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, err := getDirectoryFilesRecursive(tempDir, 3)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

// BenchmarkFileValidation tests filename validation performance
func BenchmarkFileValidation(b *testing.B) {
	testCases := []struct {
		name     string
		filename string
	}{
		{"Valid_Simple", "config.yaml"},
		{"Valid_Complex", "00-priority-application-config-v2.0.yaml"},
		{"Invalid_Traversal", "../../../etc/passwd.yaml"},
		{"Invalid_Dangerous", "config;rm -rf /.yaml"},
	}

	for _, tc := range testCases {
		b.Run(tc.name, func(b *testing.B) {
			for i := 0; i < b.N; i++ {
				isValidFilename(tc.filename)
			}
		})
	}
}

// BenchmarkMemoryUsage tests memory allocation patterns
func BenchmarkMemoryUsage(b *testing.B) {
	tempDir, err := os.MkdirTemp("", "devex-memory-benchmark")
	if err != nil {
		b.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	// Create files with different sizes
	sizes := []int{1024, 10240, 102400} // 1KB, 10KB, 100KB

	for _, size := range sizes {
		filename := fmt.Sprintf("large_%dkb.yaml", size/1024)
		content := make([]byte, size)
		for i := range content {
			if i%50 == 0 {
				content[i] = '\n'
			} else {
				content[i] = 'a'
			}
		}
		os.WriteFile(filepath.Join(tempDir, filename), content, 0644)
	}

	b.Run("Memory_Allocation", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			files, err := getDirectoryFiles(tempDir)
			if err != nil {
				b.Fatal(err)
			}
			err = loadFilesSequentially(files, tempDir, "memory_test")
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

// Helper function to create benchmark test files
func setupBenchmarkFiles(b *testing.B, tempDir string, count int) {
	b.Helper()

	// Clean up existing files
	files, _ := filepath.Glob(filepath.Join(tempDir, "*.yaml"))
	for _, file := range files {
		os.Remove(file)
	}

	// Create new test files
	for i := 0; i < count; i++ {
		filename := fmt.Sprintf("%02d-benchmark-config.yaml", i)
		content := fmt.Sprintf(`name: "Benchmark App %d"
description: "Test application for benchmarking"
category: "Benchmarks"
default: false
linux:
  install_method: apt
  install_command: benchmark-app-%d
  uninstall_command: benchmark-app-%d
  official_support: true
macos:
  install_method: brew
  install_command: benchmark-app-%d
  uninstall_command: benchmark-app-%d
  official_support: true
windows:
  install_method: winget
  install_command: BenchmarkApp%d
  uninstall_command: BenchmarkApp%d
  official_support: true`, i, i, i, i, i, i, i)

		err := os.WriteFile(filepath.Join(tempDir, filename), []byte(content), 0644)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// Helper function to create nested benchmark files
func setupNestedBenchmarkFiles(b *testing.B, tempDir string, filesPerLevel, levels int) {
	b.Helper()

	for level := 0; level < levels; level++ {
		var currentDir string
		if level == 0 {
			currentDir = tempDir
		} else {
			currentDir = filepath.Join(tempDir, fmt.Sprintf("level%d", level))
			os.MkdirAll(currentDir, 0755)
		}

		for i := 0; i < filesPerLevel; i++ {
			filename := fmt.Sprintf("level%d-file%d.yaml", level, i)
			content := fmt.Sprintf("level: %d\nfile: %d\ndata: benchmark", level, i)
			err := os.WriteFile(filepath.Join(currentDir, filename), []byte(content), 0644)
			if err != nil {
				b.Fatal(err)
			}
		}
	}
}
