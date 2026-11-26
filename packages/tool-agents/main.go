package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"

	sdk "github.com/jameswlane/devex/packages/plugin-sdk"
)

var version = "dev" // Set by goreleaser

// AgentsPlugin renders declarative DevEx coding-agent configuration into native
// files for Claude Code, OpenAI Codex CLI, Gemini CLI, and OpenCode.
type AgentsPlugin struct {
	*sdk.BasePlugin
	env Env
	out io.Writer
}

// NewAgentsPlugin creates a new tool-agents plugin with default host lookups.
func NewAgentsPlugin() *AgentsPlugin {
	return NewAgentsPluginWithDeps(DefaultEnv(), os.Stdout)
}

// NewAgentsPluginWithDeps creates a tool-agents plugin with injectable host
// environment lookups and output stream for testing.
func NewAgentsPluginWithDeps(env Env, out io.Writer) *AgentsPlugin {
	if out == nil {
		out = os.Stdout
	}
	info := sdk.PluginInfo{
		Name:        "tool-agents",
		Version:     version,
		Description: "Declarative configuration renderer for coding agents (Claude Code, Codex, Gemini CLI, OpenCode)",
		Author:      "DevEx Team",
		Repository:  "https://github.com/jameswlane/devex",
		Tags:        []string{"agents", "ai", "claude", "codex", "gemini", "opencode", "configuration"},
		Commands: []sdk.PluginCommand{
			{
				Name:        "diff",
				Description: "Preview changes to native coding-agent configuration files",
				Usage:       "diff --config FILE",
				Flags: map[string]string{
					"config": "Path to resolved agents JSON specification",
				},
			},
			{
				Name:        "apply",
				Description: "Render and apply declarative configuration to installed coding agents",
				Usage:       "apply --config FILE [--dry-run]",
				Flags: map[string]string{
					"config":  "Path to resolved agents JSON specification",
					"dry-run": "Preview changes without writing files, backups, or state",
				},
			},
		},
	}

	return &AgentsPlugin{
		BasePlugin: sdk.NewBasePlugin(info),
		env:        env.normalize(),
		out:        out,
	}
}

// Execute runs a plugin command without a caller context.
func (p *AgentsPlugin) Execute(command string, args []string) error {
	return p.ExecuteContext(context.Background(), command, args)
}

// ExecuteContext runs diff or apply with context cancellation support.
func (p *AgentsPlugin) ExecuteContext(ctx context.Context, command string, args []string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	switch command {
	case "diff":
		return p.runDiff(ctx, args)
	case "apply":
		return p.runApply(ctx, args)
	default:
		return fmt.Errorf("unknown command: %s", command)
	}
}

func (p *AgentsPlugin) runDiff(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("diff", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var configPath string
	fs.StringVar(&configPath, "config", "", "Path to resolved agents JSON specification")
	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("parse diff flags: %w", err)
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("diff takes no positional arguments (unexpected: %v)", fs.Args())
	}
	return p.executePlan(ctx, configPath, true)
}

func (p *AgentsPlugin) runApply(ctx context.Context, args []string) error {
	fs := flag.NewFlagSet("apply", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var configPath string
	var dryRun bool
	fs.StringVar(&configPath, "config", "", "Path to resolved agents JSON specification")
	fs.BoolVar(&dryRun, "dry-run", false, "Preview changes without writing files")
	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("parse apply flags: %w", err)
	}
	if fs.NArg() > 0 {
		return fmt.Errorf("apply takes no positional arguments (unexpected: %v)", fs.Args())
	}
	return p.executePlan(ctx, configPath, dryRun)
}

func (p *AgentsPlugin) executePlan(ctx context.Context, configPath string, dryRun bool) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	spec, err := LoadSpec(configPath)
	if err != nil {
		return err
	}
	plan, err := BuildPlan(spec, p.env)
	if err != nil {
		return err
	}
	if dryRun {
		return p.printDiff(plan)
	}
	return p.applyPlan(ctx, plan)
}

func (p *AgentsPlugin) printDiff(plan PlanResult) error {
	for _, ap := range plan.Agents {
		if ap.Skipped {
			if _, err := fmt.Fprintf(p.out, "skipped (%s): %s\n", ap.Reason, ap.Agent); err != nil {
				return err
			}
			continue
		}
		for _, fp := range ap.Files {
			label := diffStatusLabel(fp.Action)
			if _, err := fmt.Fprintf(p.out, "%s: %s\n", label, fp.Path); err != nil {
				return err
			}
			if fp.Action != actionUnchanged {
				if diff := UnifiedDiff(fp.Path, fp.Before, fp.After); diff != "" {
					if _, err := io.WriteString(p.out, diff); err != nil {
						return err
					}
				}
			}
		}
	}
	return nil
}

func (p *AgentsPlugin) applyPlan(ctx context.Context, plan PlanResult) error {
	// Before writing, own both the old and the new entries, so an entry being
	// added or removed never looks user-owned if a write fails part-way
	// through. Owned entries that are absent from a file are harmless.
	if plan.InitialState.equal(plan.NextState) {
		return p.writeFiles(ctx, plan)
	}
	if err := SaveManagedState(plan.StatePath, plan.InitialState.union(plan.NextState)); err != nil {
		return err
	}
	if err := p.writeFiles(ctx, plan); err != nil {
		return err
	}
	return SaveManagedState(plan.StatePath, plan.NextState)
}

func (p *AgentsPlugin) writeFiles(ctx context.Context, plan PlanResult) error {
	for _, ap := range plan.Agents {
		if err := ctx.Err(); err != nil {
			return err
		}
		if ap.Skipped {
			if _, err := fmt.Fprintf(p.out, "skipped (%s): %s\n", ap.Reason, ap.Agent); err != nil {
				return err
			}
			continue
		}
		for _, fp := range ap.Files {
			if err := ctx.Err(); err != nil {
				return err
			}
			backupPath, err := ApplyFilePlan(fp)
			if err != nil {
				return err
			}
			label := applyStatusLabel(fp.Action)
			if backupPath != "" {
				if _, err := fmt.Fprintf(p.out, "%s: %s (backup: %s)\n", label, fp.Path, backupPath); err != nil {
					return err
				}
			} else {
				if _, err := fmt.Fprintf(p.out, "%s: %s\n", label, fp.Path); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

func diffStatusLabel(action fileAction) string {
	switch action {
	case actionCreate:
		return "created"
	case actionUpdate:
		return "changed"
	case actionDelete:
		return "deleted"
	default:
		return "unchanged"
	}
}

func applyStatusLabel(action fileAction) string {
	switch action {
	case actionCreate:
		return "created"
	case actionUpdate:
		return "updated"
	case actionDelete:
		return "deleted"
	default:
		return "unchanged"
	}
}

func main() {
	plugin := NewAgentsPlugin()
	sdk.HandleArgs(plugin, os.Args[1:])
}
