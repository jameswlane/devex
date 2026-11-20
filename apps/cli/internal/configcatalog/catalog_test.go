package configcatalog_test

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/configcatalog"
	"github.com/jameswlane/devex/apps/cli/internal/types"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v3"
)

func TestConfigCatalog(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Shipped configuration catalog")
}

var _ = Describe("Shipped configuration validation", func() {
	var root string
	write := func(path, text string) {
		Expect(os.MkdirAll(filepath.Dir(path), 0750)).To(Succeed())
		Expect(os.WriteFile(path, []byte(text), 0600)).To(Succeed())
	}
	BeforeEach(func() {
		root = GinkgoT().TempDir()
		Expect(os.MkdirAll(filepath.Join(root, "assets/templates"), 0750)).To(Succeed())
		write(filepath.Join(root, "config/applications/tools/editor.yaml"), "name: editor\nlinux:\n  install_method: apt\n  install_command: editor\n")
	})
	errors := func() []config.Diagnostic {
		ds, err := configcatalog.Validate(context.Background(), root)
		Expect(err).NotTo(HaveOccurred())
		var result []config.Diagnostic
		for _, d := range ds {
			if d.Severity == config.SeverityError {
				result = append(result, d)
			}
		}
		return result
	}
	It("validates the complete repository catalog through runtime entry points", func() {
		root = filepath.Join("..", "..")
		Expect(errors()).To(BeEmpty())
	})
	It("does not require the compilers Build Essential installs", func() {
		data, err := os.ReadFile(filepath.Join("..", "..", "config", "applications", "development", "build-essential.yaml"))
		Expect(err).NotTo(HaveOccurred())
		var app types.CrossPlatformApp
		Expect(yaml.Unmarshal(data, &app)).To(Succeed())
		for _, platform := range []types.OSConfig{app.Linux, app.MacOS, app.Windows} {
			Expect(platform.SystemRequirements.RequiredCommands).NotTo(ContainElement(BeElementOf("make", "gcc")))
		}
	})
	It("runs every shipped DNF and YUM hook unattended with system privileges", func() {
		// Unattended installs close stdin, so a dnf/yum transaction without
		// --assumeyes aborts, and a user-scope hook cannot write system state.
		uses := regexp.MustCompile(`\b(dnf|yum|yum-config-manager)\b`)
		transaction := regexp.MustCompile(`\b(dnf|yum)\s+(?:-\S+\s+)*(install|remove|erase|upgrade|update|groupinstall|copr\s+enable|module\s+(?:install|enable))\b`)
		assumeYes := regexp.MustCompile(`(^|\s)(-y|--assumeyes)(\s|$)`)
		segment := regexp.MustCompile(`[;&|\n]+`)
		var hooks int
		var problems []string
		var check func(file string, cfg types.OSConfig)
		check = func(file string, cfg types.OSConfig) {
			for _, hook := range append(append([]types.InstallCommand{}, cfg.PreInstall...), cfg.PostInstall...) {
				if !uses.MatchString(hook.Shell) {
					continue
				}
				hooks++
				if hook.Scope != "system" || strings.Contains(hook.Shell, "sudo") {
					problems = append(problems, file+": needs scope: system without sudo: "+hook.Shell)
				}
				for _, part := range segment.Split(hook.Shell, -1) {
					if transaction.MatchString(part) && !assumeYes.MatchString(part) {
						problems = append(problems, file+": needs -y: "+strings.TrimSpace(part))
					}
				}
			}
			for _, alternative := range cfg.Alternatives {
				check(file, alternative)
			}
		}
		catalog := filepath.Join("..", "..", "config", "applications")
		Expect(filepath.WalkDir(catalog, func(path string, entry os.DirEntry, err error) error {
			if err != nil || entry.IsDir() || filepath.Ext(path) != ".yaml" {
				return err
			}
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			var app types.CrossPlatformApp
			if err := yaml.Unmarshal(data, &app); err != nil {
				return err
			}
			check(filepath.Base(path), app.Linux)
			return nil
		})).To(Succeed())
		Expect(hooks).To(BeNumerically(">", 0))
		Expect(problems).To(BeEmpty())
	})
	It("accepts valid catalogs without creating user files", func() {
		home := GinkgoT().TempDir()
		GinkgoT().Setenv("HOME", home)
		Expect(errors()).To(BeEmpty())
		entries, err := os.ReadDir(home)
		Expect(err).NotTo(HaveOccurred())
		Expect(entries).To(BeEmpty())
	})
	It("reports a missing asset with its file and field, then accepts the supplied asset", func() {
		path := filepath.Join(root, "config/applications/tools/editor.yaml")
		write(path, "name: editor\nlinux:\n  install_method: apt\n  install_command: editor\n  config_files:\n    - source: ~/.local/share/devex/assets/defaults/editor/init.vim\n      destination: ~/.config/editor/init.vim\n")
		Expect(errors()).To(ContainElement(And(HaveField("File", path), HaveField("Field", "linux.config_files[0].source"), HaveField("Message", ContainSubstring("missing")))))
		write(filepath.Join(root, "assets/defaults/editor/init.vim"), "set number")
		Expect(errors()).To(BeEmpty())
	})
	It("rejects unsupported declared methods in alternatives", func() {
		path := filepath.Join(root, "config/applications/tools/editor.yaml")
		write(path, "name: editor\nlinux:\n  install_method: apt\n  install_command: editor\n  alternatives:\n    - install_method: appt\n      install_command: editor\n")
		Expect(errors()).To(ContainElement(And(HaveField("File", path), HaveField("Field", ContainSubstring("alternatives[0].install_method")))))
	})
	It("rejects malformed YAML even in auxiliary template configuration", func() {
		path := filepath.Join(root, "assets/templates/backend/environment.yaml")
		write(path, "languages: [")
		Expect(errors()).To(ContainElement(HaveField("File", path)))
	})
	It("rejects a category spelled differently from an existing one", func() {
		write(filepath.Join(root, "config/applications/tools/a.yaml"), "name: a\ncategory: Utilities\nlinux:\n  install_method: apt\n  install_command: a\n")
		path := filepath.Join(root, "config/applications/tools/b.yaml")
		write(path, "name: b\ncategory: utility\nlinux:\n  install_method: apt\n  install_command: b\n")
		Expect(errors()).To(ContainElement(And(HaveField("File", path), HaveField("Field", "category"), HaveField("Message", ContainSubstring(`"utility" duplicates "Utilities"`)))))
		write(path, "name: b\ncategory: \" Utilities \"\nlinux:\n  install_method: apt\n  install_command: b\n")
		Expect(errors()).To(ContainElement(HaveField("Message", ContainSubstring(`" Utilities " duplicates "Utilities"`))))
		write(path, "name: b\ncategory: Utilities\nlinux:\n  install_method: apt\n  install_command: b\n")
		Expect(errors()).To(BeEmpty())
	})
	It("rejects unknown application schema fields", func() {
		write(filepath.Join(root, "config/applications/tools/editor.yaml"), "name: editor\nlinux:\n  install_method: apt\n  install_command: editor\n  typo: true\n")
		Expect(errors()).To(ContainElement(HaveField("Field", "linux.typo")))
	})
	It("rejects missing template manifest files", func() {
		path := filepath.Join(root, "assets/templates/backend/manifest.yaml")
		write(path, "id: backend\nname: Backend\nversion: 1.0.0\ndescription: Backend\nfiles: [applications.yaml, environment.yaml]\n")
		write(filepath.Join(filepath.Dir(path), "applications.yaml"), "applications: []\n")
		Expect(errors()).To(ContainElement(And(HaveField("Field", "files[1]"), HaveField("Message", ContainSubstring("environment.yaml")))))
	})
	It("rejects unknown catalog references in templates", func() {
		write(filepath.Join(root, "assets/templates/broken.yaml"), "metadata:\n  name: broken\n  description: Broken\n  category: development\napplications:\n  - name: nonexistent\n")
		Expect(errors()).To(ContainElement(HaveField("Message", ContainSubstring("unknown catalog application: nonexistent"))))
	})
	It("rejects unsupported template installers", func() {
		write(filepath.Join(root, "assets/templates/broken.yaml"), "metadata:\n  name: broken\n  description: Broken\n  category: development\napplications:\n  - name: editor\n    description: Editor\n    category: development\n    install_method: appt\n    install_command: editor\n")
		Expect(errors()).To(ContainElement(HaveField("Message", ContainSubstring("unsupported install method: appt"))))
	})
	It("rejects escaping manifest references", func() {
		write(filepath.Join(root, "config/templates/backend/manifest.yaml"), "id: backend\nname: Backend\nversion: 1.0.0\ndescription: Backend\nfiles: [../../../outside.yaml]\n")
		Expect(errors()).To(ContainElement(HaveField("Message", ContainSubstring("escapes shipped tree"))))
	})
	It("does not confuse native package dependencies with dangling app references", func() {
		write(filepath.Join(root, "config/applications/tools/editor.yaml"), "name: editor\nlinux:\n  install_method: apt\n  install_command: editor\n  dependencies: [native-dev-package]\n")
		Expect(errors()).To(BeEmpty())
	})
	It("rejects unknown template schema fields", func() {
		write(filepath.Join(root, "assets/templates/broken.yaml"), "metadata:\n  name: broken\n  description: Broken\n  category: development\ntypo: true\n")
		Expect(errors()).To(ContainElement(HaveField("Message", ContainSubstring("field typo not found"))))
	})
	It("rejects additional YAML documents", func() {
		write(filepath.Join(root, "config/extra.yaml"), "name: first\n---\nname: hidden\n")
		Expect(errors()).To(ContainElement(HaveField("Message", ContainSubstring("exactly one YAML document"))))
	})
	It("checks asset references behind aliases", func() {
		write(filepath.Join(root, "config/extra.yaml"), "shared: &files\n  - source: ~/.local/share/devex/assets/absent.txt\n    destination: ~/.config/absent.txt\nconfig_files: *files\n")
		Expect(errors()).To(ContainElement(HaveField("Field", "config_files[0].source")))
	})

	It("honors cancellation", func() {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		_, err := configcatalog.Validate(ctx, root)
		Expect(err).To(MatchError(context.Canceled))
	})
})
