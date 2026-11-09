package setup

import (
	"context"
	"errors"

	"github.com/jameswlane/devex/apps/cli/internal/platform"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("SSH choice in Git setup", func() {
	var model *SetupModel
	BeforeEach(func() {
		model = &SetupModel{step: StepGitConfig, cursor: 2, system: SystemInfo{detectedPlatform: platform.DetectionResult{OS: "linux"}}, git: GitConfiguration{gitFullName: "Jane", gitEmail: "jane@example.com", existingName: "Jane", existingEmail: "jane@example.com"}}
	})
	It("defaults to no SSH work and cycles explicit choices", func() {
		Expect(model.renderSSHPreview()).To(ContainSubstring("skipped"))
		model.handleEnter()
		Expect(model.git.sshMode).To(Equal("existing"))
		model.handleEnter()
		Expect(model.git.sshMode).To(Equal("generate"))
		model.handleEnter()
		Expect(model.git.sshMode).To(BeEmpty())
	})
	It("previews generated paths, lack of passphrase and permissions", func() {
		model.git.sshMode = "generate"
		model.git.sshPath = "/home/jane/.ssh/devex"
		preview := model.renderSSHPreview()
		Expect(preview).To(ContainSubstring("UNENCRYPTED"))
		Expect(preview).To(ContainSubstring(model.git.sshPath + ".pub"))
		Expect(preview).To(ContainSubstring("0600"))
	})
	It("runs only opted-in SSH when identity is unchanged", func() {
		model.git.sshMode = "generate"
		model.git.sshPath = "/home/jane/.ssh/devex"
		executor := &consentExecutor{}
		Expect(model.applyGitConfiguration(context.Background(), executor, "Jane", "jane@example.com")).To(Succeed())
		Expect(executor.calls).To(Equal([][]string{{"ssh", "--mode", "generate", "--path", model.git.sshPath, "--apply"}}))
	})
	It("does no SSH work by default", func() {
		executor := &consentExecutor{}
		Expect(model.applyGitConfiguration(context.Background(), executor, "Jane", "jane@example.com")).To(Succeed())
		Expect(executor.calls).To(BeEmpty())
	})
	It("propagates plugin failures and rejects invalid paths before execution", func() {
		model.git.sshMode = "existing"
		model.git.sshPath = "relative"
		executor := &consentExecutor{fail: errors.New("key unreadable")}
		Expect(model.applyGitConfiguration(context.Background(), executor, "Jane", "jane@example.com")).NotTo(Succeed())
		Expect(executor.calls).To(BeEmpty())
		model.git.sshPath = "/home/jane/.ssh/devex"
		Expect(model.applyGitConfiguration(context.Background(), executor, "Jane", "jane@example.com")).To(MatchError(ContainSubstring("key unreadable")))
	})
})
