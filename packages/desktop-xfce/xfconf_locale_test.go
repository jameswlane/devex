package main

import (
	"os"
	"os/exec"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/jameswlane/devex/packages/plugin-sdk/sdktest"
)

// xfconf-query prints doubles with the locale's decimal separator but parses
// -s values with g_ascii_strtod, so backup and restore pin LC_NUMERIC=C while
// keeping the user's locale for messages and output charset.
var _ = Describe("xfconf-query numeric locale", func() {
	DescribeTable("pins only the numeric category",
		func(environ, expected []string) {
			Expect(xfconfEnvironment(environ)).To(Equal(expected))
		},
		Entry("plain locale", []string{"HOME=/h", "LANG=de_DE.UTF-8"}, []string{"HOME=/h", "LANG=de_DE.UTF-8", "LC_NUMERIC=C"}),
		Entry("explicit numeric category", []string{"LANG=en_US.UTF-8", "LC_NUMERIC=de_DE.UTF-8", "LC_MESSAGES=fr_FR.UTF-8"}, []string{"LANG=en_US.UTF-8", "LC_MESSAGES=fr_FR.UTF-8", "LC_NUMERIC=C"}),
		Entry("LC_ALL override is re-expressed as LANG", []string{"PATH=/bin", "LANG=en_US.UTF-8", "LC_CTYPE=C", "LC_ALL=de_DE.UTF-8", "LANGUAGE=de"}, []string{"PATH=/bin", "LANGUAGE=de", "LANG=de_DE.UTF-8", "LC_NUMERIC=C"}),
		Entry("no locale at all", []string{"HOME=/h"}, []string{"HOME=/h", "LC_NUMERIC=C"}),
	)

	It("reaches the xfconf-query process", func() {
		sdktest.NewRecorder(GinkgoT(), "xfconf-query")
		GinkgoT().Setenv("XDG_CURRENT_DESKTOP", "XFCE")
		GinkgoT().Setenv("LANG", "en_US.UTF-8")
		GinkgoT().Setenv("LC_ALL", "de_DE.UTF-8")
		seen := filepath.Join(GinkgoT().TempDir(), "locale")
		GinkgoT().Setenv("DEVEX_TEST_LOCALE_LOG", seen)
		fake, err := exec.LookPath("xfconf-query")
		Expect(err).NotTo(HaveOccurred())
		script := "#!/bin/sh\nprintf '%s|%s|%s' \"${LC_NUMERIC-}\" \"${LC_ALL-unset}\" \"${LANG-}\" > \"$DEVEX_TEST_LOCALE_LOG\"\nexit 42\n"
		Expect(os.WriteFile(fake, []byte(script), 0o700)).To(Succeed())

		Expect(NewXFCEPlugin().Execute("backup", []string{GinkgoT().TempDir()})).To(MatchError(ContainSubstring("failed to list xfconf channels")))
		Expect(os.ReadFile(seen)).To(BeEquivalentTo("C|unset|de_DE.UTF-8"))
	})
})
