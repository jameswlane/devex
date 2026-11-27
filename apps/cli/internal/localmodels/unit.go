package localmodels

const (
	// OllamaUnitPath is the systemd service file written by the Linux Ollama install.
	OllamaUnitPath = "/etc/systemd/system/ollama.service"

	// DefaultOllamaHost is the localhost-only listen address configured for Ollama.
	DefaultOllamaHost = "127.0.0.1:11434"

	// DefaultOllamaBaseURL is the default local HTTP API endpoint for Ollama.
	DefaultOllamaBaseURL = "http://" + DefaultOllamaHost

	// DefaultRegistryBaseURL is the upstream Ollama OCI manifest registry.
	DefaultRegistryBaseURL = "https://registry.ollama.ai"

	// DockerManifestV2MediaType is the Accept header required for Ollama v2 manifests.
	DockerManifestV2MediaType = "application/vnd.docker.distribution.manifest.v2+json"
)

// RenderOllamaUnit returns the exact systemd service unit file written for
// Ollama on Linux. It binds exclusively to localhost (127.0.0.1:11434), runs
// as the dedicated ollama user/group, and restarts automatically.
func RenderOllamaUnit() string {
	return `[Unit]
Description=Ollama Service
After=network-online.target

[Service]
ExecStart=/usr/local/bin/ollama serve
User=ollama
Group=ollama
Restart=always
RestartSec=3
Environment="OLLAMA_HOST=127.0.0.1:11434"

[Install]
WantedBy=default.target
`
}
