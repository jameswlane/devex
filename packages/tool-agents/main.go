package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

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
		for _, w := range ap.Warnings {
			if _, err := fmt.Fprintf(p.out, "%s\n", w); err != nil {
				return err
			}
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
		if ap.MCPNotice != "" {
			if _, err := fmt.Fprintf(p.out, "%s\n", ap.MCPNotice); err != nil {
				return err
			}
		}
		for _, md := range ap.MCPDiffs {
			label := diffStatusLabel(md.Action)
			target := md.Path + " (mcpServers)"
			if _, err := fmt.Fprintf(p.out, "%s: %s\n", label, target); err != nil {
				return err
			}
			if md.Action != actionUnchanged {
				if diff := UnifiedDiff(target, md.Before, md.After); diff != "" {
					if _, err := io.WriteString(p.out, diff); err != nil {
						return err
					}
				}
			}
		}
	}
	return nil
}

// applyPlan writes one agent at a time and keeps managed state exact for each.
// An agent's list entries live in one file that is replaced atomically, so a
// failed write leaves that file, and therefore its previous ownership, as it
// was. While an agent is being written, state owns both its old and new
// entries, so neither an addition nor a removal loses its owner if the
// process stops or the final save fails.
func (p *AgentsPlugin) applyPlan(ctx context.Context, plan PlanResult) error {
	committed := plan.InitialState.clone()
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
		for _, w := range ap.Warnings {
			if _, err := fmt.Fprintf(p.out, "%s\n", w); err != nil {
				return err
			}
		}
		prev, hadPrev := committed.Agents[ap.Agent]
		next, hasNext := plan.NextState.Agents[ap.Agent]
		stateChanges := hadPrev != hasNext || !prev.equal(next)
		if stateChanges {
			pending := committed.clone()
			pending.Agents[ap.Agent] = prev
			pending = pending.union(ManagedState{Agents: map[string]AgentManagedState{ap.Agent: next}})
			if err := SaveManagedState(plan.StatePath, pending); err != nil {
				return err
			}
		}
		status, err := writeAgentFiles(ctx, ap)
		if err != nil {
			// Still report the files (and backups) written before the failure.
			_, _ = io.WriteString(p.out, status)
			if stateChanges {
				if saveErr := SaveManagedState(plan.StatePath, committed); saveErr != nil {
					return fmt.Errorf("%w; also failed to restore managed state: %v", err, saveErr)
				}
			}
			return err
		}
		mcpStatus, mcpRanAny, mcpErr := p.applyAgentMCP(ctx, ap)
		status += mcpStatus
		if mcpErr != nil {
			_, _ = io.WriteString(p.out, status)
			// Only restore prior state if no MCP commands executed; if commands
			// already mutated external config, keep the pending union state so
			// DevEx retains ownership of any added or partially-removed servers.
			if stateChanges && !mcpRanAny {
				if saveErr := SaveManagedState(plan.StatePath, committed); saveErr != nil {
					return fmt.Errorf("%w; also failed to restore managed state: %v", mcpErr, saveErr)
				}
			}
			return mcpErr
		}
		if stateChanges {
			if hasNext {
				committed.Agents[ap.Agent] = next
			} else {
				delete(committed.Agents, ap.Agent)
			}
			if err := SaveManagedState(plan.StatePath, committed); err != nil {
				return err
			}
		}
		// Report only after ownership matches the files, so a failing output
		// stream can never undo the record of a write that happened.
		if _, err := io.WriteString(p.out, status); err != nil {
			return err
		}
	}
	return nil
}

func (p *AgentsPlugin) applyAgentMCP(ctx context.Context, ap AgentPlan) (string, bool, error) {
	var status strings.Builder
	if ap.MCPNotice != "" {
		fmt.Fprintf(&status, "%s\n", ap.MCPNotice)
	}
	if len(ap.MCPDiffs) == 0 && len(ap.MCPActions) == 0 {
		return status.String(), false, nil
	}

	var backupPath string
	if len(ap.MCPActions) > 0 && len(ap.MCPDiffs) > 0 && ap.MCPDiffs[0].Exists {
		targetPath := ap.MCPDiffs[0].Path
		resolved, err := resolveWriteTarget(targetPath)
		if err != nil {
			return status.String(), false, err
		}
		original, readErr := os.ReadFile(resolved)
		if readErr != nil {
			return status.String(), false, fmt.Errorf("read %q for backup: %w", targetPath, readErr)
		}
		backupPath, err = ensureOneTimeBackup(resolved, original, os.FileMode(ap.MCPDiffs[0].Mode))
		if err != nil {
			return status.String(), false, err
		}
	}

	ranAny := false
	for _, act := range ap.MCPActions {
		if err := ctx.Err(); err != nil {
			return status.String(), ranAny, err
		}
		out, err := p.env.RunCommand(ctx, act.Binary, act.Args...)
		ranAny = true
		if err != nil {
			detail := strings.TrimSpace(string(out))
			if detail != "" {
				return status.String(), ranAny, fmt.Errorf("%s mcp %s %q failed: %s: %w", ap.Agent, act.Kind, act.ServerName, detail, err)
			}
			return status.String(), ranAny, fmt.Errorf("%s mcp %s %q failed: %w", ap.Agent, act.Kind, act.ServerName, err)
		}
	}

	for _, md := range ap.MCPDiffs {
		label := applyStatusLabel(md.Action)
		target := md.Path + " (mcpServers)"
		if backupPath != "" {
			fmt.Fprintf(&status, "%s: %s (backup: %s)\n", label, target, backupPath)
		} else {
			fmt.Fprintf(&status, "%s: %s\n", label, target)
		}
	}
	return status.String(), ranAny, nil
}

// writeAgentFiles applies an agent's files and returns their status lines.
func writeAgentFiles(ctx context.Context, ap AgentPlan) (string, error) {
	var status strings.Builder
	for _, fp := range ap.Files {
		if err := ctx.Err(); err != nil {
			return status.String(), err
		}
		backupPath, err := ApplyFilePlan(fp)
		if err != nil {
			return status.String(), err
		}
		label := applyStatusLabel(fp.Action)
		if backupPath != "" {
			fmt.Fprintf(&status, "%s: %s (backup: %s)\n", label, fp.Path, backupPath)
		} else {
			fmt.Fprintf(&status, "%s: %s\n", label, fp.Path)
		}
	}
	return status.String(), nil
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
