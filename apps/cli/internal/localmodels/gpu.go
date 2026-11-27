package localmodels

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
)

const (
	pciVendorNVIDIA = "0x10de"
	pciVendorAMD    = "0x1002"

	probeCommandTimeout = 5 * time.Second
	defaultHTTPTimeout  = 10 * time.Second
)

// GPUVendor identifies a supported GPU hardware vendor.
type GPUVendor string

const (
	GPUVendorNVIDIA GPUVendor = "nvidia"
	GPUVendorAMD    GPUVendor = "amd"
)

// GPUDevice describes one detected GPU device.
type GPUDevice struct {
	Vendor        GPUVendor `json:"vendor"`
	PCIAddress    string    `json:"pci_address,omitempty"`
	DeviceID      string    `json:"device_id,omitempty"`
	Name          string    `json:"name,omitempty"`
	DriverVersion string    `json:"driver_version,omitempty"`
	Memory        string    `json:"memory,omitempty"`
}

// GPUReport summarizes detected GPUs, the runtime Ollama will use, and
// actionable hints when hardware is present without a usable driver.
type GPUReport struct {
	GPUs     []GPUDevice `json:"gpus,omitempty"`
	Runtime  string      `json:"runtime"`
	DriverOK bool        `json:"driver_ok"`
	Hints    []string    `json:"hints,omitempty"`
}

// Probe injects filesystem, command, and HTTP dependencies so detection,
// planning, pulling, and doctor checks remain pure and fixture-testable.
type Probe struct {
	FSRoot     string
	GOOS       string
	Getenv     func(key string) string
	Stat       func(path string) (os.FileInfo, error)
	ReadFile   func(path string) ([]byte, error)
	ReadDir    func(path string) ([]os.DirEntry, error)
	LookPath   func(name string) (string, error)
	Run        func(ctx context.Context, name string, args ...string) ([]byte, error)
	HTTPClient *http.Client
}

// DefaultProbe returns a Probe wired to the local operating system.
func DefaultProbe() Probe {
	return Probe{
		FSRoot:   "/",
		GOOS:     runtime.GOOS,
		Getenv:   os.Getenv,
		Stat:     os.Stat,
		ReadFile: os.ReadFile,
		ReadDir:  os.ReadDir,
		LookPath: exec.LookPath,
		Run: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			cmd := exec.CommandContext(ctx, name, args...)
			cmd.WaitDelay = time.Second
			// Stdout only so diagnostics never capture stderr.
			return cmd.Output()
		},
		HTTPClient: &http.Client{Timeout: defaultHTTPTimeout},
	}
}

func (p Probe) withDefaults() Probe {
	fsBacked := p.FSRoot != ""
	if p.FSRoot == "" {
		p.FSRoot = "/"
	}
	if p.GOOS == "" {
		p.GOOS = runtime.GOOS
	}
	if p.Getenv == nil {
		p.Getenv = func(string) string { return "" }
	}
	if p.Stat == nil {
		if fsBacked {
			p.Stat = os.Stat
		} else {
			p.Stat = func(string) (os.FileInfo, error) { return nil, os.ErrNotExist }
		}
	}
	if p.ReadFile == nil {
		if fsBacked {
			p.ReadFile = os.ReadFile
		} else {
			p.ReadFile = func(string) ([]byte, error) { return nil, os.ErrNotExist }
		}
	}
	if p.ReadDir == nil {
		if fsBacked {
			p.ReadDir = os.ReadDir
		} else {
			p.ReadDir = func(string) ([]os.DirEntry, error) { return nil, os.ErrNotExist }
		}
	}
	if p.LookPath == nil {
		p.LookPath = func(string) (string, error) { return "", exec.ErrNotFound }
	}
	if p.Run == nil {
		p.Run = func(context.Context, string, ...string) ([]byte, error) {
			return nil, errors.New("probe runner not configured")
		}
	}
	if p.HTTPClient == nil {
		p.HTTPClient = &http.Client{Timeout: defaultHTTPTimeout}
	}
	return p
}

func (p Probe) fsPath(abs string) string {
	trimmed := strings.TrimPrefix(abs, "/")
	if p.FSRoot == "" || p.FSRoot == "/" {
		return "/" + trimmed
	}
	return filepath.Join(p.FSRoot, filepath.FromSlash(trimmed))
}

// DetectGPUs inspects sysfs PCI devices, NVIDIA driver tooling, and AMD ROCm
// kernel/device interfaces to determine what hardware acceleration Ollama will
// use. It is strictly read-only and never installs drivers.
func DetectGPUs(ctx context.Context, probe Probe) GPUReport {
	probe = probe.withDefaults()
	report := GPUReport{
		Runtime: "CPU only",
	}

	nvidiaPCI, amdPCI := scanPCIDisplayDevices(probe)

	var (
		runtimes     []string
		nvidiaReady  bool
		amdReady     bool
		nvidiaGPUs   []GPUDevice
		nvidiaDriver string
	)

	// Probe nvidia-smi whenever NVIDIA PCI hardware is found, or if nvidia-smi
	// is on PATH (e.g. WSL2 / container passthrough without full sysfs).
	if len(nvidiaPCI) > 0 || hasBinary(probe, "nvidia-smi") {
		parsed, ok := queryNvidiaSMI(ctx, probe)
		if ok && len(parsed) > 0 {
			nvidiaReady = true
			nvidiaDriver = parsed[0].DriverVersion
			for i := range parsed {
				if i < len(nvidiaPCI) {
					parsed[i].PCIAddress = nvidiaPCI[i].PCIAddress
					parsed[i].DeviceID = nvidiaPCI[i].DeviceID
				}
			}
			nvidiaGPUs = parsed
			if nvidiaDriver != "" {
				runtimes = append(runtimes, fmt.Sprintf("NVIDIA CUDA (driver %s)", nvidiaDriver))
			} else {
				runtimes = append(runtimes, "NVIDIA CUDA")
			}
		} else if len(nvidiaPCI) > 0 {
			nvidiaGPUs = nvidiaPCI
			report.Hints = append(report.Hints, nvidiaDriverHint(probe))
		}
	}

	report.GPUs = append(report.GPUs, nvidiaGPUs...)

	if len(amdPCI) > 0 {
		report.GPUs = append(report.GPUs, amdPCI...)
		_, amdgpuErr := probe.Stat(probe.fsPath("/sys/module/amdgpu"))
		_, kfdErr := probe.Stat(probe.fsPath("/dev/kfd"))
		hasAmdgpu := amdgpuErr == nil
		hasKFD := kfdErr == nil

		if hasAmdgpu && hasKFD {
			amdReady = true
			runtimes = append(runtimes, "AMD ROCm")
		} else {
			report.Hints = append(report.Hints, amdDriverHint(hasAmdgpu, hasKFD))
		}
	}

	if len(runtimes) > 0 {
		report.Runtime = strings.Join(runtimes, ", ")
		report.DriverOK = nvidiaReady || amdReady
	}

	return report
}

func scanPCIDisplayDevices(probe Probe) (nvidia []GPUDevice, amd []GPUDevice) {
	pciDir := probe.fsPath("/sys/bus/pci/devices")
	entries, err := probe.ReadDir(pciDir)
	if err != nil {
		return nil, nil
	}

	names := make([]string, 0, len(entries))
	for _, entry := range entries {
		names = append(names, entry.Name())
	}
	sort.Strings(names)

	for _, addr := range names {
		devDir := filepath.Join(pciDir, addr)
		classVal := readTrimmedLower(probe, filepath.Join(devDir, "class"))
		if !strings.HasPrefix(classVal, "0x03") {
			continue
		}
		vendorVal := readTrimmedLower(probe, filepath.Join(devDir, "vendor"))
		deviceVal := readTrimmedLower(probe, filepath.Join(devDir, "device"))
		switch vendorVal {
		case pciVendorNVIDIA:
			nvidia = append(nvidia, GPUDevice{
				Vendor:     GPUVendorNVIDIA,
				PCIAddress: addr,
				DeviceID:   deviceVal,
			})
		case pciVendorAMD:
			amd = append(amd, GPUDevice{
				Vendor:     GPUVendorAMD,
				PCIAddress: addr,
				DeviceID:   deviceVal,
			})
		}
	}
	return nvidia, amd
}

func queryNvidiaSMI(ctx context.Context, probe Probe) ([]GPUDevice, bool) {
	bin, err := probe.LookPath("nvidia-smi")
	if err != nil || strings.TrimSpace(bin) == "" {
		return nil, false
	}
	cmdCtx, cancel := context.WithTimeout(ctx, probeCommandTimeout)
	defer cancel()
	out, err := probe.Run(cmdCtx, bin, "--query-gpu=name,driver_version,memory.total", "--format=csv,noheader")
	if err != nil {
		return nil, false
	}
	return ParseNvidiaSMIOutput(string(out)), true
}

// ParseNvidiaSMIOutput parses CSV output from
// `nvidia-smi --query-gpu=name,driver_version,memory.total --format=csv,noheader`.
func ParseNvidiaSMIOutput(raw string) []GPUDevice {
	var devices []GPUDevice
	for _, line := range strings.Split(raw, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		parts := strings.Split(line, ",")
		if len(parts) < 2 {
			continue
		}
		dev := GPUDevice{
			Vendor:        GPUVendorNVIDIA,
			Name:          strings.TrimSpace(parts[0]),
			DriverVersion: strings.TrimSpace(parts[1]),
		}
		if len(parts) >= 3 {
			dev.Memory = strings.TrimSpace(parts[2])
		}
		if dev.Name != "" {
			devices = append(devices, dev)
		}
	}
	return devices
}

func nvidiaDriverHint(probe Probe) string {
	distro := detectDistroID(probe)
	switch distro {
	case "ubuntu":
		return "NVIDIA GPU detected without nvidia-smi; install drivers manually with `sudo ubuntu-drivers install` and reboot (DevEx never installs GPU drivers automatically)."
	case "debian":
		return "NVIDIA GPU detected without nvidia-smi; install drivers manually following https://wiki.debian.org/NvidiaGraphicsDrivers and reboot (DevEx never installs GPU drivers automatically)."
	default:
		return "NVIDIA GPU detected without nvidia-smi; install NVIDIA drivers manually (Ubuntu: `sudo ubuntu-drivers install`; Debian: https://wiki.debian.org/NvidiaGraphicsDrivers) and reboot."
	}
}

func amdDriverHint(hasAmdgpu, hasKFD bool) string {
	var missing []string
	if !hasAmdgpu {
		missing = append(missing, "amdgpu kernel module")
	}
	if !hasKFD {
		missing = append(missing, "/dev/kfd")
	}
	return fmt.Sprintf(
		"AMD GPU detected but ROCm runtime is not ready (missing %s); follow https://rocm.docs.amd.com/projects/install-on-linux/en/latest/tutorial/quick-start.html and ensure the ollama user is in the render and video groups (DevEx never installs GPU drivers automatically).",
		strings.Join(missing, " and "),
	)
}

func detectDistroID(probe Probe) string {
	data, err := probe.ReadFile(probe.fsPath("/etc/os-release"))
	if err != nil {
		return ""
	}
	var id, idLike string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "ID=") {
			id = strings.ToLower(strings.Trim(strings.TrimPrefix(line, "ID="), `"'`))
		} else if strings.HasPrefix(line, "ID_LIKE=") {
			idLike = strings.ToLower(strings.Trim(strings.TrimPrefix(line, "ID_LIKE="), `"'`))
		}
	}
	if id == "ubuntu" || strings.Contains(idLike, "ubuntu") {
		return "ubuntu"
	}
	if id == "debian" || strings.Contains(idLike, "debian") {
		return "debian"
	}
	return id
}

func readTrimmedLower(probe Probe, path string) string {
	data, err := probe.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(string(data)))
}

func hasBinary(probe Probe, name string) bool {
	p, err := probe.LookPath(name)
	return err == nil && strings.TrimSpace(p) != ""
}
