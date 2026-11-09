package main_test

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"os/exec"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"

	main "github.com/jameswlane/devex/packages/package-manager-curlpipe"
	"github.com/jameswlane/devex/packages/plugin-sdk/sdktest"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Script integrity before execution", func() {

	var recorder *sdktest.Recorder
	BeforeEach(func() {
		recorder = sdktest.NewRecorder(GinkgoT(), "bash", "curl")
		GinkgoT().Setenv("DEVEX_UNATTENDED", "1")
		original := sdk.NewCommandContext
		DeferCleanup(func() { sdk.NewCommandContext = original })
		sdk.NewCommandContext = func(ctx context.Context, name string, args ...string) *exec.Cmd {
			if name == "curl" {
				Expect(os.WriteFile(args[2], []byte("echo fixture\n"), 0600)).To(Succeed())
			}
			return original(ctx, name, args...)
		}
	})

	It("rejects a mismatched digest before requesting script execution", func() {
		err := main.NewCurlpipePlugin().Execute("install", []string{"--sha256", fmt.Sprintf("%x", sha256.Sum256([]byte("other"))), "https://get.docker.com"})
		Expect(err).To(MatchError(ContainSubstring("checksum mismatch")))
		Expect(recorder.Calls("bash")).To(BeEmpty())
	})
	It("accepts a matching digest", func() {
		Expect(main.NewCurlpipePlugin().Execute("install", []string{"--sha256", fmt.Sprintf("%x", sha256.Sum256([]byte("echo fixture\n"))), "https://get.docker.com"})).To(Succeed())
		Expect(recorder.Calls("bash")).To(HaveLen(1))
	})
})
