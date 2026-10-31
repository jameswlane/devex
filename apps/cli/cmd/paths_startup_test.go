package main

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Startup migration preview policy", func() {
	It("honors environment and explicit true/false flags, stopping at the argument boundary", func() {
		GinkgoT().Setenv("DEVEX_DRY_RUN", "true")
		Expect(migrationDryRun(nil)).To(BeTrue())
		Expect(migrationDryRun([]string{"--dry-run=false", "paths"})).To(BeFalse())
		GinkgoT().Setenv("DEVEX_DRY_RUN", "")
		Expect(migrationDryRun([]string{"paths", "--dry-run"})).To(BeTrue())
		Expect(migrationDryRun([]string{"--", "--dry-run"})).To(BeFalse())
	})
})
