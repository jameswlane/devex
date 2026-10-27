package commands_test

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/jameswlane/devex/apps/cli/internal/commands"
)

var _ = Describe("Uninstall cleanup path safety", func() {
	const home = "/home/tester"

	DescribeTable("rejects paths whose removal would destroy unrelated data",
		func(path string) {
			_, err := commands.ValidateCleanupPath(path, home)
			Expect(err).To(HaveOccurred())
		},
		Entry("empty", ""),
		Entry("relative", "some/dir"),
		Entry("filesystem root", "/"),
		Entry("system directory", "/etc"),
		Entry("system directory with trailing slash", "/usr/"),
		Entry("home directory", home),
		Entry("home directory via traversal", home+"/.config/.."),
		Entry("parent of home", "/home"),
	)

	DescribeTable("allows paths inside the home directory or app-specific system paths",
		func(path, want string) {
			got, err := commands.ValidateCleanupPath(path, home)
			Expect(err).ToNot(HaveOccurred())
			Expect(got).To(Equal(want))
		},
		Entry("app config dir", home+"/.config/myapp", home+"/.config/myapp"),
		Entry("cleans the path", home+"/.config//myapp/", home+"/.config/myapp"),
		Entry("app data outside home", "/opt/myapp/cache", "/opt/myapp/cache"),
	)

	It("removes an app directory under the home directory", func() {
		tempHome := GinkgoT().TempDir()
		GinkgoT().Setenv("HOME", tempHome)
		target := filepath.Join(tempHome, ".config", "myapp")
		Expect(os.MkdirAll(filepath.Join(target, "nested"), 0o750)).To(Succeed())

		Expect(commands.RemoveFileIfExists("~/.config/myapp")).To(Succeed())

		_, err := os.Stat(target)
		Expect(os.IsNotExist(err)).To(BeTrue())
	})

	It("refuses to remove the home directory itself", func() {
		tempHome := GinkgoT().TempDir()
		GinkgoT().Setenv("HOME", tempHome)

		Expect(commands.RemoveFileIfExists("~/.")).ToNot(Succeed())
		Expect(tempHome).To(BeADirectory())
	})
})

var _ = Describe("Environment names", func() {
	DescribeTable("accepts single safe path components",
		func(name string) {
			Expect(commands.ValidateEnvironmentName(name)).To(Succeed())
		},
		Entry("simple", "staging"),
		Entry("with separators", "prod-eu_1.2"),
	)

	DescribeTable("rejects names that could escape the environments directory",
		func(name string) {
			Expect(commands.ValidateEnvironmentName(name)).ToNot(Succeed())
		},
		Entry("empty", ""),
		Entry("parent", ".."),
		Entry("traversal", "../../etc"),
		Entry("nested", "a/b"),
		Entry("hidden", ".hidden"),
		Entry("embedded dots", "a..b"),
	)
})
