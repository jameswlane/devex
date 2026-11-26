# devex-plugin-tool-agents

Declarative coding-agent configuration plugin for the DevEx CLI. Renders shared global instructions, model defaults, conservative permission presets, allowed/denied shell command rules, and MCP servers into each installed coding agent's native configuration files.

## Commands

```sh
devex-plugin-tool-agents diff --config /path/to/spec.json
devex-plugin-tool-agents apply --config /path/to/spec.json [--dry-run]
```

- `diff --config FILE`: previews every target file and MCP configuration (`created`, `changed`, `deleted`, `unchanged`, or `skipped (not installed)`) along with unified diffs and literal-secret warnings. Performs zero filesystem writes and runs zero external commands.
- `apply --config FILE [--dry-run]`: writes changes atomically (following symlinks), manages Claude Code MCP servers through `claude mcp remove/add-json --scope user`, creates a one-time sibling backup (`<file>.devex-bak-*`) before the first modification of any existing file, and updates `$DEVEX_STATE_DIR/agents/managed.json`. With `--dry-run`, behaves identically to `diff`.

## Supported Agents and Native Files

An enabled agent is rendered when its binary is on `PATH` or its configuration directory already exists; otherwise it is reported as `skipped (not installed)` and nothing is created.

| Agent | Binary | Home Override Env Var | Instructions File | Settings / Rules Files | MCP Configuration |
| --- | --- | --- | --- | --- | --- |
| `claude` (Claude Code) | `claude` | `CLAUDE_CONFIG_DIR` (default `~/.claude`) | `<dir>/CLAUDE.md` | `<dir>/settings.json` | `~/.claude.json` (or `$CLAUDE_CONFIG_DIR/.claude.json`) `mcpServers` via `claude mcp` CLI |
| `codex` (OpenAI Codex CLI) | `codex` | `CODEX_HOME` (default `~/.codex`) | `<dir>/AGENTS.md` | `<dir>/config.toml`, `<dir>/rules/devex.rules` | `<dir>/config.toml` `[mcp_servers.<name>]` |
| `gemini` (Gemini CLI) | `gemini` | `GEMINI_CLI_HOME` (root containing `.gemini`, default `~`) | `<dir>/GEMINI.md` | `<dir>/settings.json` | `<dir>/settings.json` `mcpServers` |
| `opencode` (OpenCode) | `opencode` | `OPENCODE_CONFIG_DIR` > `$XDG_CONFIG_HOME/opencode` > `~/.config/opencode` | `<dir>/AGENTS.md` | `<dir>/opencode.json` (or existing `opencode.jsonc`) | `<dir>/opencode.json(c)` `mcp` |

## Permission Preset Mapping

Both presets are conservative. DevEx never maps any preset to an unsandboxed or skip-all-permissions mode (`bypassPermissions`, `danger-full-access`, `approval_policy = "never"`, `yolo`, or blanket `allow`), and rejects overrides requesting them. When no preset is configured for an agent, DevEx does not touch its permission-mode keys.

| Agent | `strict` | `balanced` |
| --- | --- | --- |
| `claude` | `permissions.defaultMode = "default"` (ask before edits and commands) | `permissions.defaultMode = "acceptEdits"` (allow workspace file edits; ask before commands) |
| `codex` | `approval_policy = "on-request"`, `sandbox_mode = "read-only"` | `approval_policy = "on-request"`, `sandbox_mode = "workspace-write"` |
| `gemini` | `general.defaultApprovalMode = "default"` (prompt on write and shell tools) | `general.defaultApprovalMode = "auto_edit"` (auto-approve edit tools; prompt on shell commands) |
| `opencode` | `permissions`: `{action: "edit", resource: "*", effect: "ask"}`, `{action: "shell", resource: "*", effect: "ask"}` | `permissions`: `{action: "edit", resource: "*", effect: "allow"}`, `{action: "shell", resource: "*", effect: "ask"}` |

## Command Rule Mapping

| Agent | `allowed_commands` | `denied_commands` |
| --- | --- | --- |
| `claude` | `permissions.allow`: `Bash(<cmd> *)` (or `Bash(<cmd>)` if `<cmd>` already contains `*`) | `permissions.deny`: `Bash(<cmd> *)` |
| `codex` | `rules/devex.rules`: Starlark `prefix_rule(pattern = [...], decision = "allow", ...)` | `rules/devex.rules`: Starlark `prefix_rule(pattern = [...], decision = "forbidden", ...)` |
| `gemini` | `tools.allowed`: `run_shell_command(<cmd>)` | `tools.exclude`: `run_shell_command(<cmd>)` |
| `opencode` | `permissions`: `{action: "shell", resource: "<cmd> *", effect: "allow"}` | `permissions`: `{action: "shell", resource: "<cmd> *", effect: "deny"}` |

DevEx tracks the list entries and MCP server names it added in `$DEVEX_STATE_DIR/agents/managed.json` so dropping an entry from DevEx config removes only that DevEx-managed entry on the next apply while keeping all user-added entries untouched. An entry that was already in the agent's file before DevEx declared it stays yours (an identical unmanaged MCP server is adopted; a conflicting unmanaged MCP server returns an error). Codex's `rules/devex.rules` is owned by DevEx as a whole file; DevEx refuses to replace or delete one it did not write.

## MCP Server Rendering and Secret Policy

- **Secret references only**: `env` and `headers` values must be `${env:VAR_NAME}` (or `Bearer ${env:VAR_NAME}` for `Authorization`), or non-secret literals. Literal secrets in keys, values, args, or URL query parameters are rejected without echoing the value. Keyring lookups (`${keyring:...}`) are not supported in v1. Existing native MCP configs are scanned during `diff` and `apply`, printing warnings (never values) for literal secrets found.
- **Claude Code**: renders `{"type":"stdio","command","args","env"}` or `{"type":"http","url","headers"}` with `${VAR_NAME}` references via `claude mcp remove/add-json --scope user`. Diff shows only the `mcpServers` object in `~/.claude.json` (or `$CLAUDE_CONFIG_DIR/.claude.json`). Skips MCP with a hint when `claude` is not on `PATH`.
- **Codex**: renders `[mcp_servers.<name>]` tables in `config.toml` using `command`, `args`, `env_vars`, `env` (non-secret literals only), `url`, `bearer_token_env_var`, `env_http_headers`, and `http_headers`. Returns an error if an `env` reference maps a different env var name (`KEY: "${env:OTHER}"`).
- **Gemini CLI**: renders `mcpServers.<name>` in `settings.json` (`command`/`args`/`env` or `httpUrl`/`headers`) with `${VAR_NAME}` references.
- **OpenCode**: renders `mcp.<name>` in `opencode.json(c)` (`type: "local"` with `command: [cmd, ...args]`, `environment`, `enabled: true` or `type: "remote"` with `url`, `headers`, `enabled: true`) with `{env:VAR_NAME}` references.

## Rendering Guarantees

- **Managed instructions block**: delimited by `<!-- >>> devex agents >>> -->` and `<!-- <<< devex agents <<< -->`. Every byte outside the block is preserved. Malformed markers return an error without writing.
- **Order-preserving JSON merge**: preserves unknown keys, nested objects, and key order with 2-space indentation. Files containing JSON comments (`//`, `/* ... */`) or trailing commas return a clear error rather than silently stripping comments.
- **Comment-preserving TOML editor**: updates only owned top-level scalar keys (`model`, `approval_policy`, `sandbox_mode`) and DevEx-managed `[mcp_servers.<name>]` tables (including `[mcp_servers.<name>.*]` subtables), preserving comments, ordering, and unmanaged tables byte-for-byte, and re-parses the result with a TOML parser before writing.
- **Backups and idempotency**: creates one sibling `<file>.devex-bak-*` before the first modification of an existing file. Re-applying unchanged configuration is a byte-identical no-op (mtime unchanged, zero commands run, no new backup).

## Vendor Documentation References

- **Claude Code**
  - Settings & `CLAUDE_CONFIG_DIR`: https://code.claude.com/docs/en/settings and https://code.claude.com/docs/en/env-vars
  - Settings reference (`model`, `permissions.*`): https://code.claude.com/docs/en/settings-reference
  - Permission rule syntax (`Bash(cmd *)`): https://code.claude.com/docs/en/permissions
  - Permission modes (`default`, `acceptEdits`, `bypassPermissions`): https://code.claude.com/docs/en/permission-modes
  - Memory / `~/.claude/CLAUDE.md`: https://code.claude.com/docs/en/memory
  - MCP configuration (`claude mcp add-json/remove --scope user`, `${VAR}` expansion): https://code.claude.com/docs/en/mcp
- **OpenAI Codex CLI**
  - Configuration basics & `config.toml`: https://developers.openai.com/codex/config-basic
  - Advanced config (`CODEX_HOME`, retired `untrusted` policy, `approval_policy`, `sandbox_mode`): https://developers.openai.com/codex/config-advanced
  - Configuration reference: https://developers.openai.com/codex/config-reference
  - MCP servers (`[mcp_servers.<name>]`, `env_vars`, `bearer_token_env_var`, `env_http_headers`, `http_headers`): https://developers.openai.com/codex/mcp
  - Rules & `prefix_rule` (`~/.codex/rules/*.rules`): https://developers.openai.com/codex/rules
  - Global instructions (`~/.codex/AGENTS.md`): https://developers.openai.com/codex/guides/agents-md
- **Gemini CLI**
  - Configuration reference & `GEMINI_CLI_HOME`: https://github.com/google-gemini/gemini-cli/blob/main/docs/reference/configuration.md
  - Settings JSON schema (`model.name`, `general.defaultApprovalMode`, `tools.allowed`, `tools.exclude`): https://raw.githubusercontent.com/google-gemini/gemini-cli/main/schemas/settings.schema.json
  - MCP servers (`mcpServers`, `httpUrl`, `${VAR}`/`$VAR` expansion): https://geminicli.com/docs/tools/mcp-server
  - Shell tool command restrictions (`run_shell_command(...)`): https://github.com/google-gemini/gemini-cli/blob/main/docs/tools/shell.md
  - Global instructions (`~/.gemini/GEMINI.md`): https://github.com/google-gemini/gemini-cli/blob/main/docs/cli/gemini-md.md
- **OpenCode**
  - Configuration & JSONC (`~/.config/opencode/opencode.json(c)`, `model`): https://opencode.ai/docs/config/
  - MCP servers (`mcp`, `local`/`remote`, `environment`, `headers`, `{env:VAR}`): https://opencode.ai/docs/mcp-servers/
  - Permissions V2 (`permissions` array with `action`, `resource`, `effect`; last-match-wins): https://opencode.ai/docs/permissions/
  - Instructions (`~/.config/opencode/AGENTS.md`): https://opencode.ai/docs/rules/
  - Configuration JSON Schema: https://opencode.ai/config.json
