package templates_test

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v3"

	"github.com/jameswlane/devex/apps/cli/internal/templates"
)

type repeatedTemplateReader struct{}

func (repeatedTemplateReader) Read(p []byte) (int, error) { clear(p); return len(p), nil }

var _ = Describe("Custom template import safety", func() {
	var manager *templates.CustomTemplateManager
	var root, source string
	BeforeEach(func() {
		root = GinkgoT().TempDir()
		source = filepath.Join(root, "source")
		Expect(os.Mkdir(source, 0700)).To(Succeed())
		var err error
		manager, err = templates.NewCustomTemplateManager(root, root, nil, nil, nil)
		Expect(err).NotTo(HaveOccurred())
	})
	writeManifest := func(id, org string) {
		data, err := yaml.Marshal(templates.CustomTemplateManifest{ID: id, Organization: org, Files: []string{"app.yaml"}})
		Expect(err).NotTo(HaveOccurred())
		Expect(os.WriteFile(filepath.Join(source, "manifest.yaml"), data, 0600)).To(Succeed())
		Expect(os.WriteFile(filepath.Join(source, "app.yaml"), []byte("apps: []"), 0600)).To(Succeed())
	}
	DescribeTable("rejects unsafe imported identities before publishing", func(id, org string) {
		writeManifest(id, org)
		_, _, err := manager.InstallLocalForTest(source)
		Expect(err).To(MatchError(ContainSubstring("single safe path component")))
		entries, err := os.ReadDir(filepath.Join(root, "templates", "custom", "user"))
		Expect(err).NotTo(HaveOccurred())
		Expect(entries).To(BeEmpty())
	}, Entry("parent ID", "../escaped", ""), Entry("absolute ID", "/escaped", ""), Entry("backslash ID", `..\escaped`, ""), Entry("parent organization", "valid", "../../../escaped"), Entry("absolute organization", "valid", "/escaped"), Entry("dot ID", "..", ""))
	It("publishes valid templates", func() {
		writeManifest("valid-template", "Example Team")
		_, target, err := manager.InstallLocalForTest(source)
		Expect(err).NotTo(HaveOccurred())
		Expect(target).To(Equal(filepath.Join(root, "templates", "custom", "team", "Example Team", "valid-template")))
		Expect(os.ReadFile(filepath.Join(target, "app.yaml"))).To(Equal([]byte("apps: []")))
	})
	It("confines publication when the target directory is a symlink", func() {
		writeManifest("valid-template", "")
		outside := GinkgoT().TempDir()
		Expect(os.Symlink(outside, filepath.Join(root, "templates", "custom", "user", "valid-template"))).To(Succeed())
		_, _, err := manager.InstallLocalForTest(source)
		Expect(err).To(HaveOccurred())
		entries, err := os.ReadDir(outside)
		Expect(err).NotTo(HaveOccurred())
		Expect(entries).To(BeEmpty())
	})
	It("does not serialize authentication tokens", func() {
		src := templates.TemplateSource{Type: "git", Token: "private-secret"}
		data, err := yaml.Marshal(src)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(data)).NotTo(ContainSubstring("private-secret"))
		data, err = json.Marshal(src)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(data)).NotTo(ContainSubstring("private-secret"))
	})
	It("excludes runtime tokens when saving registry metadata", func() {
		Expect(manager.RegisterTemplateForTest(&templates.CustomTemplateManifest{ID: "valid", Source: templates.TemplateSource{Type: "git", Token: "private-secret"}})).To(Succeed())
		data, err := os.ReadFile(filepath.Join(root, "templates", "custom", "registry.yaml"))
		Expect(err).NotTo(HaveOccurred())
		Expect(string(data)).NotTo(ContainSubstring("private-secret"))
		Expect(string(data)).NotTo(ContainSubstring("token:"))
	})

	It("scrubs legacy tokens from saved registry metadata on load", func() {
		path := filepath.Join(root, "templates", "custom", "registry.yaml")
		Expect(os.WriteFile(path, []byte("version: 1.0.0\ntemplates:\n  valid:\n    id: valid\n    source:\n      type: git\n      token: private-secret\n"), 0600)).To(Succeed())
		items, err := manager.ListCustomTemplates()
		Expect(err).NotTo(HaveOccurred())
		Expect(items).To(HaveLen(1))
		Expect(items[0].Source.Token).To(BeEmpty())
		data, err := os.ReadFile(path)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(data)).NotTo(ContainSubstring("private-secret"))
		Expect(string(data)).NotTo(ContainSubstring("token:"))
	})
	It("copies small archives without returning EOF", func() {
		var dst bytes.Buffer
		n, err := templates.CopyTemplateArchive(&dst, strings.NewReader("small archive"))
		Expect(err).NotTo(HaveOccurred())
		Expect(n).To(Equal(int64(13)))
		Expect(dst.String()).To(Equal("small archive"))
	})
	It("extracts small ZIP entries successfully", func() {
		path := filepath.Join(root, "archive.zip")
		file, err := os.Create(path)
		Expect(err).NotTo(HaveOccurred())
		writer := zip.NewWriter(file)
		entry, err := writer.Create("small.yaml")
		Expect(err).NotTo(HaveOccurred())
		_, err = io.WriteString(entry, "small: true")
		Expect(err).NotTo(HaveOccurred())
		Expect(writer.Close()).To(Succeed())
		Expect(file.Close()).To(Succeed())
		dest := filepath.Join(root, "extracted")
		Expect(manager.ExtractZipForTest(path, dest)).To(Succeed())
		Expect(os.ReadFile(filepath.Join(dest, "small.yaml"))).To(Equal([]byte("small: true")))
	})
	It("rejects oversized ZIP entries", func() {
		path := filepath.Join(root, "oversize.zip")
		file, err := os.Create(path)
		Expect(err).NotTo(HaveOccurred())
		writer := zip.NewWriter(file)
		entry, err := writer.Create("large.yaml")
		Expect(err).NotTo(HaveOccurred())
		_, err = io.CopyN(entry, repeatedTemplateReader{}, 100*1024*1024+1)
		Expect(err).NotTo(HaveOccurred())
		Expect(writer.Close()).To(Succeed())
		Expect(file.Close()).To(Succeed())
		Expect(manager.ExtractZipForTest(path, filepath.Join(root, "extracted"))).To(MatchError(ContainSubstring("exceeds the 100 MiB size limit")))
	})
	It("installs a small HTTP archive with an unsafe ref without using the ref as a path", func() {
		var archive bytes.Buffer
		writer := zip.NewWriter(&archive)
		entry, err := writer.Create("manifest.yaml")
		Expect(err).NotTo(HaveOccurred())
		_, err = io.WriteString(entry, "id: valid-http\nfiles: [app.yaml]\nsource:\n  token: legacy-secret\n")
		Expect(err).NotTo(HaveOccurred())
		entry, err = writer.Create("app.yaml")
		Expect(err).NotTo(HaveOccurred())
		_, err = io.WriteString(entry, "apps: []")
		Expect(err).NotTo(HaveOccurred())
		Expect(writer.Close()).To(Succeed())
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write(archive.Bytes()) }))
		defer server.Close()
		_, target, err := manager.InstallHTTPForTest(context.Background(), "../../escape", &templates.TemplateSource{Type: "http", URL: server.URL, Token: "request-secret"})
		Expect(err).NotTo(HaveOccurred())
		data, err := os.ReadFile(filepath.Join(target, "manifest.yaml"))
		Expect(err).NotTo(HaveOccurred())
		Expect(string(data)).NotTo(ContainSubstring("secret"))
		Expect(string(data)).To(ContainSubstring(server.URL))
		entries, err := os.ReadDir(target)
		Expect(err).NotTo(HaveOccurred())
		Expect(entries).To(HaveLen(2))
	})

	It("errors for oversized archives instead of truncating successfully", func() {
		n, err := templates.CopyTemplateArchive(io.Discard, repeatedTemplateReader{})
		Expect(err).To(MatchError(ContainSubstring("exceeds the 100 MiB size limit")))
		Expect(n).To(Equal(int64(100*1024*1024 + 1)))
	})
})
