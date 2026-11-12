//go:build !windows

package commands

import (
	"io/fs"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("No-replace directory publishing", func() {
	DescribeTable("never replaces an existing destination",
		func(publish func(src, dst string) error) {
			root := GinkgoT().TempDir()
			src := filepath.Join(root, "staging")
			Expect(os.MkdirAll(src, 0o750)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(src, "ours.yaml"), []byte("ours"), 0o600)).To(Succeed())

			empty := filepath.Join(root, "empty")
			Expect(os.Mkdir(empty, 0o750)).To(Succeed())
			Expect(publish(src, empty)).To(MatchError(fs.ErrExist))
			Expect(os.ReadDir(empty)).To(BeEmpty())

			full := filepath.Join(root, "full")
			Expect(os.Mkdir(full, 0o750)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(full, "theirs.yaml"), []byte("theirs"), 0o600)).To(Succeed())
			Expect(publish(src, full)).To(MatchError(fs.ErrExist))
			Expect(os.ReadFile(filepath.Join(full, "theirs.yaml"))).To(Equal([]byte("theirs")))
			Expect(filepath.Join(full, "ours.yaml")).NotTo(BeAnExistingFile())

			Expect(filepath.Join(src, "ours.yaml")).To(BeAnExistingFile())
			fresh := filepath.Join(root, "fresh")
			Expect(publish(src, fresh)).To(Succeed())
			Expect(os.ReadFile(filepath.Join(fresh, "ours.yaml"))).To(Equal([]byte("ours")))
			Expect(src).NotTo(BeAnExistingFile())
		},
		Entry("platform rename", renameNoReplace),
		Entry("reservation fallback", reserveAndRename),
	)
})
