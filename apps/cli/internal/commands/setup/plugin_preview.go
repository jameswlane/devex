package setup

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"

	"github.com/jameswlane/devex/apps/cli/internal/bootstrap"
	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
)

// pluginStep is a plan item that installs by running a plugin command, such as
// `package-manager-mise ensure-installed`.
type pluginStep struct {
	plugin  string
	command string
	args    []string
}

func (s pluginStep) key() string {
	return strings.Join(append([]string{s.plugin, s.command}, s.args...), "\x00")
}

func (s pluginStep) String() string {
	return displayText(strings.Join(append([]string{s.plugin, s.command}, s.args...), " "))
}

// pluginPreview is a plugin step's description, or why there is none.
type pluginPreview struct {
	description *sdk.Description
	reason      string
}

// pluginPreviewTimeout bounds one describe call.
const pluginPreviewTimeout = 10 * time.Second

// describePluginStep is replaced in tests.
var describePluginStep = func(ctx context.Context, plugins *bootstrap.PluginBootstrap, step pluginStep) (sdk.Description, error) {
	if plugins == nil {
		return sdk.Description{}, errPluginsNotReady
	}
	return plugins.DescribePlugin(ctx, step.plugin, step.command, step.args)
}

var errPluginsNotReady = errors.New("plugins are not installed yet")

// refreshPluginPreviews describes every plugin step in plan that will run.
func (m *SetupModel) refreshPluginPreviews(ctx context.Context, plan []planItem) {
	previews := map[string]pluginPreview{}
	for _, item := range plan {
		if item.pluginStep == nil || item.channelErr != nil || m.selections.alreadyInstalled[item.app.Name] {
			continue
		}
		key := item.pluginStep.key()
		if _, done := previews[key]; done {
			continue
		}
		previews[key] = m.describePluginStep(ctx, *item.pluginStep)
	}
	m.selections.pluginPreviews = previews
}

func (m *SetupModel) describePluginStep(ctx context.Context, step pluginStep) pluginPreview {
	ctx, cancel := context.WithTimeout(ctx, pluginPreviewTimeout)
	defer cancel()
	description, err := describePluginStep(ctx, m.pluginBootstrap, step)
	switch {
	case err == nil:
		return pluginPreview{description: &description}
	case errors.Is(err, sdk.ErrDescribeUnsupported):
		return pluginPreview{reason: "the plugin can't preview this step"}
	case errors.Is(err, errPluginsNotReady):
		return pluginPreview{reason: err.Error()}
	case errors.Is(err, context.DeadlineExceeded):
		return pluginPreview{reason: "the preview timed out"}
	default:
		reason, _, _ := strings.Cut(err.Error(), "\n")
		if runes := []rune(reason); len(runes) > maxPreviewReason {
			reason = string(runes[:maxPreviewReason]) + "…"
		}
		return pluginPreview{reason: "the preview failed: " + reason}
	}
}

// maxPreviewReason bounds the failure reason shown on one Confirmation line.
const maxPreviewReason = 160

// displayText makes plugin-supplied text safe for one terminal line: control
// characters, including newlines and escape sequences, become spaces.
func displayText(text string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return ' '
		}
		return r
	}, text)
}

// renderPluginPreview lists a plugin step's changes with the same detail as a
// catalog entry, or one line saying the step was not previewed.
func renderPluginPreview(step pluginStep, preview pluginPreview) []string {
	description := preview.description
	if description == nil {
		reason := preview.reason
		if reason == "" {
			reason = "no preview available"
		}
		return []string{fmt.Sprintf("      plugin: Run %s — not previewed (%s)", step, displayText(reason))}
	}
	if description.AlreadySatisfied {
		return append([]string{fmt.Sprintf("      plugin: %s — nothing to change", step)}, renderPluginNotes(description.Notes)...)
	}
	lines := []string{"      plugin: " + step.String()}
	for _, download := range description.Downloads {
		lines = append(lines, fmt.Sprintf("      Download: %s (%s)", displayText(download.URL), displayText(describeVerification(download.Verification))))
	}
	for _, action := range description.Actions {
		line := fmt.Sprintf("      %s: %s", actionLabel(action.Kind), displayText(action.Target))
		if action.Detail != "" {
			line += " — " + displayText(action.Detail)
		}
		if action.Scope == sdk.ScopeSystem {
			line += " (system, needs sudo)"
		} else {
			line += " (user)"
		}
		lines = append(lines, line)
	}
	return append(lines, renderPluginNotes(description.Notes)...)
}

func renderPluginNotes(notes []string) []string {
	lines := make([]string, 0, len(notes))
	for _, note := range notes {
		lines = append(lines, "      Note: "+displayText(note))
	}
	return lines
}

func actionLabel(kind sdk.ActionKind) string {
	switch kind {
	case sdk.ActionPackage:
		return "Package"
	case sdk.ActionRepository:
		return "Repository"
	case sdk.ActionService:
		return "Service"
	case sdk.ActionGroup:
		return "Group"
	case sdk.ActionFile:
		return "File"
	case sdk.ActionDownload:
		return "Download"
	default:
		return "Command"
	}
}

func describeVerification(verification sdk.ArtifactVerification) string {
	var checks []string
	if verification.SignatureURL != "" {
		check := "signature"
		if verification.Fingerprint != "" {
			check += " from key " + verification.Fingerprint
		}
		checks = append(checks, check)
	}
	if verification.SHA256 != "" {
		checks = append(checks, "sha256 "+verification.SHA256)
	}
	if len(checks) == 0 {
		return "not verified"
	}
	return "verified by " + strings.Join(checks, " and ")
}
