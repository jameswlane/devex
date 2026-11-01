package setup

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"github.com/jameswlane/devex/apps/cli/internal/bootstrap"
	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Shell setup plugin contract", Ordered, func() {
	var pluginPath string

	BeforeAll(func(ctx SpecContext) {
		_, source, _, ok := runtime.Caller(0)
		Expect(ok).To(BeTrue())
		pluginDir := filepath.Join(filepath.Dir(source), "../../../../../packages/tool-shell")
		pluginPath = filepath.Join(GinkgoT().TempDir(), "devex-plugin-tool-shell")
		cmd := exec.CommandContext(ctx, "go", "build", "-o", pluginPath, ".")
		cmd.Dir = pluginDir
		output, err := cmd.CombinedOutput()
		Expect(err).NotTo(HaveOccurred(), string(output))
	}, NodeTimeout(time.Minute))

	BeforeEach(func() {
		GinkgoT().Setenv("HOME", GinkgoT().TempDir())
		GinkgoT().Setenv("SHELL", "/bin/bash")
	})

	It("calls an advertised command and sends only the selected shell via the SDK protocol", func(ctx SpecContext) {
		output, err := exec.CommandContext(ctx, pluginPath, "--plugin-info").Output()
		Expect(err).NotTo(HaveOccurred())
		var info sdk.PluginInfo
		Expect(json.Unmarshal(output, &info)).To(Succeed())
		cmd, err := shellSetupCommand(ctx, pluginPath, "fish")
		Expect(err).NotTo(HaveOccurred())
		commands := make([]string, 0, len(info.Commands))
		for _, command := range info.Commands {
			commands = append(commands, command.Name)
		}
		Expect(cmd.Args[1:]).To(HaveLen(1))
		Expect(commands).To(ContainElement(cmd.Args[1]))
		var input sdk.PluginSetupInput
		Expect(json.NewDecoder(cmd.Stdin).Decode(&input)).To(Succeed())
		Expect(input.Command).To(Equal(cmd.Args[1]))
		Expect(input.Parameters).To(Equal(map[string]interface{}{"selected_shell": "fish"}))
	})

	It("configures only bash through setup's real plugin discovery and invocation", func(ctx SpecContext) {
		pluginBootstrap, err := bootstrap.NewPluginBootstrap(true)
		Expect(err).NotTo(HaveOccurred())
		Expect(pluginBootstrap.GetManager().InstallPlugin(pluginPath, "tool-shell")).To(Succeed())
		model := &SetupModel{}
		Expect(model.handleShellConfiguration(ctx, pluginBootstrap, "bash")).To(Succeed())
		content, err := os.ReadFile(filepath.Join(os.Getenv("HOME"), ".bashrc"))
		Expect(err).NotTo(HaveOccurred())
		Expect(string(content)).To(ContainSubstring("# DevEx Shell Configuration"))
		Expect(filepath.Join(os.Getenv("HOME"), ".zshrc")).NotTo(BeAnExistingFile())
		Expect(filepath.Join(os.Getenv("HOME"), ".config", "fish")).NotTo(BeADirectory())

		// Re-running setup must also preserve other shells' existing files.
		zshrc := filepath.Join(os.Getenv("HOME"), ".zshrc")
		fishrc := filepath.Join(os.Getenv("HOME"), ".config", "fish", "config.fish")
		Expect(os.MkdirAll(filepath.Dir(fishrc), 0755)).To(Succeed())
		for _, path := range []string{zshrc, fishrc} {
			Expect(os.WriteFile(path, []byte("keep existing configuration\n"), 0644)).To(Succeed())
		}
		Expect(model.handleShellConfiguration(ctx, pluginBootstrap, "bash")).To(Succeed())
		for _, path := range []string{zshrc, fishrc} {
			content, err := os.ReadFile(path)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(content)).To(Equal("keep existing configuration\n"))
		}

		// The summary's recovery command must work without CLI plugin registration.
		model.recordShellConfigurationFailure("bash", errors.New("retry configuration"))
		_, recovery, found := strings.Cut(model.installation.getErrors()[0], "Recover: ")
		Expect(found).To(BeTrue())
		output, err := exec.CommandContext(ctx, "sh", "-c", recovery).CombinedOutput()
		Expect(err).NotTo(HaveOccurred(), string(output))
		Expect(string(output)).To(ContainSubstring(`"status":"success"`))
	})

	It("honors an explicit fish selection even when the current shell is bash", func(ctx SpecContext) {
		Expect(runShellSetup(ctx, pluginPath, "fish")).To(Succeed())
		Expect(filepath.Join(os.Getenv("HOME"), ".config", "fish", "config.fish")).To(BeAnExistingFile())
		Expect(filepath.Join(os.Getenv("HOME"), ".bashrc")).NotTo(BeAnExistingFile())
		Expect(filepath.Join(os.Getenv("HOME"), ".zshrc")).NotTo(BeAnExistingFile())
	})

	It("reports SDK failures even when the plugin exits successfully", func(ctx SpecContext) {
		Expect(os.Mkdir(filepath.Join(os.Getenv("HOME"), ".bashrc"), 0755)).To(Succeed())
		err := runShellSetup(ctx, pluginPath, "bash")
		Expect(err).To(MatchError(ContainSubstring("failed to add configurations")))
		Expect(err.Error()).To(ContainSubstring(".bashrc"))
	})

	It("reports a missing plugin with installation guidance", func(ctx SpecContext) {
		pluginBootstrap, err := bootstrap.NewPluginBootstrap(true)
		Expect(err).NotTo(HaveOccurred())
		Expect((&SetupModel{}).handleShellConfiguration(ctx, pluginBootstrap, "bash")).To(MatchError(ContainSubstring("rerun 'devex setup'")))
	})

	It("propagates cancellation and process launch failures", func() {
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		Expect(runShellSetup(ctx, pluginPath, "bash")).To(MatchError(ContainSubstring("context canceled")))
		Expect(runShellSetup(context.Background(), filepath.Join(GinkgoT().TempDir(), "missing"), "bash")).To(MatchError(ContainSubstring("tool-shell setup failed")))
	})

	It("quotes the recovery input so a single quote cannot break out of it", func(ctx SpecContext) {
		shell := `bash'; echo injected; '`
		model := &SetupModel{}
		model.recordShellConfigurationFailure(shell, errors.New("retry configuration"))
		_, recovery, found := strings.Cut(model.installation.getErrors()[0], "Recover: ")
		Expect(found).To(BeTrue())
		printfPart, _, found := strings.Cut(recovery, " | ")
		Expect(found).To(BeTrue())

		output, err := exec.CommandContext(ctx, "sh", "-c", printfPart).CombinedOutput()
		Expect(err).NotTo(HaveOccurred(), string(output))
		expected, err := json.Marshal(sdk.PluginSetupInput{
			Command:    "setup",
			Parameters: map[string]interface{}{"selected_shell": shell},
		})
		Expect(err).NotTo(HaveOccurred())
		Expect(string(output)).To(Equal(string(expected) + "\n"))
	})

	It("includes the reason and selected-shell recovery command in both final summaries", func() {
		model := &SetupModel{step: StepComplete, system: SystemInfo{shells: []string{"bash"}}}
		model.recordShellConfigurationFailure("bash", errors.New("permission denied"))
		for _, summary := range []string{model.View(), captureAutomatedCompletion(model)} {
			Expect(summary).To(Or(ContainSubstring("completed with 1 errors"), ContainSubstring("Completed with Issues")))
			Expect(summary).To(ContainSubstring("permission denied"))
			Expect(summary).To(ContainSubstring(`"selected_shell":"bash"`))
			dirs, err := sdk.ResolvePaths()
			Expect(err).NotTo(HaveOccurred())
			Expect(summary).To(ContainSubstring("| " + singleQuote(filepath.Join(dirs.Plugins(), "devex-plugin-tool-shell")) + " setup"))
			Expect(summary).NotTo(ContainSubstring("Your environment is configured"))
		}
	})
})

func captureAutomatedCompletion(model *SetupModel) string {
	reader, writer, err := os.Pipe()
	Expect(err).NotTo(HaveOccurred())
	stdout := os.Stdout
	os.Stdout = writer
	defer func() { os.Stdout = stdout }()
	printAutomatedSetupCompletion(model)
	Expect(writer.Close()).To(Succeed())
	output, err := io.ReadAll(reader)
	Expect(err).NotTo(HaveOccurred())
	Expect(reader.Close()).To(Succeed())
	return string(output)
}
