package platform

import (
	"os"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

// Mock implementations for testing
type MockRuntimeProvider struct {
	goos   string
	goarch string
}

func (m MockRuntimeProvider) GOOS() string   { return m.goos }
func (m MockRuntimeProvider) GOARCH() string { return m.goarch }

type MockFileSystemProvider struct {
	files map[string][]byte
	stats map[string]bool
}

func (m MockFileSystemProvider) ReadFile(filename string) ([]byte, error) {
	if content, exists := m.files[filename]; exists {
		return content, nil
	}
	return nil, os.ErrNotExist
}

func (m MockFileSystemProvider) Stat(name string) (os.FileInfo, error) {
	if exists := m.stats[name]; exists {
		// Return a minimal mock file info
		return &mockFileInfo{name: name}, nil
	}
	return nil, os.ErrNotExist
}

type mockFileInfo struct {
	name string
}

func (m *mockFileInfo) Name() string       { return m.name }
func (m *mockFileInfo) Size() int64        { return 0 }
func (m *mockFileInfo) Mode() os.FileMode  { return 0 }
func (m *mockFileInfo) ModTime() time.Time { return time.Time{} }
func (m *mockFileInfo) IsDir() bool        { return false }
func (m *mockFileInfo) Sys() interface{}   { return nil }

var _ = Describe("Platform Detection", func() {
	Describe("DetectPlatform", func() {
		DescribeTable("platform detection scenarios",
			func(mockOS, mockArch string, setupEnv func(), osRelease string, expected Platform) {
				// Setup environment
				setupEnv()

				// Create mock providers
				mockRuntime := MockRuntimeProvider{
					goos:   mockOS,
					goarch: mockArch,
				}

				mockFS := MockFileSystemProvider{
					files: make(map[string][]byte),
					stats: make(map[string]bool),
				}

				if osRelease != "" {
					mockFS.files["/etc/os-release"] = []byte(osRelease)
				}

				// Create detector with mocks
				detector := NewPlatformDetectorWithProviders(mockRuntime, mockFS)
				result := detector.Detect()

				Expect(result).To(Equal(expected))
			},
			Entry("Linux with GNOME",
				"linux", "amd64",
				func() {
					os.Setenv("XDG_CURRENT_DESKTOP", "GNOME")
					os.Setenv("GNOME_DESKTOP_SESSION_ID", "1")
				},
				"",
				Platform{
					OS:           "linux",
					DesktopEnv:   "gnome",
					Distribution: "unknown",
					Version:      "unknown",
					Architecture: "amd64",
				},
			),
			Entry("Linux Ubuntu",
				"linux", "amd64",
				func() { os.Clearenv() },
				`NAME="Ubuntu"
VERSION="22.04.3 LTS (Jammy Jellyfish)"
ID=ubuntu
ID_LIKE=debian
PRETTY_NAME="Ubuntu 22.04.3 LTS"
VERSION_ID="22.04"`,
				Platform{
					OS:           "linux",
					DesktopEnv:   "unknown",
					Distribution: "ubuntu",
					Version:      "22.04",
					Architecture: "amd64",
				},
			),
			Entry("macOS",
				"darwin", "arm64",
				func() { os.Clearenv() },
				"",
				Platform{
					OS:           "darwin",
					DesktopEnv:   "darwin",
					Distribution: "",
					Version:      "",
					Architecture: "arm64",
				},
			),
			Entry("Windows",
				"windows", "amd64",
				func() { os.Clearenv() },
				"",
				Platform{
					OS:           "windows",
					DesktopEnv:   "windows",
					Distribution: "",
					Version:      "",
					Architecture: "amd64",
				},
			),
		)
	})

	Describe("IsSupported", func() {
		DescribeTable("platform support scenarios",
			func(mockOS string, expected bool) {
				mockRuntime := MockRuntimeProvider{goos: mockOS}
				mockFS := MockFileSystemProvider{
					files: make(map[string][]byte),
					stats: make(map[string]bool),
				}

				detector := NewPlatformDetectorWithProviders(mockRuntime, mockFS)
				result := detector.IsSupported()

				Expect(result).To(Equal(expected))
			},
			Entry("Linux is supported", "linux", true),
			Entry("macOS is supported", "darwin", true),
			Entry("Windows is supported", "windows", true),
			Entry("FreeBSD is not supported", "freebsd", false),
		)
	})

	Describe("GetInstallerPriority", func() {
		DescribeTable("installer priority scenarios",
			func(mockOS string, osRelease string, expectedInstallers []string) {
				mockRuntime := MockRuntimeProvider{goos: mockOS}
				mockFS := MockFileSystemProvider{
					files: make(map[string][]byte),
					stats: make(map[string]bool),
				}

				if osRelease != "" {
					mockFS.files["/etc/os-release"] = []byte(osRelease)
				}

				detector := NewPlatformDetectorWithProviders(mockRuntime, mockFS)
				result := detector.GetInstallerPriority()

				Expect(result).To(Equal(expectedInstallers))
			},
			Entry("Ubuntu installers",
				"linux",
				`ID=ubuntu
VERSION_ID="22.04"`,
				[]string{"apt", "flatpak", "snap", "mise", "curlpipe", "download"},
			),
			Entry("Fedora installers",
				"linux",
				`ID=fedora
VERSION_ID="38"`,
				[]string{"dnf", "rpm", "flatpak", "mise", "curlpipe", "download"},
			),
			Entry("Windows installers",
				"windows",
				"",
				[]string{"winget", "chocolatey", "scoop", "mise", "download"},
			),
			Entry("macOS installers",
				"darwin",
				"",
				[]string{"brew", "mas", "mise", "curlpipe", "download"},
			),
			Entry("Unsupported OS installers",
				"freebsd",
				"",
				[]string{"mise", "curlpipe", "download"},
			),
		)
	})

	Describe("GetSystemPackageManager", func() {
		DescribeTable("system package manager scenarios",
			func(mockOS string, osRelease string, expectedManager string) {
				mockRuntime := MockRuntimeProvider{goos: mockOS}
				mockFS := MockFileSystemProvider{
					files: make(map[string][]byte),
					stats: make(map[string]bool),
				}

				if osRelease != "" {
					mockFS.files["/etc/os-release"] = []byte(osRelease)
				}

				detector := NewPlatformDetectorWithProviders(mockRuntime, mockFS)
				result := detector.GetSystemPackageManager()

				Expect(result).To(Equal(expectedManager))
			},
			Entry("Ubuntu uses apt",
				"linux",
				`ID=ubuntu
VERSION_ID="22.04"`,
				"apt",
			),
			Entry("Fedora uses dnf",
				"linux",
				`ID=fedora
VERSION_ID="38"`,
				"dnf",
			),
			Entry("Arch uses pacman",
				"linux",
				`ID=arch
VERSION_ID=""`,
				"pacman",
			),
			Entry("Unknown Linux",
				"linux",
				"",
				"unknown",
			),
			Entry("macOS uses brew",
				"darwin",
				"",
				"brew",
			),
			Entry("Windows uses winget",
				"windows",
				"",
				"winget",
			),
			Entry("Unsupported OS",
				"freebsd",
				"",
				"unknown",
			),
		)
	})

	Describe("detectDistribution", func() {
		DescribeTable("distribution detection scenarios",
			func(mockOS string, osRelease string, fileStats map[string]bool, expectedDistribution string) {
				mockRuntime := MockRuntimeProvider{goos: mockOS}
				mockFS := MockFileSystemProvider{
					files: make(map[string][]byte),
					stats: make(map[string]bool),
				}

				if osRelease != "" {
					mockFS.files["/etc/os-release"] = []byte(osRelease)
				}

				for file, exists := range fileStats {
					mockFS.stats[file] = exists
				}

				detector := NewPlatformDetectorWithProviders(mockRuntime, mockFS)
				result := detector.detectDistribution()

				Expect(result).To(Equal(expectedDistribution))
			},
			Entry("Ubuntu via os-release",
				"linux",
				`NAME="Ubuntu"
ID=ubuntu
VERSION_ID="22.04"`,
				nil,
				"ubuntu",
			),
			Entry("Debian via fallback file",
				"linux",
				"",
				map[string]bool{"/etc/debian_version": true},
				"debian",
			),
			Entry("Arch via fallback file",
				"linux",
				"",
				map[string]bool{"/etc/arch-release": true},
				"arch",
			),
			Entry("Non-Linux OS",
				"darwin",
				"",
				nil,
				"",
			),
			Entry("Unknown Linux",
				"linux",
				"",
				nil,
				"unknown",
			),
		)
	})
})
