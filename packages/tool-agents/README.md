# devex-plugin-tool-agents

Declarative coding-agent configuration plugin for the DevEx CLI. Renders shared global instructions, model defaults, conservative permission presets, and allowed/denied shell command rules into each installed coding agent's native configuration files.

## Commands

```sh
devex-plugin-tool-agents diff --config /path/to/spec.json
devex-plugin-tool-agents apply --config /path/to/spec.json [--dry-run]
```

- `diff --config FILE`: previews every target file (`created`, `changed`, `deleted`, `unchanged`, or `skipped (not installed)`) along with a unified diff. Performs zero filesystem writes (no directories, backups, or state files).
- `apply --config FILE [--dry-run]`: writes changes atomically (following symlinks), creates a one-time sibling backup (`<file>.devex-bak-*`) before the first modification of any existing file, and updates `$DEVEX_STATE_DIR/agents/managed.json`. With `--dry-run`, behaves identically to `diff`.

## Supported Agents and Native Files

An enabled agent is rendered when its binary is on `PATH` or its configuration directory already exists; otherwise it is reported as `skipped (not installed)` and nothing is created.

| Agent | Binary | Home Override Env Var | Instructions File | Settings / Rules Files |
| --- | --- | --- | --- | --- |
| `claude` (Claude Code) | `claude` | `CLAUDE_CONFIG_DIR` (default `~/.claude`) | `<dir>/CLAUDE.md` | `<dir>/settings.json` |
| `codex` (OpenAI Codex CLI) | `codex` | `CODEX_HOME` (default `~/.codex`) | `<dir>/AGENTS.md` | `<dir>/config.toml`, `<dir>/rules/devex.rules` |
| `gemini` (Gemini CLI) | `gemini` | `GEMINI_CLI_HOME` (root containing `.gemini`, default `~`) | `<dir>/GEMINI.md` | `<dir>/settings.json` |
| `opencode` (OpenCode) | `opencode` | `OPENCODE_CONFIG_DIR` > `$XDG_CONFIG_HOME/opencode` > `~/.config/opencode` | `<dir>/AGENTS.md` | `<dir>/opencode.json` (or existing `opencode.jsonc`) |

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

DevEx tracks the list entries it added in `$DEVEX_STATE_DIR/agents/managed.json` so dropping an entry from DevEx config removes only that DevEx-managed entry on the next apply while keeping all user-added entries untouched.

## Rendering Guarantees

- **Managed instructions block**: delimited by `<!-- >>> devex agents >>> -->` and `<!-- <<< devex agents <<< -->`. Every byte outside the block is preserved. Malformed markers return an error without writing.
- **Order-preserving JSON merge**: preserves unknown keys, nested objects, and key order with 2-space indentation. Files containing JSON comments (`//`, `/* ... */`) or trailing commas return a clear error rather than silently stripping comments.
- **Comment-preserving TOML editor**: updates only owned top-level scalar keys (`model`, `approval_policy`, `sandbox_mode`), preserving comments, ordering, and tables, and re-parses the result with a TOML parser before writing. Non-scalar/table definitions of owned keys return an error.
- **Backups and idempotency**: creates one sibling `<file>.devex-bak-*` before the first modification of an existing file. Re-applying unchanged configuration is a byte-identical no-op (mtime unchanged, no new backup).

## Vendor Documentation References

- **Claude Code**
  - Settings & `CLAUDE_CONFIG_DIR`: https://code.claude.com/docs/en/settings and https://code.claude.com/docs/en/env-vars
  - Settings reference (`model`, `permissions.*`): https://code.claude.com/docs/en/settings-reference
  - Permission rule syntax (`Bash(cmd *)`): https://code.claude.com/docs/en/permissions
  - Permission modes (`default`, `acceptEdits`, `bypassPermissions`): https://code.claude.com/docs/en/permission-modes
  - Memory / `~/.claude/CLAUDE.md`: https://code.claude.com/docs/en/memory
- **OpenAI Codex CLI**
  - Configuration basics & `config.toml`: https://developers.openai.com/codex/config-basic
  - Advanced config (`CODEX_HOME`, retired `untrusted` policy, `approval_policy`, `sandbox_mode`): https://developers.openai.com/codex/config-advanced
  - Configuration reference: https://developers.openai.com/codex/config-reference
  - Rules & `prefix_rule` (`~/.codex/rules/*.rules`): https://developers.openai.com/codex/rules
  - Global instructions (`~/.codex/AGENTS.md`): https://developers.openai.com/codex/guides/agents-md
- **Gemini CLI**
  - Configuration reference & `GEMINI_CLI_HOME`: https://github.com/google-gemini/gemini-cli/blob/main/docs/reference/configuration.md
  - Settings JSON schema (`model.name`, `general.defaultApprovalMode`, `tools.allowed`, `tools.exclude`): https://raw.githubusercontent.com/google-gemini/gemini-cli/main/schemas/settings.schema.json
  - Shell tool command restrictions (`run_shell_command(...)`): https://github.com/google-gemini/gemini-cli/blob/main/docs/tools/shell.md
  - Global instructions (`~/.gemini/GEMINI.md`): https://github.com/google-gemini/gemini-cli/blob/main/docs/cli/gemini-md.md
- **OpenCode**
  - Configuration & JSONC (`~/.config/opencode/opencode.json(c)`, `model`): https://opencode.ai/docs/config/
  - Permissions V2 (`permissions` array with `action`, `resource`, `effect`; last-match-wins): https://opencode.ai/docs/permissions/
  - Instructions (`~/.config/opencode/AGENTS.md`): https://opencode.ai/docs/rules/
  - Configuration JSON Schema: https://opencode.ai/config.json
