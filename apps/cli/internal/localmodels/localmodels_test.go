package localmodels_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"

	shipconfig "github.com/jameswlane/devex/apps/cli/config"
	"github.com/jameswlane/devex/apps/cli/internal/localmodels"
	"github.com/jameswlane/devex/apps/cli/internal/security"
	"github.com/jameswlane/devex/apps/cli/internal/types"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v3"
)

var _ = Describe("Local model runtimes", func() {
	fixtureRoot := func(name string) string {
		abs, err := filepath.Abs(filepath.Join("testdata", name))
		Expect(err).NotTo(HaveOccurred())
		return abs
	}

	Describe("Ollama systemd unit and catalog sync", func() {
		It("renders a localhost-bound ollama systemd unit matching the shipped catalog entry verbatim", func() {
			unit := localmodels.RenderOllamaUnit()
			Expect(unit).To(And(
				ContainSubstring("Description=Ollama Service"),
				ContainSubstring("After=network-online.target"),
				ContainSubstring("ExecStart=/usr/local/bin/ollama serve"),
				ContainSubstring("User=ollama"),
				ContainSubstring("Group=ollama"),
				ContainSubstring("Restart=always"),
				ContainSubstring("RestartSec=3"),
				ContainSubstring(`Environment="OLLAMA_HOST=127.0.0.1:11434"`),
				ContainSubstring("WantedBy=default.target"),
			))

			data, err := fs.ReadFile(shipconfig.Files, "applications/ai-agents/ollama.yaml")
			Expect(err).NotTo(HaveOccurred())
			var app types.CrossPlatformApp
			Expect(yaml.Unmarshal(data, &app)).To(Succeed())

			Expect(app.Name).To(Equal("Ollama"))
			Expect(app.Default).To(BeFalse())
			Expect(app.Linux.InstalledBinary).To(Equal("/usr/local/bin/ollama"))
			Expect(app.Linux.DownloadURL).NotTo(ContainSubstring("install.sh"))
			Expect(app.Linux.InstallCommand).NotTo(ContainSubstring("install.sh"))
			Expect(app.Linux.PostInstall).NotTo(BeEmpty())

			installShell := app.Linux.PostInstall[0].Shell
			Expect(installShell).NotTo(ContainSubstring("install.sh"))
			Expect(installShell).To(ContainSubstring(unit))
			Expect(installShell).To(ContainSubstring("/usr/local/lib/ollama"))
			Expect(installShell).To(ContainSubstring("https://ollama.com/download/ollama-linux-${arch}.tar.zst"))
			Expect(installShell).To(ContainSubstring("https://ollama.com/download/ollama-linux-amd64-rocm.tar.zst"))
			Expect(installShell).To(ContainSubstring("DEVEX_INVOKING_USER"))
			Expect(installShell).To(ContainSubstring("sudo ubuntu-drivers install"))
			Expect(installShell).To(ContainSubstring("https://wiki.debian.org/NvidiaGraphicsDrivers"))

			// Security validator must accept the shipped install hook under moderate defaults.
			validator := security.NewCommandValidatorWithConfig(security.SecurityLevelModerate, nil)
			Expect(validator.ValidateCommandForApp(installShell, app.Name)).To(Succeed())

			// Uninstall must remove every path install creates.
			uninstall := app.Linux.UninstallCommand
			for _, path := range []string{
				"/etc/systemd/system/ollama.service",
				"/usr/local/bin/ollama",
				"/usr/local/lib/ollama",
				"/usr/share/ollama",
				"userdel ollama",
				"groupdel ollama",
				"systemctl stop ollama",
				"systemctl disable ollama",
				"systemctl daemon-reload",
			} {
				Expect(uninstall).To(ContainSubstring(path))
			}
		})
	})

	Describe("GPU detection from fixtures", func() {
		It("detects NVIDIA CUDA with driver version when nvidia-smi succeeds", func() {
			smiCSV, err := os.ReadFile(filepath.Join("testdata", "nvidia-smi-single.csv"))
			Expect(err).NotTo(HaveOccurred())

			probe := localmodels.Probe{
				GOOS:   "linux",
				FSRoot: fixtureRoot("nvidia-gpu"),
				LookPath: func(file string) (string, error) {
					if file == "nvidia-smi" {
						return "/usr/bin/nvidia-smi", nil
					}
					return "", exec.ErrNotFound
				},
				Run: func(_ context.Context, name string, args ...string) ([]byte, error) {
					Expect(name).To(Equal("/usr/bin/nvidia-smi"))
					Expect(args).To(Equal([]string{"--query-gpu=name,driver_version,memory.total", "--format=csv,noheader"}))
					return smiCSV, nil
				},
			}
			report := localmodels.DetectGPUs(context.Background(), probe)
			Expect(report.Runtime).To(Equal("NVIDIA CUDA (driver 550.54.15)"))
			Expect(report.DriverOK).To(BeTrue())
			Expect(report.Hints).To(BeEmpty())
			Expect(report.GPUs).To(HaveLen(1))
			Expect(report.GPUs[0].Vendor).To(Equal(localmodels.GPUVendorNVIDIA))
			Expect(report.GPUs[0].Name).To(Equal("NVIDIA GeForce RTX 4090"))
			Expect(report.GPUs[0].DriverVersion).To(Equal("550.54.15"))
			Expect(report.GPUs[0].Memory).To(Equal("24564 MiB"))
			Expect(report.GPUs[0].PCIAddress).To(Equal("0000-01-00.0")) // fixture dirs use - for : (Windows paths cannot contain colons)
		})

		It("parses multi-GPU nvidia-smi CSV fixtures", func() {
			smiCSV, err := os.ReadFile(filepath.Join("testdata", "nvidia-smi-multi.csv"))
			Expect(err).NotTo(HaveOccurred())

			gpus := localmodels.ParseNvidiaSMIOutput(string(smiCSV))
			Expect(gpus).To(HaveLen(2))
			Expect(gpus[0].Name).To(Equal("NVIDIA GeForce RTX 4090"))
			Expect(gpus[0].DriverVersion).To(Equal("550.90.07"))
			Expect(gpus[0].Memory).To(Equal("24564 MiB"))
			Expect(gpus[1].Name).To(Equal("NVIDIA GeForce RTX 3090"))
			Expect(gpus[1].Memory).To(Equal("24576 MiB"))
		})

		It("falls back to CPU only with Ubuntu driver hint when NVIDIA GPU lacks nvidia-smi on Ubuntu", func() {
			probe := localmodels.Probe{
				GOOS:     "linux",
				FSRoot:   fixtureRoot("sysfs-nvidia-ubuntu"),
				LookPath: func(string) (string, error) { return "", exec.ErrNotFound },
			}
			report := localmodels.DetectGPUs(context.Background(), probe)
			Expect(report.Runtime).To(Equal("CPU only"))
			Expect(report.DriverOK).To(BeFalse())
			Expect(report.GPUs).To(HaveLen(1))
			Expect(report.GPUs[0].Vendor).To(Equal(localmodels.GPUVendorNVIDIA))
			Expect(report.Hints).To(HaveLen(1))
			Expect(report.Hints[0]).To(ContainSubstring("sudo ubuntu-drivers install"))
		})

		It("falls back to CPU only with Debian driver hint when NVIDIA GPU lacks nvidia-smi on Debian", func() {
			probe := localmodels.Probe{
				GOOS:     "linux",
				FSRoot:   fixtureRoot("sysfs-nvidia-debian"),
				LookPath: func(string) (string, error) { return "", exec.ErrNotFound },
			}
			report := localmodels.DetectGPUs(context.Background(), probe)
			Expect(report.Runtime).To(Equal("CPU only"))
			Expect(report.DriverOK).To(BeFalse())
			Expect(report.GPUs).To(HaveLen(1))
			Expect(report.Hints).To(HaveLen(1))
			Expect(report.Hints[0]).To(ContainSubstring("https://wiki.debian.org/NvidiaGraphicsDrivers"))
		})

		It("falls back to CPU only with both Ubuntu and Debian hints when distro is unknown", func() {
			probe := localmodels.Probe{
				GOOS:     "linux",
				FSRoot:   fixtureRoot("nvidia-gpu"),
				LookPath: func(string) (string, error) { return "", exec.ErrNotFound },
			}
			report := localmodels.DetectGPUs(context.Background(), probe)
			Expect(report.Runtime).To(Equal("CPU only"))
			Expect(report.Hints).To(HaveLen(1))
			Expect(report.Hints[0]).To(And(
				ContainSubstring("sudo ubuntu-drivers install"),
				ContainSubstring("https://wiki.debian.org/NvidiaGraphicsDrivers"),
			))
		})

		It("detects AMD ROCm when amdgpu module and /dev/kfd are present", func() {
			probe := localmodels.Probe{
				GOOS:     "linux",
				FSRoot:   fixtureRoot("amd-rocm-ready"),
				LookPath: func(string) (string, error) { return "", exec.ErrNotFound },
			}
			report := localmodels.DetectGPUs(context.Background(), probe)
			Expect(report.Runtime).To(Equal("AMD ROCm"))
			Expect(report.DriverOK).To(BeTrue())
			Expect(report.Hints).To(BeEmpty())
			Expect(report.GPUs).To(HaveLen(1))
			Expect(report.GPUs[0].Vendor).To(Equal(localmodels.GPUVendorAMD))
		})

		It("falls back to CPU only with a ROCm hint when AMD GPU lacks amdgpu and /dev/kfd", func() {
			probe := localmodels.Probe{
				GOOS:     "linux",
				FSRoot:   fixtureRoot("amd-no-driver"),
				LookPath: func(string) (string, error) { return "", exec.ErrNotFound },
			}
			report := localmodels.DetectGPUs(context.Background(), probe)
			Expect(report.Runtime).To(Equal("CPU only"))
			Expect(report.DriverOK).To(BeFalse())
			Expect(report.GPUs).To(HaveLen(1))
			Expect(report.GPUs[0].Vendor).To(Equal(localmodels.GPUVendorAMD))
			Expect(report.Hints).To(HaveLen(1))
			Expect(report.Hints[0]).To(And(
				ContainSubstring("amdgpu"),
				ContainSubstring("/dev/kfd"),
			))
		})

		It("falls back to CPU only with a /dev/kfd hint when amdgpu is loaded without /dev/kfd", func() {
			probe := localmodels.Probe{
				GOOS:     "linux",
				FSRoot:   fixtureRoot("amd-no-kfd"),
				LookPath: func(string) (string, error) { return "", exec.ErrNotFound },
			}
			report := localmodels.DetectGPUs(context.Background(), probe)
			Expect(report.Runtime).To(Equal("CPU only"))
			Expect(report.DriverOK).To(BeFalse())
			Expect(report.Hints).To(HaveLen(1))
			Expect(report.Hints[0]).To(ContainSubstring("/dev/kfd"))
		})

		It("reports CPU only with no hints when no display GPUs are present", func() {
			probe := localmodels.Probe{
				GOOS:     "linux",
				FSRoot:   fixtureRoot("no-gpu"),
				LookPath: func(string) (string, error) { return "", exec.ErrNotFound },
			}
			report := localmodels.DetectGPUs(context.Background(), probe)
			Expect(report.Runtime).To(Equal("CPU only"))
			Expect(report.DriverOK).To(BeFalse())
			Expect(report.GPUs).To(BeEmpty())
			Expect(report.Hints).To(BeEmpty())
		})

		It("handles multi-GPU systems with both NVIDIA and AMD cards", func() {
			probe := localmodels.Probe{
				GOOS:   "linux",
				FSRoot: fixtureRoot("multi-gpu"),
				LookPath: func(file string) (string, error) {
					if file == "nvidia-smi" {
						return "/usr/bin/nvidia-smi", nil
					}
					return "", exec.ErrNotFound
				},
				Run: func(_ context.Context, _ string, _ ...string) ([]byte, error) {
					return []byte("NVIDIA RTX 4090, 550.90.07, 24564 MiB\n"), nil
				},
			}
			report := localmodels.DetectGPUs(context.Background(), probe)
			Expect(report.GPUs).To(HaveLen(2))
			Expect(report.DriverOK).To(BeTrue())
			Expect(report.Runtime).To(Equal("NVIDIA CUDA (driver 550.90.07), AMD ROCm"))
			Expect(report.Hints).To(BeEmpty())
		})
		It("leaves GPU selection to the Ollama app outside Linux, without Linux driver hints", func() {
			report := localmodels.DetectGPUs(context.Background(), localmodels.Probe{GOOS: "darwin", FSRoot: fixtureRoot("nvidia-gpu")})
			Expect(report.Runtime).To(Equal("chosen by the Ollama app on darwin"))
			Expect(report.GPUs).To(BeEmpty())
			Expect(report.Hints).To(BeEmpty())
		})
	})

	Describe("Model name validation", func() {
		DescribeTable("accepts valid Ollama model references",
			func(name string) {
				Expect(localmodels.ValidateModelName(name)).To(Succeed())
			},
			Entry("plain model", "qwen2.5-coder"),
			Entry("tagged model", "qwen2.5-coder:7b"),
			Entry("quantized tag", "llama3.2:3b-instruct-q4_K_M"),
			Entry("user namespaced", "user/my-model:v1.2"),
		)

		DescribeTable("rejects invalid or unsafe model references",
			func(name string) {
				Expect(localmodels.ValidateModelName(name)).To(HaveOccurred())
			},
			Entry("empty", ""),
			Entry("whitespace", "qwen 2.5"),
			Entry("shell semicolon", "qwen2.5-coder:7b;rm -rf /"),
			Entry("command substitution", "$(whoami)"),
			Entry("uppercase repo", "Qwen/Coder:7b"),
			Entry("leading dash", "-flag"),
			Entry("path traversal", "../etc/passwd"),
			Entry("double slash", "a//b"),
			Entry("empty tag", "qwen2.5-coder:"),
		)
	})

	Describe("Registry manifest size parsing and model planning", func() {
		It("parses manifest config and layer sizes and resolves library vs user namespaces", func() {
			var paths []string
			var accepts []string
			registry := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				paths = append(paths, r.URL.Path)
				accepts = append(accepts, r.Header.Get("Accept"))
				switch r.URL.Path {
				case "/v2/library/qwen2.5-coder/manifests/7b":
					_, _ = w.Write([]byte(`{"config":{"size":487},"layers":[{"size":4683074048},{"size":13026}]}`))
				case "/v2/library/llama3.2/manifests/latest":
					_, _ = w.Write([]byte(`{"config":{"size":500},"layers":[{"size":2000000000}]}`))
				case "/v2/alice/custom/manifests/v1":
					_, _ = w.Write([]byte(`{"config":{"size":100},"layers":[{"size":1000000000}]}`))
				default:
					http.NotFound(w, r)
				}
			}))
			DeferCleanup(registry.Close)

			size, err := localmodels.FetchModelSize(context.Background(), registry.Client(), registry.URL, "qwen2.5-coder:7b")
			Expect(err).NotTo(HaveOccurred())
			Expect(size).To(Equal(int64(4683087561)))
			Expect(localmodels.FormatSize(size)).To(Equal("4.7 GB"))

			sizeLatest, err := localmodels.FetchModelSize(context.Background(), registry.Client(), registry.URL, "llama3.2")
			Expect(err).NotTo(HaveOccurred())
			Expect(sizeLatest).To(Equal(int64(2000000500)))

			sizeUser, err := localmodels.FetchModelSize(context.Background(), registry.Client(), registry.URL, "alice/custom:v1")
			Expect(err).NotTo(HaveOccurred())
			Expect(sizeUser).To(Equal(int64(1000000100)))

			Expect(paths).To(Equal([]string{
				"/v2/library/qwen2.5-coder/manifests/7b",
				"/v2/library/llama3.2/manifests/latest",
				"/v2/alice/custom/manifests/v1",
			}))
			for _, accept := range accepts {
				Expect(accept).To(Equal(localmodels.DockerManifestV2MediaType))
			}
		})

		It("builds and formats a plan with already present, pull with size, size unknown, and GPU line", func() {
			ollamaSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/api/tags" {
					_, _ = w.Write([]byte(`{"models":[{"name":"llama3.2:latest"}]}`))
					return
				}
				http.NotFound(w, r)
			}))
			DeferCleanup(ollamaSrv.Close)

			registrySrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/v2/library/qwen2.5-coder/manifests/7b" {
					_, _ = w.Write([]byte(`{"config":{"size":487},"layers":[{"size":4683074048}]}`))
					return
				}
				http.Error(w, "unavailable", http.StatusServiceUnavailable)
			}))
			DeferCleanup(registrySrv.Close)

			probe := localmodels.Probe{
				GOOS:       "linux",
				FSRoot:     fixtureRoot("no-gpu"),
				HTTPClient: ollamaSrv.Client(),
				LookPath: func(name string) (string, error) {
					if name == "ollama" {
						return "/usr/local/bin/ollama", nil
					}
					return "", exec.ErrNotFound
				},
			}

			plan, err := localmodels.BuildPlan(context.Background(), probe, []string{
				"qwen2.5-coder:7b",
				"llama3.2",
				"missing-size:1b",
			}, localmodels.PlanOptions{
				OllamaBaseURL:   ollamaSrv.URL,
				RegistryBaseURL: registrySrv.URL,
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(plan.Reachable).To(BeTrue())
			Expect(plan.Models).To(HaveLen(3))
			Expect(plan.Models[0].Present).To(BeFalse())
			Expect(plan.Models[0].SizeKnown).To(BeTrue())
			Expect(plan.Models[1].Present).To(BeTrue())
			Expect(plan.Models[2].Present).To(BeFalse())
			Expect(plan.Models[2].SizeKnown).To(BeFalse())

			rendered := localmodels.FormatPlan(plan)
			Expect(rendered).To(And(
				ContainSubstring("GPU: CPU only"),
				ContainSubstring("qwen2.5-coder:7b"),
				ContainSubstring("pull (4.7 GB)"),
				ContainSubstring("llama3.2"),
				ContainSubstring("already present"),
				ContainSubstring("missing-size:1b"),
				ContainSubstring("size unknown"),
			))
		})

		It("falls back to ollama list when the HTTP API is unreachable", func() {
			unreachable := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
			badURL := unreachable.URL
			unreachable.Close()

			probe := localmodels.Probe{
				GOOS:   "linux",
				FSRoot: fixtureRoot("no-gpu"),
				LookPath: func(name string) (string, error) {
					if name == "ollama" {
						return "/usr/local/bin/ollama", nil
					}
					return "", exec.ErrNotFound
				},
				Run: func(_ context.Context, name string, args ...string) ([]byte, error) {
					Expect(name).To(Equal("/usr/local/bin/ollama"))
					Expect(args).To(Equal([]string{"list"}))
					return []byte("NAME              ID           SIZE   MODIFIED\nqwen2.5-coder:7b  abc123       4.7 GB 2 days ago\n"), nil
				},
			}

			present, reachable, err := localmodels.ListInstalledModels(context.Background(), probe, badURL)
			Expect(err).NotTo(HaveOccurred())
			Expect(reachable).To(BeTrue())
			Expect(present).To(HaveKeyWithValue("qwen2.5-coder:7b", true))
		})
	})

	Describe("Apply and dry-run behavior", func() {
		It("performs zero writes and zero pulls in dry-run mode", func() {
			home := GinkgoT().TempDir()
			GinkgoT().Setenv("HOME", home)

			var calls [][]string
			probe := localmodels.Probe{
				LookPath: func(string) (string, error) { return "/usr/local/bin/ollama", nil },
				Run: func(_ context.Context, name string, args ...string) ([]byte, error) {
					calls = append(calls, append([]string{name}, args...))
					return nil, nil
				},
			}

			var out bytes.Buffer
			err := localmodels.PullModels(context.Background(), probe, []string{"qwen2.5-coder:7b"}, localmodels.PullOptions{DryRun: true}, &out)
			Expect(err).NotTo(HaveOccurred())
			Expect(calls).To(BeEmpty())

			entries, readErr := os.ReadDir(home)
			Expect(readErr).NotTo(HaveOccurred())
			Expect(entries).To(BeEmpty())
		})

		It("returns an actionable error when ollama is not installed", func() {
			probe := localmodels.Probe{
				LookPath: func(string) (string, error) { return "", exec.ErrNotFound },
			}
			var out bytes.Buffer
			err := localmodels.PullModels(context.Background(), probe, []string{"qwen2.5-coder:7b"}, localmodels.PullOptions{}, &out)
			Expect(err).To(MatchError(ContainSubstring("Ollama is not installed: run `devex install ollama`")))
		})

		It("returns a systemctl start hint when the Ollama service is unreachable", func() {
			unreachable := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
			badURL := unreachable.URL
			unreachable.Close()

			probe := localmodels.Probe{
				GOOS:       "linux", // the hint is Linux-specific; macOS and Windows get "start the Ollama app"
				HTTPClient: &http.Client{},
				LookPath:   func(string) (string, error) { return "/usr/local/bin/ollama", nil },
			}
			var out bytes.Buffer
			err := localmodels.PullModels(context.Background(), probe, []string{"qwen2.5-coder:7b"}, localmodels.PullOptions{OllamaBaseURL: badURL}, &out)
			Expect(err).To(MatchError(ContainSubstring("sudo systemctl start ollama")))
		})

		It("pulls only missing models directly via ollama pull without invoking a shell", func() {
			ollamaSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/api/tags" {
					_, _ = w.Write([]byte(`{"models":[{"name":"llama3.2:latest"}]}`))
					return
				}
				http.NotFound(w, r)
			}))
			DeferCleanup(ollamaSrv.Close)

			var pulled [][]string
			probe := localmodels.Probe{
				HTTPClient: ollamaSrv.Client(),
				LookPath:   func(string) (string, error) { return "/usr/local/bin/ollama", nil },
				Run: func(_ context.Context, name string, args ...string) ([]byte, error) {
					pulled = append(pulled, append([]string{name}, args...))
					return []byte("success\n"), nil
				},
			}
			var out bytes.Buffer
			Expect(localmodels.PullModels(context.Background(), probe, []string{"llama3.2", "qwen2.5-coder:7b"}, localmodels.PullOptions{OllamaBaseURL: ollamaSrv.URL}, &out)).To(Succeed())
			Expect(pulled).To(Equal([][]string{
				{"/usr/local/bin/ollama", "pull", "qwen2.5-coder:7b"},
			}))
		})

		It("streams pull progress to the given writers instead of buffering it", func() {
			ollamaSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(`{"models":[]}`))
			}))
			DeferCleanup(ollamaSrv.Close)

			var out, progress bytes.Buffer
			probe := localmodels.Probe{
				HTTPClient: ollamaSrv.Client(),
				LookPath:   func(string) (string, error) { return "/usr/local/bin/ollama", nil },
				Run: func(context.Context, string, ...string) ([]byte, error) {
					Fail("pulls must use Stream, not the buffered Run")
					return nil, nil
				},
				Stream: func(_ context.Context, stdout, stderr io.Writer, name string, args ...string) error {
					Expect(stdout).To(BeIdenticalTo(&out))
					Expect(stderr).To(BeIdenticalTo(&progress))
					Expect(append([]string{name}, args...)).To(Equal([]string{"/usr/local/bin/ollama", "pull", "qwen2.5-coder:7b"}))
					_, _ = io.WriteString(stderr, "pulling manifest\n")
					return nil
				},
			}
			Expect(localmodels.PullModels(context.Background(), probe, []string{"qwen2.5-coder:7b"}, localmodels.PullOptions{OllamaBaseURL: ollamaSrv.URL, Stderr: &progress}, &out)).To(Succeed())
			Expect(progress.String()).To(Equal("pulling manifest\n"))
			Expect(out.String()).To(ContainSubstring("Pulled local model qwen2.5-coder:7b"))
		})

		It("tells macOS and Windows users to start the Ollama app when the API is unreachable", func() {
			probe := localmodels.Probe{
				GOOS:     "darwin",
				LookPath: func(string) (string, error) { return "/opt/homebrew/bin/ollama", nil },
			}
			err := localmodels.PullModels(context.Background(), probe, []string{"qwen2.5-coder:7b"}, localmodels.PullOptions{OllamaBaseURL: "http://127.0.0.1:1"}, io.Discard)
			Expect(err).To(MatchError(ContainSubstring("start the Ollama app")))
			Expect(err.Error()).NotTo(ContainSubstring("systemctl"))
		})

		It("surfaces pull errors clearly", func() {
			ollamaSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(`{"models":[]}`))
			}))
			DeferCleanup(ollamaSrv.Close)

			probe := localmodels.Probe{
				HTTPClient: ollamaSrv.Client(),
				LookPath:   func(string) (string, error) { return "/usr/local/bin/ollama", nil },
				Run: func(_ context.Context, _ string, _ ...string) ([]byte, error) {
					return nil, errors.New("exit status 1")
				},
			}
			var out bytes.Buffer
			err := localmodels.PullModels(context.Background(), probe, []string{"qwen2.5-coder:7b"}, localmodels.PullOptions{OllamaBaseURL: ollamaSrv.URL}, &out)
			Expect(err).To(MatchError(ContainSubstring("qwen2.5-coder:7b")))
		})
	})

	Describe("Runtime inspection", func() {
		It("reports installed binary, active systemd service, bound address from unit, API reachability, and GPU", func() {
			ollamaSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/api/tags" {
					_, _ = w.Write([]byte(`{"models":[]}`))
					return
				}
				http.NotFound(w, r)
			}))
			DeferCleanup(ollamaSrv.Close)

			fsRoot := GinkgoT().TempDir()
			Expect(os.MkdirAll(filepath.Join(fsRoot, "run/systemd/system"), 0o755)).To(Succeed())
			Expect(os.MkdirAll(filepath.Join(fsRoot, "etc/systemd/system"), 0o755)).To(Succeed())
			Expect(os.WriteFile(filepath.Join(fsRoot, "etc/systemd/system/ollama.service"), []byte(localmodels.RenderOllamaUnit()), 0o644)).To(Succeed())

			probe := localmodels.Probe{
				GOOS:       "linux",
				FSRoot:     fsRoot,
				HTTPClient: ollamaSrv.Client(),
				Getenv: func(key string) string {
					if key == "OLLAMA_HOST" {
						return ollamaSrv.URL
					}
					return ""
				},
				LookPath: func(name string) (string, error) {
					switch name {
					case "ollama":
						return "/usr/local/bin/ollama", nil
					case "systemctl":
						return "/usr/bin/systemctl", nil
					default:
						return "", exec.ErrNotFound
					}
				},
				Run: func(_ context.Context, name string, args ...string) ([]byte, error) {
					switch name {
					case "/usr/local/bin/ollama":
						return []byte("ollama version is 0.6.2\n"), nil
					case "/usr/bin/systemctl":
						Expect(args).To(Equal([]string{"is-active", "ollama"}))
						return []byte("active\n"), nil
					default:
						return nil, errors.New("unexpected command")
					}
				},
			}

			status := localmodels.InspectRuntime(context.Background(), probe)
			Expect(status.Installed).To(BeTrue())
			Expect(status.BinaryPath).To(Equal("/usr/local/bin/ollama"))
			Expect(status.Version).To(Equal("0.6.2"))
			Expect(status.ServiceActive).To(BeTrue())
			Expect(status.ServiceState).To(Equal("active"))
			Expect(status.APIReachable).To(BeTrue())
			Expect(status.GPU.Runtime).To(Equal("CPU only"))
		})
	})
})
