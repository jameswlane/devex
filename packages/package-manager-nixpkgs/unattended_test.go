package main_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	main "github.com/jameswlane/devex/packages/package-manager-nixpkgs"
	"github.com/jameswlane/devex/packages/plugin-sdk/sdktest"
)

var _ = Describe("Unattended NIXPKGS execution", func() {
	It("completes every command with exact argv, closed stdin and no sudo", func() {
		recorder := sdktest.NewRecorder(GinkgoT(), "nix-env", "nix-channel")
		sdktest.CloseStdin(GinkgoT())
		GinkgoT().Setenv("CI", "true")
		plugin := main.NewNixpkgsPlugin()
		for _, contract := range contracts {
			Expect(plugin.Execute(contract.command, contract.args)).To(Succeed(), contract.command)
		}
		for _, contract := range contracts {
			Expect(recorder.Calls(executableFor(contract))).To(ContainElement(
				sdktest.Call{Args: contract.nativeArgv}), contract.command)
		}
		// nix-env and nix-channel never prompt and act on the user's profile,
		// so unattended mode needs neither a confirmation flag nor sudo.
		Expect(recorder.Calls("sudo")).To(BeEmpty())
	})
})
