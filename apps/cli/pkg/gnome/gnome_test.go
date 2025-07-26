package gnome_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/jameswlane/devex/pkg/fs"
	"github.com/jameswlane/devex/pkg/gnome"
	"github.com/jameswlane/devex/pkg/mocks"
	"github.com/jameswlane/devex/pkg/utils"
)

var _ = Describe("SetBackground", func() {
	It("sets the desktop background successfully", func() {
		mockExecutor := &mocks.MockCommandExecutor{}
		utils.CommandExec = mockExecutor

		err := gnome.SetBackground("/tmp/background.jpg")
		Expect(err).To(BeNil())
		Expect(mockExecutor.Commands).To(ContainElement(
			`gsettings set org.gnome.desktop.background picture-uri "file:///tmp/background.jpg"`,
		))
	})

	It("returns an error when the command fails", func() {
		mockExecutor := &mocks.MockCommandExecutor{FailingCommand: `gsettings set org.gnome.desktop.background picture-uri "file:///tmp/background.jpg"`}
		utils.CommandExec = mockExecutor

		err := gnome.SetBackground("/tmp/background.jpg")
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("mock shell command failed"))
	})
})

var _ = Describe("SetFavoriteApps", func() {
	var mockExecutor *mocks.MockCommandExecutor

	BeforeEach(func() {
		mockExecutor = mocks.NewMockCommandExecutor()
		utils.CommandExec = mockExecutor // Mock the entire CommandExec
	})

	It("sets favorite apps successfully", func() {
		fs.UseMemMapFs()

		// Create mock desktop files
		err := fs.WriteFile("/usr/share/applications/app1.desktop", []byte{}, 0o644)
		if err != nil {
			return
		}
		err = fs.WriteFile("/usr/share/applications/app2.desktop", []byte{}, 0o644)
		if err != nil {
			return
		}

		config := gnome.Config{
			Favorites: []gnome.App{
				{Name: "App1", DesktopFile: "app1.desktop"},
				{Name: "App2", DesktopFile: "app2.desktop"},
			},
		}

		err = gnome.SetFavoriteApps(config)
		Expect(err).ToNot(HaveOccurred())
		Expect(mockExecutor.Commands).To(ContainElement("gsettings set org.gnome.shell favorite-apps ['app1.desktop','app2.desktop']"))
	})

	It("returns an error when no valid apps are found", func() {
		config := gnome.Config{
			Favorites: []gnome.App{
				{Name: "App1", DesktopFile: "nonexistent.desktop"},
			},
		}

		err := gnome.SetFavoriteApps(config)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("validation failed"))
	})
})

var _ = Describe("InstallGnomeExtension", func() {
	var mockExecutor *mocks.MockCommandExecutor

	BeforeEach(func() {
		mockExecutor = mocks.NewMockCommandExecutor()
		utils.CommandExec = mockExecutor // Mock the entire CommandExec
	})

	It("installs a GNOME extension successfully", func() {
		fs.UseMemMapFs()

		extension := gnome.GnomeExtension{
			ID: "test-extension",
			SchemaFiles: []gnome.SchemaFile{
				{Source: "/tmp/source", Destination: "/tmp/destination"},
			},
		}

		err := fs.WriteFile("/tmp/source", []byte("schema data"), 0o644)
		if err != nil {
			return
		}
		err = gnome.InstallGnomeExtension(extension)
		Expect(err).ToNot(HaveOccurred())
		Expect(mockExecutor.Commands).To(ContainElement("gext install test-extension"))
	})

	It("returns an error when schema file copying fails", func() {
		extension := gnome.GnomeExtension{
			ID: "test-extension",
			SchemaFiles: []gnome.SchemaFile{
				{Source: "/nonexistent/source", Destination: "/tmp/destination"},
			},
		}

		err := gnome.InstallGnomeExtension(extension)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("failed to copy schema file"))
	})
})

var _ = Describe("LoadGnomeExtensions", func() {
	It("loads GNOME extensions from a valid YAML file", func() {
		fs.UseMemMapFs() // Ensure in-memory FS

		extensionsYAML := `
- id: extension1
  schema_files:
    - source: /tmp/source1
      destination: /tmp/destination1
`
		filename := "/tmp/extensions.yaml"
		err := fs.WriteFile(filename, []byte(extensionsYAML), 0o644)
		if err != nil {
			return
		}

		extensions, err := gnome.LoadGnomeExtensions(filename)
		Expect(err).ToNot(HaveOccurred())
		Expect(extensions).To(HaveLen(1))
		Expect(extensions[0].ID).To(Equal("extension1"))
	})

	It("returns an error for invalid YAML", func() {
		fs.UseMemMapFs() // Ensure in-memory FS

		filename := "/tmp/invalid.yaml"
		err := fs.WriteFile(filename, []byte("invalid yaml"), 0o644)
		if err != nil {
			return
		}

		_, err = gnome.LoadGnomeExtensions(filename)
		Expect(err).To(HaveOccurred())
		Expect(err.Error()).To(ContainSubstring("failed to parse GNOME extensions YAML"))
	})
})
