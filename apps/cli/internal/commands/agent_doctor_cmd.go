package commands

import (
	"fmt"
	"os"
	"strings"

	"github.com/jameswlane/devex/apps/cli/internal/agentdoctor"
	"github.com/spf13/cobra"
)

func newAgentDoctorCmd(probe agentdoctor.Probe) *cobra.Command {
	var (
		format     string
		jsonOutput bool
		agentIDs   []string
	)
	cmd := &cobra.Command{
		Use:          "doctor",
		Short:        "Diagnose installed coding agents, auth status, sandboxes, and MCP configs",
		SilenceUsage: true,
		Args:         cobra.NoArgs,
		Long: `Inspect known AI coding agents on this host without reading, storing, or
printing credentials.

For each agent, devex agent doctor reports installation status, version,
resolved PATH entry and duplicate/shadowed binaries (including npm globals),
login state (via the agent's status command exit code, credential file
presence via stat only, or API key environment variable names), Node.js
version requirements, Linux sandbox prerequisites, and MCP configuration
parse status.

Exit status is 0 whenever the diagnostic report is produced; detected setup
problems are listed with actionable fixes and do not change the exit code.`,
		Example: `  # Inspect all known coding agents
  devex agent doctor

  # Emit machine-readable JSON
  devex agent doctor --format json

  # Filter to specific agents
  devex agent doctor --agent claude --agent codex`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if jsonOutput {
				format = "json"
			}
			if format != "table" && format != "json" {
				return fmt.Errorf("unsupported format: use table or json")
			}
			specs, err := agentdoctor.FilterAgents(agentIDs)
			if err != nil {
				return err
			}
			report := agentdoctor.Check(cmd.Context(), probe, specs)
			secrets := collectDefenseInDepthSecrets(probe)
			if format == "json" {
				out, err := agentdoctor.FormatJSON(report, secrets)
				if err != nil {
					return err
				}
				_, err = cmd.OutOrStdout().Write(out)
				return err
			}
			_, err = fmt.Fprint(cmd.OutOrStdout(), agentdoctor.FormatTable(report, secrets))
			return err
		},
	}
	cmd.Flags().StringVar(&format, "format", "table", "Output format (table, json)")
	cmd.Flags().BoolVar(&jsonOutput, "json", false, "Shorthand for --format json")
	cmd.Flags().StringSliceVar(&agentIDs, "agent", nil, fmt.Sprintf("Filter to specific agent IDs (%s)", strings.Join(agentdoctor.ValidAgentIDs(), ", ")))
	return cmd
}

func newAgentLoginCmd(probe agentdoctor.Probe, runner agentdoctor.LoginRunner) *cobra.Command {
	if runner == nil {
		runner = agentdoctor.DefaultLoginRunner()
	}
	cmd := &cobra.Command{
		Use:          "login <agent>",
		Short:        "Hand off interactive login to a coding agent's own CLI",
		SilenceUsage: true,
		Args:         cobra.ExactArgs(1),
		Long: fmt.Sprintf(`Run an installed coding agent's own interactive login command with your
terminal attached. DevEx never prompts for, stores, or logs credentials.

Supported agent IDs: %s.
Agents without a standalone interactive login subcommand (such as aider or
gemini) return guidance for environment variables or the vendor's own flow.`, strings.Join(agentdoctor.ValidAgentIDs(), ", ")),
		Example: `  # Hand off login to Claude Code
  devex agent login claude

  # Hand off login to Codex CLI
  devex agent login codex`,
		RunE: func(cmd *cobra.Command, args []string) error {
			spec, ok := agentdoctor.LookupAgent(args[0])
			if !ok {
				return fmt.Errorf("unknown agent: valid agents are %s", strings.Join(agentdoctor.ValidAgentIDs(), ", "))
			}
			binaryPath, installed := agentdoctor.FindBinary(probe, spec.Binary)
			if !installed {
				return fmt.Errorf("%s (%s) is not installed; install it first with: devex install %q", spec.Name, spec.Binary, spec.Name)
			}
			if len(spec.LoginArgs) == 0 {
				guidance := spec.NoLoginGuidance
				if guidance == "" {
					guidance = fmt.Sprintf("provide credentials via environment variables or %s's own configuration outside DevEx", spec.Name)
				}
				return fmt.Errorf("%s does not have a CLI login handoff: %s", spec.Name, guidance)
			}
			return runner(cmd.Context(), binaryPath, spec.LoginArgs, cmd.InOrStdin(), cmd.OutOrStdout(), cmd.ErrOrStderr())
		},
	}
	return cmd
}

func collectDefenseInDepthSecrets(probe agentdoctor.Probe) []string {
	return agentdoctor.CollectSecretValues(probe.Getenv, os.Getenv)
}
