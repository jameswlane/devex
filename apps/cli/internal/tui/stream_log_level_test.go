//go:build linux

package tui

import (
	"bytes"
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/log"
	"github.com/jameswlane/devex/apps/cli/internal/mocks"
	"github.com/jameswlane/devex/packages/plugin-sdk/sdktest"
)

var _ = Describe("Native command output logging", func() {
	It("logs a successful command's stderr progress at info level, not error", func() {
		recorder := sdktest.NewRecorder(GinkgoT(), "tool")
		recorder.Script("tool", []string{"--install"}, sdktest.Response{Stderr: "installing tool...\n1.1%\n"})
		var buffer bytes.Buffer
		log.InitDefaultLogger(&buffer)
		DeferCleanup(log.InitTestLogger)

		installer := NewStreamingInstallerWithExecutor(nil, mocks.NewMockRepository(), context.Background(), NewDefaultCommandExecutor(), config.CrossPlatformSettings{})
		Expect(installer.executeCommandStream(context.Background(), "tool --install")).To(Succeed())

		output := buffer.String()
		Expect(output).To(ContainSubstring("installing tool..."))
		Expect(output).To(ContainSubstring("stream=stderr"))
		Expect(output).NotTo(ContainSubstring("ERRO"))
	})
})
