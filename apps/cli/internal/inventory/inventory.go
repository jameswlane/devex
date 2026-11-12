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
	Name       string   `json:"name" yaml:"name"`
	Manager    string   `json:"manager" yaml:"manager"`
	Packages   []string `json:"packages" yaml:"packages"`
	Version    string   `json:"version,omitempty" yaml:"version,omitempty"`
	State      string   `json:"state" yaml:"state"`
	ManagedBy  string   `json:"managed_by" yaml:"managed_by"`
	Definition bool     `json:"definition_recorded" yaml:"definition_recorded"`
	Detail     string   `json:"detail,omitempty" yaml:"detail,omitempty"`
}

// Machine identifies the host without revealing it.
type Machine struct {
	ID       string `json:"id" yaml:"id"`
	OS       string `json:"os" yaml:"os"`
	Arch     string `json:"arch" yaml:"arch"`
	Hostname string `json:"hostname,omitempty" yaml:"hostname,omitempty"`
}

// Inventory is the export.
type Inventory struct {
	SchemaVersion string    `json:"schema_version" yaml:"schema_version"`
	GeneratedAt   time.Time `json:"generated_at" yaml:"generated_at"`
	DevexVersion  string    `json:"devex_version" yaml:"devex_version"`
	Machine       Machine   `json:"machine" yaml:"machine"`
	Items         []Item    `json:"items" yaml:"items"`
	Drift         []Drift   `json:"drift" yaml:"drift"`
}

// Drift is a difference between DevEx's records and the machine.
type Drift struct {
	App    string `json:"app" yaml:"app"`
	Kind   string `json:"kind" yaml:"kind"` // missing, no_definition
	Detail string `json:"detail" yaml:"detail"`
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
		if err := inspect(ctx, &item, config, options.Query); err != nil {
			return result, err
		}
		if item.State == StateMissing {
			result.Drift = append(result.Drift, Drift{App: record.Name, Kind: "missing", Detail: "tracked by DevEx but not installed according to " + item.Manager})
		}
		result.Items = append(result.Items, item)
	}
	// A query canceled during the last item must not yield a successful export.
	if err := ctx.Err(); err != nil {
		return result, err
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

// queryable lists the install methods NativeQuery can check.
func queryable(manager string) bool {
	switch manager {
	case "apt", "deb", "snap", "flatpak":
		return true
	}
	return false
}

// inspect resolves the native package identifiers of a definition and checks
// them. It returns an error only when the context ends; every other failure
// leaves the item unknown with a detail.
func inspect(ctx context.Context, item *Item, config types.OSConfig, query Query) error {
	request, parseErr := types.ResolvePackageInstall(config.InstallMethod, config.InstallCommand, config.Packages, config.Options)
	if parseErr == nil {
		item.Packages = request.Packages
	} else if len(config.Packages) > 0 {
		item.Packages = append([]string(nil), config.Packages...)
	}
	switch {
	case item.Manager == "unknown":
		item.Detail = "no package recorded"
		return nil
	case !queryable(item.Manager):
		item.Detail = item.Manager + " packages are not queried"
		return nil
	case parseErr != nil:
		item.Packages = []string{}
		item.Detail = "cannot read package identifiers from the definition: " + parseErr.Error() + "; list them under packages in the definition"
		return nil
	case len(item.Packages) == 0:
		item.Detail = "no package recorded"
		return nil
	}
	if item.Manager == "apt" || item.Manager == "deb" {
		for _, pkg := range item.Packages {
			if debArtifact(pkg) {
				item.Detail = pkg + " is a .deb file, not a dpkg package name; DevEx does not record the package name it installs, so its state cannot be verified"
				return nil
			}
		}
	}
	return check(ctx, item, query)
}

// debArtifact reports whether a deb or apt identifier names a downloaded
// package file rather than the package dpkg registers.
func debArtifact(pkg string) bool {
	return strings.Contains(pkg, "/") || strings.HasSuffix(strings.ToLower(pkg), ".deb")
}

// check queries every package; the item is installed only if all are.
func check(ctx context.Context, item *Item, query Query) error {
	versions := make([]string, 0, len(item.Packages))
	for _, pkg := range item.Packages {
		installed, version, err := query(ctx, item.Manager, pkg)
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return err
		}
		if errors.Is(err, ErrUnsupported) {
			item.State, item.Detail = StateUnknown, item.Manager+" packages are not queried"
			return nil
		}
		if err != nil {
			item.State, item.Detail = StateUnknown, "query failed: "+firstLine(err.Error())
			return nil
		}
		if !installed {
			item.State = StateMissing
			item.Detail = pkg + " is not installed"
			return nil
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
	return nil
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

// NativeQuery asks the package manager. It reports a package as absent only
// when the manager says so; a missing executable, a daemon or permission
// failure or any other error is returned so the item stays unknown.
func NativeQuery(ctx context.Context, manager, pkg string) (bool, string, error) {
	if pkg == "" || strings.HasPrefix(pkg, "-") {
		return false, "", fmt.Errorf("invalid package name %q", pkg)
	}
	switch manager {
	case "apt", "deb":
		output, err := runNative(ctx, "dpkg-query", "-W", "-f=${Status}\t${Version}", "--", pkg)
		if err != nil {
			if notInstalled(err, "no packages found matching") {
				return false, "", nil
			}
			return false, "", err
		}
		status, version, _ := strings.Cut(output, "\t")
		// Status is "want error state"; only the state "installed" counts, not
		// half-installed, config-files or not-installed.
		words := strings.Fields(status)
		if len(words) == 0 || words[len(words)-1] != "installed" {
			return false, "", nil
		}
		return true, strings.TrimSpace(version), nil
	case "snap":
		output, err := runNative(ctx, "snap", "list", "--", pkg)
		if err != nil {
			if notInstalled(err, "no matching snaps installed") {
				return false, "", nil
			}
			return false, "", err
		}
		lines := strings.Split(strings.TrimSpace(output), "\n")
		if len(lines) < 2 {
			return false, "", fmt.Errorf("snap list printed no row for %s", pkg)
		}
		fields := strings.Fields(lines[1])
		if len(fields) < 2 {
			return true, "", nil
		}
		return true, fields[1], nil
	case "flatpak":
		output, err := runNative(ctx, "flatpak", "info", "--show-version", "--", pkg)
		if err != nil {
			if notInstalled(err, "not installed") {
				return false, "", nil
			}
			return false, "", err
		}
		return true, strings.TrimSpace(output), nil
	}
	return false, "", ErrUnsupported
}

// nativeError is a package manager that ran and failed.
type nativeError struct {
	tool   string
	code   int
	stderr string
}

func (e *nativeError) Error() string {
	detail := firstLine(e.stderr)
	if detail == "" {
		detail = "no error output"
	}
	return fmt.Sprintf("%s exited with status %d: %s", e.tool, e.code, detail)
}

// notInstalled reports whether a failed query is the manager's own answer that
// the package is not installed.
func notInstalled(err error, message string) bool {
	var native *nativeError
	return errors.As(err, &native) && native.code == 1 && strings.Contains(strings.ToLower(native.stderr), message)
}

// runNative runs a read-only query in the C locale, so its messages can be
// recognized, and returns its standard output.
func runNative(ctx context.Context, tool string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, tool, args...)
	cmd.Env = append(os.Environ(), "LC_ALL=C")
	var stderr strings.Builder
	cmd.Stderr = &stderr
	output, err := cmd.Output()
	if ctxErr := ctx.Err(); ctxErr != nil {
		return "", fmt.Errorf("%s query stopped: %w", tool, ctxErr)
	}
	if err == nil {
		return string(output), nil
	}
	if errors.Is(err, exec.ErrNotFound) {
		return "", fmt.Errorf("%s is not on PATH; install it or add it to PATH so DevEx can check these packages: %w", tool, err)
	}
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		return "", &nativeError{tool: tool, code: exit.ExitCode(), stderr: stderr.String()}
	}
	return "", fmt.Errorf("run %s: %w", tool, err)
}
