package commands

import (
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/spf13/cobra"

	"github.com/jameswlane/devex/apps/cli/internal/config"
	"github.com/jameswlane/devex/apps/cli/internal/mcpserver"
	"github.com/jameswlane/devex/apps/cli/internal/types"
)

// mcpVersion is the DevEx version reported to MCP clients.
var mcpVersion = "dev"

// NewMCPCmd builds the mcp command group.
func NewMCPCmd(repo types.Repository, settings config.CrossPlatformSettings) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "mcp",
		Short: "Serve DevEx to Model Context Protocol clients",
		Long: `Serve read-only DevEx tools and resources to Model Context Protocol (MCP)
clients such as Claude Code and Codex.`,
		SilenceUsage: true,
	}
	cmd.AddCommand(newMCPServeCmd(repo, settings))
	return cmd
}

func newMCPServeCmd(repo types.Repository, settings config.CrossPlatformSettings) *cobra.Command {
	var (
		timeout         time.Duration
		mutationTimeout time.Duration
		allowMutations  bool
	)
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Serve read-only DevEx tools over stdio",
		Long: `Serve read-only DevEx tools and resources to an MCP client over stdin and
stdout. The client starts this command; it is not meant to be run by hand.

Tools: list_apps, detect_platform, get_status, validate_config, plan_install.
Resource: devex://config/effective (allowlisted settings only).

By default the server never installs, removes or configures anything, runs
package managers, plugins or hooks, downloads plugins, or creates
configuration or state. Stdout carries only MCP messages; diagnostics go to
stderr.

--allow-mutations adds prepare_install, install_apps, prepare_uninstall and
uninstall_apps. A prepare tool returns a plan with a plan_id and digest; the
client must show the plan to the user and get approval, then call the
matching tool with both. A plan is used once, expires after 10 minutes, and
is rejected if the applications, their definitions or the platform changed.
Installs run without a terminal and fail if a password is required. Linux and
macOS only.`,
		Example: `  # Claude Code
  claude mcp add devex -- devex mcp serve

  # Codex (~/.codex/config.toml)
  [mcp_servers.devex]
  command = "devex"
  args = ["mcp", "serve"]`,
		Args:         cobra.NoArgs,
		SilenceUsage: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if timeout <= 0 {
				return fmt.Errorf("--request-timeout must be positive")
			}
			// Resolve applications from the local cache or the bundled catalog;
			// the server never contacts the registry or writes a cache.
			if err := config.LoadCatalogOffline(cmd.Context(), settings); err != nil {
				return fmt.Errorf("load application catalog: %w", err)
			}
			if mutationTimeout <= 0 {
				return fmt.Errorf("--mutation-timeout must be positive")
			}
			options := mcpserver.Options{
				Version:         mcpVersion,
				Settings:        settings,
				Repo:            repo,
				RequestTimeout:  timeout,
				MutationTimeout: mutationTimeout,
			}
			var transport mcp.Transport = &mcp.StdioTransport{}
			if allowMutations {
				// Installers and plugins may write to stdout; keep it for MCP only.
				stdout, err := mcpserver.ReserveStdout()
				if err != nil {
					return err
				}
				defer func() { _ = stdout.Close() }()
				transport = &mcp.IOTransport{Reader: os.Stdin, Writer: stdout}
				options.Mutations = true
				options.Executor = &mcpExecutor{repo: repo, settings: settings, log: cmd.ErrOrStderr()}
			}
			server := mcpserver.New(options)
			err := server.Run(cmd.Context(), transport)
			// A client closing the connection or an interrupt is a normal shutdown.
			if err == nil || errors.Is(err, cmd.Context().Err()) || errors.Is(err, mcp.ErrConnectionClosed) {
				return nil
			}
			return err
		},
	}
	cmd.Flags().DurationVar(&timeout, "request-timeout", mcpserver.DefaultRequestTimeout, "maximum time for each tool call or resource read")
	cmd.Flags().BoolVar(&allowMutations, "allow-mutations", false, "also serve install and uninstall tools that run only a plan the user approved")
	cmd.Flags().DurationVar(&mutationTimeout, "mutation-timeout", mcpserver.DefaultMutationTimeout, "maximum time for a confirmed install or uninstall")
	return cmd
}

// mcpServeCommand reports whether cmd is mcp serve.
func mcpServeCommand(cmd *cobra.Command) bool {
	return cmd.Name() == "serve" && cmd.Parent() != nil && cmd.Parent().Name() == "mcp"
}
