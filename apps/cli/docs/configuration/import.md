# Importing configuration

`devex config import <file>` accepts exported YAML, JSON, and tar/tar.gz
bundles. Imports merge by default. Use `--dry-run` to validate and preview
exact semantic before/after values without creating files or backups.
`--format json` emits one JSON report on stdout; diagnostics use stderr.
The report includes the mode, changes, conflict paths, backup paths, and
whether the plan was applied. Text mode prints the plan before applying it.

## Merge and conflict rules

- Maps merge recursively. Existing keys absent from the import survive.
- Lists retain existing order. Equal entries are deduplicated; new entries
  append in imported order. Mapping entries with the same nonempty `name`
  (case sensitive) merge recursively in place. Other mapping entries use
  whole-value equality. Duplicate names already present are not removed;
  an imported named entry merges with the first matching existing entry.
- Different scalar values, explicit nulls, and changes between value types
  are conflicts. An empty map or list preserves existing map keys or list
  entries; use replacement to clear them.
- Conflicts are reported by file and field path. Without `--force`, a
  conflicting import fails without applying any files. `--force` accepts
  imported values only at conflicting fields and preserves unrelated data.
- File and map traversal is sorted, so results and conflict order are stable.
  Legacy sections are converted to the configuration tree before merging.

```sh
devex config import shared.yaml --dry-run --format json
devex config import shared.yaml --force
```

`--merge=false` replaces each supplied file completely, including its lists.
Changed existing files require `--force`. Files absent from the import are
preserved, even in replacement mode. The preview shows the resulting full
values, including fields removed by replacement. YAML comments and formatting
are not retained when a file changes.

## Validation and backups

The shared semantic validator checks the final merged configuration in an
isolated staging directory, including the effective configuration tiers and
application dependency graph. A validation failure makes no destination
changes and creates no backups. This validation is mandatory even with the
legacy `--validate=false` flag.

Every changed existing file receives an adjacent, uniquely named
`<filename>.backup-*` file containing its exact previous bytes. Backups are
mandatory even with the legacy `--backup=false` flag. Backups and temporary
output files must all succeed before any destination is replaced. Each file
is replaced with an atomic rename; this is not a filesystem transaction across
multiple files if an I/O failure occurs during the rename phase. Backup paths
are included in the report. No-op imports create no backups.

Import options follow flags, `DEVEX_*` environment variables, configuration,
and defaults, in that precedence order (for example `DEVEX_DRY_RUN=true`).
