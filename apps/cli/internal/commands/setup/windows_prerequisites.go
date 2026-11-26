package setup

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/jameswlane/devex/apps/cli/internal/winprereq"
)

// WindowsPreflight reports Windows prerequisites before setup plans anything and
// applies only the changes the user consented to.
type WindowsPreflight struct {
	Probe winprereq.Probe
	// Elevated reports whether DevEx runs with an elevated token. Registry
	// fixes are offered only when it is true.
	Elevated bool
	// DryRun prints each check and planned change without applying anything.
	DryRun bool
	// Interactive allows a y/N prompt for changes not already selected.
	Interactive bool
	// Selected holds prerequisite IDs consented to in advance through
	// --windows-prerequisites or setup.windows.prerequisites.
	Selected map[string]bool
	In       io.Reader
	Out      io.Writer
	// InstallApps installs catalog applications by name through winget.
	InstallApps func(ctx context.Context, names []string) error
}

type preflightChange struct {
	result  winprereq.Result
	summary string
}

// Run prints the report and applies consented changes. Failures to apply a
// change are reported with the manual step and do not stop setup.
func (p WindowsPreflight) Run(ctx context.Context) error {
	results := winprereq.Check(p.Probe)
	out := p.Out
	fmt.Fprintln(out, "Windows prerequisites:")
	for _, r := range results {
		fmt.Fprintf(out, "  %s %s: %s", stateMark(r.State), r.Name, r.State)
		if r.Detail != "" && r.State == winprereq.StateUnknown {
			fmt.Fprintf(out, " (%s)", r.Detail)
		}
		fmt.Fprintln(out)
	}

	var changes []preflightChange
	var manual []winprereq.Result
	for _, r := range results {
		switch {
		case r.State == winprereq.StateOn:
		case r.Kind == winprereq.KindTool && r.State == winprereq.StateOff:
			changes = append(changes, preflightChange{r, fmt.Sprintf("Install %s with winget (%s)", r.Name, r.WingetID)})
		case r.Kind == winprereq.KindRegistry && r.State == winprereq.StateOff && p.Elevated:
			changes = append(changes, preflightChange{r, fmt.Sprintf(`Enable %s: set %s\%s to 1`, r.Name, r.RegistryPath(), r.Value)})
		default:
			// Not elevated, or the state could not be read: never write blind.
			manual = append(manual, r)
		}
	}

	if len(manual) > 0 {
		fmt.Fprintln(out, "Manual steps:")
		for _, r := range manual {
			reason := "it " + r.Why
			switch {
			case r.State == winprereq.StateUnknown:
				reason = "DevEx could not read its state"
			case r.Kind == winprereq.KindRegistry:
				reason = "DevEx is not elevated; " + reason
			}
			where := "Run"
			if r.Kind == winprereq.KindRegistry {
				where = "Run in an elevated terminal"
			}
			fmt.Fprintf(out, "  • %s (%s). %s:\n      %s\n", r.Name, reason, where, r.ManualStep())
		}
	}
	if len(changes) == 0 {
		return nil
	}

	if p.DryRun {
		fmt.Fprintln(out, "Planned changes (preview, nothing is applied):")
		for _, c := range changes {
			fmt.Fprintf(out, "  • Would %s, %s\n", lowerFirst(c.summary), p.consentNote(c.result.ID))
		}
		return nil
	}

	var reader *bufio.Reader
	if p.In != nil {
		reader = bufio.NewReader(p.In)
	}
	var approved []preflightChange
	for _, c := range changes {
		switch {
		case p.Selected[c.result.ID]:
			approved = append(approved, c)
		case p.Interactive && reader != nil:
			fmt.Fprintf(out, "%s? It %s. [y/N]: ", c.summary, c.result.Why)
			answer, _ := reader.ReadString('\n')
			answer = strings.ToLower(strings.TrimSpace(answer))
			if answer == "y" || answer == "yes" {
				approved = append(approved, c)
			} else {
				fmt.Fprintf(out, "  Skipped. To do it later: %s\n", c.result.ManualStep())
			}
		default:
			fmt.Fprintf(out, "  Skipped %s: pass --windows-prerequisites=%s to apply, or run: %s\n", c.result.Name, c.result.ID, c.result.ManualStep())
		}
	}

	var apps []string
	var tools []winprereq.Result
	for _, c := range approved {
		if c.result.Kind == winprereq.KindTool {
			apps = append(apps, c.result.App)
			tools = append(tools, c.result)
			continue
		}
		if err := winprereq.Enable(p.Probe, c.result.Prerequisite); err != nil {
			fmt.Fprintf(out, "  ✗ Could not enable %s: %v\n    Run in an elevated terminal: %s\n", c.result.Name, err, c.result.ManualStep())
			continue
		}
		fmt.Fprintf(out, "  ✓ Enabled %s\n", c.result.Name)
		if c.result.ID == winprereq.LongPaths {
			fmt.Fprintln(out, "    Programs already running pick this up after they restart.")
		}
	}
	if len(apps) == 0 {
		return nil
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if p.InstallApps == nil {
		return fmt.Errorf("no installer is available for %s", strings.Join(apps, ", "))
	}
	if err := p.InstallApps(ctx, apps); err != nil {
		if ctxErr := ctx.Err(); ctxErr != nil {
			return ctxErr
		}
		fmt.Fprintf(out, "  ✗ Could not install %s: %v\n", strings.Join(apps, ", "), err)
		for _, t := range tools {
			fmt.Fprintf(out, "    To install %s yourself: %s\n", t.Name, t.ManualStep())
		}
	}
	return nil
}

func (p WindowsPreflight) consentNote(id string) string {
	switch {
	case p.Selected[id]:
		return "selected with --windows-prerequisites"
	case p.Interactive:
		return "after asking you"
	default:
		return "only with --windows-prerequisites=" + id
	}
}

func stateMark(s winprereq.State) string {
	switch s {
	case winprereq.StateOn:
		return "✓"
	case winprereq.StateOff:
		return "✗"
	default:
		return "?"
	}
}

func lowerFirst(s string) string {
	if s == "" {
		return s
	}
	return strings.ToLower(s[:1]) + s[1:]
}
