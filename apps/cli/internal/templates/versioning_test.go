package templates_test

import (
	"os"
	"path/filepath"
	"testing/fstest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/jameswlane/devex/apps/cli/internal/templates"
)

func builtinTemplateFS(version, applications string) fstest.MapFS {
	return fstest.MapFS{
		"backend/manifest.yaml": {Data: []byte(`id: backend
name: Backend
version: ` + version + `
description: Backend services
files:
  - applications.yaml
  - system.yaml
`)},
		"backend/applications.yaml": {Data: []byte(applications)},
		"backend/system.yaml":       {Data: []byte("git: {}\n")},
	}
}

var _ = Describe("Template version manager", func() {
	var baseDir string

	BeforeEach(func() {
		baseDir = GinkgoT().TempDir()
	})

	registered := func(fsys fstest.MapFS) *templates.TemplateVersionManager {
		tvm := templates.NewTemplateVersionManagerWithBuiltins(baseDir, fsys)
		Expect(tvm.InitializeTemplateVersioning()).To(Succeed())
		Expect(tvm.ScanAndRegisterTemplates()).To(Succeed())
		return tvm
	}

	recorded := func(tvm *templates.TemplateVersionManager, id string) templates.TemplateVersion {
		versions, err := tvm.GetTemplateVersions()
		Expect(err).NotTo(HaveOccurred())
		Expect(versions).To(HaveKey(id))
		return *versions[id]
	}

	It("records the shipped version and checksum without changing template files", func() {
		registered(builtinTemplateFS("1.0.0", "applications: []\n"))
		before := recorded(templates.NewTemplateVersionManagerWithBuiltins(baseDir, nil), "backend")

		shipped := builtinTemplateFS("1.1.0", "applications:\n  - name: git\n")
		original := string(shipped["backend/applications.yaml"].Data)
		tvm := templates.NewTemplateVersionManagerWithBuiltins(baseDir, shipped)

		result, err := tvm.UpdateTemplate("backend", false)

		Expect(err).NotTo(HaveOccurred())
		Expect(result.Success).To(BeTrue())
		Expect(result.OldVersion).To(Equal("1.0.0"))
		Expect(result.NewVersion).To(Equal("1.1.0"))
		Expect(result.FilesVerified).To(Equal([]string{"applications.yaml", "system.yaml"}))
		Expect(result.Message).To(ContainSubstring("no template files were changed"))
		Expect(string(shipped["backend/applications.yaml"].Data)).To(Equal(original))

		after := recorded(tvm, "backend")
		Expect(after.Version).To(Equal("1.1.0"))
		Expect(after.Checksum).NotTo(Equal(before.Checksum))
		Expect(after.UpdateAvailable).To(BeFalse())

		updates, err := tvm.CheckForUpdates()
		Expect(err).NotTo(HaveOccurred())
		Expect(updates).To(BeEmpty())
	})

	It("reports an up-to-date template and points to devex upgrade", func() {
		tvm := registered(builtinTemplateFS("1.0.0", "applications: []\n"))
		before := recorded(tvm, "backend")

		result, err := tvm.UpdateTemplate("backend", false)

		Expect(err).NotTo(HaveOccurred())
		Expect(result.Success).To(BeTrue())
		Expect(result.NewVersion).To(Equal("1.0.0"))
		Expect(result.Message).To(ContainSubstring("devex upgrade"))
		Expect(recorded(tvm, "backend").LastUpdated).To(Equal(before.LastUpdated))
	})

	It("leaves the record unchanged when a listed file is missing", func() {
		registered(builtinTemplateFS("1.0.0", "applications: []\n"))
		shipped := builtinTemplateFS("1.1.0", "applications: []\n")
		delete(shipped, "backend/system.yaml")
		tvm := templates.NewTemplateVersionManagerWithBuiltins(baseDir, shipped)

		result, err := tvm.UpdateTemplate("backend", true)

		Expect(err).To(MatchError(ContainSubstring("missing system.yaml")))
		Expect(result.Success).To(BeFalse())
		Expect(recorded(tvm, "backend").Version).To(Equal("1.0.0"))
	})

	It("treats a listed path outside the template as missing", func() {
		registered(builtinTemplateFS("1.0.0", "applications: []\n"))
		shipped := builtinTemplateFS("1.1.0", "applications: []\n")
		shipped["backend/manifest.yaml"] = &fstest.MapFile{Data: []byte(`id: backend
name: Backend
version: 1.1.0
files:
  - ../secret.yaml
`)}
		shipped["secret.yaml"] = &fstest.MapFile{Data: []byte("token: x\n")}
		tvm := templates.NewTemplateVersionManagerWithBuiltins(baseDir, shipped)

		_, err := tvm.UpdateTemplate("backend", true)

		Expect(err).To(MatchError(ContainSubstring("missing ../secret.yaml")))
		Expect(recorded(tvm, "backend").Version).To(Equal("1.0.0"))
	})

	It("leaves the record unchanged when the shipped version is invalid", func() {
		registered(builtinTemplateFS("1.0.0", "applications: []\n"))
		tvm := templates.NewTemplateVersionManagerWithBuiltins(baseDir, builtinTemplateFS("latest", "applications: []\n"))

		_, err := tvm.UpdateTemplate("backend", true)

		Expect(err).To(MatchError(ContainSubstring(`invalid version "latest"`)))
		Expect(recorded(tvm, "backend").Version).To(Equal("1.0.0"))
	})

	It("fails when a recorded built-in template is no longer shipped", func() {
		registered(builtinTemplateFS("1.0.0", "applications: []\n"))
		tvm := templates.NewTemplateVersionManagerWithBuiltins(baseDir, fstest.MapFS{})

		_, err := tvm.UpdateTemplate("backend", false)

		Expect(err).To(MatchError(ContainSubstring("not shipped with this DevEx release")))
		Expect(recorded(tvm, "backend").Version).To(Equal("1.0.0"))
	})

	It("refuses to update user templates and leaves their files untouched", func() {
		userTemplate := filepath.Join(baseDir, "config", templates.UserTemplatesDir, "mine")
		Expect(os.MkdirAll(userTemplate, 0o750)).To(Succeed())
		manifest := []byte("id: mine\nname: Mine\nversion: 1.0.0\nfiles:\n  - applications.yaml\n")
		Expect(os.WriteFile(filepath.Join(userTemplate, templates.TemplateManifest), manifest, 0o600)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(userTemplate, "applications.yaml"), []byte("applications: []\n"), 0o600)).To(Succeed())
		tvm := registered(fstest.MapFS{})
		Expect(recorded(tvm, "mine").Source).To(Equal("user"))

		_, err := tvm.UpdateTemplate("mine", true)

		Expect(err).To(MatchError(ContainSubstring("only builtin templates")))
		data, readErr := os.ReadFile(filepath.Join(userTemplate, templates.TemplateManifest))
		Expect(readErr).NotTo(HaveOccurred())
		Expect(data).To(Equal(manifest))
	})

	It("reports failed templates in an update of all templates", func() {
		registered(builtinTemplateFS("1.0.0", "applications: []\n"))
		shipped := builtinTemplateFS("1.1.0", "applications: []\n")
		delete(shipped, "backend/applications.yaml")
		tvm := templates.NewTemplateVersionManagerWithBuiltins(baseDir, shipped)

		results, err := tvm.UpdateAllTemplates(true)

		Expect(err).NotTo(HaveOccurred())
		Expect(results).To(HaveLen(1))
		Expect(results[0].Success).To(BeFalse())
		Expect(results[0].Message).To(ContainSubstring("missing applications.yaml"))
	})

	It("does not create a built-in templates directory in the working directory", func() {
		workDir := GinkgoT().TempDir()
		DeferCleanup(os.Chdir, mustGetwd())
		Expect(os.Chdir(workDir)).To(Succeed())

		Expect(templates.NewTemplateVersionManager(baseDir).InitializeTemplateVersioning()).To(Succeed())

		Expect(filepath.Join(workDir, "assets")).NotTo(BeADirectory())
	})

	It("finds the templates compiled into the binary from any working directory", func() {
		DeferCleanup(os.Chdir, mustGetwd())
		Expect(os.Chdir(GinkgoT().TempDir())).To(Succeed())

		tvm := templates.NewTemplateVersionManager(baseDir)
		Expect(tvm.InitializeTemplateVersioning()).To(Succeed())
		Expect(tvm.ScanAndRegisterTemplates()).To(Succeed())
		Expect(recorded(tvm, "backend").Source).To(Equal("builtin"))

		specs, err := templates.NewTemplateManager(GinkgoT().TempDir()).GetAvailableTemplateSpecsWithSources()
		Expect(err).NotTo(HaveOccurred())
		sources := map[string]string{}
		for _, spec := range specs {
			sources[spec.Spec.ID] = spec.Source
		}
		Expect(sources).To(HaveKeyWithValue("backend", "builtin"))
		Expect(sources).To(HaveKeyWithValue("full-stack", "builtin"))
	})
})

func mustGetwd() string {
	wd, err := os.Getwd()
	Expect(err).NotTo(HaveOccurred())
	return wd
}
