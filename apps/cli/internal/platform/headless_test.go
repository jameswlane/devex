package platform

import (
	"os"
	"path/filepath"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Headless and graphical desktop detection", func() {
	emptyEnv := func(string) string { return "" }
	envFrom := func(values map[string]string) func(string) string {
		return func(key string) string { return values[key] }
	}
	missingDir := func(string) ([]os.DirEntry, error) {
		return nil, os.ErrNotExist
	}

	DescribeTable("detects graphical desktop environment variables on Linux",
		func(env map[string]string, expected bool) {
			Expect(HasGraphicalDesktopWith("linux", envFrom(env), missingDir, defaultGraphicalSessionDirs)).To(Equal(expected))
		},
		Entry("DISPLAY set", map[string]string{"DISPLAY": ":0"}, true),
		Entry("WAYLAND_DISPLAY set", map[string]string{"WAYLAND_DISPLAY": "wayland-0"}, true),
		Entry("XDG_CURRENT_DESKTOP set", map[string]string{"XDG_CURRENT_DESKTOP": "ubuntu:GNOME"}, true),
		Entry("whitespace-only env vars", map[string]string{"DISPLAY": "   ", "WAYLAND_DISPLAY": "\t"}, false),
		Entry("no env vars and no session dirs", map[string]string{}, false),
	)

	Context("when checking installed session directories on Linux", func() {
		It("returns true when an X11 session .desktop file is installed", func() {
			xDir := GinkgoT().TempDir()
			wDir := GinkgoT().TempDir()
			Expect(os.WriteFile(filepath.Join(xDir, "ubuntu.desktop"), []byte("[Desktop Entry]\n"), 0600)).To(Succeed())

			Expect(HasGraphicalDesktopWith("linux", emptyEnv, os.ReadDir, []string{xDir, wDir})).To(BeTrue())
		})

		It("returns true when a Wayland session .desktop file is installed", func() {
			xDir := GinkgoT().TempDir()
			wDir := GinkgoT().TempDir()
			Expect(os.WriteFile(filepath.Join(wDir, "sway.desktop"), []byte("[Desktop Entry]\n"), 0600)).To(Succeed())

			Expect(HasGraphicalDesktopWith("linux", emptyEnv, os.ReadDir, []string{xDir, wDir})).To(BeTrue())
		})

		It("returns false when session directories are empty or contain no .desktop files", func() {
			xDir := GinkgoT().TempDir()
			wDir := GinkgoT().TempDir()
			Expect(os.WriteFile(filepath.Join(xDir, "README.txt"), []byte("info"), 0600)).To(Succeed())
			Expect(os.Mkdir(filepath.Join(wDir, "nested.desktop"), 0700)).To(Succeed())

			Expect(HasGraphicalDesktopWith("linux", emptyEnv, os.ReadDir, []string{xDir, wDir})).To(BeFalse())
		})
	})

	It("treats non-Linux operating systems as graphical", func() {
		Expect(HasGraphicalDesktopWith("darwin", emptyEnv, missingDir, nil)).To(BeTrue())
		Expect(HasGraphicalDesktopWith("windows", emptyEnv, missingDir, nil)).To(BeTrue())
	})

	It("allows tests to override and restore the detector", func() {
		restoreFalse := SetGraphicalDesktopForTesting(false)
		Expect(HasGraphicalDesktop()).To(BeFalse())
		Expect(IsHeadless()).To(BeTrue())
		restoreFalse()

		restoreTrue := SetGraphicalDesktopForTesting(true)
		Expect(HasGraphicalDesktop()).To(BeTrue())
		Expect(IsHeadless()).To(BeFalse())
		restoreTrue()
	})
})
