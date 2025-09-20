package utils_test

import (
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/afero"

	"github.com/jameswlane/devex/apps/cli/internal/fs"
	"github.com/jameswlane/devex/apps/cli/internal/utils"
)

var _ = Describe("CopyConfigFiles", func() {
	var (
		srcDir     string
		dstDir     string
		maxWorkers int
	)

	BeforeEach(func() {
		fs.UseMemMapFs()
		srcDir = "test_src"
		dstDir = "test_dst"
		maxWorkers = 5

		err := fs.MkdirAll(srcDir, 0o755)
		if err != nil {
			return
		}
		err = fs.MkdirAll(dstDir, 0o755)
		if err != nil {
			return
		}
	})

	AfterEach(func() {
		fs.SetFs(afero.NewOsFs())
	})

	It("copies files from source to destination", func() {
		srcFile := filepath.Join(srcDir, "config.yaml")
		err := fs.WriteFile(srcFile, []byte("content"), 0o644)
		if err != nil {
			return
		}

		err = utils.CopyConfigFiles(srcDir, dstDir, maxWorkers)
		Expect(err).ToNot(HaveOccurred())

		dstFile := filepath.Join(dstDir, "config.yaml")
		Expect(fs.FileExistsAndIsFile(dstFile)).To(BeTrue())
	})

	It("returns an error if source directory does not exist", func() {
		err := utils.CopyConfigFiles("nonexistent_src", dstDir, maxWorkers)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("source directory does not exist"))
	})

	It("skips copying if file already exists in destination", func() {
		srcFile := filepath.Join(srcDir, "config.yaml")
		err := fs.WriteFile(srcFile, []byte("content"), 0o644)
		if err != nil {
			return
		}

		dstFile := filepath.Join(dstDir, "config.yaml")
		err = fs.WriteFile(dstFile, []byte("existing content"), 0o644)
		if err != nil {
			return
		}

		err = utils.CopyConfigFiles(srcDir, dstDir, maxWorkers)
		Expect(err).ToNot(HaveOccurred())

		content, _ := fs.ReadFile(dstFile)
		Expect(string(content)).To(Equal("existing content"))
	})

	It("returns an error if walking source directory fails", func() {
		fs.SetFs(&fs.MockFsWithErrors{FailStat: true})
		err := utils.CopyConfigFiles(srcDir, dstDir, maxWorkers)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("mock failure in Stat"))
	})
})
