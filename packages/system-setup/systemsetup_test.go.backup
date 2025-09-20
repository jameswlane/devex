package main_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/jameswlane/devex/pkg/mocks"
	"github.com/jameswlane/devex/pkg/systemsetup"
	"github.com/jameswlane/devex/pkg/utils"
)

var _ = Describe("SystemSetup", func() {
	var mockExecutor *mocks.MockCommandExecutor

	BeforeEach(func() {
		mockExecutor = mocks.NewMockCommandExecutor()
		utils.CommandExec = mockExecutor // Replace the real CommandExec with the mock
		mockExecutor.Commands = nil      // Reset the executed commands
	})

	It("runs apt update and installs required packages successfully", func() {
		err := systemsetup.UpdateApt()
		Expect(err).ToNot(HaveOccurred())
		Expect(mockExecutor.Commands).To(ContainElement("sudo apt update -y"))
		Expect(mockExecutor.Commands).To(ContainElement("sudo apt install -y curl git unzip"))
	})

	It("returns an error if apt update fails", func() {
		mockExecutor.FailingCommand = "sudo apt update -y"

		err := systemsetup.UpdateApt()
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("mock shell command failed: sudo apt update -y"))
		Expect(mockExecutor.Commands).To(ContainElement("sudo apt update -y"))
	})

	It("upgrades system packages successfully", func() {
		err := systemsetup.UpgradeSystem()
		Expect(err).ToNot(HaveOccurred())
		Expect(mockExecutor.Commands).To(ContainElement("sudo apt upgrade -y"))
	})

	It("returns an error if upgrading packages fails", func() {
		mockExecutor.FailingCommand = "sudo apt upgrade -y"

		err := systemsetup.UpgradeSystem()
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("mock shell command failed: sudo apt upgrade -y"))
		Expect(mockExecutor.Commands).To(ContainElement("sudo apt upgrade -y"))
	})

	It("disables sleep settings successfully", func() {
		err := systemsetup.DisableSleepSettings()
		Expect(err).ToNot(HaveOccurred())
		Expect(mockExecutor.Commands).To(ContainElement("gsettings set org.gnome.desktop.screensaver lock-enabled false"))
		Expect(mockExecutor.Commands).To(ContainElement("gsettings set org.gnome.desktop.session idle-delay 0"))
	})

	It("returns an error if disabling sleep settings fails", func() {
		mockExecutor.FailingCommand = "gsettings set org.gnome.desktop.screensaver lock-enabled false"

		err := systemsetup.DisableSleepSettings()
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("mock shell command failed: gsettings set org.gnome.desktop.screensaver lock-enabled false"))
		Expect(mockExecutor.Commands).To(ContainElement("gsettings set org.gnome.desktop.screensaver lock-enabled false"))
	})

	It("reverts sleep settings successfully", func() {
		err := systemsetup.RevertSleepSettings()
		Expect(err).ToNot(HaveOccurred())
		Expect(mockExecutor.Commands).To(ContainElement("gsettings set org.gnome.desktop.screensaver lock-enabled true"))
		Expect(mockExecutor.Commands).To(ContainElement("gsettings set org.gnome.desktop.session idle-delay 300"))
	})

	It("returns an error if reverting sleep settings fails", func() {
		mockExecutor.FailingCommand = "gsettings set org.gnome.desktop.screensaver lock-enabled true"

		err := systemsetup.RevertSleepSettings()
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("mock shell command failed: gsettings set org.gnome.desktop.screensaver lock-enabled true"))
		Expect(mockExecutor.Commands).To(ContainElement("gsettings set org.gnome.desktop.screensaver lock-enabled true"))
	})

	It("logs out successfully", func() {
		err := systemsetup.Logout()
		Expect(err).ToNot(HaveOccurred())
		Expect(mockExecutor.Commands).To(ContainElement("gnome-session-quit --logout --no-prompt"))
	})

	It("returns an error if logout fails", func() {
		mockExecutor.FailingCommand = "gnome-session-quit --logout --no-prompt"

		err := systemsetup.Logout()
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("mock shell command failed: gnome-session-quit --logout --no-prompt"))
		Expect(mockExecutor.Commands).To(ContainElement("gnome-session-quit --logout --no-prompt"))
	})
})
