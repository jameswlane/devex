package fs_test

import (
	"os"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/afero"

	"github.com/jameswlane/devex/apps/cli/internal/fs"
	"github.com/jameswlane/devex/apps/cli/internal/log"
)

var _ = Describe("fs package", func() {
	BeforeEach(func() {
		fs.SetFs(afero.NewMemMapFs()) // Use in-memory filesystem for testing
	})

	Describe("EnsureDir", func() {
		It("creates a directory if it does not exist", func() {
			dirPath := "/tmp/test-dir"

			err := fs.EnsureDir(dirPath, 0o755)
			Expect(err).ToNot(HaveOccurred())

			exists, err := fs.DirExists(dirPath)
			Expect(err).ToNot(HaveOccurred())
			Expect(exists).To(BeTrue())
		})

		It("does nothing if the directory already exists", func() {
			dirPath := "/tmp/existing-dir"

			err := fs.MkdirAll(dirPath, 0o755)
			Expect(err).ToNot(HaveOccurred())

			err = fs.EnsureDir(dirPath, 0o755)
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Describe("TempFile", func() {
		It("creates a temporary file", func() {
			tmpFile, err := fs.TempFile("", "test-")
			Expect(err).ToNot(HaveOccurred())
			Expect(tmpFile).ToNot(BeNil())

			defer func(AppFs afero.Fs, name string) {
				err := AppFs.Remove(name)
				if err != nil {
					log.Fatal("", err)
				}
			}(fs.AppFs, tmpFile.Name()) // Cleanup

			exists, err := fs.Exists(tmpFile.Name())
			Expect(err).ToNot(HaveOccurred())
			Expect(exists).To(BeTrue())
		})
	})

	Describe("FileExistsAndIsFile", func() {
		It("returns true for an existing file", func() {
			filePath := "/tmp/test-file.txt"
			err := fs.WriteFile(filePath, []byte("test content"), 0o644)
			Expect(err).ToNot(HaveOccurred())

			isFile, err := fs.FileExistsAndIsFile(filePath)
			Expect(err).ToNot(HaveOccurred())
			Expect(isFile).To(BeTrue())
		})

		It("returns false for a directory", func() {
			dirPath := "/tmp/test-dir"
			err := fs.MkdirAll(dirPath, 0o755)
			Expect(err).ToNot(HaveOccurred())

			isFile, err := fs.FileExistsAndIsFile(dirPath)
			Expect(err).ToNot(HaveOccurred())
			Expect(isFile).To(BeFalse())
		})

		It("returns false for a non-existent path", func() {
			isFile, err := fs.FileExistsAndIsFile("/tmp/non-existent")
			Expect(err).ToNot(HaveOccurred())
			Expect(isFile).To(BeFalse())
		})
	})

	Describe("ReadFileIfExists", func() {
		It("reads content if the file exists", func() {
			filePath := "/tmp/existing-file.txt"
			content := []byte("Test Content")

			err := fs.WriteFile(filePath, content, 0o644)
			Expect(err).ToNot(HaveOccurred())

			data, err := fs.ReadFileIfExists(filePath)
			Expect(err).ToNot(HaveOccurred())
			Expect(data).To(Equal(content))
		})

		It("returns nil for a non-existent file", func() {
			filePath := "/tmp/non-existent-file.txt"

			data, err := fs.ReadFileIfExists(filePath)
			Expect(err).ToNot(HaveOccurred())
			Expect(data).To(BeNil())
		})
	})

	Describe("MockFsWithErrors", func() {
		var mockFs *fs.MockFsWithErrors

		BeforeEach(func() {
			mockFs = &fs.MockFsWithErrors{}
			fs.SetFs(mockFs)
		})
	})

	Describe("Walk", func() {
		It("walks through a directory tree", func() {
			err := fs.MkdirAll("/tmp/test-dir/sub-dir", 0o755)
			Expect(err).ToNot(HaveOccurred())

			err = fs.WriteFile("/tmp/test-dir/file1.txt", []byte("content1"), 0o644)
			Expect(err).ToNot(HaveOccurred())

			err = fs.WriteFile("/tmp/test-dir/sub-dir/file2.txt", []byte("content2"), 0o644)
			Expect(err).ToNot(HaveOccurred())

			var visited []string
			err = fs.Walk("/tmp/test-dir", func(path string, info os.FileInfo, err error) error {
				Expect(err).ToNot(HaveOccurred())
				visited = append(visited, path)
				return nil
			})
			Expect(err).ToNot(HaveOccurred())
			Expect(visited).To(ContainElements(
				"/tmp/test-dir",
				"/tmp/test-dir/file1.txt",
				"/tmp/test-dir/sub-dir",
				"/tmp/test-dir/sub-dir/file2.txt",
			))
		})
	})
})
