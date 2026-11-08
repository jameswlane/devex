package main

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

const extensionLimit = 32 << 20

// ExtensionManager resolves bundles without writing or executing native tools.
// Fetch is injectable; tests never need the public extension service.
type ExtensionManager struct {
	Fetch func(context.Context, string) ([]byte, error)
}

type extensionDeclaration struct {
	ShellVersion string          `json:"shell_version"`
	Extensions   []extensionSpec `json:"extensions"`
}
type extensionSpec struct {
	UUID     string             `json:"uuid"`
	Archive  string             `json:"archive,omitempty"`
	Settings []extensionSetting `json:"settings,omitempty"`
}
type extensionSetting struct {
	Schema string `json:"schema"`
	Key    string `json:"key"`
	Value  string `json:"value"`
}
type extensionBundle struct {
	spec extensionSpec
	data []byte
	path string
}
type extensionPlan struct {
	commands [][]string
	bundles  []extensionBundle
	shell    string
}

func NewExtensionManager() *ExtensionManager { return &ExtensionManager{Fetch: fetchExtension} }

func fetchExtension(ctx context.Context, address string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return nil, err
	}
	client := &http.Client{Timeout: 30 * time.Second}
	response, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("extension download: HTTP %d", response.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, extensionLimit+1))
	if len(data) > extensionLimit {
		return nil, fmt.Errorf("extension response exceeds 32 MiB")
	}
	return data, err
}

var extensionUUID = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_.@+-]*$`)
var shellNumber = regexp.MustCompile(`^([0-9]+)(?:\.([0-9]+))?(?:\.[0-9]+)?$`)

// GNOME 40+ compatibility is major-based; older shells use major.minor.
func shellCompatibility(version string) (string, error) {
	match := shellNumber.FindStringSubmatch(version)
	if match == nil {
		return "", fmt.Errorf("invalid GNOME Shell version %q", version)
	}
	major, err := strconv.Atoi(match[1])
	if err != nil {
		return "", err
	}
	if major >= 40 {
		return match[1], nil
	}
	if match[2] == "" {
		return "", fmt.Errorf("legacy Shell version requires major.minor")
	}
	return match[1] + "." + match[2], nil
}

func (em *ExtensionManager) resolve(ctx context.Context, filename string) (*extensionPlan, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, err
	}
	var declaration extensionDeclaration
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(&declaration); err != nil {
		return nil, fmt.Errorf("extension declaration: %w", err)
	}
	var extra any
	if decoder.Decode(&extra) != io.EOF {
		return nil, fmt.Errorf("extension declaration must contain one JSON value")
	}
	shell, err := shellCompatibility(declaration.ShellVersion)
	if err != nil {
		return nil, err
	}
	if len(declaration.Extensions) == 0 {
		return nil, fmt.Errorf("declare at least one extension")
	}
	cache, err := os.UserCacheDir()
	if err != nil {
		return nil, err
	}
	plan := &extensionPlan{shell: shell}
	seen := map[string]bool{}
	for _, spec := range declaration.Extensions {
		if !extensionUUID.MatchString(spec.UUID) || seen[spec.UUID] {
			return nil, fmt.Errorf("invalid or duplicate extension UUID %q", spec.UUID)
		}
		seen[spec.UUID] = true
		for _, setting := range spec.Settings {
			if setting.Schema == "" || strings.HasPrefix(setting.Schema, "-") || setting.Key == "" || strings.HasPrefix(setting.Key, "-") {
				return nil, fmt.Errorf("extension %s requires schema and key", spec.UUID)
			}
		}
		source := spec.Archive
		var bundle []byte
		if source != "" {
			if !filepath.IsAbs(source) {
				source = filepath.Join(filepath.Dir(filename), source)
			}
			source, err = filepath.Abs(source)
			if err != nil {
				return nil, err
			}
			file, openErr := os.Open(source)
			if openErr != nil {
				return nil, openErr
			}
			bundle, err = io.ReadAll(io.LimitReader(file, extensionLimit+1))
			_ = file.Close()
		} else {
			address := "https://extensions.gnome.org/extension-info/?" + url.Values{"uuid": {spec.UUID}, "shell_version": {declaration.ShellVersion}}.Encode()
			var details []byte
			details, err = em.Fetch(ctx, address)
			if err != nil {
				return nil, fmt.Errorf("resolve %s: %w", spec.UUID, err)
			}
			var info struct {
				UUID        string `json:"uuid"`
				DownloadURL string `json:"download_url"`
			}
			if err = json.Unmarshal(details, &info); err != nil {
				return nil, err
			}
			if info.UUID != spec.UUID || info.DownloadURL == "" {
				return nil, fmt.Errorf("no compatible release for %s on Shell %s", spec.UUID, shell)
			}
			base, _ := url.Parse("https://extensions.gnome.org")
			relative, parseErr := url.Parse(info.DownloadURL)
			if parseErr != nil {
				return nil, parseErr
			}
			resolved := base.ResolveReference(relative)
			if resolved.Scheme != "https" || resolved.Host != "extensions.gnome.org" {
				return nil, fmt.Errorf("untrusted extension download URL")
			}
			source = resolved.String()
			bundle, err = em.Fetch(ctx, source)
		}
		if err != nil {
			return nil, err
		}
		if len(bundle) > extensionLimit {
			return nil, fmt.Errorf("extension archive exceeds 32 MiB")
		}
		if err = validateExtensionArchive(bundle, spec.UUID, shell); err != nil {
			return nil, err
		}
		hash := fmt.Sprintf("%x", sha256.Sum256(bundle))
		path := filepath.Join(cache, "devex", "gnome-extensions", hash+".zip")
		plan.commands = append(plan.commands, []string{"extension-bundle", spec.UUID, source, hash, shell}, []string{"gnome-extensions", "install", "--force", path}, []string{"gnome-extensions", "enable", spec.UUID})
		for _, setting := range spec.Settings {
			command := []string{"gsettings"}
			zipReader, _ := zip.NewReader(bytes.NewReader(bundle), int64(len(bundle)))
			for _, entry := range zipReader.File {
				if entry.Name == "schemas/gschemas.compiled" {
					dataHome := os.Getenv("XDG_DATA_HOME")
					if dataHome == "" {
						home, homeErr := os.UserHomeDir()
						if homeErr != nil {
							return nil, homeErr
						}
						dataHome = filepath.Join(home, ".local", "share")
					}
					command = append(command, "--schemadir", filepath.Join(dataHome, "gnome-shell", "extensions", spec.UUID, "schemas"))
					break
				}
			}
			command = append(command, "set", setting.Schema, setting.Key, setting.Value)
			plan.commands = append(plan.commands, command)
		}
		plan.bundles = append(plan.bundles, extensionBundle{spec: spec, data: bundle, path: path})
	}
	return plan, nil
}

func validateExtensionArchive(data []byte, uuid, shell string) error {
	reader, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return fmt.Errorf("invalid extension zip: %w", err)
	}
	found := false
	for _, file := range reader.File {
		if strings.Contains(file.Name, `\`) || strings.HasPrefix(file.Name, "/") || strings.Contains("/"+file.Name+"/", "/../") || file.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("unsafe extension archive entry %q", file.Name)
		}
		if file.Name != "metadata.json" {
			continue
		}
		if found {
			return fmt.Errorf("duplicate extension metadata")
		}
		found = true
		stream, openErr := file.Open()
		if openErr != nil {
			return openErr
		}
		metadata, readErr := io.ReadAll(io.LimitReader(stream, 1<<20))
		_ = stream.Close()
		if readErr != nil {
			return readErr
		}
		var info struct {
			UUID     string   `json:"uuid"`
			Versions []string `json:"shell-version"`
		}
		if err = json.Unmarshal(metadata, &info); err != nil {
			return err
		}
		if info.UUID != uuid {
			return fmt.Errorf("extension archive UUID mismatch: wanted %s, got %s", uuid, info.UUID)
		}
		compatible := false
		for _, version := range info.Versions {
			key, e := shellCompatibility(version)
			if e == nil && key == shell {
				compatible = true
			}
		}
		if !compatible {
			return fmt.Errorf("extension %s incompatible with Shell %s", uuid, shell)
		}
	}
	if !found {
		return fmt.Errorf("extension archive lacks metadata.json")
	}
	return nil
}

func extensionArgs(args []string) (string, string, error) {
	var filename, expected string
	for len(args) > 0 {
		if len(args) < 2 {
			return "", "", fmt.Errorf("use --extensions FILE [--expected-plan HASH]")
		}
		switch args[0] {
		case "--extensions":
			if filename != "" {
				return "", "", fmt.Errorf("duplicate --extensions")
			}
			filename = args[1]
		case "--expected-plan":
			if expected != "" {
				return "", "", fmt.Errorf("duplicate --expected-plan")
			}
			expected = args[1]
		default:
			return "", "", fmt.Errorf("unknown extension argument %s", args[0])
		}
		args = args[2:]
	}
	if filename == "" {
		return "", "", fmt.Errorf("declare extensions with --extensions FILE; preview with describe install-extensions")
	}
	return filename, expected, nil
}

func (em *ExtensionManager) Describe(ctx context.Context, args []string) error {
	filename, _, err := extensionArgs(args)
	if err != nil {
		return err
	}
	plan, err := em.resolve(ctx, filename)
	if err != nil {
		return err
	}
	data, err := json.Marshal(plan.commands)
	if err != nil {
		return err
	}
	fmt.Println(string(data))
	return nil
}

// InstallExtensions requires the hash of an explicit, side-effect-free preview.
func (em *ExtensionManager) InstallExtensions(ctx context.Context, args []string) error {
	filename, expected, err := extensionArgs(args)
	if err != nil {
		return err
	}
	if expected == "" {
		return fmt.Errorf("extension installation requires accepted --expected-plan HASH; preview with describe install-extensions")
	}
	plan, err := em.resolve(ctx, filename)
	if err != nil {
		return err
	}
	data, err := json.Marshal(plan.commands)
	if err != nil {
		return err
	}
	if expected != fmt.Sprintf("%x", sha256.Sum256(data)) {
		return fmt.Errorf("extension plan changed; preview and confirm again")
	}
	output, err := exec.CommandContext(ctx, "gdbus", "call", "--session", "--dest", "org.gnome.Shell", "--object-path", "/org/gnome/Shell", "--method", "org.freedesktop.DBus.Properties.Get", "org.gnome.Shell", "ShellVersion").Output()
	if err != nil {
		return fmt.Errorf("detect running GNOME Shell version through session D-Bus: %w", err)
	}
	version := regexp.MustCompile(`^\(<'([^']+)'>,\)\s*$`).FindStringSubmatch(string(output))
	if version == nil {
		return fmt.Errorf("unrecognized running GNOME Shell version response %q", string(output))
	}
	running, err := shellCompatibility(version[1])
	if err != nil {
		return err
	}

	if running != plan.shell {
		return fmt.Errorf("running GNOME Shell %s differs from accepted %s", running, plan.shell)
	}
	for _, bundle := range plan.bundles {
		if err = os.MkdirAll(filepath.Dir(bundle.path), 0o700); err != nil {
			return err
		}
		file, createErr := os.CreateTemp(filepath.Dir(bundle.path), "bundle-*.zip")
		if createErr != nil {
			return createErr
		}
		name := file.Name()
		_, err = file.Write(bundle.data)
		closeErr := file.Close()
		if err == nil {
			err = closeErr
		}
		if err == nil {
			err = os.Rename(name, bundle.path)
		}
		if err != nil {
			_ = os.Remove(name)
			return err
		}
	}
	for _, command := range plan.commands {
		if command[0] == "extension-bundle" {
			continue
		}
		cmd := exec.CommandContext(ctx, command[0], command[1:]...)
		output, err = cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("extension operation %v failed: %w: %s; earlier changes remain; log out and back in, then retry the accepted plan if Shell has not discovered a newly installed extension", command, err, strings.TrimSpace(string(output)))
		}
	}
	fmt.Fprintln(os.Stderr, "Extensions installed and enable requested. Log out and back in to load new or replaced code (required on Wayland); no session restart was performed.")
	return nil
}
