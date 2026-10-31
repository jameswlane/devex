# CLI datastore schema

DevEx embeds these SQL resources in its binary. Every database initialization,
including CLI startup, validates the existing schema and applies pending `_up.sql`
migrations in numeric order. No migration directory, checkout, or network access
is needed at runtime. `_down.sql` resources are retained for reference; startup
never downgrades or automatically resets a database.

The supported schema is version **5**. `schema_migrations` is the authoritative,
contiguous history (versions 1 through the current version). Each migration's SQL
and history row, plus the legacy `schema_version` synchronization, commit together
in one transaction covering the entire pending upgrade. An error or cancellation
rolls back the upgrade, including any newly created tables and version records.
Repeat startup preserves app records and migration timestamps.

## Compatibility

- Fresh databases receive the complete schema, including dependency tracking,
  uninstall history, indexes, and default protected packages.
- The old minimal schema (`system_data`, `installed_apps` with `id` and unique
  `app_name`, and an empty or absent `schema_migrations`) upgrades without
  recreating the app table. App IDs, names, and system data are preserved.
- Databases with a valid contiguous migration history from versions 1–5 are
  validated against the matching embedded SQL before upgrading. The original
  `schema_version` table is retained and synchronized for legacy readers; it is
  not used to select pending migrations. Its old value of 1 is accepted when
  `schema_migrations` records a later version.
- The nullable `schema_migrations.applied_at` column created by the old minimal
  initializer remains compatible. Extra application tables are allowed.
- A newer version, missing history entry, mismatched required table, column,
  index or foreign key, invalid legacy version, SQLite integrity failure, or
  foreign key violation stops startup with an error. DevEx does not silently
  recreate missing versioned objects or discard records to conceal corruption.
- Migration 005 references `installed_apps(app_name)`. Experimental databases
  created with its former `installed_apps(name)` references are incompatible and
  rejected; their constraints need repair on a backup copy before use.

## Recovery

The startup error includes the database path and the failing check or migration.
The default path is `~/.devex/datastore.db` for the resolved user home.

1. Stop all DevEx processes before copying or restoring the database. Keep a
   backup of the database and any `-wal`/`-shm` sidecars; use SQLite's backup
   command for a consistent snapshot if another process must remain open.
2. For a newer schema version, use a DevEx release that supports that version.
   Do not edit the migration history to make an older binary accept it.
3. For a failed SQL migration or cancellation, the previous schema and app data
   remain intact. Resolve the reported cause (for example, file permissions or
   a migration defect), then restart to retry the complete pending upgrade.
4. For corruption or structural drift, restore a known compatible backup, or
   inspect and repair a separate copy with SQLite tools before replacing the
   original. `PRAGMA integrity_check` and `PRAGMA foreign_key_check` help diagnose
   integrity and reference problems. Keep the original until the repaired copy
   successfully starts and its tracked apps have been verified.

Do not delete the datastore or run down migrations as an automatic recovery step:
that can lose app tracking and installation history. Registry Prisma migrations
are independent and do not change this SQLite schema.
