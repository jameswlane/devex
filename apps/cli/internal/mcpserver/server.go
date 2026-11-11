// Package mcpserver exposes read-only DevEx operations to Model Context
// Protocol clients over stdio.
//
// Every tool is backed by internal/agentapi and is read-only: no tool installs,
// removes or configures anything, runs package managers, plugins or hooks, or
// writes files. Mutating operations are intentionally absent; see issue #173.
package mcpserver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/jameswlane/devex/apps/cli/internal/agentapi"
	"github.com/jameswlane/devex/apps/cli/internal/commands/status"
	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/platform"
	"github.com/jameswlane/devex/apps/cli/internal/types"
)

const (
	// DefaultRequestTimeout bounds each tool call and resource read.
	DefaultRequestTimeout = 30 * time.Second
	// DefaultListLimit and MaxListLimit bound list_apps results.
	DefaultListLimit = 50
	MaxListLimit     = 200
	// MaxNames bounds the application names a status or plan request may name.
	MaxNames = 100
	// MaxTextLength bounds free-text request fields.
	MaxTextLength = 200
	// EffectiveConfigURI is the only resource the server exposes.
	EffectiveConfigURI = "devex://config/effective"
)

// Options configures a read-only DevEx MCP server.
type Options struct {
	Version        string
	Settings       config.CrossPlatformSettings
	Repo           types.Repository
	RequestTimeout time.Duration
	// DetectPlatform replaces platform detection in tests.
	DetectPlatform func(context.Context) (*platform.Platform, error)
}

// ReadOnlyTools lists every tool the server registers.
var ReadOnlyTools = []string{"list_apps", "detect_platform", "get_status", "validate_config", "plan_install"}

type server struct {
	opts Options
	// mu serializes requests: agentapi reads process-global Viper state, so
	// concurrent requests must not observe each other's configuration access.
	mu sync.Mutex
}

// New returns an MCP server with the read-only DevEx tools and the redacted
// effective-configuration resource.
func New(opts Options) *mcp.Server {
	if opts.RequestTimeout <= 0 {
		opts.RequestTimeout = DefaultRequestTimeout
	}
	if opts.DetectPlatform == nil {
		opts.DetectPlatform = agentapi.DetectPlatform
	}
	s := &server{opts: opts}

	impl := &mcp.Implementation{Name: "devex", Title: "DevEx", Version: opts.Version}
	srv := mcp.NewServer(impl, &mcp.ServerOptions{
		Instructions: "Read-only DevEx tools. They inspect the application catalog, platform, tracked installation records, configuration and install plans. None of them installs, removes or changes anything.",
	})

	readOnly := func(title string) *mcp.ToolAnnotations {
		closed := false
		return &mcp.ToolAnnotations{Title: title, ReadOnlyHint: true, IdempotentHint: true, OpenWorldHint: &closed}
	}
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "list_apps",
		Description: "List applications DevEx can install, applications DevEx has recorded as installed, or application categories. Results are capped; truncated reports whether more matched.",
		Annotations: readOnly("List applications"),
	}, s.listApps)
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "detect_platform",
		Description: "Detect the operating system, distribution, desktop environment, architecture and available package managers.",
		Annotations: readOnly("Detect platform"),
	}, s.detectPlatform)
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "get_status",
		Description: "Report applications from DevEx's installation records. It does not run package managers or plugins, so it reports installation status as unknown rather than probing the system.",
		Annotations: readOnly("Get status"),
	}, s.getStatus)
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "validate_config",
		Description: "Validate the DevEx configuration and return structured diagnostics. It never fixes anything.",
		Annotations: readOnly("Validate configuration"),
	}, s.validateConfig)
	mcp.AddTool(srv, &mcp.Tool{
		Name:        "plan_install",
		Description: "Build the install plan for the named applications or categories (the configured defaults when none are named) without executing anything.",
		Annotations: readOnly("Plan install"),
	}, s.planInstall)

	srv.AddResource(&mcp.Resource{
		URI:         EffectiveConfigURI,
		Name:        "effective-config",
		Title:       "Effective DevEx configuration",
		Description: "Allowlisted effective application settings and their precedence. Scripts, URLs, environment values and paths are never included.",
		MIMEType:    "application/json",
	}, s.effectiveConfig)
	return srv
}

// begin serializes a request and applies the request deadline.
func (s *server) begin(ctx context.Context) (context.Context, func()) {
	s.mu.Lock()
	ctx, cancel := context.WithTimeout(ctx, s.opts.RequestTimeout)
	return ctx, func() {
		cancel()
		s.mu.Unlock()
	}
}

// toolError converts an error into a sanitized, categorized tool error.
func toolError(err error) error {
	if errors.Is(err, context.DeadlineExceeded) {
		return fmt.Errorf("timeout: the request exceeded its deadline")
	}
	if errors.Is(err, context.Canceled) {
		return fmt.Errorf("cancelled: the request was cancelled")
	}
	apiErr := agentapi.APIError(err)
	if apiErr.Message == apiErr.Category {
		return errors.New(apiErr.Category)
	}
	return fmt.Errorf("%s: %s", apiErr.Category, apiErr.Message)
}

func invalid(format string, args ...any) error {
	return fmt.Errorf("invalid_request: "+format, args...)
}

func checkText(field, value string) error {
	if len(value) > MaxTextLength {
		return invalid("%s must be at most %d characters", field, MaxTextLength)
	}
	return nil
}

func checkNames(field string, values []string) error {
	if len(values) > MaxNames {
		return invalid("%s may name at most %d entries", field, MaxNames)
	}
	for _, value := range values {
		if err := checkText(field, value); err != nil {
			return err
		}
	}
	return nil
}

// ListAppsInput selects what list_apps returns.
type ListAppsInput struct {
	Kind        string `json:"kind,omitempty" jsonschema:"available (default), installed or categories"`
	Category    string `json:"category,omitempty" jsonschema:"only applications in this category"`
	Search      string `json:"search,omitempty" jsonschema:"only applications whose name or description contains this text"`
	Method      string `json:"method,omitempty" jsonschema:"only applications using this install method"`
	Recommended bool   `json:"recommended,omitempty" jsonschema:"only recommended applications"`
	Limit       int    `json:"limit,omitempty" jsonschema:"maximum results to return (default 50, at most 200)"`
}

// ListAppsOutput is the list_apps result.
type ListAppsOutput struct {
	Kind       string                  `json:"kind"`
	Installed  []agentapi.InstalledApp `json:"installed"`
	Available  []agentapi.AvailableApp `json:"available"`
	Categories []agentapi.CategoryInfo `json:"categories"`
	Total      int                     `json:"total"`
	Truncated  bool                    `json:"truncated"`
}

func (s *server) listApps(ctx context.Context, _ *mcp.CallToolRequest, in ListAppsInput) (*mcp.CallToolResult, ListAppsOutput, error) {
	out := ListAppsOutput{Installed: []agentapi.InstalledApp{}, Available: []agentapi.AvailableApp{}, Categories: []agentapi.CategoryInfo{}}
	kind := in.Kind
	if kind == "" {
		kind = "available"
	}
	if kind != "available" && kind != "installed" && kind != "categories" {
		return nil, out, invalid("kind must be available, installed or categories")
	}
	for field, value := range map[string]string{"category": in.Category, "search": in.Search, "method": in.Method} {
		if err := checkText(field, value); err != nil {
			return nil, out, err
		}
	}
	limit := in.Limit
	if limit == 0 {
		limit = DefaultListLimit
	}
	if limit < 1 || limit > MaxListLimit {
		return nil, out, invalid("limit must be between 1 and %d", MaxListLimit)
	}
	out.Kind = kind

	ctx, done := s.begin(ctx)
	defer done()
	result, err := agentapi.List(ctx, s.opts.Repo, s.opts.Settings, agentapi.ListRequest{Kind: kind, Options: agentapi.ListCommandOptions{
		Format: "json", Category: in.Category, Search: in.Search, Method: in.Method, Recommended: in.Recommended,
	}})
	if err != nil {
		return nil, out, toolError(err)
	}
	out.Installed, out.Total, out.Truncated = capped(result.Installed, limit)
	if kind == "available" {
		out.Available, out.Total, out.Truncated = capped(result.Available, limit)
	}
	if kind == "categories" {
		out.Categories, out.Total, out.Truncated = capped(result.Categories, limit)
	}
	return nil, out, nil
}

func capped[T any](items []T, limit int) ([]T, int, bool) {
	if items == nil {
		items = []T{}
	}
	if len(items) > limit {
		return items[:limit], len(items), true
	}
	return items, len(items), false
}

// Empty is the input of tools that take no arguments.
type Empty struct{}

func (s *server) detectPlatform(ctx context.Context, _ *mcp.CallToolRequest, _ Empty) (*mcp.CallToolResult, platform.Platform, error) {
	ctx, done := s.begin(ctx)
	defer done()
	result, err := s.opts.DetectPlatform(ctx)
	if err != nil {
		return nil, platform.Platform{}, toolError(err)
	}
	if result == nil {
		return nil, platform.Platform{}, errors.New("unavailable: platform detection returned no result")
	}
	return nil, *result, nil
}

// StatusInput selects the applications get_status reports.
type StatusInput struct {
	Apps     []string `json:"apps,omitempty" jsonschema:"application names; all recorded applications when empty"`
	Category string   `json:"category,omitempty" jsonschema:"only applications in this category"`
}

// StatusOutput is the get_status result.
type StatusOutput struct {
	Apps []status.AppStatus `json:"apps"`
}

func (s *server) getStatus(ctx context.Context, _ *mcp.CallToolRequest, in StatusInput) (*mcp.CallToolResult, StatusOutput, error) {
	out := StatusOutput{Apps: []status.AppStatus{}}
	if err := checkNames("apps", in.Apps); err != nil {
		return nil, out, err
	}
	if err := checkText("category", in.Category); err != nil {
		return nil, out, err
	}
	ctx, done := s.begin(ctx)
	defer done()
	var reader agentapi.StatusReader
	if s.opts.Repo != nil {
		reader = s.opts.Repo
	}
	// A nil probe never runs package managers or plugins.
	results, err := agentapi.Status(ctx, s.opts.Settings, reader, agentapi.StatusRequest{Names: in.Apps, Category: in.Category}, nil)
	if err != nil {
		return nil, out, toolError(err)
	}
	out.Apps = results
	return nil, out, nil
}

// ValidateInput configures validate_config.
type ValidateInput struct {
	Strict bool `json:"strict,omitempty" jsonschema:"treat warnings as failures"`
}

func (s *server) validateConfig(ctx context.Context, _ *mcp.CallToolRequest, in ValidateInput) (*mcp.CallToolResult, agentapi.ValidationResult, error) {
	ctx, done := s.begin(ctx)
	defer done()
	result, err := agentapi.ValidateConfig(ctx, s.opts.Settings, in.Strict)
	if err != nil {
		return nil, agentapi.ValidationResult{}, toolError(err)
	}
	return nil, result, nil
}

// PlanInput selects what plan_install plans.
type PlanInput struct {
	Apps       []string `json:"apps,omitempty" jsonschema:"application names to plan"`
	Categories []string `json:"categories,omitempty" jsonschema:"categories to plan"`
}

func (s *server) planInstall(ctx context.Context, _ *mcp.CallToolRequest, in PlanInput) (*mcp.CallToolResult, agentapi.Plan, error) {
	if err := checkNames("apps", in.Apps); err != nil {
		return nil, agentapi.Plan{}, err
	}
	if err := checkNames("categories", in.Categories); err != nil {
		return nil, agentapi.Plan{}, err
	}
	ctx, done := s.begin(ctx)
	defer done()
	plan, err := agentapi.PlanInstall(ctx, s.opts.Settings, agentapi.PlanRequest{Names: in.Apps, Categories: in.Categories})
	if err != nil {
		return nil, agentapi.Plan{}, toolError(err)
	}
	return nil, plan, nil
}

func (s *server) effectiveConfig(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	if req.Params.URI != EffectiveConfigURI {
		return nil, mcp.ResourceNotFoundError(req.Params.URI)
	}
	ctx, done := s.begin(ctx)
	defer done()
	effective, err := agentapi.Effective(ctx, s.opts.Settings)
	if err != nil {
		return nil, toolError(err)
	}
	data, err := json.Marshal(effective)
	if err != nil {
		return nil, fmt.Errorf("internal: encode effective configuration: %w", err)
	}
	return &mcp.ReadResourceResult{Contents: []*mcp.ResourceContents{{URI: EffectiveConfigURI, MIMEType: "application/json", Text: string(data)}}}, nil
}
