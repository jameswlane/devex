// Package agentsandbox probes Linux coding-agent sandbox prerequisites
// (bubblewrap, socat, user-namespace sysctls, and AppArmor profiles) without
// modifying host configuration.
package agentsandbox

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"time"

	devexpaths "github.com/jameswlane/devex/packages/plugin-sdk/paths"
)

// Status summarizes whether a Linux bubblewrap sandbox is ready to use.
type Status string

const (
	StatusOK          Status = "ok"
	StatusMissing     Status = "missing"
	StatusRestricted  Status = "restricted"
	StatusFailed      Status = "failed"
	StatusUnsupported Status = "unsupported"
)

// ReportFileName is the JSON report stored in the DevEx state directory for
// reuse by diagnostic commands such as devex agent doctor.
const ReportFileName = "agent-sandbox.json"

const (
	pathAppArmorRestrictUserns   = "/proc/sys/kernel/apparmor_restrict_unprivileged_userns"
	pathUnprivilegedUsernsClone  = "/proc/sys/kernel/unprivileged_userns_clone"
	pathMaxUserNamespaces        = "/proc/sys/user/max_user_namespaces"
	pathAppArmorDir              = "/etc/apparmor.d"
	pathAppArmorBwrapRestrict    = "/etc/apparmor.d/bwrap-userns-restrict"
	pathAppArmorBwrapCustom      = "/etc/apparmor.d/bwrap"
	pathAppArmorExtraBwrap       = "/usr/share/apparmor/extra-profiles/bwrap-userns-restrict"
	pathOSRelease                = "/etc/os-release"
	defaultBwrapBinaryPath       = "/usr/bin/bwrap"
	bwrapProbeCommandDescription = "bwrap --ro-bind / / --dev /dev --unshare-all true"
)

// Probe injects every host-inspecting operation so Check stays pure and
// testable without touching real sysctls, AppArmor files, or binaries.
type Probe struct {
	GOOS     string
	Now      func() time.Time
	ReadFile func(path string) ([]byte, error)
	LookPath func(name string) (string, error)
	Run      func(ctx context.Context, name string, args ...string) ([]byte, error)
	Stat     func(path string) (os.FileInfo, error)
	Glob     func(pattern string) ([]string, error)
}

// DefaultProbe returns a Probe wired to the local operating system.
func DefaultProbe() Probe {
	return Probe{
		GOOS:     runtime.GOOS,
		Now:      time.Now,
		ReadFile: os.ReadFile,
		LookPath: exec.LookPath,
		Run: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			return exec.CommandContext(ctx, name, args...).CombinedOutput()
		},
		Stat: os.Stat,
		Glob: filepath.Glob,
	}
}

func (p Probe) withDefaults() Probe {
	if p.GOOS == "" {
		p.GOOS = runtime.GOOS
	}
	if p.Now == nil {
		p.Now = time.Now
	}
	if p.ReadFile == nil {
		p.ReadFile = func(string) ([]byte, error) { return nil, os.ErrNotExist }
	}
	if p.LookPath == nil {
		p.LookPath = func(name string) (string, error) { return "", exec.ErrNotFound }
	}
	if p.Run == nil {
		p.Run = func(context.Context, string, ...string) ([]byte, error) {
			return nil, errors.New("probe runner not configured")
		}
	}
	if p.Stat == nil {
		p.Stat = func(string) (os.FileInfo, error) { return nil, os.ErrNotExist }
	}
	if p.Glob == nil {
		p.Glob = func(string) ([]string, error) { return nil, nil }
	}
	return p
}

// Report captures the observed sandbox prerequisite state and ordered
// remediation commands.
type Report struct {
	BwrapPath               string    `json:"bwrap_path"`
	SocatPath               string    `json:"socat_path"`
	UsernsRestricted        bool      `json:"userns_restricted"`
	UnprivilegedUsernsClone *int      `json:"unprivileged_userns_clone,omitempty"`
	MaxUserNamespaces       *int      `json:"max_user_namespaces,omitempty"`
	AppArmorBwrapProfile    string    `json:"apparmor_bwrap_profile,omitempty"`
	BwrapWorks              bool      `json:"bwrap_works"`
	BwrapError              string    `json:"bwrap_error,omitempty"`
	Status                  Status    `json:"status"`
	CheckedAt               time.Time `json:"checked_at"`
	Remediation             []string  `json:"remediation"`
}

// Check inspects Linux sandbox prerequisites using probe and never modifies
// host configuration (sysctls, AppArmor profiles, or installed packages).
func Check(ctx context.Context, probe Probe) Report {
	probe = probe.withDefaults()
	report := Report{
		CheckedAt:   probe.Now().UTC(),
		Remediation: []string{},
	}
	if probe.GOOS != "linux" {
		report.Status = StatusUnsupported
		return report
	}

	if path, err := probe.LookPath("bwrap"); err == nil {
		report.BwrapPath = path
	}
	if path, err := probe.LookPath("socat"); err == nil {
		report.SocatPath = path
	}

	if v, ok := readSysctlInt(probe, pathAppArmorRestrictUserns); ok {
		report.UsernsRestricted = v == 1
	}
	if v, ok := readSysctlInt(probe, pathUnprivilegedUsernsClone); ok {
		val := v
		report.UnprivilegedUsernsClone = &val
	}
	if v, ok := readSysctlInt(probe, pathMaxUserNamespaces); ok {
		val := v
		report.MaxUserNamespaces = &val
	}

	report.AppArmorBwrapProfile = findAppArmorBwrapProfile(probe, report.BwrapPath)
	distro := detectLinuxDistro(probe)

	if report.BwrapPath == "" {
		report.Status = StatusMissing
		report.Remediation = missingPackagesRemediation(distro, true, report.SocatPath == "")
		return report
	}

	// Run bwrap with --ro-bind / / --dev /dev --unshare-all true rather than
	// bare --ro-bind / / true: coding agent harnesses (Claude Code, Codex CLI,
	// Copilot CLI) unshare user/mount/PID/IPC/UTS/net namespaces and mount
	// /dev, so exercising the full unshare set catches both AppArmor userns
	// blocks and container capability restrictions.
	probeCtx, cancel := context.WithTimeout(ctx, bwrapProbeTimeout)
	defer cancel()
	out, err := probe.Run(probeCtx, report.BwrapPath, "--ro-bind", "/", "/", "--dev", "/dev", "--unshare-all", "true")
	if err == nil {
		report.BwrapWorks = true
		if report.SocatPath == "" {
			report.Status = StatusMissing
			report.Remediation = missingPackagesRemediation(distro, false, true)
			return report
		}
		report.Status = StatusOK
		return report
	}

	report.BwrapWorks = false
	report.BwrapError = formatRunError(out, err)

	usernsCloneDisabled := report.UnprivilegedUsernsClone != nil && *report.UnprivilegedUsernsClone == 0
	maxUsernsDisabled := report.MaxUserNamespaces != nil && *report.MaxUserNamespaces == 0
	if report.UsernsRestricted || usernsCloneDisabled || maxUsernsDisabled {
		report.Status = StatusRestricted
		report.Remediation = restrictedRemediation(probe, report.UsernsRestricted, usernsCloneDisabled, maxUsernsDisabled)
		if report.SocatPath == "" {
			report.Remediation = append(report.Remediation, missingPackagesRemediation(distro, false, true)...)
		}
		return report
	}

	report.Status = StatusFailed
	report.Remediation = []string{
		fmt.Sprintf("Inspect why `%s` failed (%s) and verify your kernel or container allows unprivileged user namespaces.", bwrapProbeCommandDescription, report.BwrapError),
	}
	if report.SocatPath == "" {
		report.Remediation = append(report.Remediation, missingPackagesRemediation(distro, false, true)...)
	}
	return report
}

func readSysctlInt(probe Probe, path string) (int, bool) {
	data, err := probe.ReadFile(path)
	if err != nil {
		return 0, false
	}
	v, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		return 0, false
	}
	return v, true
}

func findAppArmorBwrapProfile(probe Probe, bwrapPath string) string {
	if _, err := probe.Stat(pathAppArmorBwrapRestrict); err == nil {
		return pathAppArmorBwrapRestrict
	}
	if _, err := probe.ReadFile(pathAppArmorBwrapRestrict); err == nil {
		return pathAppArmorBwrapRestrict
	}

	targets := []string{defaultBwrapBinaryPath}
	if bwrapPath != "" && bwrapPath != defaultBwrapBinaryPath {
		targets = append(targets, bwrapPath)
	}

	matches, _ := probe.Glob(filepath.Join(pathAppArmorDir, "*"))
	sort.Strings(matches)
	if len(matches) == 0 {
		// Fallback for minimal fixtures without Glob that populate /etc/apparmor.d/bwrap.
		matches = []string{pathAppArmorBwrapCustom}
	}
	for _, path := range matches {
		data, err := probe.ReadFile(path)
		if err != nil {
			continue
		}
		content := string(data)
		for _, target := range targets {
			if strings.Contains(content, target) {
				return path
			}
		}
	}
	return ""
}

func detectLinuxDistro(probe Probe) string {
	data, err := probe.ReadFile(pathOSRelease)
	if err != nil {
		return ""
	}
	var id, idLike string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if val, ok := strings.CutPrefix(line, "ID="); ok {
			id = strings.ToLower(strings.Trim(val, "\"'"))
		} else if val, ok := strings.CutPrefix(line, "ID_LIKE="); ok {
			idLike = strings.ToLower(strings.Trim(val, "\"'"))
		}
	}
	for _, token := range append([]string{id}, strings.Fields(idLike)...) {
		switch token {
		case "ubuntu", "debian":
			return "apt"
		case "fedora", "rhel", "centos", "rocky", "almalinux":
			return "dnf"
		case "arch", "manjaro", "endeavouros":
			return "pacman"
		case "opensuse", "opensuse-tumbleweed", "opensuse-leap", "suse", "sles":
			return "zypper"
		}
	}
	return ""
}

func missingPackagesRemediation(pkgManager string, missingBwrap, missingSocat bool) []string {
	var pkgs, devexApps []string
	if missingBwrap {
		pkgs = append(pkgs, "bubblewrap")
		devexApps = append(devexApps, "Bubblewrap")
	}
	if missingSocat {
		pkgs = append(pkgs, "socat")
		devexApps = append(devexApps, "socat")
	}
	if len(pkgs) == 0 {
		return []string{}
	}
	joinedPkgs := strings.Join(pkgs, " ")
	var installCmd string
	switch pkgManager {
	case "dnf":
		installCmd = "sudo dnf install -y " + joinedPkgs
	case "pacman":
		installCmd = "sudo pacman -S --needed " + joinedPkgs
	case "zypper":
		installCmd = "sudo zypper install -y " + joinedPkgs
	default:
		installCmd = "sudo apt-get install -y " + joinedPkgs
	}
	return []string{
		installCmd,
		"devex install " + strings.Join(devexApps, " "),
	}
}

func restrictedRemediation(probe Probe, apparmorRestricted, usernsCloneDisabled, maxUsernsDisabled bool) []string {
	var steps []string
	if apparmorRestricted {
		if _, err := probe.Stat(pathAppArmorExtraBwrap); err == nil {
			steps = append(steps,
				"Install and load Ubuntu's AppArmor profile for bwrap (preferred):\n"+
					"  sudo cp /usr/share/apparmor/extra-profiles/bwrap-userns-restrict /etc/apparmor.d/ && sudo apparmor_parser -r /etc/apparmor.d/bwrap-userns-restrict",
			)
		} else {
			steps = append(steps,
				"Create and load a minimal AppArmor profile for bwrap (preferred):\n"+
					"  "+minimalBwrapProfileCommand,
			)
		}
		steps = append(steps,
			"Not recommended (weakens kernel hardening for every process on the host):\n"+
				"  sudo sysctl -w kernel.apparmor_restrict_unprivileged_userns=0",
		)
	}
	if usernsCloneDisabled {
		steps = append(steps,
			"Enable unprivileged user namespace creation on Debian kernels:\n"+
				"  sudo sysctl -w kernel.unprivileged_userns_clone=1",
		)
	}
	if maxUsernsDisabled {
		steps = append(steps,
			"Allow unprivileged user namespaces (currently 0):\n"+
				"  sudo sysctl -w user.max_user_namespaces=15000",
		)
	}
	return steps
}

// minimalBwrapProfileCommand writes an unconfined AppArmor profile that lets
// /usr/bin/bwrap create user namespaces. It is a single line so it can be
// pasted as is (an indented heredoc terminator would not end the heredoc).
const minimalBwrapProfileCommand = "printf '%s\\n' 'abi <abi/4.0>,' 'include <tunables/global>' " +
	"'profile bwrap /usr/bin/bwrap flags=(unconfined) {' '  userns,' '  include if exists <local/bwrap>' '}' " +
	"| sudo tee /etc/apparmor.d/bwrap >/dev/null && sudo apparmor_parser -r /etc/apparmor.d/bwrap"

// bwrapProbeTimeout bounds the sandbox start check; `true` exits immediately.
const bwrapProbeTimeout = 15 * time.Second

func formatRunError(output []byte, err error) string {
	trimmed := strings.TrimSpace(string(output))
	if trimmed == "" {
		return err.Error()
	}
	return trimmed
}

// FormatTable renders Report as a human-readable summary with numbered
// remediation steps when action is needed.
func FormatTable(r Report) string {
	var b strings.Builder
	fmt.Fprintf(&b, "Agent sandbox status: %s\n", r.Status)
	if r.Status == StatusUnsupported {
		b.WriteString("Linux bubblewrap sandbox checks only apply on Linux hosts.\n")
		return b.String()
	}
	writePath := func(label, value string) {
		if value == "" {
			fmt.Fprintf(&b, "  %-28s missing\n", label+":")
			return
		}
		fmt.Fprintf(&b, "  %-28s %s\n", label+":", value)
	}
	writePath("bwrap", r.BwrapPath)
	writePath("socat", r.SocatPath)
	fmt.Fprintf(&b, "  %-28s %t\n", "apparmor_restrict_userns:", r.UsernsRestricted)
	if r.UnprivilegedUsernsClone != nil {
		fmt.Fprintf(&b, "  %-28s %d\n", "unprivileged_userns_clone:", *r.UnprivilegedUsernsClone)
	}
	if r.MaxUserNamespaces != nil {
		fmt.Fprintf(&b, "  %-28s %d\n", "max_user_namespaces:", *r.MaxUserNamespaces)
	}
	if r.AppArmorBwrapProfile != "" {
		fmt.Fprintf(&b, "  %-28s %s\n", "apparmor_bwrap_profile:", r.AppArmorBwrapProfile)
	} else {
		fmt.Fprintf(&b, "  %-28s none detected\n", "apparmor_bwrap_profile:")
	}
	if r.BwrapPath != "" {
		if r.BwrapWorks {
			fmt.Fprintf(&b, "  %-28s ok (%s)\n", "bwrap_probe:", bwrapProbeCommandDescription)
		} else {
			fmt.Fprintf(&b, "  %-28s failed (%s)\n", "bwrap_probe:", r.BwrapError)
		}
	}
	if len(r.Remediation) > 0 {
		b.WriteString("\nRemediation (run manually; DevEx does not modify sysctls or AppArmor):\n")
		for i, step := range r.Remediation {
			fmt.Fprintf(&b, "  %d. %s\n", i+1, step)
		}
	}
	return b.String()
}

// ReportPath returns the default path where the last sandbox report is stored.
func ReportPath() (string, error) {
	dirs, err := devexpaths.Resolve()
	if err != nil {
		return "", err
	}
	return filepath.Join(dirs.State, ReportFileName), nil
}

// ReportPathForHome returns the report path for an explicit home directory.
func ReportPathForHome(home string) string {
	return filepath.Join(devexpaths.ForHome(home).State, ReportFileName)
}

// SaveReport writes report atomically (0600) to the DevEx state directory.
func SaveReport(report Report) (string, error) {
	path, err := ReportPath()
	if err != nil {
		return "", err
	}
	if err := SaveReportToPath(path, report); err != nil {
		return "", err
	}
	return path, nil
}

// SaveReportToPath writes report atomically with mode 0600 to path.
func SaveReportToPath(path string, report Report) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create sandbox report directory: %w", err)
	}
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal sandbox report: %w", err)
	}
	data = append(data, '\n')

	tmp, err := os.CreateTemp(dir, ".agent-sandbox-*.json")
	if err != nil {
		return fmt.Errorf("create temporary sandbox report: %w", err)
	}
	tmpName := tmp.Name()
	cleanup := true
	defer func() {
		if cleanup {
			_ = os.Remove(tmpName)
		}
	}()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write sandbox report: %w", err)
	}
	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("chmod sandbox report: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("sync sandbox report: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close sandbox report: %w", err)
	}
	if err := os.Rename(tmpName, path); err != nil {
		return fmt.Errorf("publish sandbox report: %w", err)
	}
	cleanup = false
	return nil
}

// LoadLastReport reads the last recorded sandbox report from the DevEx state
// directory so commands like `devex agent doctor` can inspect it.
func LoadLastReport() (Report, error) {
	path, err := ReportPath()
	if err != nil {
		return Report{}, err
	}
	return LoadReportFromPath(path)
}

// LoadReportFromPath reads and decodes a sandbox report from path.
func LoadReportFromPath(path string) (Report, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Report{}, err
	}
	var report Report
	if err := json.Unmarshal(data, &report); err != nil {
		return Report{}, fmt.Errorf("decode sandbox report %q: %w", path, err)
	}
	return report, nil
}
