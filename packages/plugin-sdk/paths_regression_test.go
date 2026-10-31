package sdk_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
)

var _ = Describe("Directory policy regression guard", func() {
	It("keeps legacy home paths confined to migration and fallback code", func() {
		_, source, _, ok := runtime.Caller(0)
		Expect(ok).To(BeTrue())
		repo := filepath.Clean(filepath.Join(filepath.Dir(source), "../.."))
		if _, err := os.Stat(filepath.Join(repo, "apps/cli")); os.IsNotExist(err) {
			Skip("monorepo guard only applies in a DevEx checkout")
		}
		hardcoded := regexp.MustCompile(`(?:"|/|\\)\.devex(?:"|/|\\)`)
		var violations []string
		for _, base := range []string{"apps/cli", "packages"} {
			Expect(filepath.WalkDir(filepath.Join(repo, base), func(path string, entry fs.DirEntry, err error) error {
				if err != nil {
					return err
				}
				if entry.IsDir() {
					if entry.Name() == "node_modules" || entry.Name() == "dist" {
						return filepath.SkipDir
					}
					return nil
				}
				if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") || path == filepath.Join(repo, "packages/plugin-sdk/paths_legacy.go") {
					return nil
				}
				content, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				if hardcoded.Match(content) {
					rel, _ := filepath.Rel(repo, path)
					violations = append(violations, rel)
				}
				return nil
			})).To(Succeed())
		}
		Expect(violations).To(BeEmpty(), "use the shared paths package; legacy literals belong only in paths_legacy.go")
	})
})
