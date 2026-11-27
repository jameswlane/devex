package localmodels

import (
	"context"
	"net"
	"net/url"
	"regexp"
	"strings"
)

// RuntimeStatus captures the health of the local Ollama runtime for diagnostics.
type RuntimeStatus struct {
	Installed     bool      `json:"installed"`
	BinaryPath    string    `json:"binary_path,omitempty"`
	Version       string    `json:"version,omitempty"`
	ServiceActive bool      `json:"service_active"`
	ServiceState  string    `json:"service_state,omitempty"`
	BoundAddress  string    `json:"bound_address,omitempty"`
	APIReachable  bool      `json:"api_reachable"`
	GPU           GPUReport `json:"gpu"`
}

var ollamaVersionRegexp = regexp.MustCompile(`\bv?\d+(?:\.\d+)+(?:-[0-9A-Za-z.]+)?`)

// InspectRuntime inspects Ollama installation state, systemd service status,
// configured listen address, local API reachability, and GPU acceleration.
func InspectRuntime(ctx context.Context, probe Probe) RuntimeStatus {
	probe = probe.withDefaults()
	status := RuntimeStatus{
		BoundAddress: resolveOllamaHost(probe),
		GPU:          DetectGPUs(ctx, probe),
	}

	if bin, err := probe.LookPath("ollama"); err == nil && strings.TrimSpace(bin) != "" {
		status.Installed = true
		status.BinaryPath = bin

		cmdCtx, cancel := context.WithTimeout(ctx, probeCommandTimeout)
		out, runErr := probe.Run(cmdCtx, bin, "--version")
		cancel()
		if runErr == nil {
			status.Version = ollamaVersionRegexp.FindString(string(out))
		}
	}

	if probe.GOOS == "linux" {
		status.ServiceState, status.ServiceActive = inspectSystemdService(ctx, probe)
	} else {
		status.ServiceState = "n/a"
	}

	if status.Installed || status.ServiceActive {
		_, reachable := fetchInstalledModelsFromAPI(ctx, probe.HTTPClient, hostToProbeBaseURL(status.BoundAddress))
		status.APIReachable = reachable
	}

	return status
}

func inspectSystemdService(ctx context.Context, probe Probe) (string, bool) {
	if _, err := probe.Stat(probe.fsPath("/run/systemd/system")); err != nil {
		return "unavailable", false
	}
	systemctl, err := probe.LookPath("systemctl")
	if err != nil || strings.TrimSpace(systemctl) == "" {
		return "unknown", false
	}
	cmdCtx, cancel := context.WithTimeout(ctx, probeCommandTimeout)
	defer cancel()
	out, runErr := probe.Run(cmdCtx, systemctl, "is-active", "ollama")
	state := strings.TrimSpace(string(out))
	if runErr == nil && state == "active" {
		return "active", true
	}
	if state == "" {
		return "inactive", false
	}
	return state, false
}

func resolveOllamaHost(probe Probe) string {
	if envHost := strings.TrimSpace(probe.Getenv("OLLAMA_HOST")); envHost != "" {
		return normalizeOllamaHost(envHost)
	}
	if unitHost := parseUnitOllamaHost(probe); unitHost != "" {
		return normalizeOllamaHost(unitHost)
	}
	return DefaultOllamaHost
}

func resolveOllamaBaseURL(probe Probe) string {
	return hostToProbeBaseURL(resolveOllamaHost(probe))
}

func parseUnitOllamaHost(probe Probe) string {
	data, err := probe.ReadFile(probe.fsPath(OllamaUnitPath))
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "Environment=") {
			continue
		}
		val := strings.TrimPrefix(line, "Environment=")
		val = strings.Trim(val, `"'`)
		if strings.HasPrefix(val, "OLLAMA_HOST=") {
			return strings.TrimSpace(strings.TrimPrefix(val, "OLLAMA_HOST="))
		}
	}
	return ""
}

func normalizeOllamaHost(raw string) string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return DefaultOllamaHost
	}
	if strings.Contains(raw, "://") {
		if u, err := url.Parse(raw); err == nil && u.Host != "" {
			if _, _, splitErr := net.SplitHostPort(u.Host); splitErr == nil {
				return u.Host
			}
			return net.JoinHostPort(u.Hostname(), "11434")
		}
	}
	if _, _, err := net.SplitHostPort(raw); err == nil {
		return raw
	}
	if strings.HasPrefix(raw, ":") {
		return "127.0.0.1" + raw
	}
	return net.JoinHostPort(raw, "11434")
}

func hostToProbeBaseURL(host string) string {
	h, port, err := net.SplitHostPort(normalizeOllamaHost(host))
	if err != nil {
		return DefaultOllamaBaseURL
	}
	if h == "" || h == "0.0.0.0" || h == "::" {
		h = "127.0.0.1"
	}
	return "http://" + net.JoinHostPort(h, port)
}
