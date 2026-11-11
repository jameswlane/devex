package stackdetect_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"

	"github.com/jameswlane/devex/apps/cli/internal/stackdetect"
	"github.com/jameswlane/devex/apps/cli/internal/types"
)

type fakeExecutor struct {
	stdout string
	err    error
	args   []string
	plugin string
	run    func(ctx context.Context, streams sdk.OperationIO) error
}

func (f *fakeExecutor) ExecutePluginWithIO(ctx context.Context, pluginName string, args []string, streams sdk.OperationIO) error {
	f.plugin, f.args = pluginName, args
	if f.run != nil {
		return f.run(ctx, streams)
	}
	_, _ = io.WriteString(streams.Stdout, f.stdout)
	return f.err
}

const report = `{"project_path":"/work/demo","technologies":[
 {"name":"Go","category":"Language","confidence":10,"description":"Go","files":["go.mod"]},
 {"name":"React","category":"Framework","confidence":9,"files":["package.json"]},
 {"name":"Node.js","category":"Runtime","confidence":9},
 {"name":"Docker","category":"Containerization","confidence":4},
 {"name":"PHP","category":"Language","confidence":10}
],"dependencies":[{"name":"react","version":"18","type":"direct","source":"package.json"}],
"project_size":{"total_files":3,"code_files":2},"recommendations":["Add tests"],"issues":[],
"summary":{"primary_language":"Go","project_type":"Web Application"}}`

var _ = Describe("Stack detection", func() {
	Describe("ParseReport", func() {
		It("decodes the report", func() {
			parsed, err := stackdetect.ParseReport([]byte(report))
			Expect(err).NotTo(HaveOccurred())
			Expect(parsed.Technologies).To(HaveLen(5))
			Expect(parsed.Summary.PrimaryLanguage).To(Equal("Go"))
			Expect(parsed.ProjectSize.TotalFiles).To(Equal(3))
		})

		It("skips the progress line released plugins print before the JSON and accepts their capitalized keys", func() {
			legacy := "📄 Generating stack report for: .\n" + `{"technologies":[{"Name":"Go","Category":"Language","Confidence":10}],"project_size":{}}`
			parsed, err := stackdetect.ParseReport([]byte(legacy))
			Expect(err).NotTo(HaveOccurred())
			Expect(parsed.Technologies).To(ConsistOf(HaveField("Name", "Go")))
			Expect(parsed.Dependencies).To(BeEmpty())
			Expect(parsed.Issues).NotTo(BeNil())
		})

		It("explains output that is not a JSON report", func() {
			_, err := stackdetect.ParseReport([]byte("🔍 Detecting development stack\nGo\n"))
			Expect(err).To(MatchError(ContainSubstring("did not return a JSON report")))
		})
	})

	It("filters by confidence and category and sorts by confidence", func() {
		parsed, err := stackdetect.ParseReport([]byte(report))
		Expect(err).NotTo(HaveOccurred())

		Expect(stackdetect.Filter(parsed.Technologies, 0.5, "")).To(HaveLen(4))
		Expect(stackdetect.Filter(parsed.Technologies, 0.95, "")).To(HaveExactElements(HaveField("Name", "Go"), HaveField("Name", "PHP")))
		Expect(stackdetect.Filter(parsed.Technologies, 0, "language")).To(HaveLen(2))
		Expect(stackdetect.Filter(parsed.Technologies, 0, "Database")).To(BeEmpty())
	})

	It("rejects a confidence outside 0.0-1.0", func() {
		Expect(stackdetect.ValidateConfidence(-0.1)).To(HaveOccurred())
		Expect(stackdetect.ValidateConfidence(1.5)).To(HaveOccurred())
		Expect(stackdetect.ValidateConfidence(0.7)).To(Succeed())
	})

	It("suggests each catalog application once, skips unmapped technologies and marks configured ones", func() {
		parsed, err := stackdetect.ParseReport([]byte(report))
		Expect(err).NotTo(HaveOccurred())
		catalog := []types.CrossPlatformApp{{Name: "Go", Category: "Programming Languages"}, {Name: "Node.js", Category: "Programming Languages"}, {Name: "Docker"}}

		suggestions := stackdetect.Suggest(parsed.Technologies, catalog, map[string]bool{"go": true})

		Expect(suggestions).To(HaveLen(3))
		Expect(suggestions[0]).To(SatisfyAll(HaveField("App", "Go"), HaveField("Configured", true), HaveField("Confidence", 1.0)))
		Expect(suggestions[1]).To(SatisfyAll(HaveField("App", "Node.js"), HaveField("Technologies", ConsistOf("Node.js", "React")), HaveField("Confidence", 0.9)))
		Expect(suggestions[2].App).To(Equal("Docker"))
	})

	It("skips mapped applications that are not in the catalog", func() {
		suggestions := stackdetect.Suggest([]stackdetect.Technology{{Name: "Rust", Confidence: 10}}, nil, nil)
		Expect(suggestions).To(BeEmpty())
	})

	Describe("Run", func() {
		It("asks the plugin for a JSON report of the directory", func() {
			executor := &fakeExecutor{stdout: report}
			parsed, err := stackdetect.Run(context.Background(), executor, "/work/demo", io.Discard)
			Expect(err).NotTo(HaveOccurred())
			Expect(executor.plugin).To(Equal(stackdetect.PluginName))
			Expect(executor.args).To(Equal([]string{"report", "--format", "json", "/work/demo"}))
			Expect(parsed.ProjectPath).To(Equal("/work/demo"))
		})

		It("reports plugin failures with the directory", func() {
			_, err := stackdetect.Run(context.Background(), &fakeExecutor{err: errors.New("exit status 1")}, "/work/demo", io.Discard)
			Expect(err).To(MatchError(ContainSubstring("failed to analyze /work/demo")))
		})

		It("explains a missing plugin", func() {
			_, err := stackdetect.Run(context.Background(), nil, ".", io.Discard)
			Expect(err).To(MatchError(ContainSubstring("devex plugin install tool-stackdetector")))
		})

		It("stops on cancellation", func() {
			ctx, cancel := context.WithCancel(context.Background())
			executor := &fakeExecutor{run: func(context.Context, sdk.OperationIO) error {
				cancel()
				return errors.New("signal: killed")
			}}
			_, err := stackdetect.Run(ctx, executor, ".", io.Discard)
			Expect(err).To(MatchError(context.Canceled))
		})

		It("refuses oversized reports", func() {
			executor := &fakeExecutor{run: func(_ context.Context, streams sdk.OperationIO) error {
				_, err := streams.Stdout.Write(bytes.Repeat([]byte("x"), stackdetect.MaxReportBytes+1))
				return err
			}}
			_, err := stackdetect.Run(context.Background(), executor, ".", io.Discard)
			Expect(err).To(MatchError(ContainSubstring("exceeds")))
		})
	})

	It("rejects a directory that does not exist or is a file", func() {
		_, err := stackdetect.ResolveDir("/definitely/not/here")
		Expect(err).To(HaveOccurred())
		file := GinkgoT().TempDir() + "/file"
		Expect(writeFile(file)).To(Succeed())
		_, err = stackdetect.ResolveDir(file)
		Expect(err).To(MatchError(ContainSubstring("not a directory")))
		resolved, err := stackdetect.ResolveDir(GinkgoT().TempDir())
		Expect(err).NotTo(HaveOccurred())
		Expect(strings.HasPrefix(resolved, "/")).To(BeTrue())
	})
})

func writeFile(path string) error { return os.WriteFile(path, []byte("x"), 0o600) }
