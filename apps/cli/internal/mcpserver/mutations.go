package mcpserver

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/jameswlane/devex/apps/cli/internal/agentapi"
	"github.com/jameswlane/devex/apps/cli/internal/platform"
)

// DefaultPlanTTL is how long a prepared plan can be confirmed.
const DefaultPlanTTL = 10 * time.Minute

// MutatingTools lists the tools registered only with mutations enabled.
var MutatingTools = []string{"prepare_install", "install_apps", "prepare_uninstall", "uninstall_apps"}

// Outcome is the result for one application.
type Outcome struct {
	App    string `json:"app"`
	Status string `json:"status"` // installed, skipped, removed, failed, cancelled, not_run
	Error  string `json:"error,omitempty"`
	// Recovery preserves the approved paths for retry/manual cleanup after an uninstall failure.
	Recovery *UninstallTarget `json:"recovery,omitempty"`
}

// UninstallOptions selects what an uninstall removes besides the package.
type UninstallOptions struct {
	RemoveConfig bool `json:"remove_config"`
	RemoveData   bool `json:"remove_data"`
}

// UninstallTarget is one application an uninstall plan removes and exactly
// what removing it does.
type UninstallTarget struct {
	Name   string `json:"name"`
	Method string `json:"method"`
	// Command is the uninstall command or package DevEx removes, redacted.
	Command string `json:"command"`
	// Privileges and UnknownEffects describe what the user cannot see in
	// advance: package-manager plugins decide their own commands and may use
	// sudo, which fails without a terminal.
	Privileges     string `json:"privileges"`
	UnknownEffects bool   `json:"unknown_effects"`
	Config         string `json:"config"` // kept or removed
	Data           string `json:"data"`   // kept or removed
	// ConfigFiles and DataPaths are the files and paths deleted when
	// configuration or data is removed.
	ConfigFiles []string `json:"config_files"`
	DataPaths   []string `json:"data_paths"`
	// Definition is a SHA-256 digest of everything the uninstall uses: the
	// resolved definition, the tracking record and the managed-file record.
	// The executor refuses an application whose definition no longer matches.
	Definition string `json:"definition"`
}

// Executor performs confirmed installs and uninstalls. The CLI provides it.
type Executor interface {
	// ResolveInstall binds the complete effective execution inputs without effects.
	ResolveInstall(context.Context, agentapi.PlanRequest) (agentapi.ExecutionSnapshot, error)
	// Install consumes the verified snapshot without resolving names again.
	Install(ctx context.Context, snapshot agentapi.ExecutionSnapshot) ([]Outcome, error)
	// PlanUninstall resolves DevEx-tracked applications without changing
	// anything; it refuses applications DevEx does not track.
	PlanUninstall(ctx context.Context, apps []string, options UninstallOptions) ([]UninstallTarget, error)
	// Uninstall removes exactly the approved targets and reports each one. It
	// re-resolves every target immediately before acting on it and refuses,
	// without any effect, a target that is no longer tracked or whose
	// Definition differs; it executes the definition it verified.
	Uninstall(ctx context.Context, targets []UninstallTarget, options UninstallOptions) ([]Outcome, error)
}

type storedPlan struct {
	operation string
	digest    string
	expires   time.Time
	apps      []string
	uninstall UninstallOptions
	targets   []UninstallTarget
}

// PreparedPlan is returned by prepare_install and prepare_uninstall.
type PreparedPlan struct {
	PlanID       string            `json:"plan_id"`
	Digest       string            `json:"digest"`
	ExpiresAt    time.Time         `json:"expires_at"`
	Operation    string            `json:"operation"`
	Install      *agentapi.Plan    `json:"install,omitempty"`
	Uninstall    []UninstallTarget `json:"uninstall,omitempty"`
	NotIncluded  []string          `json:"not_included"`
	Confirmation string            `json:"confirmation"`
}

// ExecuteInput confirms a prepared plan.
type ExecuteInput struct {
	PlanID string `json:"plan_id" jsonschema:"plan_id returned by the prepare tool"`
	Digest string `json:"digest" jsonschema:"digest returned by the prepare tool"`
}

// ExecuteOutput reports a confirmed operation.
type ExecuteOutput struct {
	Result   string    `json:"result"` // completed, partial_failure, failed, cancelled
	Outcomes []Outcome `json:"outcomes"`
	Note     string    `json:"note"`
}

const confirmationText = "Show this plan to the user and get their explicit approval before calling %s with this plan_id and digest. The plan expires at expires_at and can be used once."

func (s *server) registerMutations(srv *mcp.Server) {
	open := true
	notDestructive := false
	destructive := true
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "prepare_install",
		Description: "Prepare an install of the named applications or categories and return the plan with a plan_id and digest. Nothing is installed. At least one application or category is required.",
		Annotations: &mcp.ToolAnnotations{Title: "Prepare install", ReadOnlyHint: true, OpenWorldHint: &notDestructive},
	}, s.prepareInstall)
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "install_apps",
		Description: "Install exactly the applications of a plan from prepare_install, after the user approved that plan. Requires its plan_id and digest; a plan can be used once and expires.",
		Annotations: &mcp.ToolAnnotations{Title: "Install applications", DestructiveHint: &notDestructive, OpenWorldHint: &open},
	}, s.installApps)
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "prepare_uninstall",
		Description: "Prepare an uninstall of named, DevEx-tracked applications and return the plan with a plan_id and digest. Configuration and data are kept unless remove_config or remove_data is set. Nothing is removed.",
		Annotations: &mcp.ToolAnnotations{Title: "Prepare uninstall", ReadOnlyHint: true, OpenWorldHint: &notDestructive},
	}, s.prepareUninstall)
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "uninstall_apps",
		Description: "Uninstall exactly the applications of a plan from prepare_uninstall, after the user approved that plan. Requires its plan_id and digest; a plan can be used once and expires.",
		Annotations: &mcp.ToolAnnotations{Title: "Uninstall applications", DestructiveHint: &destructive, OpenWorldHint: &open},
	}, s.uninstallApps)
}

func newPlanID() (string, error) {
	buffer := make([]byte, 16)
	if _, err := rand.Read(buffer); err != nil {
		return "", err
	}
	return hex.EncodeToString(buffer), nil
}

func digestOf(values ...any) (string, error) {
	data, err := json.Marshal(values)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:]), nil
}

func sortedUnique(values []string) []string {
	seen := map[string]bool{}
	out := []string{}
	for _, value := range values {
		for _, part := range strings.Split(value, ",") {
			part = strings.TrimSpace(part)
			key := strings.ToLower(part)
			if part == "" || seen[key] {
				continue
			}
			seen[key] = true
			out = append(out, part)
		}
	}
	sort.Strings(out)
	return out
}

// installDigest binds the complete execution snapshot and stable platform.
func (s *server) installDigest(ctx context.Context, names, categories []string) (agentapi.ExecutionSnapshot, []string, string, error) {
	request := agentapi.PlanRequest{Names: names, Categories: categories}
	snapshot, err := s.opts.Executor.ResolveInstall(ctx, request)
	if err != nil {
		return agentapi.ExecutionSnapshot{}, nil, "", err
	}
	apps := snapshot.RootNames()
	sort.Strings(apps)
	host, err := s.opts.DetectPlatform(ctx)
	if err != nil {
		return agentapi.ExecutionSnapshot{}, nil, "", err
	}
	digest, err := digestOf("install", apps, snapshot.Digest, stablePlatform(host))
	return snapshot, apps, digest, err
}

// stablePlatform returns host with its package managers sorted: detection
// order varies between calls, which must not change a plan's digest.
func stablePlatform(host *platform.Platform) platform.Platform {
	if host == nil {
		return platform.Platform{}
	}
	stable := *host
	stable.PackageManagers = append([]string(nil), host.PackageManagers...)
	sort.Strings(stable.PackageManagers)
	return stable
}

func (s *server) store(operation, digest string, apps []string, uninstall UninstallOptions, targets []UninstallTarget) (string, time.Time, error) {
	id, err := newPlanID()
	if err != nil {
		return "", time.Time{}, err
	}
	expires := s.opts.Clock().Add(s.opts.PlanTTL)
	now := s.opts.Clock()
	for key, plan := range s.plans {
		if now.After(plan.expires) {
			delete(s.plans, key)
		}
	}
	s.plans[id] = &storedPlan{operation: operation, digest: digest, expires: expires, apps: apps, uninstall: uninstall, targets: targets}
	return id, expires, nil
}

// take removes and returns a plan, rejecting unknown, expired, replayed,
// cross-operation and mismatched confirmations.
func (s *server) take(operation string, input ExecuteInput) (*storedPlan, error) {
	if strings.TrimSpace(input.PlanID) == "" || strings.TrimSpace(input.Digest) == "" {
		return nil, invalid("plan_id and digest from the prepare tool are required")
	}
	plan, ok := s.plans[input.PlanID]
	if !ok {
		return nil, errors.New("plan_rejected: unknown or already used plan_id; prepare a new plan")
	}
	delete(s.plans, input.PlanID)
	if plan.operation != operation {
		return nil, fmt.Errorf("plan_rejected: the plan is for %s, not %s", plan.operation, operation)
	}
	if s.opts.Clock().After(plan.expires) {
		return nil, errors.New("plan_rejected: the plan expired; prepare a new plan")
	}
	if input.Digest != plan.digest {
		return nil, errors.New("plan_rejected: the digest does not match the prepared plan")
	}
	return plan, nil
}

// PrepareInstallInput selects what to install.
type PrepareInstallInput struct {
	Apps       []string `json:"apps,omitempty" jsonschema:"application names"`
	Categories []string `json:"categories,omitempty" jsonschema:"categories"`
}

func (s *server) prepareInstall(ctx context.Context, _ *mcp.CallToolRequest, in PrepareInstallInput) (*mcp.CallToolResult, PreparedPlan, error) {
	if err := checkNames("apps", in.Apps); err != nil {
		return nil, PreparedPlan{}, err
	}
	if err := checkNames("categories", in.Categories); err != nil {
		return nil, PreparedPlan{}, err
	}
	names, categories := sortedUnique(in.Apps), sortedUnique(in.Categories)
	if len(names) == 0 && len(categories) == 0 {
		return nil, PreparedPlan{}, invalid("name at least one application or category; DevEx never installs its defaults or everything through MCP")
	}
	ctx, done, err := s.begin(ctx)
	if err != nil {
		return nil, PreparedPlan{}, err
	}
	defer done()
	snapshot, apps, digest, err := s.installDigest(ctx, names, categories)
	if err != nil {
		return nil, PreparedPlan{}, toolError(err)
	}
	plan := snapshot.Plan
	for _, app := range plan.Apps {
		if app.UnknownEffects {
			plan.Warnings = appendOnce(plan.Warnings, "Some plugin effects and required privileges are unknown until the plugin runs; installation runs without a terminal and fails if a password is required.")
		}
	}
	id, expires, err := s.store("install", digest, apps, UninstallOptions{}, nil)
	if err != nil {
		return nil, PreparedPlan{}, toolError(err)
	}
	return nil, PreparedPlan{
		PlanID: id, Digest: digest, ExpiresAt: expires, Operation: "install", Install: &plan,
		NotIncluded:  []string{"No other applications, defaults or system changes; dependencies are listed in the plan."},
		Confirmation: fmt.Sprintf(confirmationText, "install_apps"),
	}, nil
}

func appendOnce(values []string, value string) []string {
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

func (s *server) installApps(ctx context.Context, _ *mcp.CallToolRequest, in ExecuteInput) (*mcp.CallToolResult, ExecuteOutput, error) {
	ctx, done, err := s.beginWithin(ctx, s.opts.MutationTimeout)
	if err != nil {
		return nil, ExecuteOutput{}, err
	}
	defer done()
	plan, err := s.take("install", in)
	if err != nil {
		return nil, ExecuteOutput{}, err
	}
	// The configuration, catalog or platform may have changed since the plan.
	snapshot, apps, digest, err := s.installDigest(ctx, plan.apps, nil)
	if err != nil {
		return nil, ExecuteOutput{}, toolError(err)
	}
	if digest != plan.digest || strings.Join(apps, "\x00") != strings.Join(plan.apps, "\x00") {
		return nil, ExecuteOutput{}, errors.New("plan_rejected: the execution inputs, security, policy, trust or platform changed since the plan was prepared; prepare a new plan")
	}
	outcomes, err := s.opts.Executor.Install(ctx, snapshot)
	note := "Installed applications are recorded by DevEx; there is no automatic rollback."
	if snapshot.Policy.RollbackOnFailure {
		note = "Rollback was requested; hooks and dependencies may require manual recovery."
	}
	return nil, finish(ctx, plan.apps, outcomes, err, note), nil
}

// PrepareUninstallInput selects what to uninstall.
type PrepareUninstallInput struct {
	Apps         []string `json:"apps" jsonschema:"DevEx-tracked application names"`
	RemoveConfig bool     `json:"remove_config,omitempty" jsonschema:"also remove the applications' configuration files"`
	RemoveData   bool     `json:"remove_data,omitempty" jsonschema:"also remove the applications' data files"`
}

func (s *server) uninstallDigest(ctx context.Context, apps []string, options UninstallOptions) ([]UninstallTarget, string, error) {
	targets, err := s.opts.Executor.PlanUninstall(ctx, apps, options)
	if err != nil {
		return nil, "", err
	}
	host, err := s.opts.DetectPlatform(ctx)
	if err != nil {
		return nil, "", err
	}
	digest, err := digestOf("uninstall", apps, options, targets, stablePlatform(host))
	return targets, digest, err
}

func (s *server) prepareUninstall(ctx context.Context, _ *mcp.CallToolRequest, in PrepareUninstallInput) (*mcp.CallToolResult, PreparedPlan, error) {
	if err := checkNames("apps", in.Apps); err != nil {
		return nil, PreparedPlan{}, err
	}
	apps := sortedUnique(in.Apps)
	if len(apps) == 0 {
		return nil, PreparedPlan{}, invalid("name at least one application; DevEx never removes everything or a whole category through MCP")
	}
	options := UninstallOptions{RemoveConfig: in.RemoveConfig, RemoveData: in.RemoveData}
	ctx, done, err := s.begin(ctx)
	if err != nil {
		return nil, PreparedPlan{}, err
	}
	defer done()
	targets, digest, err := s.uninstallDigest(ctx, apps, options)
	if err != nil {
		return nil, PreparedPlan{}, toolError(err)
	}
	id, expires, err := s.store("uninstall", digest, apps, options, targets)
	if err != nil {
		return nil, PreparedPlan{}, toolError(err)
	}
	return nil, PreparedPlan{
		PlanID: id, Digest: digest, ExpiresAt: expires, Operation: "uninstall", Uninstall: targets,
		NotIncluded: []string{
			"Only the listed command, config_files and data_paths are removed; an application whose definition or files change before confirmation is refused.",
			"Dependent applications are never removed; an uninstall that would break a dependent application fails.",
			"Orphaned packages, services, system files and Docker data are not cleaned up.",
		},
		Confirmation: fmt.Sprintf(confirmationText, "uninstall_apps"),
	}, nil
}

func (s *server) uninstallApps(ctx context.Context, _ *mcp.CallToolRequest, in ExecuteInput) (*mcp.CallToolResult, ExecuteOutput, error) {
	ctx, done, err := s.beginWithin(ctx, s.opts.MutationTimeout)
	if err != nil {
		return nil, ExecuteOutput{}, err
	}
	defer done()
	plan, err := s.take("uninstall", in)
	if err != nil {
		return nil, ExecuteOutput{}, err
	}
	_, digest, err := s.uninstallDigest(ctx, plan.apps, plan.uninstall)
	if err != nil {
		return nil, ExecuteOutput{}, toolError(err)
	}
	if digest != plan.digest {
		return nil, ExecuteOutput{}, errors.New("plan_rejected: the tracked applications, their definitions, files or the platform changed since the plan was prepared; prepare a new plan")
	}
	// The executor verifies each approved target again right before acting.
	outcomes, err := s.opts.Executor.Uninstall(ctx, plan.targets, plan.uninstall)
	return nil, finish(ctx, plan.apps, outcomes, err, "Removed applications are not reinstalled automatically."), nil
}

// finish completes outcomes for every planned app and summarizes the result.
func finish(ctx context.Context, apps []string, outcomes []Outcome, err error, note string) ExecuteOutput {
	reported := map[string]bool{}
	clean := make([]Outcome, 0, len(apps))
	for _, outcome := range outcomes {
		outcome.Error = Sanitize(outcome.Error)
		reported[strings.ToLower(outcome.App)] = true
		clean = append(clean, outcome)
	}
	missing := "not_run"
	if ctx.Err() != nil {
		missing = "cancelled"
	}
	for _, app := range apps {
		if !reported[strings.ToLower(app)] {
			clean = append(clean, Outcome{App: app, Status: missing})
		}
	}
	result := "completed"
	failed, succeeded := 0, 0
	for _, outcome := range clean {
		switch outcome.Status {
		case "failed", "cancelled", "not_run":
			failed++
		default:
			succeeded++
		}
	}
	switch {
	case ctx.Err() != nil:
		result = "cancelled"
		note = "The request was cancelled; applications that finished before cancellation keep their changes. " + note
	case failed > 0 && succeeded > 0:
		result = "partial_failure"
	case failed > 0:
		result = "failed"
	}
	if err != nil {
		// Operation-level failures, such as recording the run, are reported
		// even when every application reports success.
		if result == "completed" {
			result = "failed"
			if succeeded > 0 {
				result = "partial_failure"
			}
		}
		note = "Error: " + Sanitize(strings.ReplaceAll(err.Error(), "\n", "; ")) + " " + note
	}
	return ExecuteOutput{Result: result, Outcomes: clean, Note: note}
}

var (
	urlUserinfo = regexp.MustCompile(`(?i)([a-z][a-z0-9+.-]*://)[^/@\s]+@`)
	urlQuery    = regexp.MustCompile(`(?i)(https?://[^\s?#]+)\?[^\s#]+`)
	secretPair  = regexp.MustCompile(`(?i)\b(token|password|passwd|secret|api[_-]?key|authorization)(\s*[=:]\s*)(?:(?:bearer|basic)\s+)?\S+`)
	bearer      = regexp.MustCompile(`(?i)\b(bearer|basic)\s+[^\s,;]+`)
)

// MaxErrorLength bounds the error text returned for one application.
const MaxErrorLength = 300

// Sanitize removes credentials and query strings from URLs and values of
// secret-looking keys, keeps the first line and bounds its length.
func Sanitize(message string) string {
	message, _, _ = strings.Cut(message, "\n")
	return bound(redact(message), MaxErrorLength)
}

func redact(message string) string {
	message = urlUserinfo.ReplaceAllString(message, "$1[redacted]@")
	message = urlQuery.ReplaceAllString(message, "$1?[redacted]")
	message = secretPair.ReplaceAllString(message, "$1$2[redacted]")
	return bearer.ReplaceAllString(message, "$1 [redacted]")
}

func bound(message string, limit int) string {
	if len(message) > limit {
		return message[:limit] + "…"
	}
	return message
}
