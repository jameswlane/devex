package common_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/jameswlane/devex/apps/cli/internal/common"
)

var _ = Describe("Validation", func() {
	Context("ValidateNonEmpty", func() {
		It("returns an error for an empty value", func() {
			err := common.ValidateNonEmpty("", "field")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("field cannot be empty"))
		})

		It("does not return an error for a non-empty value", func() {
			err := common.ValidateNonEmpty("value", "field")
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Context("ValidateListNotEmpty", func() {
		It("returns an error for an empty list", func() {
			err := common.ValidateListNotEmpty([]string{}, "list")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("list cannot be empty"))
		})

		It("does not return an error for a non-empty list", func() {
			err := common.ValidateListNotEmpty([]string{"item"}, "list")
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Context("ValidateAppConfig", func() {
		It("returns an error if app name is empty", func() {
			err := common.ValidateAppConfig("", "method")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("app name cannot be empty"))
		})

		It("returns an error if method is empty", func() {
			err := common.ValidateAppConfig("app", "")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("install method cannot be empty"))
		})

		It("does not return an error for valid input", func() {
			err := common.ValidateAppConfig("app", "method")
			Expect(err).ToNot(HaveOccurred())
		})
	})
})
