//go:build e2e

package e2e_test

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"syscall"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

type suiteEnvironment struct {
	home, binary, registry, desktop, artifacts, work string
	homes                                            []string
	commands                                         int
}

type pluginState struct {
	Checksum string
	Modified time.Time
	Mode     os.FileMode
}

func envDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func artifactDirectory() string {
	directory := os.Getenv("DEVEX_E2E_ARTIFACTS")
	if directory != "" {
		Expect(os.MkdirAll(directory, 0750)).To(Succeed(), "create requested artifact directory")
		return directory
	}
	if err := os.MkdirAll("/artifacts", 0750); err == nil {
		probe, err := os.CreateTemp("/artifacts", ".writable-")
		if err == nil {
			probe.Close()
			os.Remove(probe.Name())
			return "/artifacts"
		}
	}
	directory, err := os.MkdirTemp("", "devex-e2e-artifacts-")
	Expect(err).NotTo(HaveOccurred(), "create fallback artifact directory")
	GinkgoWriter.Printf("Failure artifacts: %s\n", directory)
	return directory
}

func (s *suiteEnvironment) environment(home string, overrides map[string]string) []string {
	values := map[string]string{}
	for _, entry := range os.Environ() {
		key, value, _ := strings.Cut(entry, "=")
		// Prevent host DevEx settings and desktop/session variables from leaking in.
		if strings.HasPrefix(key, "DEVEX_") || strings.HasPrefix(key, "XDG_") || strings.HasPrefix(key, "KDE_") || strings.HasPrefix(key, "GNOME_") || key == "DESKTOP_SESSION" || key == "GO_TEST" || key == "TESTING" {
			continue
		}
		values[key] = value
	}
	values["HOME"] = home
	values["DEVEX_NONINTERACTIVE"] = "1"
	values["DEVEX_PLUGIN_REGISTRY_URL"] = s.registry
	values["XDG_CURRENT_DESKTOP"] = strings.ToUpper(s.desktop)
	values["XDG_DATA_HOME"] = filepath.Join(home, ".local/share")
	values["XDG_CONFIG_HOME"] = filepath.Join(home, ".config")
	values["XDG_CACHE_HOME"] = filepath.Join(home, ".cache")
	for key, value := range overrides {
		values[key] = value
	}
	environment := make([]string, 0, len(values))
	for key, value := range values {
		environment = append(environment, key+"="+value)
	}
	return environment
}

func (s *suiteEnvironment) run(home, binary string, timeout time.Duration, overrides map[string]string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, binary, args...)
	cmd.Dir = s.work
	cmd.Env = s.environment(home, overrides)
	// Kill the entire process group on timeout, including plugin subprocesses.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
	cmd.WaitDelay = 2 * time.Second
	output, err := cmd.CombinedOutput()
	s.commands++
	transcript := fmt.Sprintf("command: %s %s\nhome: %s\nerror: %v\n%s", binary, strings.Join(args, " "), home, err, output)
	Expect(os.WriteFile(filepath.Join(s.artifacts, fmt.Sprintf("command-%03d.log", s.commands)), []byte(transcript), 0600)).To(Succeed(), "save command transcript")
	Expect(ctx.Err()).NotTo(HaveOccurred(), "command exceeded %s: %s", timeout, transcript)
	return string(output), err
}

func (s *suiteEnvironment) cli(args ...string) string {
	output, err := s.run(s.home, s.binary, 3*time.Minute, nil, args...)
	Expect(err).NotTo(HaveOccurred(), "devex %s failed:\n%s", strings.Join(args, " "), output)
	return output
}

func checksum(path string) string {
	file, err := os.Open(path)
	Expect(err).NotTo(HaveOccurred(), "open %s for SHA256", path)
	defer file.Close()
	hash := sha256.New()
	_, err = io.Copy(hash, file)
	Expect(err).NotTo(HaveOccurred(), "hash %s", path)
	return hex.EncodeToString(hash.Sum(nil))
}

func pluginPath(home, name string) string {
	return filepath.Join(home, ".devex/plugins/devex-plugin-"+name)
}

func expectedPlugins() []string {
	release, err := os.ReadFile("/etc/os-release")
	Expect(err).NotTo(HaveOccurred(), "read Linux distribution")
	distro := ""
	for _, line := range strings.Split(string(release), "\n") {
		if value, ok := strings.CutPrefix(line, "ID="); ok {
			distro = strings.Trim(value, `"`)
		}
	}
	Expect(distro).To(BeElementOf("ubuntu", "debian"), "phase 1 supports apt-based ubuntu/debian, got %q", distro)
	plugins := []string{"package-manager-apt", "tool-shell", "tool-git"}
	// Bootstrap detects universal package managers from PATH, not distro ID.
	for command, name := range map[string]string{"flatpak": "flatpak", "snap": "snap", "appimagetool": "appimage"} {
		if _, err := exec.LookPath(command); err == nil {
			plugins = append(plugins, "package-manager-"+name)
		}
	}
	if sandbox.desktop != "none" {
		plugins = append(plugins, "desktop-"+sandbox.desktop)
	}
	sort.Strings(plugins)
	return plugins
}

func registryChecksum(name string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	url := strings.TrimRight(sandbox.registry, "/") + "/api/v1/plugins/" + name + "/versions/latest"
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	Expect(err).NotTo(HaveOccurred(), "construct registry request")
	response, err := http.DefaultClient.Do(request)
	Expect(err).NotTo(HaveOccurred(), "fetch expected checksum from %s", url)
	defer response.Body.Close()
	Expect(response.StatusCode).To(Equal(http.StatusOK), "registry response from %s", url)
	var metadata struct {
		Binaries map[string]struct {
			Checksum string `json:"checksum"`
		} `json:"binaries"`
	}
	Expect(json.NewDecoder(io.LimitReader(response.Body, 2<<20)).Decode(&metadata)).To(Succeed(), "decode registry metadata for %s", name)
	expected := strings.TrimPrefix(metadata.Binaries[runtime.GOOS+"-"+runtime.GOARCH].Checksum, "sha256:")
	Expect(expected).To(MatchRegexp(`^[a-fA-F0-9]{64}$`), "registry must supply a SHA256 for %s on this platform", name)
	return strings.ToLower(expected)
}

func snapshotPlugins(home string) map[string]pluginState {
	entries, err := os.ReadDir(filepath.Join(home, ".devex/plugins"))
	Expect(err).NotTo(HaveOccurred(), "list plugin directory")
	snapshot := make(map[string]pluginState, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		path := filepath.Join(home, ".devex/plugins", entry.Name())
		info, err := entry.Info()
		Expect(err).NotTo(HaveOccurred(), "stat plugin %s", path)
		snapshot[entry.Name()] = pluginState{checksum(path), info.ModTime(), info.Mode()}
	}
	return snapshot
}

func (s *suiteEnvironment) isolatedHome() string {
	home, err := os.MkdirTemp(s.work, "home-")
	Expect(err).NotTo(HaveOccurred(), "create empty failure-mode HOME")
	s.homes = append(s.homes, home)
	return home
}

func (s *suiteEnvironment) collectArtifacts(spec string) {
	directory, err := os.MkdirTemp(s.artifacts, "failure-")
	if err != nil {
		GinkgoWriter.Printf("Cannot create failure artifacts: %v\n", err)
		return
	}
	_ = os.WriteFile(filepath.Join(directory, "spec.txt"), []byte(spec), 0600)
	for index, home := range s.homes {
		target := filepath.Join(directory, fmt.Sprintf("home-%d", index))
		if err := os.MkdirAll(target, 0750); err != nil {
			GinkgoWriter.Printf("Artifact directory: %v\n", err)
			continue
		}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		listing, listErr := exec.CommandContext(ctx, "ls", "-laR", filepath.Join(home, ".devex")).CombinedOutput()
		cancel()
		_ = os.WriteFile(filepath.Join(target, "devex-listing.txt"), append(listing, []byte(fmt.Sprintf("\nls error: %v\n", listErr))...), 0600)
		for _, relative := range []string{".devex", ".local/share/devex/logs"} {
			source := filepath.Join(home, relative)
			destination := filepath.Join(target, relative)
			if _, err := os.Stat(source); os.IsNotExist(err) {
				continue
			}
			if err := os.MkdirAll(filepath.Dir(destination), 0750); err != nil {
				GinkgoWriter.Printf("Artifact parent: %v\n", err)
				continue
			}
			if err := os.CopyFS(destination, os.DirFS(source)); err != nil {
				GinkgoWriter.Printf("Copy %s: %v\n", source, err)
			}
		}
	}
	GinkgoWriter.Printf("Failure artifacts: %s\n", directory)
}

// processRunning mirrors the CLI's desktop fallback (pgrep by process name).
func processRunning(name string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return exec.CommandContext(ctx, "pgrep", name).Run() == nil
}
