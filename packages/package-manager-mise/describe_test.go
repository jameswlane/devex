package main_test

import (
	"context"
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	main "github.com/jameswlane/devex/packages/package-manager-mise"
	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
	"github.com/jameswlane/devex/packages/plugin-sdk/sdktest"
)

var _ = Describe("describe ensure-installed", func() {
	var (
		plugin   *main.MisePlugin
		recorder *sdktest.Recorder
		bashrc   string
	)

	BeforeEach(func() {
		recorder = sdktest.NewRecorder(GinkgoT(), "which", "sh", "curl")
		plugin = main.NewMisePlugin()
		plugin.InitLogger(sdk.NewDefaultLogger(true))
		bash := filepath.Join(GinkgoT().TempDir(), "bash")
		Expect(os.WriteFile(bash, []byte("#!/bin/sh\nexit 0\n"), 0o755)).To(Succeed())
		GinkgoT().Setenv("SHELL", bash)
		GinkgoT().Setenv("MISE_INSTALL_PATH", "")
		bashrc = filepath.Join(os.Getenv("HOME"), ".bashrc")
	})

	It("lists the verified download, the installer and the rc change without making them", func() {
		recorder.Script("which", []string{"mise"}, sdktest.Response{ExitCode: 1})

		description, err := plugin.Describe(context.Background(), "ensure-installed", nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(description.Validate()).To(Succeed())
		Expect(description.AlreadySatisfied).To(BeFalse())
		Expect(description.Downloads).To(Equal([]sdk.PlannedDownload{{URL: sdk.MiseInstallScriptURL, Verification: sdk.MiseInstallVerification()}}))
		Expect(description.Actions).To(Equal([]sdk.PlannedAction{
			{Kind: sdk.ActionShell, Target: "sh <verified mise installer>", Scope: sdk.ScopeUser, Detail: "installs mise to ~/.local/bin/mise"},
			{Kind: sdk.ActionFile, Target: bashrc, Scope: sdk.ScopeUser, Detail: "append mise activation"},
		}))

		Expect(recorder.Calls("which")).To(HaveLen(1))
		Expect(recorder.Calls("sh")).To(BeEmpty())
		Expect(recorder.Calls("curl")).To(BeEmpty())
		Expect(bashrc).NotTo(BeAnExistingFile())
	})

	It("shows the MISE_INSTALL_PATH destination the installer will use", func() {
		recorder.Script("which", []string{"mise"}, sdktest.Response{ExitCode: 1})
		GinkgoT().Setenv("MISE_INSTALL_PATH", "/opt/tools/mise")

		description, err := plugin.Describe(context.Background(), "ensure-installed", nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(description.Actions[0].Detail).To(Equal("installs mise to /opt/tools/mise"))
	})

	It("lists only the rc change when mise is installed but not activated", func() {
		description, err := plugin.Describe(context.Background(), "ensure-installed", nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(description.Downloads).To(BeEmpty())
		Expect(description.Actions).To(Equal([]sdk.PlannedAction{
			{Kind: sdk.ActionFile, Target: bashrc, Scope: sdk.ScopeUser, Detail: "append mise activation"},
		}))
		Expect(bashrc).NotTo(BeAnExistingFile())
	})

	It("is already satisfied when mise is installed and activated", func() {
		Expect(os.WriteFile(bashrc, []byte("eval \"$(mise activate bash)\"\n"), 0o644)).To(Succeed())

		description, err := plugin.Describe(context.Background(), "ensure-installed", nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(description.AlreadySatisfied).To(BeTrue())
		Expect(description.Actions).To(BeEmpty())
	})

	It("notes an unsupported shell instead of failing", func() {
		tcsh := filepath.Join(GinkgoT().TempDir(), "tcsh")
		Expect(os.WriteFile(tcsh, []byte("#!/bin/sh\nexit 0\n"), 0o755)).To(Succeed())
		GinkgoT().Setenv("SHELL", tcsh)

		description, err := plugin.Describe(context.Background(), "ensure-installed", nil)
		Expect(err).NotTo(HaveOccurred())
		Expect(description.AlreadySatisfied).To(BeTrue())
		Expect(description.Notes).To(ConsistOf(ContainSubstring("unsupported shell: tcsh")))
	})

	It("does not describe other commands", func() {
		_, err := plugin.Describe(context.Background(), "install", []string{"node@lts"})
		Expect(err).To(MatchError(sdk.ErrDescribeUnsupported))
		Expect(recorder.Calls("which")).To(BeEmpty())
	})
})
