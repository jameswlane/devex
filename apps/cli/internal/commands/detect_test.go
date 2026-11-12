package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/viper"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"

	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/mocks"
	"github.com/jameswlane/devex/apps/cli/internal/stackdetect"
	"github.com/jameswlane/devex/apps/cli/internal/types"
)

type fakeStackDetector struct {
	report string
	err    error
	calls  int
}

func (f *fakeStackDetector) ExecutePluginWithIO(_ context.Context, _ string, _ []string, streams sdk.OperationIO) error {
	f.calls++
	_, _ = io.WriteString(streams.Stdout, f.report)
	return f.err
}

const detectReport = `{"project_path":"/work/demo","technologies":[
 {"name":"Go","category":"Language","confidence":10,"files":["go.mod"]},
 {"name":"React","category":"Framework","confidence":9},
 {"name":"Docker","category":"Containerization","confidence":3}
],"dependencies":[{"name":"react"}],"project_size":{"total_files":4,"code_files":2},
"recommendations":["Add a CI workflow"],"issues":["No tests found"],
"summary":{"primary_language":"Go","project_type":"Web Application"}}`

var _ = Describe("Detect commands", func() {
	var (
		settings            config.CrossPlatformSettings
		detector            *fakeStackDetector
		executorErr         error
		output, diagnostics bytes.Buffer
		project             string
	)
	app := func(name string) types.CrossPlatformApp {
		return types.CrossPlatformApp{Name: name, Description: name + " runtime", Category: "development", Linux: types.OSConfig{InstallMethod: "apt", InstallCommand: strings.ToLower(strings.ReplaceAll(name, ".", ""))}}
	}

	BeforeEach(func() {
		viper.Reset()
		DeferCleanup(viper.Reset)
		home := GinkgoT().TempDir()
		GinkgoT().Setenv("HOME", home)
		GinkgoT().Setenv("DEVEX_TEAM_CONFIG_DIR", "")
		GinkgoT().Setenv("DEVEX_ENV", "dev")
		GinkgoT().Setenv("CI", "true")
		settings = config.CrossPlatformSettings{HomeDir: home, ApplicationOverrides: []types.CrossPlatformApp{app("Go"), app("Node.js"), app("Docker")}}
		project = GinkgoT().TempDir()
		detector = &fakeStackDetector{report: detectReport}
		executorErr = nil
		previous := newStackDetectExecutor
		newStackDetectExecutor = func(context.Context) (stackdetect.Executor, error) {
			if executorErr != nil {
				return nil, executorErr
			}
			return detector, nil
		}
		DeferCleanup(func() { newStackDetectExecutor = previous })
		output.Reset()
		diagnostics.Reset()
	})

	execute := func(args ...string) error {
		cmd := NewDetectCmd(mocks.NewMockRepository(), settings)
		cmd.SetOut(&output)
		cmd.SetErr(&diagnostics)
		cmd.SetIn(strings.NewReader("n\n"))
		cmd.SetArgs(args)
		return cmd.ExecuteContext(context.Background())
	}
	configured := func() []string {
		apps, err := config.LoadApplicationConfigs(settings.GetConfigDir())
		Expect(err).NotTo(HaveOccurred())
		names := make([]string, 0, len(apps))
		for _, configuredApp := range apps {
			names = append(names, configuredApp.Name)
		}
		return names
	}

	It("is recognized so root startup skips other plugins", func() {
		root := NewRootCmd("test", mocks.NewMockRepository(), settings)
		suggest, _, err := root.Find([]string{"detect", "suggest"})
		Expect(err).NotTo(HaveOccurred())
		Expect(detectCommand(suggest)).To(BeTrue())
		Expect(requiresCatalog(suggest)).To(BeTrue())
		list, _, err := root.Find([]string{"list"})
		Expect(err).NotTo(HaveOccurred())
		Expect(detectCommand(list)).To(BeFalse())
	})

	It("reports technologies above the confidence threshold", func() {
		Expect(execute(project)).To(Succeed())
		Expect(output.String()).To(ContainSubstring("Go"))
		Expect(output.String()).To(ContainSubstring("React"))
		Expect(output.String()).NotTo(ContainSubstring("Docker"))
		Expect(output.String()).To(ContainSubstring("Primary language: Go"))
	})

	It("filters by category and writes machine-readable JSON", func() {
		Expect(execute(project, "--category", "Language", "--format", "json")).To(Succeed())
		var result struct {
			Technologies []stackdetect.Technology `json:"technologies"`
		}
		Expect(json.Unmarshal(output.Bytes(), &result)).To(Succeed())
		Expect(result.Technologies).To(ConsistOf(HaveField("Name", "Go")))
	})

	It("saves the result to a file", func() {
		path := GinkgoT().TempDir() + "/detect.json"
		Expect(execute(project, "--output", path, "--confidence", "0")).To(Succeed())
		data, err := os.ReadFile(path)
		Expect(err).NotTo(HaveOccurred())
		Expect(string(data)).To(ContainSubstring(`"Docker"`))
	})

	It("rejects invalid options before running the plugin", func() {
		Expect(execute(project, "--confidence", "2")).To(MatchError(ContainSubstring("between 0.0 and 1.0")))
		Expect(execute(project, "--format", "yaml")).To(MatchError(ContainSubstring("unsupported format")))
		Expect(execute("/definitely/not/a/project")).To(HaveOccurred())
		Expect(detector.calls).To(BeZero())
	})

	It("surfaces plugin and installation failures", func() {
		detector.err = errors.New("exit status 2")
		Expect(execute(project)).To(MatchError(ContainSubstring("tool-stackdetector failed to analyze")))
		executorErr = errors.New("plugin tool-stackdetector is not installed and downloads are disabled by --offline")
		Expect(execute(project)).To(MatchError(ContainSubstring("--offline")))
	})

	It("analyzes dependencies, size, issues and recommendations", func() {
		Expect(execute("analyze", project)).To(Succeed())
		Expect(output.String()).To(ContainSubstring("Dependencies: 1"))
		Expect(output.String()).To(ContainSubstring("No tests found"))
		Expect(output.String()).To(ContainSubstring("Add a CI workflow"))
	})

	It("suggests catalog applications and marks configured ones", func() {
		Expect(execute("suggest", project, "--format", "json")).To(Succeed())
		var suggestions []stackdetect.Suggestion
		Expect(json.Unmarshal(output.Bytes(), &suggestions)).To(Succeed())
		Expect(suggestions).To(HaveExactElements(HaveField("App", "Go"), HaveField("App", "Node.js")))
		Expect(suggestions[0].Configured).To(BeFalse())
	})

	It("previews additions with --dry-run without changing configuration", func() {
		Expect(execute("apply", project, "--dry-run")).To(Succeed())
		Expect(output.String()).To(ContainSubstring("Would add: Go, Node.js"))
		Expect(configured()).To(BeEmpty())
	})

	Context("when the suggestions need planning", func() {
		dryRunJSON := func() DetectApplyResult {
			Expect(execute("apply", project, "--dry-run", "--format", "json")).To(Succeed())
			var result DetectApplyResult
			Expect(json.Unmarshal(output.Bytes(), &result)).To(Succeed())
			output.Reset()
			return result
		}
		unchanged := func() {
			files, err := config.ConfigurationFiles(settings.GetConfigDir())
			if !os.IsNotExist(err) {
				Expect(err).NotTo(HaveOccurred())
			}
			Expect(files).To(BeEmpty())
		}

		It("previews catalog dependencies exactly as --yes adds them", func() {
			settings.ApplicationOverrides = append(settings.ApplicationOverrides, app("Build Essential"))
			settings.ApplicationOverrides[0].Linux.Dependencies = []string{"Build Essential"}
			preview := dryRunJSON()
			Expect(preview.Added).To(Equal([]string{"Build Essential", "Go", "Node.js"}))
			Expect(diagnostics.String()).To(ContainSubstring("Configuration preview: add Build Essential, Go, Node.js"))
			unchanged()

			Expect(execute("apply", project, "--yes", "--format", "json")).To(Succeed())
			var result DetectApplyResult
			Expect(json.Unmarshal(output.Bytes(), &result)).To(Succeed())
			Expect(result.Added).To(Equal(preview.Added))
			Expect(configured()).To(ConsistOf("Build Essential", "Go", "Node.js"))
		})

		It("plans a dependency shared by several suggestions once", func() {
			settings.ApplicationOverrides = append(settings.ApplicationOverrides, app("git"))
			settings.ApplicationOverrides[0].Linux.Dependencies = []string{"git"}
			settings.ApplicationOverrides[1].Linux.Dependencies = []string{"git"}
			Expect(dryRunJSON().Added).To(Equal([]string{"git", "Go", "Node.js"}))

			Expect(execute("apply", project, "--yes", "--format", "json")).To(Succeed())
			var result DetectApplyResult
			Expect(json.Unmarshal(output.Bytes(), &result)).To(Succeed())
			Expect(result.Added).To(Equal([]string{"git", "Go", "Node.js"}))
			Expect(result.Skipped).To(BeEmpty())
			Expect(configured()).To(ConsistOf("git", "Go", "Node.js"))
		})

		It("fails the dry run on a configuration path collision", func() {
			path, err := applicationConfigPath(settings.GetConfigDir(), "applications", "development", "Go")
			Expect(err).NotTo(HaveOccurred())
			Expect(writeNewApplicationConfig(app("Other"), path)).To(Succeed())
			Expect(execute("apply", project, "--dry-run")).To(MatchError(ContainSubstring("already exists")))
			Expect(output.String()).NotTo(ContainSubstring("Would add"))
			Expect(configured()).To(ConsistOf("Other"))
		})

		It("fails the dry run on an invalid definition", func() {
			settings.ApplicationOverrides[1].Linux.InstallCommand = ""
			Expect(execute("apply", project, "--dry-run")).To(MatchError(ContainSubstring("linux.install_command")))
			unchanged()
		})

		It("validates the suggestions together before adding any of them", func() {
			settings.ApplicationOverrides[1].Linux.Conflicts = []string{"Go"}
			Expect(execute("apply", project, "--dry-run")).To(MatchError(ContainSubstring("remove one application")))
			Expect(execute("apply", project, "--yes")).To(MatchError(ContainSubstring("remove one application")))
			unchanged()
		})
	})

	It("requires --yes without a terminal and changes nothing", func() {
		Expect(execute("apply", project)).To(MatchError(ContainSubstring("--yes")))
		Expect(configured()).To(BeEmpty())
	})

	It("adds suggestions with --yes, backs up first, and skips them on the next run", func() {
		Expect(execute("apply", project, "--yes", "--format", "json")).To(Succeed())
		var result DetectApplyResult
		Expect(json.Unmarshal(output.Bytes(), &result)).To(Succeed())
		Expect(result.Added).To(ConsistOf("Go", "Node.js"))
		Expect(configured()).To(ConsistOf("Go", "Node.js"))

		output.Reset()
		Expect(execute("apply", project, "--yes")).To(Succeed())
		Expect(output.String()).To(ContainSubstring("Already configured: Go, Node.js"))
		Expect(diagnostics.String()).To(ContainSubstring("Nothing to add"))
	})

	It("reports no change when nothing matches", func() {
		detector.report = `{"technologies":[{"name":"PHP","category":"Language","confidence":10}]}`
		Expect(execute("apply", project, "--yes")).To(Succeed())
		Expect(diagnostics.String()).To(ContainSubstring("Nothing to add"))
		Expect(configured()).To(BeEmpty())
	})

	It("stops when cancelled", func() {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		cmd := NewDetectCmd(mocks.NewMockRepository(), settings)
		cmd.SetOut(&output)
		cmd.SetErr(&diagnostics)
		cmd.SetArgs([]string{project})
		Expect(cmd.ExecuteContext(ctx)).To(MatchError(context.Canceled))
	})
})
