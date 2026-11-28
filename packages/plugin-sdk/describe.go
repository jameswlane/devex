package sdk

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"slices"
	"strings"
)

// DescribeCommand asks a plugin what a command would change, without running
// it: `<plugin> --describe <command> [args...]`. The plugin prints one
// Description as JSON on stdout and exits 0. It is a flag, like
// --plugin-info, so it never collides with a plugin's own commands.
const DescribeCommand = "--describe"

// DescriptionSchemaVersion is the Description JSON schema this SDK writes and
// reads. Readers reject newer versions instead of guessing at them.
const DescriptionSchemaVersion = 1

// DescribeUnsupportedExitCode is the exit status of `--describe` when the plugin,
// or the requested command, cannot be described. Callers then fall back to a
// step that is marked as not previewed.
const DescribeUnsupportedExitCode = 3

// ErrDescribeUnsupported is returned by Describer.Describe for commands the
// plugin cannot describe.
var ErrDescribeUnsupported = errors.New("command does not support describe")

// ActionKind says what an action changes.
type ActionKind string

// Action kinds a Description can contain.
const (
	ActionPackage    ActionKind = "package"
	ActionRepository ActionKind = "repository"
	ActionService    ActionKind = "service"
	ActionGroup      ActionKind = "group"
	ActionFile       ActionKind = "file"
	ActionDownload   ActionKind = "download"
	ActionShell      ActionKind = "shell"
)

// PlannedAction is one change a command would make.
type PlannedAction struct {
	Kind   ActionKind `json:"kind"`
	Target string     `json:"target"`
	// Scope is ScopeUser for the user's account, or ScopeSystem when the
	// change needs sudo.
	Scope  OperationScope `json:"scope"`
	Detail string         `json:"detail,omitempty"`
}

// PlannedDownload is a file a command would download, with how it is verified
// before use.
type PlannedDownload struct {
	URL          string               `json:"url"`
	Verification ArtifactVerification `json:"verification"`
}

// Description is what a command would change. AlreadySatisfied means the
// command would change nothing.
type Description struct {
	SchemaVersion    int               `json:"schema_version"`
	Plugin           string            `json:"plugin"`
	Command          string            `json:"command"`
	Args             []string          `json:"args,omitempty"`
	AlreadySatisfied bool              `json:"already_satisfied"`
	Actions          []PlannedAction   `json:"actions"`
	Downloads        []PlannedDownload `json:"downloads,omitempty"`
	Notes            []string          `json:"notes,omitempty"`
}

// Describer is implemented by plugins that can describe commands. Build the
// Description with Changes from the same code path the command runs, so the
// description and the behavior cannot drift.
type Describer interface {
	Describe(ctx context.Context, command string, args []string) (Description, error)
}

// Validate checks a Description against the schema.
func (d Description) Validate() error {
	if d.SchemaVersion != DescriptionSchemaVersion {
		return fmt.Errorf("unsupported description schema_version %d (want %d)", d.SchemaVersion, DescriptionSchemaVersion)
	}
	if strings.TrimSpace(d.Plugin) == "" || strings.TrimSpace(d.Command) == "" {
		return errors.New("description needs plugin and command")
	}
	changes := len(d.Actions) > 0 || len(d.Downloads) > 0
	if d.AlreadySatisfied && changes {
		return errors.New("description is already_satisfied but lists changes")
	}
	if !d.AlreadySatisfied && !changes {
		return errors.New("description lists no changes but is not already_satisfied")
	}
	for i, action := range d.Actions {
		switch action.Kind {
		case ActionPackage, ActionRepository, ActionService, ActionGroup, ActionFile, ActionDownload, ActionShell:
		default:
			return fmt.Errorf("action %d: unknown kind %q", i, action.Kind)
		}
		if action.Scope != ScopeUser && action.Scope != ScopeSystem {
			return fmt.Errorf("action %d: unknown scope %q", i, action.Scope)
		}
		if strings.TrimSpace(action.Target) == "" {
			return fmt.Errorf("action %d: empty target", i)
		}
	}
	for i, download := range d.Downloads {
		parsed, err := url.Parse(download.URL)
		if err != nil || parsed.Host == "" || (parsed.Scheme != "https" && parsed.Scheme != "http") {
			return fmt.Errorf("download %d: invalid URL %q", i, download.URL)
		}
	}
	return nil
}

// ParseDescription decodes and validates a plugin's describe output.
func ParseDescription(data []byte) (Description, error) {
	var description Description
	if err := json.Unmarshal(data, &description); err != nil {
		return Description{}, fmt.Errorf("invalid description JSON: %w", err)
	}
	if err := description.Validate(); err != nil {
		return Description{}, err
	}
	return description, nil
}

// Changes runs a command's changes, or only records them when describing.
// Write the command once against Changes, and Describe and Execute share it.
type Changes struct {
	describing bool
	actions    []PlannedAction
	downloads  []PlannedDownload
	notes      []string
}

// NewChanges returns Changes that run each change, or, when describing is
// true, only record it.
func NewChanges(describing bool) *Changes {
	return &Changes{describing: describing}
}

// Describing reports whether changes are only being recorded.
func (c *Changes) Describing() bool {
	return c.describing
}

// Do records action and, unless describing, runs it.
func (c *Changes) Do(action PlannedAction, run func() error) error {
	c.actions = append(c.actions, action)
	if c.describing || run == nil {
		return nil
	}
	return run()
}

// Download records a download the next change performs. Perform the download
// itself inside Do.
func (c *Changes) Download(download PlannedDownload) {
	c.downloads = append(c.downloads, download)
}

// Note records something the user should know, such as needing to log out.
func (c *Changes) Note(note string) {
	c.notes = append(c.notes, note)
}

// Description returns what was recorded. A command that recorded no actions or
// downloads is already satisfied.
func (c *Changes) Description(plugin, command string, args []string) Description {
	return Description{
		SchemaVersion:    DescriptionSchemaVersion,
		Plugin:           plugin,
		Command:          command,
		Args:             append([]string(nil), args...),
		AlreadySatisfied: len(c.actions) == 0 && len(c.downloads) == 0,
		Actions:          append([]PlannedAction{}, c.actions...),
		Downloads:        append([]PlannedDownload(nil), c.downloads...),
		Notes:            append([]string(nil), c.notes...),
	}
}

// runDescribe handles `--describe <command> [args...]` and returns the exit
// status. Anything the plugin prints while describing goes to stderr, so
// stdout carries only the JSON.
func runDescribe(ctx context.Context, plugin Plugin, args []string, stdout, stderr io.Writer) int {
	describer, ok := plugin.(Describer)
	if !ok {
		_, _ = fmt.Fprintf(stderr, "Error: %s does not support %s\n", plugin.Info().Name, DescribeCommand)
		return DescribeUnsupportedExitCode
	}
	if len(args) < 1 {
		_, _ = fmt.Fprintf(stderr, "Usage: %s <command> [args...]\n", DescribeCommand)
		return 1
	}
	command := args[0]

	realStdout := os.Stdout
	os.Stdout = os.Stderr
	description, err := describer.Describe(WithUnattended(ctx), command, args[1:])
	os.Stdout = realStdout

	if errors.Is(err, ErrDescribeUnsupported) {
		_, _ = fmt.Fprintf(stderr, "Error: %s cannot describe %q\n", plugin.Info().Name, command)
		return DescribeUnsupportedExitCode
	}
	if err != nil {
		writeExecutionError(stderr, err)
		return 1
	}
	if description.SchemaVersion == 0 {
		description.SchemaVersion = DescriptionSchemaVersion
	}
	if description.Plugin == "" {
		description.Plugin = plugin.Info().Name
	}
	if description.Command == "" {
		description.Command = command
	}
	if description.Args == nil && len(args) > 1 {
		description.Args = append([]string(nil), args[1:]...)
	}
	if description.Actions == nil {
		description.Actions = []PlannedAction{}
	}
	if err := description.Validate(); err != nil {
		_, _ = fmt.Fprintf(stderr, "Error: invalid description: %v\n", err)
		return 1
	}
	data, err := json.Marshal(description)
	if err != nil {
		_, _ = fmt.Fprintf(stderr, "Error: %v\n", err)
		return 1
	}
	_, _ = fmt.Fprintf(stdout, "%s\n", data)
	return 0
}

// maxDescriptionSize bounds the describe output a caller reads.
const maxDescriptionSize = 1 << 20

// limitedBuffer keeps at most max bytes and records overflow.
type limitedBuffer struct {
	bytes.Buffer
	max      int
	overflow bool
}

func (b *limitedBuffer) Write(p []byte) (int, error) {
	if room := b.max - b.Len(); len(p) > room {
		if room > 0 {
			b.Buffer.Write(p[:room])
		}
		b.overflow = true
		return len(p), nil
	}
	return b.Buffer.Write(p)
}

// DescribePlugin asks an installed plugin what command would change. It never
// elevates, never reads stdin, and returns ErrDescribeUnsupported when the
// plugin does not advertise describe support or declines the command.
func (em *ExecutableManager) DescribePlugin(ctx context.Context, pluginName, command string, args []string) (Description, error) {
	if err := ctx.Err(); err != nil {
		return Description{}, err
	}
	em.mu.RLock()
	filter := em.pluginFilter
	em.mu.RUnlock()
	if filter != nil {
		if err := filter(pluginName); err != nil {
			return Description{}, err
		}
	}
	plugin, exists := em.ListPluginsWithContext(ctx)[pluginName]
	if err := ctx.Err(); err != nil {
		return Description{}, err
	}
	if !exists {
		return Description{}, fmt.Errorf("plugin %s is not installed", pluginName)
	}
	if !plugin.SupportsDescribe {
		return Description{}, ErrDescribeUnsupported
	}

	stdout := &limitedBuffer{max: maxDescriptionSize}
	var diagnostic diagnosticTail
	cmd := NewCommandContext(ctx, plugin.Path, append([]string{DescribeCommand, command}, args...)...)
	configureOperationCommand(cmd, ExecutionRequest{Unattended: true})
	err := runPreparedCommand(cmd, OperationIO{Stdout: stdout, Stderr: &diagnostic})
	if ctx.Err() != nil {
		return Description{}, ctx.Err()
	}
	if err != nil {
		if ExitStatus(err) == DescribeUnsupportedExitCode {
			return Description{}, ErrDescribeUnsupported
		}
		if tail := strings.TrimSpace(diagnostic.String()); tail != "" {
			return Description{}, fmt.Errorf("%s describe %s failed: %w: %s", pluginName, command, err, tail)
		}
		return Description{}, fmt.Errorf("%s describe %s failed: %w", pluginName, command, err)
	}
	if stdout.overflow {
		return Description{}, fmt.Errorf("%s describe %s printed more than %d bytes", pluginName, command, maxDescriptionSize)
	}
	description, err := ParseDescription(stdout.Bytes())
	if err != nil {
		return Description{}, fmt.Errorf("%s describe %s: %w", pluginName, command, err)
	}
	if description.Plugin != pluginName || description.Command != command || !slices.Equal(description.Args, args) {
		return Description{}, fmt.Errorf("%s describe %s returned a description of %s %s %v", pluginName, command, description.Plugin, description.Command, description.Args)
	}
	return description, nil
}
