package sdk_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("ExecutableManager plugin filter", func() {
	It("never runs a rejected plugin, for discovery or execution", func() {
		if runtime.GOOS == "windows" {
			Skip("POSIX executable fixtures")
		}
		root := GinkgoT().TempDir()
		marker := filepath.Join(root, "ran")
		source := filepath.Join(root, "source")
		Expect(os.WriteFile(source, []byte(fmt.Sprintf("#!/bin/sh\necho \"$*\" >> %q\nif [ \"$1\" = --plugin-info ]; then\n echo '{\"name\":\"demo\",\"version\":\"1\"}'\nfi\n", marker)), 0o700)).To(Succeed())
		manager := sdk.NewExecutableManager(filepath.Join(root, "plugins"))
		Expect(manager.InstallPlugin(source, "demo")).To(Succeed())
		denied := errors.New("denied by policy")
		manager.SetPluginFilter(func(name string) error {
			if name == "demo" {
				return denied
			}
			return nil
		})

		Expect(manager.DiscoverPluginsWithContext(context.Background())).To(Succeed())
		Expect(manager.ListPluginsWithContext(context.Background())).NotTo(HaveKey("demo"))
		Expect(manager.ExecutePluginWithIO(context.Background(), "demo", []string{"run"}, sdk.OperationIO{})).To(MatchError(denied))
		Expect(marker).NotTo(BeAnExistingFile())

		manager.SetPluginFilter(nil)
		Expect(manager.ListPluginsWithContext(context.Background())).To(HaveKey("demo"))
	})
})
