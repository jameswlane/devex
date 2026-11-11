// Package inventory exports the software DevEx manages on a machine, with the
// versions the native package managers report, and reconciles DevEx's records
// with what is actually installed.
package inventory

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"sort"
	"strings"
	"time"

	"github.com/jameswlane/devex/apps/cli/internal/types"
)

// SchemaVersion identifies the export format.
const SchemaVersion = "devex.inventory/v1"

// Item states.
const (
	StateInstalled = "installed" // tracked and present natively
	StateMissing   = "missing"   // tracked but not present natively
	StateUnknown   = "unknown"   // the install method cannot be queried
)

// Item is one managed application.
type Item struct {
	Name       string   `json:"name"`
	Manager    string   `json:"manager"`
	Packages   []string `json:"packages"`
	Version    string   `json:"version,omitempty"`
	State      string   `json:"state"`
	ManagedBy  string   `json:"managed_by"`
	Definition bool     `json:"definition_recorded"`
	Detail     string   `json:"detail,omitempty"`
}

// Machine identifies the host without revealing it.
type Machine struct {
	ID       string `json:"id"`
	OS       string `json:"os"`
	Arch     string `json:"arch"`
	Hostname string `json:"hostname,omitempty"`
}

// Inventory is the export.
type Inventory struct {
	SchemaVersion string    `json:"schema_version"`
	GeneratedAt   time.Time `json:"generated_at"`
	DevexVersion  string    `json:"devex_version"`
	Machine       Machine   `json:"machine"`
	Items         []Item    `json:"items"`
	Drift         []Drift   `json:"drift"`
}

// Drift is a difference between DevEx's records and the machine.
type Drift struct {
	App    string `json:"app"`
	Kind   string `json:"kind"` // missing, no_definition
	Detail string `json:"detail"`
}

// Query reports whether a package is installed natively and its version.
type Query func(ctx context.Context, manager, pkg string) (installed bool, version string, err error)

// ErrUnsupported means the manager cannot be queried.
var ErrUnsupported = errors.New("unsupported install method")

// Options configures Build.
type Options struct {
	DevexVersion    string
	IncludeHostname bool
	Query           Query
	Now             func() time.Time
	MachineIDPath   string
	// Resolve returns the current catalog definition for an application
	// without a recorded one.
	Resolve func(name string) (types.CrossPlatformApp, bool)
}

// Reader is the part of the DevEx datastore Build reads.
type Reader interface {
	ListApps() ([]types.AppConfig, error)
	Get(key string) (string, error)
}

// Build lists tracked applications and checks each one natively.
func Build(ctx context.Context, repo Reader, options Options) (Inventory, error) {
	if options.Query == nil {
		options.Query = NativeQuery
	}
	if options.Now == nil {
		options.Now = time.Now
	}
	tracked, err := repo.ListApps()
	if err != nil {
		return Inventory{}, fmt.Errorf("read DevEx records: %w", err)
	}
	result := Inventory{
		SchemaVersion: SchemaVersion, GeneratedAt: options.Now().UTC(), DevexVersion: options.DevexVersion,
		Machine: machine(options), Items: []Item{}, Drift: []Drift{},
	}
	for _, record := range tracked {
		if err := ctx.Err(); err != nil {
			return result, err
		}
		item := Item{Name: record.Name, ManagedBy: "devex", Manager: "unknown", Packages: []string{}, State: StateUnknown}
		definition, found := recordedDefinition(repo, record.Name)
		item.Definition = found
		if !found {
			detail := "DevEx has no recorded definition; reinstall it with devex install to record one"
			if options.Resolve != nil {
				if current, ok := options.Resolve(record.Name); ok {
					definition = current
					detail += "; checked with the current catalog definition"
				}
			}
			result.Drift = append(result.Drift, Drift{App: record.Name, Kind: "no_definition", Detail: detail})
		}
		config := definition.GetOSConfig()
		if config.InstallMethod != "" {
			item.Manager = config.InstallMethod
		}
		item.Packages = packagesOf(config)
		check(ctx, &item, options.Query)
		if item.State == StateMissing {
			result.Drift = append(result.Drift, Drift{App: record.Name, Kind: "missing", Detail: "tracked by DevEx but not installed according to " + item.Manager})
		}
		result.Items = append(result.Items, item)
	}
	sort.Slice(result.Items, func(i, j int) bool { return result.Items[i].Name < result.Items[j].Name })
	sort.Slice(result.Drift, func(i, j int) bool { return result.Drift[i].App < result.Drift[j].App })
	return result, nil
}

func recordedDefinition(repo Reader, name string) (types.CrossPlatformApp, bool) {
	data, err := repo.Get("installed-app-definition:" + strings.ToLower(strings.TrimSpace(name)))
	if err != nil || data == "" {
		return types.CrossPlatformApp{Name: name}, false
	}
	var app types.CrossPlatformApp
	if json.Unmarshal([]byte(data), &app) != nil {
		return types.CrossPlatformApp{Name: name}, false
	}
	return app, true
}

func packagesOf(config types.OSConfig) []string {
	if len(config.Packages) > 0 {
		return append([]string(nil), config.Packages...)
	}
	fields := strings.Fields(config.InstallCommand)
	if len(fields) == 0 {
		return []string{}
	}
	return fields
}

// check queries every package; the item is installed only if all are.
func check(ctx context.Context, item *Item, query Query) {
	if len(item.Packages) == 0 || item.Manager == "unknown" {
		item.Detail = "no package recorded"
		return
	}
	versions := make([]string, 0, len(item.Packages))
	for _, pkg := range item.Packages {
		installed, version, err := query(ctx, item.Manager, pkg)
		if errors.Is(err, ErrUnsupported) {
			item.State, item.Detail = StateUnknown, item.Manager+" packages are not queried"
			return
		}
		if err != nil {
			item.State, item.Detail = StateUnknown, "query failed: "+firstLine(err.Error())
			return
		}
		if !installed {
			item.State = StateMissing
			item.Detail = pkg + " is not installed"
			return
		}
		if version != "" {
			versions = append(versions, version)
		}
	}
	item.State = StateInstalled
	if len(versions) > 0 {
		item.Version = versions[0]
		if len(item.Packages) > 1 {
			item.Version = strings.Join(versions, ", ")
		}
	}
}

func firstLine(text string) string {
	line, _, _ := strings.Cut(strings.TrimSpace(text), "\n")
	return line
}

func machine(options Options) Machine {
	path := options.MachineIDPath
	if path == "" {
		path = "/etc/machine-id"
	}
	id := "unavailable"
	if data, err := os.ReadFile(path); err == nil && strings.TrimSpace(string(data)) != "" {
		// A salted hash identifies the machine across exports without revealing
		// its machine-id.
		sum := sha256.Sum256([]byte("devex-inventory:" + strings.TrimSpace(string(data))))
		id = hex.EncodeToString(sum[:])[:16]
	}
	result := Machine{ID: id, OS: runtime.GOOS, Arch: runtime.GOARCH}
	if options.IncludeHostname {
		result.Hostname, _ = os.Hostname()
	}
	return result
}

// NativeQuery asks the package manager.
func NativeQuery(ctx context.Context, manager, pkg string) (bool, string, error) {
	if strings.HasPrefix(pkg, "-") {
		return false, "", fmt.Errorf("invalid package name %q", pkg)
	}
	switch manager {
	case "apt", "deb":
		output, err := exec.CommandContext(ctx, "dpkg-query", "-W", "-f=${Status}\t${Version}", "--", pkg).Output()
		if err != nil {
			var exit *exec.ExitError
			if errors.As(err, &exit) {
				return false, "", nil
			}
			return false, "", err
		}
		status, version, _ := strings.Cut(string(output), "\t")
		return strings.HasSuffix(strings.TrimSpace(status), "installed") && !strings.Contains(status, "not-installed"), strings.TrimSpace(version), nil
	case "snap":
		output, err := exec.CommandContext(ctx, "snap", "list", "--", pkg).Output()
		if err != nil {
			return false, "", nil
		}
		lines := strings.Split(strings.TrimSpace(string(output)), "\n")
		if len(lines) < 2 {
			return false, "", nil
		}
		fields := strings.Fields(lines[1])
		if len(fields) < 2 {
			return true, "", nil
		}
		return true, fields[1], nil
	case "flatpak":
		output, err := exec.CommandContext(ctx, "flatpak", "info", "--show-version", "--", pkg).Output()
		if err != nil {
			return false, "", nil
		}
		return true, strings.TrimSpace(string(output)), nil
	}
	return false, "", ErrUnsupported
}
