package setup

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/platform"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v3"
)

var _ = Describe("Setup database menu", func() {
	expectEveryDatabaseInstalls := func(model *SetupModel) {
		Expect(model.system.databases).NotTo(BeEmpty())
		for i, db := range model.system.databases {
			Expect(databaseContainers).To(HaveKey(db), "the setup menu offers %s but has no container for it", db)
			model.selections.selectedDBs = map[int]bool{i: true}
			apps := model.getDatabaseApps()
			Expect(apps).To(HaveLen(1), "selecting %s should install one container", db)
			Expect(apps[0].Name).To(Equal("docker-" + strings.ToLower(db)))
			Expect(apps[0].Linux.InstallMethod).To(Equal("docker"))
			Expect(apps[0].Linux.InstallCommand).To(HavePrefix("docker run -d"))
		}
	}

	It("installs a container for every database the interactive menu offers", func() {
		model := NewSetupModel(nil, config.CrossPlatformSettings{}, platform.DetectionResult{OS: "linux"})
		expectEveryDatabaseInstalls(model)
	})

	It("installs a container for every database automated setup offers", func() {
		model, err := createAutomatedSetupModel(nil, config.CrossPlatformSettings{})
		Expect(err).NotTo(HaveOccurred())
		expectEveryDatabaseInstalls(model)
	})

	It("runs MongoDB in a mongo container on localhost", func() {
		model := NewSetupModel(nil, config.CrossPlatformSettings{}, platform.DetectionResult{OS: "linux"})
		Expect(model.system.databases).To(ContainElement("MongoDB"))
		for i, db := range model.system.databases {
			model.selections.selectedDBs[i] = db == "MongoDB"
		}
		apps := model.getDatabaseApps()
		Expect(apps).To(HaveLen(1))
		Expect(apps[0].Linux.InstallCommand).To(Equal("docker run -d --name mongodb --restart unless-stopped -p 127.0.0.1:27017:27017 mongo:7"))
	})

	It("has a catalog definition for every database the config-driven setup offers", func() {
		configDir := filepath.Join("..", "..", "..", "config")
		data, err := os.ReadFile(filepath.Join(configDir, "setup.yaml"))
		Expect(err).NotTo(HaveOccurred())
		var setupConfig struct {
			Steps []struct {
				ID       string `yaml:"id"`
				Question struct {
					Options []struct {
						Value string `yaml:"value"`
					} `yaml:"options"`
				} `yaml:"question"`
			} `yaml:"steps"`
		}
		Expect(yaml.Unmarshal(data, &setupConfig)).To(Succeed())
		var values []string
		for _, step := range setupConfig.Steps {
			if step.ID != "databases" {
				continue
			}
			for _, option := range step.Question.Options {
				values = append(values, option.Value)
			}
		}
		Expect(values).To(ContainElement("mongodb"))
		for _, value := range values {
			Expect(filepath.Join(configDir, "applications", "databases", value+".yaml")).To(BeAnExistingFile(),
				"setup.yaml offers %s but there is no install definition for it", value)
		}
	})
})
