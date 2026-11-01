package main_test

import (
	"context"

	main "github.com/jameswlane/devex/packages/package-manager-mise"
	"github.com/jameswlane/devex/packages/plugin-sdk/sdktest"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Unattended mise execution", func() {
	It("completes all mutations with exact argv and closed stdin", func() {
		recorder := sdktest.NewRecorder(GinkgoT(), "mise", "which")
		sdktest.CloseStdin(GinkgoT())
		GinkgoT().Setenv("DEVEX_UNATTENDED", "1")
		plugin := main.NewMisePlugin()
		GinkgoT().Setenv("MISE_LOCAL", "0")
		Expect(plugin.HandleInstall(context.Background(), []string{"node@22"})).To(Succeed())
		Expect(plugin.HandleRemove(context.Background(), []string{"node@22"})).To(Succeed())
		Expect(plugin.HandleUpdate(context.Background(), nil)).To(Succeed())
		Expect(recorder.Calls("mise")).To(Equal([]sdktest.Call{
			{Args: []string{"install", "--global", "node@22"}},
			{Args: []string{"uninstall", "node@22"}},
			{Args: []string{"plugins", "update"}},
			{Args: []string{"upgrade"}},
		}))
	})
})
