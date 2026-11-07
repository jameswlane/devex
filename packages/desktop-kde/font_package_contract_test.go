package main

import (
	"context"

	"github.com/jameswlane/devex/packages/plugin-sdk/sdktest"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Font package operands", func() {
	for _, manager := range []string{"apt-get", "dnf", "pacman", "zypper"} {
		It("rejects option operands before invoking "+manager, func() {
			recorder := sdktest.NewRecorder(GinkgoT(), manager)
			err := NewFontManager().InstallFonts(context.Background(), []string{"hack", "--remove"})
			Expect(err).To(MatchError(ContainSubstring("must not start with '-'")))
			Expect(recorder.Calls(manager)).To(BeEmpty())
			Expect(recorder.Calls("sudo")).To(BeEmpty())
		})
	}
	It("rejects apt removal operands before any package installation", func() {
		recorder := sdktest.NewRecorder(GinkgoT(), "apt-get")
		err := NewFontManager().InstallFonts(context.Background(), []string{"hack", "fonts-custom-"})
		Expect(err).To(MatchError(ContainSubstring("must not end with '-'")))
		Expect(recorder.Calls("apt-get")).To(BeEmpty())
		Expect(recorder.Calls("sudo")).To(BeEmpty())
	})
	It("preserves valid mapped and explicit apt package operands", func() {
		recorder := sdktest.NewRecorder(GinkgoT(), "apt-get")
		Expect(NewFontManager().InstallFonts(context.Background(), []string{"hack", "fonts-custom"})).To(Succeed())
		Expect(recorder.Calls("apt-get")).To(Equal([]sdktest.Call{{Args: []string{"install", "-y", "fonts-hack", "fonts-custom"}, ElevationRequested: true}}))
	})
})
