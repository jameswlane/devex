package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestPrivilege(t *testing.T) { RegisterFailHandler(Fail); RunSpecs(t, "Yay privilege suite") }

var _ = Describe("Yay user operations", func() {
	It("declares every operation as user scope", func() {
		for _, command := range NewYayPlugin().Info().Commands {
			Expect(command.Privilege).NotTo(BeNil())
			Expect(command.Privilege.Scope).To(Equal(sdk.ScopeUser))
		}
	})
	It("runs install as the current non-root user and preserves native failure", func() {
		if os.Geteuid() == 0 {
			Skip("requires a non-root runner; root policy is covered by SDK contract tests")
		}
		dir := GinkgoT().TempDir()
		Expect(os.WriteFile(filepath.Join(dir, "yay"), []byte("#!/bin/sh\nexit 42\n"), 0755)).To(Succeed())
		GinkgoT().Setenv("PATH", dir)
		err := NewYayPlugin().handleInstall(context.Background(), []string{"example"})
		Expect(err).To(HaveOccurred())
		Expect(sdk.ExitStatus(err)).To(Equal(42))
	})
})
