package platform

import (
	"os"
	"testing"
	"time"
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

func TestDetectPlatform(t *testing.T) {
	tests := []struct {
		name      string
		mockOS    string
		mockArch  string
		setupEnv  func()
		osRelease string
		want      Platform
	}{
		{
			name:     "Linux with GNOME",
			mockOS:   "linux",
			mockArch: "amd64",
			setupEnv: func() {
				os.Setenv("XDG_CURRENT_DESKTOP", "GNOME")
				os.Setenv("GNOME_DESKTOP_SESSION_ID", "1")
			},
			want: Platform{
				OS:           "linux",
				DesktopEnv:   "gnome",
				Distribution: "unknown",
				Version:      "unknown",
				Architecture: "amd64",
			},
		},
		{
			name:     "Linux Ubuntu",
			mockOS:   "linux",
			mockArch: "amd64",
			setupEnv: func() {
				os.Clearenv()
			},
			osRelease: `NAME="Ubuntu"
VERSION="22.04.3 LTS (Jammy Jellyfish)"
ID=ubuntu
ID_LIKE=debian
PRETTY_NAME="Ubuntu 22.04.3 LTS"
VERSION_ID="22.04"`,
			want: Platform{
				OS:           "linux",
				DesktopEnv:   "unknown",
				Distribution: "ubuntu",
				Version:      "22.04",
				Architecture: "amd64",
			},
		},
		{
			name:     "macOS",
			mockOS:   "darwin",
			mockArch: "arm64",
			setupEnv: func() {
				os.Clearenv()
			},
			want: Platform{
				OS:           "darwin",
				DesktopEnv:   "darwin",
				Distribution: "",
				Version:      "",
				Architecture: "arm64",
			},
		},
		{
			name:     "Windows",
			mockOS:   "windows",
			mockArch: "amd64",
			setupEnv: func() {
				os.Clearenv()
			},
			want: Platform{
				OS:           "windows",
				DesktopEnv:   "windows",
				Distribution: "",
				Version:      "",
				Architecture: "amd64",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Setup environment
			tt.setupEnv()

			// Create mock providers
			mockRuntime := MockRuntimeProvider{
				goos:   tt.mockOS,
				goarch: tt.mockArch,
			}

			mockFS := MockFileSystemProvider{
				files: make(map[string][]byte),
				stats: make(map[string]bool),
			}

			if tt.osRelease != "" {
				mockFS.files["/etc/os-release"] = []byte(tt.osRelease)
			}

			// Create detector with mocks
			detector := NewPlatformDetectorWithProviders(mockRuntime, mockFS)
			got := detector.Detect()

			if got != tt.want {
				t.Errorf("DetectPlatform() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestIsSupportedPlatform(t *testing.T) {
	tests := []struct {
		name    string
		mockOS  string
		wantRes bool
	}{
		{name: "Linux", mockOS: "linux", wantRes: true},
		{name: "macOS", mockOS: "darwin", wantRes: true},
		{name: "Windows", mockOS: "windows", wantRes: true},
		{name: "Unsupported OS", mockOS: "freebsd", wantRes: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRuntime := MockRuntimeProvider{goos: tt.mockOS}
			mockFS := MockFileSystemProvider{
				files: make(map[string][]byte),
				stats: make(map[string]bool),
			}

			detector := NewPlatformDetectorWithProviders(mockRuntime, mockFS)
			res := detector.IsSupported()

			if res != tt.wantRes {
				t.Errorf("IsSupported() = %v, want %v", res, tt.wantRes)
			}
		})
	}
}

func TestGetInstallerPriority(t *testing.T) {
	tests := []struct {
		name              string
		mockOS            string
		osRelease         string
		wantInstallerList []string
	}{
		{
			name:   "Ubuntu",
			mockOS: "linux",
			osRelease: `ID=ubuntu
VERSION_ID="22.04"`,
			wantInstallerList: []string{"apt", "flatpak", "snap", "mise", "curlpipe", "download"},
		},
		{
			name:   "Fedora",
			mockOS: "linux",
			osRelease: `ID=fedora
VERSION_ID="38"`,
			wantInstallerList: []string{"dnf", "rpm", "flatpak", "mise", "curlpipe", "download"},
		},
		{
			name:              "Windows",
			mockOS:            "windows",
			wantInstallerList: []string{"winget", "chocolatey", "scoop", "mise", "download"},
		},
		{
			name:              "macOS",
			mockOS:            "darwin",
			wantInstallerList: []string{"brew", "mas", "mise", "curlpipe", "download"},
		},
		{
			name:              "Unsupported OS",
			mockOS:            "freebsd",
			wantInstallerList: []string{"mise", "curlpipe", "download"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRuntime := MockRuntimeProvider{goos: tt.mockOS}
			mockFS := MockFileSystemProvider{
				files: make(map[string][]byte),
				stats: make(map[string]bool),
			}

			if tt.osRelease != "" {
				mockFS.files["/etc/os-release"] = []byte(tt.osRelease)
			}

			detector := NewPlatformDetectorWithProviders(mockRuntime, mockFS)
			got := detector.GetInstallerPriority()

			if len(got) != len(tt.wantInstallerList) {
				t.Errorf("GetInstallerPriority() length = %d, want %d", len(got), len(tt.wantInstallerList))
				return
			}

			for i, value := range got {
				if value != tt.wantInstallerList[i] {
					t.Errorf("GetInstallerPriority()[%d] = %v, want %v", i, value, tt.wantInstallerList[i])
				}
			}
		})
	}
}

func TestGetSystemPackageManager(t *testing.T) {
	tests := []struct {
		name        string
		mockOS      string
		osRelease   string
		wantManager string
	}{
		{
			name:   "Linux Ubuntu",
			mockOS: "linux",
			osRelease: `ID=ubuntu
VERSION_ID="22.04"`,
			wantManager: "apt",
		},
		{
			name:   "Linux Fedora",
			mockOS: "linux",
			osRelease: `ID=fedora
VERSION_ID="38"`,
			wantManager: "dnf",
		},
		{
			name:   "Linux Arch",
			mockOS: "linux",
			osRelease: `ID=arch
VERSION_ID=""`,
			wantManager: "pacman",
		},
		{
			name:        "Linux Unknown",
			mockOS:      "linux",
			wantManager: "unknown",
		},
		{
			name:        "macOS",
			mockOS:      "darwin",
			wantManager: "brew",
		},
		{
			name:        "Windows",
			mockOS:      "windows",
			wantManager: "winget",
		},
		{
			name:        "Unsupported OS",
			mockOS:      "freebsd",
			wantManager: "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRuntime := MockRuntimeProvider{goos: tt.mockOS}
			mockFS := MockFileSystemProvider{
				files: make(map[string][]byte),
				stats: make(map[string]bool),
			}

			if tt.osRelease != "" {
				mockFS.files["/etc/os-release"] = []byte(tt.osRelease)
			}

			detector := NewPlatformDetectorWithProviders(mockRuntime, mockFS)
			res := detector.GetSystemPackageManager()

			if res != tt.wantManager {
				t.Errorf("GetSystemPackageManager() = %s, want %s", res, tt.wantManager)
			}
		})
	}
}

func TestDetectDistribution(t *testing.T) {
	tests := []struct {
		name             string
		mockOS           string
		osRelease        string
		fileStats        map[string]bool
		wantDistribution string
	}{
		{
			name:   "Ubuntu via os-release",
			mockOS: "linux",
			osRelease: `NAME="Ubuntu"
ID=ubuntu
VERSION_ID="22.04"`,
			wantDistribution: "ubuntu",
		},
		{
			name:   "Debian via fallback file",
			mockOS: "linux",
			fileStats: map[string]bool{
				"/etc/debian_version": true,
			},
			wantDistribution: "debian",
		},
		{
			name:   "Arch via fallback file",
			mockOS: "linux",
			fileStats: map[string]bool{
				"/etc/arch-release": true,
			},
			wantDistribution: "arch",
		},
		{
			name:             "Non-Linux OS",
			mockOS:           "darwin",
			wantDistribution: "",
		},
		{
			name:             "Unknown Linux",
			mockOS:           "linux",
			wantDistribution: "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRuntime := MockRuntimeProvider{goos: tt.mockOS}
			mockFS := MockFileSystemProvider{
				files: make(map[string][]byte),
				stats: make(map[string]bool),
			}

			if tt.osRelease != "" {
				mockFS.files["/etc/os-release"] = []byte(tt.osRelease)
			}

			for file, exists := range tt.fileStats {
				mockFS.stats[file] = exists
			}

			detector := NewPlatformDetectorWithProviders(mockRuntime, mockFS)
			got := detector.detectDistribution()

			if got != tt.wantDistribution {
				t.Errorf("detectDistribution() = %s, want %s", got, tt.wantDistribution)
			}
		})
	}
}
