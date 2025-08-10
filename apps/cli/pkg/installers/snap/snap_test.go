package snap_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/jameswlane/devex/pkg/installers/snap"
	"github.com/jameswlane/devex/pkg/mocks"
)

var _ = Describe("Snap Installer", func() {
	Describe("New", func() {
		It("creates a new Snap installer instance", func() {
			installer := snap.NewSnapInstaller()
			Expect(installer).ToNot(BeNil())
		})
	})

	Describe("Install", func() {
		It("returns error for unimplemented installer", func() {
			installer := snap.NewSnapInstaller()
			mockRepo := &mocks.MockRepository{}

			err := installer.Install("test-package", mockRepo)
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("Uninstall", func() {
		It("returns error for unimplemented installer", func() {
			installer := snap.NewSnapInstaller()
			mockRepo := &mocks.MockRepository{}

			err := installer.Uninstall("test-package", mockRepo)
			Expect(err).To(HaveOccurred())
		})
	})

	Describe("IsInstalled", func() {
		It("returns error for unimplemented installer", func() {
			installer := snap.NewSnapInstaller()

			_, err := installer.IsInstalled("test-package")
			Expect(err).To(HaveOccurred())
		})
	})
})
