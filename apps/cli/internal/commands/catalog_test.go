package commands

import (
	"github.com/jameswlane/devex/apps/cli/internal/config"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/spf13/cobra"
)

var _ = Describe("Catalog opt-ins", func() {
	It("inherits a group's opt-in without inheriting the root setup opt-in", func() {
		root := &cobra.Command{Use: "root", Annotations: map[string]string{catalogAnnotation: catalogRequired}}
		unrelated := &cobra.Command{Use: "install"}
		group := &cobra.Command{Use: "renamed-group", Annotations: map[string]string{catalogAnnotation: catalogRequired}}
		child := &cobra.Command{Use: "child"}
		root.AddCommand(unrelated, group)
		group.AddCommand(child)
		Expect(requiresCatalog(root)).To(BeTrue())
		Expect(requiresCatalog(unrelated)).To(BeFalse())
		Expect(requiresCatalog(group)).To(BeTrue())
		Expect(requiresCatalog(child)).To(BeTrue())
	})

	It("loads the catalog for template plans, whose steps name catalog applications", func() {
		Expect(requiresCatalog(newTemplatePlanCmd(config.CrossPlatformSettings{}, nil))).To(BeTrue())
	})
})
