# Template Runtime

DevEx lifecycle templates are YAML automation bundles for installing, inspecting, maintaining, and removing resources. They are used with `devex install --template`, `devex status --template`, `devex maintain --template`, `devex remove --template`, and `devex template plan`.

The older configuration-template workflow is managed through `devex template custom ...` and template version commands. Do not use `devex template apply` in lifecycle examples; lifecycle templates execute through the top-level lifecycle commands.

## Supported Commands

```bash
# List lifecycle and versioned templates
devex template list
devex template list --include-user
devex template list --source builtin --format json

# Show template metadata
devex template info databases

# Validate and plan lifecycle templates
devex template validate ./templates/databases.yaml
devex template plan ./templates/databases.yaml --action install --answer databases=postgres,redis --non-interactive
devex template plan databases --action remove --format yaml

# Execute lifecycle actions
devex install --template databases --answer databases=postgres,redis --non-interactive
devex status --template databases --run latest --format json
devex maintain --template databases --run latest --dry-run
devex remove --template databases --run latest --force

# Inspect recorded runs
devex runs list
devex runs show <run-id>
devex runs resources <run-id>
devex runs operations <run-id>
```

## Template Schema Reference

Canonical lifecycle templates use `schema_version: devex.template/v1`.

Required top-level fields:

- `schema_version`: must be `devex.template/v1`
- `id`: stable template identifier
- `name`: display name
- `version`: template version
- `lifecycle`: map of lifecycle actions to ordered steps

Optional top-level fields:

- `description`, `author`, `homepage`, `license`
- `tags`, `categories`
- `inputs`
- `metadata`

Supported lifecycle actions:

- `install`: create or configure resources
- `maintain`: update, repair, restart, or reconcile recorded resources
- `status`: inspect recorded resources
- `remove`: remove resources created by a previous install

Supported step types:

- `app`: install or manage an application by reference, inline fields, or `definition`
- `plugin`: call a plugin command directly
- `shell`: run a validated executable with explicit arguments
- `file`: run a file operation such as write, copy, template, or remove
- `service`: run a service action such as start, stop, restart, enable, or disable
- `group`: logical dependency boundary; currently a no-op at execution time

Each lifecycle step must define exactly one primary action. Steps support `id`, `order`, `if`, `depends_on`, `on_error`, `description`, and `resources`.

## Inputs

Supported input types are `text`, `select`, `multiselect`, and `bool`.

Input fields:

- `type`: input type
- `prompt`: interactive prompt text
- `required`: fail when missing in non-interactive mode
- `sensitive`: redact from persisted state and displayed plans
- `default`: fallback value
- `options`: allowed values for `select` and `multiselect`
- `validation`: `pattern`, `message`, `min`, and `max`
- `description`: additional documentation

Input precedence, from lowest to highest:

1. Template defaults
2. Environment variables named `DEVEX_INPUT_<KEY>`, for example `DEVEX_INPUT_DATABASES`
3. YAML files passed with `--answers path.yaml`
4. CLI flags passed with `--answer key=value`

Interactive prompts are used only for missing required inputs when `--non-interactive` is not set. With `--non-interactive`, missing required inputs fail fast.

```yaml
# prod.answers.yaml
databases:
  - postgres
  - redis
postgres_name: devex-postgres
postgres_password: change-me
```

```bash
DEVEX_INPUT_POSTGRES_NAME=ci-postgres \
  devex install --template databases \
  --answers prod.answers.yaml \
  --answer databases=postgres,redis \
  --non-interactive
```

## Conditions And Interpolation

Supported condition helpers:

- `equals(value, expected)`
- `contains(list, value)`
- `exists(value)`
- `platform(os)`
- `has_resource(step_id)`
- boolean operators: `&&`, `||`, `!`

Interpolation supports:

- `{{ inputs.key }}`
- `{{ resource.step_id.external_id }}`
- `{{ system.os }}`
- `{{ system.arch }}`

Use `if: has_resource("step-id")` for `maintain`, `status`, and `remove` steps that operate on resources from a previous install.

## bbolt State Store

Lifecycle runs are recorded in a local bbolt database. The default path is `~/.devex/state.db`.

State path precedence follows normal DevEx configuration:

- config key: `state.db_path`
- environment variable: `DEVEX_STATE_DB`
- compatibility environment variable: `DEVEX_STATE_DB_PATH`
- default: `~/.devex/state.db`

The state store records:

- `TemplateRun`: lifecycle action, resolved inputs, redacted plan, status, host, platform, timestamps, and error
- `OperationRecord`: each planned step, execution request, status, outputs, timestamps, and error
- `ManagedResource`: resources created by a run, external IDs, installer/plugin metadata, state, timestamps, and removal metadata

Removal uses recorded resources and removal hints from the state store. If the template file changes after install, `devex remove --template <id> --run <run-id>` still targets resources recorded by the selected install run. When current remove steps are incomplete, DevEx augments or synthesizes remove operations from recorded resources where removal hints exist.

## Plugin Argument Conventions

Template runtime plugin calls are structured as:

```yaml
plugin:
  name: package-manager-docker
  command: remove
  args:
    - "{{ resource.postgres.external_id }}"
    - --volumes
  env:
    EXAMPLE: value
  working_dir: /tmp
```

At execution time the command is passed as the first plugin argument, followed by `args`. Prefer one argument per list item. Do not encode shell pipelines or multiple commands inside one string.

For Docker-style templates, use explicit flags such as `--name`, `--port`, `--env`, `--volume`, `--restart`, and `--detach`.

## Sensitive Input Redaction

Set `sensitive: true` for passwords, tokens, private keys, API keys, and secret-like values.

Sensitive values may be held in memory long enough to build and execute a plan. Persisted runs, resources, operation requests, plan snapshots, logs, and metadata redact sensitive values with `[REDACTED]`. This applies whether the value came from `--answer`, `--answers`, environment variables, defaults, or interactive prompts.

## Database Template Example

```yaml
schema_version: devex.template/v1
id: databases
name: Databases
version: 1.0.0
description: Local PostgreSQL and Redis containers
inputs:
  databases:
    type: multiselect
    prompt: Databases to install
    required: true
    options:
      - label: PostgreSQL
        value: postgres
      - label: Redis
        value: redis
  postgres_name:
    type: text
    prompt: PostgreSQL container name
    default: devex-postgres
  postgres_password:
    type: text
    prompt: PostgreSQL password
    sensitive: true
    required: true
lifecycle:
  install:
    - id: docker-engine
      order: 10
      plugin:
        name: package-manager-docker
        command: ensure-installed
        args:
          - --add-user
    - id: postgres
      order: 20
      if: contains(inputs.databases, "postgres")
      app:
        name: PostgreSQL
        category: databases
        installer: docker
        command: install
        args:
          - postgres:16
          - --name={{ inputs.postgres_name }}
          - --port=127.0.0.1:5432:5432
          - --env=POSTGRES_PASSWORD={{ inputs.postgres_password }}
          - --detach
      resources:
        - type: container
          name: "{{ inputs.postgres_name }}"
          external_id: "{{ inputs.postgres_name }}"
  status:
    - id: postgres-status
      order: 10
      if: has_resource("postgres")
      plugin:
        name: package-manager-docker
        command: status
        args:
          - "{{ resource.postgres.external_id }}"
  maintain:
    - id: postgres-restart
      order: 10
      if: has_resource("postgres")
      plugin:
        name: package-manager-docker
        command: restart
        args:
          - "{{ resource.postgres.external_id }}"
  remove:
    - id: postgres-remove
      order: 10
      if: has_resource("postgres")
      plugin:
        name: package-manager-docker
        command: remove
        args:
          - "{{ resource.postgres.external_id }}"
          - --volumes
```

## Gaming Server Template Example

```yaml
schema_version: devex.template/v1
id: factorio-server
name: Factorio Server
version: 1.0.0
description: Dedicated Factorio server in Docker
inputs:
  server_name:
    type: text
    prompt: Server container name
    default: factorio
  save_dir:
    type: text
    prompt: Host save directory
    default: ~/.devex/factorio
lifecycle:
  install:
    - id: factorio
      order: 10
      app:
        name: Factorio Server
        category: gaming
        installer: docker
        command: install
        args:
          - factoriotools/factorio:stable
          - --name={{ inputs.server_name }}
          - --port=34197:34197/udp
          - --volume={{ inputs.save_dir }}:/factorio
          - --restart=unless-stopped
          - --detach
      resources:
        - type: container
          name: "{{ inputs.server_name }}"
          external_id: "{{ inputs.server_name }}"
  status:
    - id: factorio-status
      order: 10
      if: has_resource("factorio")
      plugin:
        name: package-manager-docker
        command: status
        args:
          - "{{ resource.factorio.external_id }}"
  remove:
    - id: factorio-remove
      order: 10
      if: has_resource("factorio")
      plugin:
        name: package-manager-docker
        command: remove
        args:
          - "{{ resource.factorio.external_id }}"
```

## Automation Examples

```bash
# Validate and plan in CI
devex template validate ./templates/databases.yaml --format json
devex template plan ./templates/databases.yaml \
  --action install \
  --answers ./ci.answers.yaml \
  --non-interactive \
  --format json

# Use an isolated state database for ephemeral automation
DEVEX_STATE_DB=/tmp/devex-state.db \
  devex install --template ./templates/databases.yaml \
  --answers ./ci.answers.yaml \
  --non-interactive

# Remove the latest recorded run in automation
DEVEX_STATE_DB=/tmp/devex-state.db \
  devex remove --template ./templates/databases.yaml \
  --run latest \
  --force \
  --non-interactive
```

## Troubleshooting

```bash
# Missing input in automation
devex template plan databases --non-interactive

# Inspect what was recorded
devex runs list --format table
devex runs show <run-id> --format yaml
devex runs operations <run-id> --format json

# Preview removal from recorded state
devex remove --template databases --run latest --dry-run --format yaml
```

If removal fails after a template changed, inspect `devex runs resources <run-id>`. Resources without removal hints may require a template remove step or manual cleanup.
