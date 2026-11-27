package localmodels

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

const (
	maxModelNameLength  = 256
	maxManifestBodySize = 4 << 20 // 4 MiB
	maxTagsBodySize     = 4 << 20 // 4 MiB
	localAPITimeout     = 2 * time.Second
)

var validModelNameRegexp = regexp.MustCompile(`^([a-z0-9][a-z0-9._-]*/)?[a-z0-9][a-z0-9._-]*(?::[a-zA-Z0-9][a-zA-Z0-9._-]*)?$`)

// ValidateModelName strictly validates an Ollama model reference before it can
// ever reach a registry URL or subprocess argument list.
func ValidateModelName(name string) error {
	if name == "" || strings.TrimSpace(name) != name {
		return fmt.Errorf("invalid model name %q: must be non-empty without surrounding whitespace", name)
	}
	if len(name) > maxModelNameLength {
		return fmt.Errorf("invalid model name %q: exceeds %d characters", name, maxModelNameLength)
	}
	if strings.Contains(name, "..") {
		return fmt.Errorf("invalid model name %q: path traversal sequence is not allowed", name)
	}
	if !validModelNameRegexp.MatchString(name) {
		return fmt.Errorf("invalid model name %q: expected [namespace/]model[:tag] using lowercase letters, digits, '.', '_', '-'", name)
	}
	return nil
}

// ParseModelRef splits a validated Ollama model name into (namespace, model, tag).
// Unqualified names default to namespace "library"; untagged names default to "latest".
func ParseModelRef(name string) (namespace, model, tag string, err error) {
	if err := ValidateModelName(name); err != nil {
		return "", "", "", err
	}
	rest := name
	tag = "latest"
	if idx := strings.LastIndex(rest, ":"); idx >= 0 {
		tag = rest[idx+1:]
		rest = rest[:idx]
	}
	namespace = "library"
	model = rest
	if ns, m, ok := strings.Cut(rest, "/"); ok {
		namespace = ns
		model = m
	}
	return namespace, model, tag, nil
}

// CanonicalModelName returns name with ":latest" appended when no tag is given.
func CanonicalModelName(name string) string {
	if strings.Contains(name, ":") {
		return name
	}
	return name + ":latest"
}

type ociManifest struct {
	Config struct {
		Size int64 `json:"size"`
	} `json:"config"`
	Layers []struct {
		Size int64 `json:"size"`
	} `json:"layers"`
}

// FetchModelSize queries the Ollama OCI registry manifest for modelName and
// returns the total download size in bytes (config size plus all layer sizes).
func FetchModelSize(ctx context.Context, client *http.Client, registryBaseURL, modelName string) (int64, error) {
	namespace, model, tag, err := ParseModelRef(modelName)
	if err != nil {
		return 0, err
	}
	if strings.TrimSpace(registryBaseURL) == "" {
		registryBaseURL = DefaultRegistryBaseURL
	}
	if client == nil {
		client = &http.Client{Timeout: defaultHTTPTimeout}
	}

	endpoint := fmt.Sprintf("%s/v2/%s/%s/manifests/%s",
		strings.TrimRight(registryBaseURL, "/"),
		url.PathEscape(namespace),
		url.PathEscape(model),
		url.PathEscape(tag),
	)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return 0, fmt.Errorf("build manifest request for %s: %w", modelName, err)
	}
	req.Header.Set("Accept", DockerManifestV2MediaType)

	resp, err := client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("fetch manifest for %s: %w", modelName, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("manifest for %s returned HTTP %d", modelName, resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxManifestBodySize))
	if err != nil {
		return 0, fmt.Errorf("read manifest for %s: %w", modelName, err)
	}

	var manifest ociManifest
	if err := json.Unmarshal(body, &manifest); err != nil {
		return 0, fmt.Errorf("decode manifest for %s: %w", modelName, err)
	}

	total := manifest.Config.Size
	for _, layer := range manifest.Layers {
		total += layer.Size
	}
	if total <= 0 {
		return 0, fmt.Errorf("manifest for %s reported no content size", modelName)
	}
	return total, nil
}

// FormatSize renders a byte count in concise decimal units matching Ollama CLI
// output (for example 4683087500 -> "4.7 GB").
func FormatSize(size int64) string {
	if size < 0 {
		return "size unknown"
	}
	const (
		kb = 1000.0
		mb = 1000.0 * kb
		gb = 1000.0 * mb
		tb = 1000.0 * gb
	)
	f := float64(size)
	switch {
	case f >= tb:
		return fmt.Sprintf("%.1f TB", f/tb)
	case f >= gb:
		return fmt.Sprintf("%.1f GB", f/gb)
	case f >= mb:
		return fmt.Sprintf("%.1f MB", f/mb)
	case f >= kb:
		return fmt.Sprintf("%.1f KB", f/kb)
	default:
		return fmt.Sprintf("%d B", size)
	}
}

type ollamaTagsResponse struct {
	Models []struct {
		Name string `json:"name"`
	} `json:"models"`
}

// ListInstalledModels queries the local Ollama HTTP API (falling back to
// `ollama list`) to find which models are already downloaded.
func ListInstalledModels(ctx context.Context, probe Probe, baseURL string) (map[string]bool, bool, error) {
	probe = probe.withDefaults()
	if strings.TrimSpace(baseURL) == "" {
		baseURL = resolveOllamaBaseURL(probe)
	}

	present, ok := fetchInstalledModelsFromAPI(ctx, probe.HTTPClient, baseURL)
	if ok {
		return present, true, nil
	}

	// Fall back to `ollama list` if the binary is installed.
	if bin, err := probe.LookPath("ollama"); err == nil && strings.TrimSpace(bin) != "" {
		cmdCtx, cancel := context.WithTimeout(ctx, probeCommandTimeout)
		out, runErr := probe.Run(cmdCtx, bin, "list")
		cancel()
		if runErr == nil {
			return parseOllamaListOutput(string(out)), true, nil
		}
	}

	return map[string]bool{}, false, nil
}

func fetchInstalledModelsFromAPI(ctx context.Context, client *http.Client, baseURL string) (map[string]bool, bool) {
	if client == nil {
		client = &http.Client{Timeout: localAPITimeout}
	}
	reqCtx, cancel := context.WithTimeout(ctx, localAPITimeout)
	defer cancel()

	endpoint := strings.TrimRight(baseURL, "/") + "/api/tags"
	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, false
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, false
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, false
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, maxTagsBodySize))
	if err != nil {
		return nil, false
	}
	var payload ollamaTagsResponse
	if err := json.Unmarshal(body, &payload); err != nil {
		return nil, false
	}
	present := make(map[string]bool, len(payload.Models))
	for _, m := range payload.Models {
		name := strings.TrimSpace(m.Name)
		if name != "" {
			present[CanonicalModelName(name)] = true
		}
	}
	return present, true
}

func parseOllamaListOutput(raw string) map[string]bool {
	present := map[string]bool{}
	lines := strings.Split(raw, "\n")
	for i, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		if i == 0 && strings.EqualFold(fields[0], "NAME") {
			continue
		}
		present[CanonicalModelName(fields[0])] = true
	}
	return present
}

// ModelPlanEntry describes the planned action for one configured local model.
type ModelPlanEntry struct {
	Name        string `json:"name"`
	Present     bool   `json:"present"`
	SizeBytes   int64  `json:"size_bytes,omitempty"`
	SizeKnown   bool   `json:"size_known"`
	SizeDisplay string `json:"size_display,omitempty"`
}

// Plan holds the read-only local model reconciliation plan.
type Plan struct {
	GPU       GPUReport        `json:"gpu"`
	Models    []ModelPlanEntry `json:"models"`
	Reachable bool             `json:"reachable"`
}

// PlanOptions configures endpoints used when building a local model plan.
type PlanOptions struct {
	OllamaBaseURL   string
	RegistryBaseURL string
	Offline         bool
}

// BuildPlan inspects local hardware, already-present Ollama models, and
// upstream registry sizes without modifying anything on disk or pulling models.
func BuildPlan(ctx context.Context, probe Probe, models []string, opts PlanOptions) (Plan, error) {
	probe = probe.withDefaults()
	for _, m := range models {
		if err := ValidateModelName(m); err != nil {
			return Plan{}, err
		}
	}

	gpu := DetectGPUs(ctx, probe)
	present, reachable, _ := ListInstalledModels(ctx, probe, opts.OllamaBaseURL)

	entries := make([]ModelPlanEntry, 0, len(models))
	for _, m := range models {
		entry := ModelPlanEntry{
			Name:    m,
			Present: present[CanonicalModelName(m)],
		}
		if !entry.Present {
			if opts.Offline {
				entry.SizeDisplay = "size unknown"
			} else {
				size, err := FetchModelSize(ctx, probe.HTTPClient, opts.RegistryBaseURL, m)
				if err != nil {
					entry.SizeDisplay = "size unknown"
				} else {
					entry.SizeBytes = size
					entry.SizeKnown = true
					entry.SizeDisplay = FormatSize(size)
				}
			}
		}
		entries = append(entries, entry)
	}

	return Plan{
		GPU:       gpu,
		Models:    entries,
		Reachable: reachable,
	}, nil
}

// FormatPlan renders a human-readable local model plan for `devex agent diff`
// and `devex agent apply --dry-run`.
func FormatPlan(plan Plan) string {
	var b strings.Builder
	b.WriteString("Local Models:\n")
	fmt.Fprintf(&b, "  GPU: %s\n", plan.GPU.Runtime)
	for _, hint := range plan.GPU.Hints {
		fmt.Fprintf(&b, "  Hint: %s\n", hint)
	}
	for _, entry := range plan.Models {
		if entry.Present {
			fmt.Fprintf(&b, "  - %s: already present\n", entry.Name)
			continue
		}
		sizeLabel := entry.SizeDisplay
		if sizeLabel == "" {
			sizeLabel = "size unknown"
		}
		fmt.Fprintf(&b, "  - %s: pull (%s)\n", entry.Name, sizeLabel)
	}
	return b.String()
}

// PullOptions configures how PullModels connects to the local Ollama service.
type PullOptions struct {
	OllamaBaseURL string
	DryRun        bool
}

// PullModels pulls every configured model that is not already present in the
// local Ollama runtime. In dry-run mode it validates model names and returns
// immediately with zero writes and zero subprocess calls. It never passes model
// names through a shell.
func PullModels(ctx context.Context, probe Probe, models []string, opts PullOptions, out io.Writer) error {
	if len(models) == 0 {
		return nil
	}
	for _, m := range models {
		if err := ValidateModelName(m); err != nil {
			return err
		}
	}
	if opts.DryRun {
		return nil
	}
	probe = probe.withDefaults()

	ollamaBin, err := probe.LookPath("ollama")
	if err != nil || strings.TrimSpace(ollamaBin) == "" {
		return fmt.Errorf("Ollama is not installed: run `devex install ollama`") //nolint:staticcheck // Ollama is a proper noun matching the CLI error contract
	}

	baseURL := strings.TrimSpace(opts.OllamaBaseURL)
	if baseURL == "" {
		baseURL = resolveOllamaBaseURL(probe)
	}
	present, reachable := fetchInstalledModelsFromAPI(ctx, probe.HTTPClient, baseURL)
	if !reachable {
		return fmt.Errorf("Ollama service is not reachable at %s: start it with `sudo systemctl start ollama`", baseURL) //nolint:staticcheck // Ollama is a proper noun
	}

	for _, m := range models {
		if present[CanonicalModelName(m)] {
			if out != nil {
				_, _ = fmt.Fprintf(out, "Local model %s: already present\n", m)
			}
			continue
		}
		if out != nil {
			_, _ = fmt.Fprintf(out, "Pulling local model %s...\n", m)
		}
		// Model names are strictly validated to begin with [a-z0-9] and are
		// passed directly as a single argv element with no shell.
		output, runErr := probe.Run(ctx, ollamaBin, "pull", m)
		if runErr != nil {
			detail := strings.TrimSpace(string(output))
			if detail != "" {
				return fmt.Errorf("failed to pull local model %s: %w (%s)", m, runErr, detail)
			}
			return fmt.Errorf("failed to pull local model %s: %w", m, runErr)
		}
		if out != nil {
			_, _ = fmt.Fprintf(out, "Pulled local model %s\n", m)
		}
	}
	return nil
}
