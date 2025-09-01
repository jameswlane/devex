package main_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/jameswlane/devex/pkg/gitconfig"
	"github.com/jameswlane/devex/pkg/mocks"
	"github.com/jameswlane/devex/pkg/utils"
)

var _ = Describe("GitConfig", func() {
	var mockUtils *mocks.MockUtils

	BeforeEach(func() {
		mockUtils = mocks.NewMockUtils()
		utils.CommandExec = mockUtils // Replace the real CommandExec with the mock
	})

	Describe("ApplyGitConfig", func() {
		It("applies Git aliases and settings successfully", func() {
			config := &gitconfig.GitConfig{
				Aliases: map[string]string{
					"co": "checkout",
					"br": "branch",
				},
				Settings: map[string]string{
					"user.name":  "Test User",
					"user.email": "test@example.com",
				},
			}

			err := gitconfig.ApplyGitConfig(config)
			Expect(err).ToNot(HaveOccurred())

			// Verify commands executed by the mock
			Expect(mockUtils.Commands).To(ContainElements(
				"git config --global alias.co checkout",
				"git config --global alias.br branch",
				"git config --global user.name Test User",
				"git config --global user.email test@example.com",
			))
		})

		It("returns an error if applying an alias fails", func() {
			// Simulate failure for a specific command
			mockUtils.FailCommand("git config --global alias.co checkout")

			config := &gitconfig.GitConfig{
				Aliases: map[string]string{
					"co": "checkout",
				},
				Settings: map[string]string{},
			}

			err := gitconfig.ApplyGitConfig(config)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to set git alias co"))
		})

		It("returns an error if applying a setting fails", func() {
			mockUtils.FailCommand("git config --global user.name Test User")

			config := &gitconfig.GitConfig{
				Aliases: map[string]string{},
				Settings: map[string]string{
					"user.name": "Test User",
				},
			}

			err := gitconfig.ApplyGitConfig(config)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to apply settings"))
			Expect(err.Error()).To(ContainSubstring("failed to set git configuration user.name"))
		})
	})
})
