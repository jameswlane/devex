// Package winprereq checks the Windows settings and tools DevEx relies on:
// Developer Mode, Win32 long paths, PowerShell 7 and Windows Terminal.
package winprereq

import (
	"errors"
	"fmt"
	"io/fs"
	"strings"
)

// Kind distinguishes registry settings from tools found on PATH.
type Kind int

const (
	// KindRegistry is a DWORD under HKEY_LOCAL_MACHINE that must equal 1.
	KindRegistry Kind = iota
	// KindTool is an executable that must be on PATH.
	KindTool
)

// State is the observed state of one prerequisite.
type State string

const (
	StateOn      State = "on"
	StateOff     State = "off"
	StateUnknown State = "unknown"
)

// Prerequisite IDs accepted by --windows-prerequisites.
const (
	DeveloperMode   = "developer-mode"
	LongPaths       = "long-paths"
	PowerShell      = "powershell"
	WindowsTerminal = "windows-terminal"
	// All selects every prerequisite.
	All = "all"
)

// Prerequisite describes one check and how to satisfy it.
type Prerequisite struct {
	ID   string
	Name string
	Kind Kind
	// Why says what fails without it.
	Why string
	// Key and Value name the HKLM registry DWORD for KindRegistry.
	Key   string
	Value string
	// Binary is the executable looked up on PATH for KindTool.
	Binary string
	// App is the catalog application that installs the tool, and WingetID its
	// winget package, for KindTool.
	App      string
	WingetID string
}

// Prerequisites lists every check in report order.
var Prerequisites = []Prerequisite{
	{
		ID:    DeveloperMode,
		Name:  "Developer Mode",
		Kind:  KindRegistry,
		Why:   "lets DevEx create symlinks without administrator rights",
		Key:   `SOFTWARE\Microsoft\Windows\CurrentVersion\AppModelUnlock`,
		Value: "AllowDevelopmentWithoutDevLicense",
	},
	{
		ID:    LongPaths,
		Name:  "Win32 long paths",
		Kind:  KindRegistry,
		Why:   "lets tools use paths longer than 260 characters",
		Key:   `SYSTEM\CurrentControlSet\Control\FileSystem`,
		Value: "LongPathsEnabled",
	},
	{
		ID:       PowerShell,
		Name:     "PowerShell 7",
		Kind:     KindTool,
		Why:      "runs PowerShell 7 profiles and scripts",
		Binary:   "pwsh",
		App:      "PowerShell",
		WingetID: "Microsoft.PowerShell",
	},
	{
		ID:       WindowsTerminal,
		Name:     "Windows Terminal",
		Kind:     KindTool,
		Why:      "renders the setup interface and hosts terminal profiles",
		Binary:   "wt",
		App:      "Windows Terminal",
		WingetID: "Microsoft.WindowsTerminal",
	},
}

// IDs returns the prerequisite IDs in report order.
func IDs() []string {
	ids := make([]string, 0, len(Prerequisites))
	for _, p := range Prerequisites {
		ids = append(ids, p.ID)
	}
	return ids
}

// ParseSelection validates IDs from flags or configuration. "all" selects every
// prerequisite; an unknown ID is an error naming the valid ones.
func ParseSelection(values []string) (map[string]bool, error) {
	selected := map[string]bool{}
	valid := map[string]bool{}
	for _, id := range IDs() {
		valid[id] = true
	}
	for _, raw := range values {
		id := strings.ToLower(strings.TrimSpace(raw))
		switch {
		case id == "":
			continue
		case id == All:
			for _, known := range IDs() {
				selected[known] = true
			}
		case valid[id]:
			selected[id] = true
		default:
			return nil, fmt.Errorf("unknown Windows prerequisite %q: use %s or %s", raw, strings.Join(IDs(), ", "), All)
		}
	}
	return selected, nil
}

// RegistryPath is the full HKLM key shown to users.
func (p Prerequisite) RegistryPath() string {
	return `HKLM\` + p.Key
}

// ManualStep is the exact command a user runs to satisfy the prerequisite.
func (p Prerequisite) ManualStep() string {
	if p.Kind == KindTool {
		return fmt.Sprintf("winget install --id %s --exact --source winget", p.WingetID)
	}
	return fmt.Sprintf(`reg add "%s" /v %s /t REG_DWORD /d 1 /f`, p.RegistryPath(), p.Value)
}

// Probe reads and changes the system; tests replace every function.
type Probe struct {
	// ReadDWORD returns an error matching fs.ErrNotExist when the key or value is missing.
	ReadDWORD  func(key, value string) (uint32, error)
	WriteDWORD func(key, value string, data uint32) error
	LookPath   func(file string) (string, error)
}

// Result is one prerequisite with its observed state.
type Result struct {
	Prerequisite
	State State
	// Detail is the path a tool was found at, or why the state is unknown.
	Detail string
}

// Check inspects every prerequisite without changing anything.
func Check(probe Probe) []Result {
	results := make([]Result, 0, len(Prerequisites))
	for _, p := range Prerequisites {
		results = append(results, CheckOne(probe, p))
	}
	return results
}

// CheckOne inspects a single prerequisite.
func CheckOne(probe Probe, p Prerequisite) Result {
	result := Result{Prerequisite: p, State: StateUnknown}
	switch p.Kind {
	case KindRegistry:
		if probe.ReadDWORD == nil {
			result.Detail = "the registry is not available on this platform"
			return result
		}
		data, err := probe.ReadDWORD(p.Key, p.Value)
		switch {
		case errors.Is(err, fs.ErrNotExist):
			// Windows treats a missing value as disabled.
			result.State = StateOff
		case err != nil:
			result.Detail = fmt.Sprintf(`could not read %s\%s: %v`, p.RegistryPath(), p.Value, err)
		case data == 1:
			result.State = StateOn
		default:
			result.State = StateOff
		}
	case KindTool:
		if probe.LookPath == nil {
			result.Detail = "PATH lookup is not available"
			return result
		}
		path, err := probe.LookPath(p.Binary)
		if err != nil {
			result.State = StateOff
			return result
		}
		result.State = StateOn
		result.Detail = path
	}
	return result
}

// Enable sets a registry prerequisite to 1 and reads it back.
func Enable(probe Probe, p Prerequisite) error {
	if p.Kind != KindRegistry {
		return fmt.Errorf("%s is not a registry setting", p.Name)
	}
	if probe.WriteDWORD == nil {
		return errors.New("the registry is not available on this platform")
	}
	if err := probe.WriteDWORD(p.Key, p.Value, 1); err != nil {
		return fmt.Errorf(`set %s\%s: %w`, p.RegistryPath(), p.Value, err)
	}
	if result := CheckOne(probe, p); result.State != StateOn {
		return fmt.Errorf(`%s\%s is still %s after writing it`, p.RegistryPath(), p.Value, result.State)
	}
	return nil
}
