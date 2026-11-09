package commands

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"

	"github.com/jameswlane/devex/apps/cli/internal/config"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v3"
)

var _ = Describe("Configuration import merge", func() {
	var settings config.CrossPlatformSettings
	var target string
	var original []byte
	BeforeEach(func() {
		settings = config.CrossPlatformSettings{HomeDir: GinkgoT().TempDir()}
		GinkgoT().Setenv("HOME", settings.HomeDir)
		GinkgoT().Setenv("DEVEX_TEAM_CONFIG_DIR", "")
		GinkgoT().Setenv("DEVEX_ENV", "")
		target = filepath.Join(settings.GetUserConfigDir(), "applications/tools/git.yaml")
		original = []byte("name: git\ndescription: existing\nlinux:\n  install_method: apt\n  install_command: git\n  dependencies: [curl]\n")
		Expect(os.MkdirAll(filepath.Dir(target), 0750)).To(Succeed())
		Expect(os.WriteFile(target, original, 0600)).To(Succeed())
	})
	inputFile := func(format string, entries map[string]interface{}) string {
		path := filepath.Join(GinkgoT().TempDir(), "import."+format)
		data := map[string]interface{}{"configurations": map[string]interface{}{"user": entries}}
		var content []byte
		var err error
		switch format {
		case "json":
			content, err = json.Marshal(data)
		case "yaml":
			content, err = yaml.Marshal(data)
		case "tar":
			var buf bytes.Buffer
			w := tar.NewWriter(&buf)
			for _, key := range importKeys(entries) {
				body, e := yaml.Marshal(entries[key])
				Expect(e).NotTo(HaveOccurred())
				Expect(w.WriteHeader(&tar.Header{Name: "user/" + key + ".yaml", Mode: 0600, Size: int64(len(body))})).To(Succeed())
				_, e = w.Write(body)
				Expect(e).NotTo(HaveOccurred())
			}
			Expect(w.Close()).To(Succeed())
			content = buf.Bytes()
		}
		Expect(err).NotTo(HaveOccurred())
		Expect(os.WriteFile(path, content, 0600)).To(Succeed())
		return path
	}
	DescribeTable("preserves unrelated fields and entries, reports conflicts, and backs up exact bytes", func(format string) {
		input := inputFile(format, map[string]interface{}{
			"applications/tools/git": map[string]interface{}{"name": "git", "description": "imported", "linux": map[string]interface{}{"dependencies": []interface{}{"curl", "wget"}}},
			"applications/tools/new": map[string]interface{}{"name": "new", "linux": map[string]interface{}{"install_method": "apt", "install_command": "new"}},
		})
		var out bytes.Buffer
		err := runConfigurationImport(context.Background(), &out, settings, input, true, false, true, true, false, "json")
		Expect(err).To(MatchError(ContainSubstring("--force")))
		Expect(json.Valid(out.Bytes())).To(BeTrue())
		Expect(out.String()).To(ContainSubstring("$.description"))
		content, err := os.ReadFile(target)
		Expect(err).NotTo(HaveOccurred())
		Expect(content).To(Equal(original))
		Expect(filepath.Join(filepath.Dir(target), "new.yaml")).NotTo(BeAnExistingFile())
		out.Reset()
		Expect(runConfigurationImport(context.Background(), &out, settings, input, true, true, true, true, false, "json")).To(Succeed())
		var report importPlan
		Expect(json.Unmarshal(out.Bytes(), &report)).To(Succeed())
		Expect(report.Applied).To(BeTrue())
		Expect(report.Backups).To(HaveLen(1))
		saved, err := os.ReadFile(report.Backups[0])
		Expect(err).NotTo(HaveOccurred())
		Expect(saved).To(Equal(original))
		content, err = os.ReadFile(target)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(content)).To(ContainSubstring("install_command: git"))
		Expect(string(content)).To(ContainSubstring("description: imported"))
		Expect(string(content)).To(ContainSubstring("- curl"))
		Expect(string(content)).To(ContainSubstring("- wget"))
		Expect(filepath.Join(filepath.Dir(target), "new.yaml")).To(BeAnExistingFile())
	}, Entry("YAML", "yaml"), Entry("JSON", "json"), Entry("bundle", "tar"))
	It("previews the exact forced replacement without backups or writes", func() {
		input := inputFile("yaml", map[string]interface{}{"applications/tools/git": map[string]interface{}{"name": "git", "linux": map[string]interface{}{"install_method": "apt", "install_command": "git"}}})
		var out bytes.Buffer
		Expect(runConfigurationImport(context.Background(), &out, settings, input, false, true, true, true, true, "json")).To(Succeed())
		var report importPlan
		Expect(json.Unmarshal(out.Bytes(), &report)).To(Succeed())
		Expect(report.Mode).To(Equal("replace"))
		Expect(report.Applied).To(BeFalse())
		Expect(report.Backups).To(BeEmpty())
		Expect(report.Changes).To(HaveLen(1))
		Expect(report.Changes[0].After).NotTo(HaveKey("description"))
		content, err := os.ReadFile(target)
		Expect(err).NotTo(HaveOccurred())
		Expect(content).To(Equal(original))
	})
	It("rejects a graph cycle introduced only by the merged candidate before any writes or backups", func() {
		input := inputFile("yaml", map[string]interface{}{
			"applications/tools/git": map[string]interface{}{"linux": map[string]interface{}{"dependencies": []interface{}{"new"}}},
			"applications/tools/new": map[string]interface{}{"name": "new", "linux": map[string]interface{}{"install_method": "apt", "install_command": "new", "dependencies": []interface{}{"git"}}},
		})
		var out bytes.Buffer
		Expect(runConfigurationImport(context.Background(), &out, settings, input, true, true, true, false, false, "json")).To(MatchError(ContainSubstring("cycle")))
		files, err := os.ReadDir(filepath.Dir(target))
		Expect(err).NotTo(HaveOccurred())
		Expect(files).To(HaveLen(1))
		content, err := os.ReadFile(target)
		Expect(err).NotTo(HaveOccurred())
		Expect(content).To(Equal(original))
	})
	It("recursively merges named list entries and appends distinct unnamed values deterministically", func() {
		var old, incoming interface{}
		Expect(yaml.Unmarshal([]byte("items: [{name: one, keep: yes, value: old}, {name: two}]\nvalues: [a, b]\n"), &old)).To(Succeed())
		Expect(yaml.Unmarshal([]byte("items: [{name: one, value: new}, {name: three}]\nvalues: [b, c]\n"), &incoming)).To(Succeed())
		conflicts := []string{}
		got := mergeImportValues(old, incoming, "$", &conflicts)
		var expected interface{}
		Expect(yaml.Unmarshal([]byte("items: [{name: one, keep: yes, value: new}, {name: two}, {name: three}]\nvalues: [a, b, c]\n"), &expected)).To(Succeed())
		Expect(got).To(Equal(expected))
		Expect(conflicts).To(Equal([]string{"$.items[0].value"}))
	})
	It("treats null and type changes as conflicts while empty lists preserve entries", func() {
		conflicts := []string{}
		Expect(mergeImportValues("old", nil, "$.key", &conflicts)).To(BeNil())
		Expect(conflicts).To(Equal([]string{"$.key"}))
		Expect(mergeImportValues([]interface{}{"keep"}, []interface{}{}, "$", &conflicts)).To(Equal([]interface{}{"keep"}))
	})
	It("refuses changes made after planning", func() {
		input := inputFile("yaml", map[string]interface{}{"applications/tools/git": map[string]interface{}{"description": "new"}})
		data, err := readConfigurationImport(context.Background(), input)
		Expect(err).NotTo(HaveOccurred())
		plan, err := planConfigurationImport(context.Background(), settings, data, true)
		Expect(err).NotTo(HaveOccurred())
		Expect(os.WriteFile(target, []byte("name: changed\n"), 0600)).To(Succeed())
		Expect(applyImportPlan(context.Background(), plan, true)).To(MatchError(ContainSubstring("changed since preview")))
	})
	It("merges disjoint fields without force and leaves no-op imports untouched", func() {
		input := inputFile("json", map[string]interface{}{"applications/tools/git": map[string]interface{}{"name": "git", "category": "tools"}})
		var out bytes.Buffer
		Expect(runConfigurationImport(context.Background(), &out, settings, input, true, false, false, false, false, "json")).To(Succeed())
		var report importPlan
		Expect(json.Unmarshal(out.Bytes(), &report)).To(Succeed())
		Expect(report.Backups).To(HaveLen(1))
		out.Reset()
		Expect(runConfigurationImport(context.Background(), &out, settings, input, true, false, true, true, false, "json")).To(Succeed())
		Expect(json.Unmarshal(out.Bytes(), &report)).To(Succeed())
		Expect(report.Changes).To(BeEmpty())
		Expect(report.Backups).To(BeEmpty())
	})
	It("requires force to replace supplied files and preserves other files", func() {
		sibling := filepath.Join(filepath.Dir(target), "other.yaml")
		Expect(os.WriteFile(sibling, []byte("name: other\nlinux:\n  install_method: apt\n  install_command: other\n"), 0600)).To(Succeed())
		input := inputFile("yaml", map[string]interface{}{"applications/tools/git": map[string]interface{}{"name": "git", "linux": map[string]interface{}{"install_method": "apt", "install_command": "git"}}})
		var out bytes.Buffer
		Expect(runConfigurationImport(context.Background(), &out, settings, input, false, false, true, true, false, "json")).To(MatchError(ContainSubstring("--force")))
		out.Reset()
		Expect(runConfigurationImport(context.Background(), &out, settings, input, false, true, true, true, false, "json")).To(Succeed())
		content, err := os.ReadFile(target)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(content)).NotTo(ContainSubstring("description"))
		Expect(sibling).To(BeAnExistingFile())
	})
	It("respects environment options and explicit flags while producing JSON through Cobra", func() {
		input := inputFile("yaml", map[string]interface{}{"applications/tools/git": map[string]interface{}{"description": "new"}})
		GinkgoT().Setenv("DEVEX_DRY_RUN", "true")
		GinkgoT().Setenv("DEVEX_FORCE", "false")
		GinkgoT().Setenv("DEVEX_FORMAT", "json")
		var out bytes.Buffer
		cmd := newConfigImportCmd(settings)
		cmd.SetOut(&out)
		cmd.SetArgs([]string{input, "--force"})
		Expect(cmd.Execute()).To(Succeed())
		var report importPlan
		Expect(json.Unmarshal(out.Bytes(), &report)).To(Succeed())
		Expect(report.DryRun).To(BeTrue())
		Expect(report.Applied).To(BeFalse())
		Expect(report.Changes[0].After).To(HaveKeyWithValue("description", "new"))
	})

	It("does not change an earlier file when preparing a later output fails", func() {
		input := inputFile("yaml", map[string]interface{}{
			"applications/tools/git": map[string]interface{}{"description": "new"},
			"applications/z/new":     map[string]interface{}{"name": "new", "linux": map[string]interface{}{"install_method": "apt", "install_command": "new"}},
		})
		data, err := readConfigurationImport(context.Background(), input)
		Expect(err).NotTo(HaveOccurred())
		plan, err := planConfigurationImport(context.Background(), settings, data, true)
		Expect(err).NotTo(HaveOccurred())
		Expect(os.WriteFile(filepath.Join(settings.GetUserConfigDir(), "applications/z"), []byte("obstruction"), 0600)).To(Succeed())
		Expect(applyImportPlan(context.Background(), plan, true)).NotTo(Succeed())
		content, err := os.ReadFile(target)
		Expect(err).NotTo(HaveOccurred())
		Expect(content).To(Equal(original))
		temps, err := filepath.Glob(filepath.Join(filepath.Dir(target), ".devex-import-*"))
		Expect(err).NotTo(HaveOccurred())
		Expect(temps).To(BeEmpty())
	})

})
